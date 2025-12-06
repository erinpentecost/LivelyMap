package dds

import (
	"encoding/binary"
	"errors"
	"image"
	"image/color"
	"image/draw"
	"io"
)

// EncodeDXT1 writes m encoded as DDS DXT1 (BC1) into w.
func EncodeDXT1(w io.Writer, m image.Image) error {
	var rgba *image.RGBA
	if im, ok := m.(*image.RGBA); ok {
		rgba = im
	} else {
		b := m.Bounds()
		rgba = image.NewRGBA(b)
		draw.Draw(rgba, b, m, b.Min, draw.Src)
	}

	width := rgba.Bounds().Dx()
	height := rgba.Bounds().Dy()

	if width == 0 || height == 0 {
		return errors.New("dds: empty image")
	}

	if err := writeDDSHeaderDXT1(w, width, height); err != nil {
		return err
	}

	for by := 0; by < height; by += 4 {
		for bx := 0; bx < width; bx += 4 {
			var px [16]color.RGBA
			i := 0
			for dy := 0; dy < 4; dy++ {
				for dx := 0; dx < 4; dx++ {
					x := bx + dx
					y := by + dy
					if x < width && y < height {
						off := (y-rgba.Rect.Min.Y)*rgba.Stride + (x-rgba.Rect.Min.X)*4
						px[i] = color.RGBA{
							R: rgba.Pix[off+0],
							G: rgba.Pix[off+1],
							B: rgba.Pix[off+2],
							A: rgba.Pix[off+3],
						}
					}
					i++
				}
			}

			// DXT1 has ONLY the color block
			c := compressDXT1Color(px)

			if _, err := w.Write(c); err != nil {
				return err
			}
		}
	}

	return nil
}

func writeDDSHeaderDXT1(w io.Writer, width, height int) error {
	if _, err := w.Write([]byte("DDS ")); err != nil {
		return err
	}

	var header [124]byte
	binary.LittleEndian.PutUint32(header[0:], 124)

	const (
		DDSD_CAPS        = 0x1
		DDSD_HEIGHT      = 0x2
		DDSD_WIDTH       = 0x4
		DDSD_PIXELFORMAT = 0x1000
		DDSD_LINEARSIZE  = 0x80000

		DDPF_FOURCC     = 0x4
		DDSCAPS_TEXTURE = 0x1000
	)

	binary.LittleEndian.PutUint32(header[4:], DDSD_CAPS|
		DDSD_HEIGHT|DDSD_WIDTH|DDSD_PIXELFORMAT|DDSD_LINEARSIZE)

	binary.LittleEndian.PutUint32(header[8:], uint32(height))
	binary.LittleEndian.PutUint32(header[12:], uint32(width))

	// DXT1 = 8 bytes per 4×4 block
	blocksAcross := (width + 3) / 4
	blocksDown := (height + 3) / 4
	linearSize := blocksAcross * blocksDown * 8 // <-- DXT1 uses 8
	binary.LittleEndian.PutUint32(header[16:], uint32(linearSize))

	// PixelFormat @ offset 72
	pf := 72
	binary.LittleEndian.PutUint32(header[pf+0:], 32)          // size
	binary.LittleEndian.PutUint32(header[pf+4:], DDPF_FOURCC) // flags
	binary.LittleEndian.PutUint32(header[pf+8:], 0x31545844)  // 'DXT1'

	binary.LittleEndian.PutUint32(header[104:], DDSCAPS_TEXTURE)

	_, err := w.Write(header[:])
	return err
}
