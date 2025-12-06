package dds

import (
	"encoding/binary"
	"errors"
	"image"
	"image/draw"
	"io"
)

// EncodeLossless writes m as an uncompressed 32-bit RGBA DDS (lossless).
// The pixel layout in the file is one byte per channel in order R G B A.
func EncodeLossless(w io.Writer, m image.Image) error {
	// Ensure we have an *image.RGBA for easy access to Pix/Stride
	var rgba *image.RGBA
	if im, ok := m.(*image.RGBA); ok {
		rgba = im
	} else {
		b := m.Bounds()
		if b.Dx() == 0 || b.Dy() == 0 {
			return errors.New("dds: empty image")
		}
		rgba = image.NewRGBA(b)
		draw.Draw(rgba, b, m, b.Min, draw.Src)
	}

	width := rgba.Bounds().Dx()
	height := rgba.Bounds().Dy()
	if width == 0 || height == 0 {
		return errors.New("dds: empty image")
	}

	// Write "DDS " signature
	if _, err := w.Write([]byte("DDS ")); err != nil {
		return err
	}

	var header [124]byte
	// header.size = 124
	binary.LittleEndian.PutUint32(header[0:], 124)

	const (
		// DDS header flags
		DDSD_CAPS        = 0x1
		DDSD_HEIGHT      = 0x2
		DDSD_WIDTH       = 0x4
		DDSD_PITCH       = 0x8
		DDSD_PIXELFORMAT = 0x1000
		// pixel format flags
		DDPF_ALPHAPIXELS = 0x1
		DDPF_RGB         = 0x40
		// caps
		DDSCAPS_TEXTURE = 0x1000
	)

	// Flags: include PITCH for uncompressed images (pitch = bytes per scanline)
	binary.LittleEndian.PutUint32(header[4:], DDSD_CAPS|DDSD_HEIGHT|DDSD_WIDTH|DDSD_PIXELFORMAT|DDSD_PITCH)

	// Height & Width
	binary.LittleEndian.PutUint32(header[8:], uint32(height))
	binary.LittleEndian.PutUint32(header[12:], uint32(width))

	// PitchOrLinearSize: for uncompressed, set bytes-per-row
	bytesPerPixel := 4 // RGBA8
	pitch := uint32(width * bytesPerPixel)
	binary.LittleEndian.PutUint32(header[16:], pitch)

	// Depth (unused for 2D textures) and mipmap count left at zero (already zero)

	// PixelFormat structure starts at offset 72 within the header
	pf := 72
	// pf.size = 32
	binary.LittleEndian.PutUint32(header[pf+0:], 32)
	// pf.flags = DDPF_RGB | DDPF_ALPHAPIXELS
	binary.LittleEndian.PutUint32(header[pf+4:], DDPF_RGB|DDPF_ALPHAPIXELS)
	// pf.fourCC = 0 (not a compressed format)
	binary.LittleEndian.PutUint32(header[pf+8:], 0)
	// pf.RGBBitCount = 32
	binary.LittleEndian.PutUint32(header[pf+12:], 32)

	// IMPORTANT: choose masks so that the on-disk byte-order is R, G, B, A
	// On little-endian systems the least-significant byte of the 32-bit pixel
	// corresponds to mask 0x000000FF. So set:
	//   R mask = 0x000000FF  (byte 0)
	//   G mask = 0x0000FF00  (byte 1)
	//   B mask = 0x00FF0000  (byte 2)
	//   A mask = 0xFF000000  (byte 3)
	binary.LittleEndian.PutUint32(header[pf+16:], 0x000000FF) // R mask
	binary.LittleEndian.PutUint32(header[pf+20:], 0x0000FF00) // G mask
	binary.LittleEndian.PutUint32(header[pf+24:], 0x00FF0000) // B mask
	binary.LittleEndian.PutUint32(header[pf+28:], 0xFF000000) // A mask

	// Caps
	binary.LittleEndian.PutUint32(header[104:], DDSCAPS_TEXTURE)

	// Write the header
	if _, err := w.Write(header[:]); err != nil {
		return err
	}

	// Now write raw pixel bytes row by row in R,G,B,A order.
	// Our rgba.Pix is in RGBA order (R,G,B,A, R,G,B,A, ...), so we can write it directly,
	// but we must account for Stride (in case image.Rect.Min.X != 0).
	r0x := rgba.Rect.Min.X
	r0y := rgba.Rect.Min.Y
	rowBytes := width * bytesPerPixel
	buf := make([]byte, rowBytes)

	for y := 0; y < height; y++ {
		off := (y+r0y)*rgba.Stride + (r0x * 4)
		// copy the row into buf to ensure contiguous row layout
		copy(buf, rgba.Pix[off:off+rowBytes])
		if _, err := w.Write(buf); err != nil {
			return err
		}
	}

	return nil
}
