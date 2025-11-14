package dds

import (
	"encoding/binary"
	"image"
	"image/color"
	"io"
)

const (
	ddsMagic = 0x20534444 // "DDS "

	ddsHeaderSize = 124
	ddsPfSize     = 32

	// DDSD flags
	DDSD_CAPS        = 0x1
	DDSD_HEIGHT      = 0x2
	DDSD_WIDTH       = 0x4
	DDSD_PITCH       = 0x8
	DDSD_PIXELFORMAT = 0x1000

	// Pixel format flags
	DDPF_RGB  = 0x40
	DDPF_RGBA = 0x41 // RGB + alpha flag

	// Caps
	DDSCAPS_TEXTURE = 0x1000
)

// DDSRGBA is similar to image.RGBA but for DDS output.
type DDSRGBA struct {
	Rect   image.Rectangle
	Stride int
	Pix    []uint8
}

// NewDDSRGBA creates a new DDSRGBA with the given bounds.
func NewDDSRGBA(r image.Rectangle) *DDSRGBA {
	w, h := r.Dx(), r.Dy()
	stride := 4 * w
	pix := make([]byte, h*stride)
	return &DDSRGBA{
		Rect:   r,
		Stride: stride,
		Pix:    pix,
	}
}

func (m *DDSRGBA) ColorModel() color.Model { return color.RGBAModel }
func (m *DDSRGBA) Bounds() image.Rectangle { return m.Rect }

// RGBA returns the RGBA color at (x, y)
func (m *DDSRGBA) RGBAAt(x, y int) color.RGBA {
	if !(image.Point{x, y}.In(m.Rect)) {
		return color.RGBA{}
	}
	i := (y-m.Rect.Min.Y)*m.Stride + (x-m.Rect.Min.X)*4
	return color.RGBA{
		R: m.Pix[i+0],
		G: m.Pix[i+1],
		B: m.Pix[i+2],
		A: m.Pix[i+3],
	}
}

// SetRGBA sets the pixel at (x, y).
func (m *DDSRGBA) SetRGBA(x, y int, c color.RGBA) {
	if !(image.Point{x, y}.In(m.Rect)) {
		return
	}
	i := (y-m.Rect.Min.Y)*m.Stride + (x-m.Rect.Min.X)*4
	m.Pix[i+0] = c.R
	m.Pix[i+1] = c.G
	m.Pix[i+2] = c.B
	m.Pix[i+3] = c.A
}

// Encode writes a DDS file containing uncompressed RGBA8
func Encode(w io.Writer, img *DDSRGBA) error {
	b := img.Bounds()
	wid, hei := uint32(b.Dx()), uint32(b.Dy())

	// Write magic
	if err := binary.Write(w, binary.LittleEndian, uint32(ddsMagic)); err != nil {
		return err
	}

	// --- DDS Header ---
	var header [ddsHeaderSize]byte
	h := header[:]
	bo := func(off int, v uint32) { binary.LittleEndian.PutUint32(h[off:], v) }

	bo(0, ddsHeaderSize)
	bo(4, DDSD_CAPS|DDSD_HEIGHT|DDSD_WIDTH|DDSD_PIXELFORMAT|DDSD_PITCH)
	bo(8, hei)
	bo(12, wid)
	bo(16, uint32(img.Stride)) // pitch

	// PixelFormat block (starts at offset 76)
	bo(76, ddsPfSize)   // size
	bo(80, DDPF_RGBA)   // flags (RGBA)
	bo(84, 32)          // bpp
	bo(88, 0x00FF0000)  // R mask
	bo(92, 0x0000FF00)  // G mask
	bo(96, 0x000000FF)  // B mask
	bo(100, 0xFF000000) // A mask

	// Caps (offset 108)
	bo(108, DDSCAPS_TEXTURE)

	// Write header
	if _, err := w.Write(h); err != nil {
		return err
	}

	// Image data
	_, err := w.Write(img.Pix)
	return err
}
