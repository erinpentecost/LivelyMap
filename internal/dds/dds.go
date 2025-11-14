package dds

import (
	"encoding/binary"
	"image"
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
	DDPF_RGBA = 0x41 // RGB + alpha

	// Caps
	DDSCAPS_TEXTURE = 0x1000
)

// Encode writes m as DDS (uncompressed RGBA8)
func Encode(w io.Writer, m image.Image) error {
	b := m.Bounds()
	wid := uint32(b.Dx())
	hei := uint32(b.Dy())
	stride := int(wid) * 4

	// Write magic
	if err := binary.Write(w, binary.LittleEndian, uint32(ddsMagic)); err != nil {
		return err
	}

	// ----- Build DDS header -----
	var header [ddsHeaderSize]byte
	h := header[:]
	put := func(off int, v uint32) {
		binary.LittleEndian.PutUint32(h[off:], v)
	}

	put(0, ddsHeaderSize)                                                // dwSize
	put(4, DDSD_CAPS|DDSD_HEIGHT|DDSD_WIDTH|DDSD_PIXELFORMAT|DDSD_PITCH) // dwFlags
	put(8, hei)                                                          // dwHeight
	put(12, wid)                                                         // dwWidth
	put(16, uint32(stride))                                              // dwPitchOrLinearSize

	// PixelFormat section (offset 76)
	put(76, ddsPfSize)   // size
	put(80, DDPF_RGBA)   // flags
	put(84, 32)          // bpp
	put(88, 0x00FF0000)  // R mask
	put(92, 0x0000FF00)  // G mask
	put(96, 0x000000FF)  // B mask
	put(100, 0xFF000000) // A mask

	// Caps (offset 108)
	put(108, DDSCAPS_TEXTURE)

	// write header
	if _, err := w.Write(h); err != nil {
		return err
	}

	// ----- Write pixel data -----
	// If *image.RGBA, dump directly
	if rgba, ok := m.(*image.RGBA); ok &&
		rgba.Rect.Min.X == b.Min.X &&
		rgba.Rect.Min.Y == b.Min.Y &&
		rgba.Stride == stride {

		_, err := w.Write(rgba.Pix)
		return err
	}

	// Otherwise, convert pixel-by-pixel
	row := make([]byte, stride)
	for y := b.Min.Y; y < b.Max.Y; y++ {
		i := 0
		for x := b.Min.X; x < b.Max.X; x++ {
			r, g, b2, a := m.At(x, y).RGBA()
			row[i+0] = uint8(r >> 8)
			row[i+1] = uint8(g >> 8)
			row[i+2] = uint8(b2 >> 8)
			row[i+3] = uint8(a >> 8)
			i += 4
		}
		if _, err := w.Write(row); err != nil {
			return err
		}
	}

	return nil
}
