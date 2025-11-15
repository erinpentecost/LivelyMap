package dds

import (
	"encoding/binary"
	"errors"
	"image"
	"image/color"
	"image/draw"
	"io"
)

// Encode writes m encoded as DDS DXT5 (BC3) into w.
func Encode(w io.Writer, m image.Image) error {
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

	if err := writeDDSHeader(w, width, height); err != nil {
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

			// DXT5 alpha (8 bytes)
			a := compressDXT5Alpha(px)

			// Color block (8 bytes)
			c := compressDXT1Color(px)

			if _, err := w.Write(a); err != nil {
				return err
			}
			if _, err := w.Write(c); err != nil {
				return err
			}
		}
	}

	return nil
}

func writeDDSHeader(w io.Writer, width, height int) error {
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

	// Flags
	binary.LittleEndian.PutUint32(header[4:], DDSD_CAPS|
		DDSD_HEIGHT|DDSD_WIDTH|DDSD_PIXELFORMAT|DDSD_LINEARSIZE)

	binary.LittleEndian.PutUint32(header[8:], uint32(height))
	binary.LittleEndian.PutUint32(header[12:], uint32(width))

	// Correct LinearSize for DXT5
	blocksAcross := (width + 3) / 4
	blocksDown := (height + 3) / 4
	linearSize := blocksAcross * blocksDown * 16
	binary.LittleEndian.PutUint32(header[16:], uint32(linearSize))

	// ----- PixelFormat (correct offset = 72) -----
	pf := 72
	binary.LittleEndian.PutUint32(header[pf+0:], 32)          // pfSize
	binary.LittleEndian.PutUint32(header[pf+4:], DDPF_FOURCC) // pfFlags
	binary.LittleEndian.PutUint32(header[pf+8:], 0x35545844)  // 'DXT5'

	// ----- Caps -----
	binary.LittleEndian.PutUint32(header[104:], DDSCAPS_TEXTURE)

	_, err := w.Write(header[:])
	return err
}

//////////////////
// DXT5 Alpha   //
//////////////////

func compressDXT5Alpha(px [16]color.RGBA) []byte {
	minA, maxA := uint8(255), uint8(0)
	for _, p := range px {
		if p.A < minA {
			minA = p.A
		}
		if p.A > maxA {
			maxA = p.A
		}
	}

	a0 := maxA
	a1 := minA

	var palette [8]uint8
	palette[0], palette[1] = a0, a1

	if a0 > a1 {
		for i := 1; i <= 6; i++ {
			num := uint32((7-i)*int(a0) + i*int(a1))
			palette[1+i] = uint8((num + 3) / 7)
		}
	} else {
		for i := 1; i <= 4; i++ {
			num := uint32((5-i)*int(a0) + i*int(a1))
			palette[1+i] = uint8((num + 2) / 5)
		}
		palette[6] = 0
		palette[7] = 255
	}

	var idx [16]uint8
	for i, p := range px {
		best := uint8(0)
		bestDist := uint32(1<<32 - 1)
		for j := 0; j < 8; j++ {
			d := int(p.A) - int(palette[j])
			d *= d
			if uint32(d) < bestDist {
				bestDist = uint32(d)
				best = uint8(j)
			}
		}
		idx[i] = best
	}

	var packed [6]byte
	bit := 0
	for i := 0; i < 16; i++ {
		v := uint(idx[i]) & 0x7
		bi := bit / 8
		sh := bit % 8
		packed[bi] |= byte(v << sh)
		if sh > 5 && bi+1 < 6 {
			packed[bi+1] |= byte(v >> (8 - sh))
		}
		bit += 3
	}

	out := make([]byte, 8)
	out[0], out[1] = a0, a1
	copy(out[2:], packed[:])
	return out
}

/////////////////////////
// DXT1-style Color    //
/////////////////////////

func compressDXT1Color(px [16]color.RGBA) []byte {
	minR, minG, minB := uint8(255), uint8(255), uint8(255)
	maxR, maxG, maxB := uint8(0), uint8(0), uint8(0)

	for _, p := range px {
		if p.R < minR {
			minR = p.R
		}
		if p.G < minG {
			minG = p.G
		}
		if p.B < minB {
			minB = p.B
		}
		if p.R > maxR {
			maxR = p.R
		}
		if p.G > maxG {
			maxG = p.G
		}
		if p.B > maxB {
			maxB = p.B
		}
	}

	c0 := rgbTo565(maxR, maxG, maxB)
	c1 := rgbTo565(minR, minG, minB)

	if c0 <= c1 {
		c0, c1 = c1, c0
	}

	col0 := decode565(c0)
	col1 := decode565(c1)

	var palette [4][3]uint8
	palette[0] = col0
	palette[1] = col1

	for i := 0; i < 3; i++ {
		palette[2][i] = uint8((2*uint16(col0[i]) + uint16(col1[i]) + 1) / 3)
		palette[3][i] = uint8((uint16(col0[i]) + 2*uint16(col1[i]) + 1) / 3)
	}

	var idx [16]uint8
	for i, p := range px {
		br, bg, bb := p.R, p.G, p.B
		best := uint8(0)
		bestDist := uint32(1<<32 - 1)
		for j := 0; j < 4; j++ {
			dr := int(br) - int(palette[j][0])
			dg := int(bg) - int(palette[j][1])
			db := int(bb) - int(palette[j][2])
			d := dr*dr + dg*dg + db*db
			if uint32(d) < bestDist {
				bestDist = uint32(d)
				best = uint8(j)
			}
		}
		idx[i] = best
	}

	var packed uint32
	for i := 0; i < 16; i++ {
		packed |= uint32(idx[i]&0x3) << (2 * uint(i))
	}

	out := make([]byte, 8)
	binary.LittleEndian.PutUint16(out, c0)
	binary.LittleEndian.PutUint16(out[2:], c1)
	binary.LittleEndian.PutUint32(out[4:], packed)
	return out
}

func rgbTo565(r, g, b uint8) uint16 {
	return uint16((uint32(r)>>3)<<11 | (uint32(g)>>2)<<5 | (uint32(b) >> 3))
}

func decode565(v uint16) [3]uint8 {
	return [3]uint8{
		uint8(((v >> 11) & 0x1F) << 3),
		uint8(((v >> 5) & 0x3F) << 2),
		uint8((v & 0x1F) << 3),
	}
}
