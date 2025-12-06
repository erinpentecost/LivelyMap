package dds

import (
	"encoding/binary"
	"errors"
	"image"
	"image/color"
	"image/draw"
	"io"
	"math"
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

			// Color block (8 bytes) - NOW USING PCA
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
			// Fast division-by-7 approximation: (num + 3) / 7
			palette[1+i] = uint8((num + 3) / 7)
		}
	} else {
		for i := 1; i <= 4; i++ {
			num := uint32((5-i)*int(a0) + i*int(a1))
			// Fast division-by-5 approximation: (num + 2) / 5
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

// Color is a 3D float vector (R, G, B)
type Color [3]float64

// compressDXT1Color uses PCA to find better color endpoints.
func compressDXT1Color(px [16]color.RGBA) []byte {
	// 1. Calculate the block's centroid (mean color)
	var avg Color
	for _, p := range px {
		avg[0] += float64(p.R)
		avg[1] += float64(p.G)
		avg[2] += float64(p.B)
	}
	avg[0] /= 16.0
	avg[1] /= 16.0
	avg[2] /= 16.0

	// 2. Calculate the 3x3 covariance matrix (S)
	var S [3][3]float64
	for _, p := range px {
		r := float64(p.R) - avg[0]
		g := float64(p.G) - avg[1]
		b := float64(p.B) - avg[2]

		S[0][0] += r * r
		S[0][1] += r * g
		S[0][2] += r * b
		S[1][1] += g * g
		S[1][2] += g * b
		S[2][2] += b * b
	}

	S[1][0] = S[0][1]
	S[2][0] = S[0][2]
	S[2][1] = S[1][2]

	// 3. Find the principal eigenvector (v) of S (Power Iteration is often fast enough)
	// For simplicity and speed in a real-time compressor, we'll use a simplified
	// power iteration or a simple axis based on the bounding box for a robust start.
	// Since full 3D PCA (eigenvalue decomposition) is complex, we will use
	// a simplified 'Power Iteration' for the largest eigenvalue's eigenvector.
	v := simplifyPowerIteration(S)

	// 4. Project all points onto the principal axis (v)
	minProj, maxProj := math.MaxFloat64, -math.MaxFloat64

	for _, p := range px {
		pColor := Color{float64(p.R), float64(p.G), float64(p.B)}
		proj := dot(pColor, v)

		if proj < minProj {
			minProj = proj
		}
		if proj > maxProj {
			maxProj = proj
		}
		// The projected points themselves (minProj, maxProj) are used to define the endpoints
	}

	// 5. Calculate the DXT endpoints (c0, c1) from the min/max projections.
	// We want to scale the vector v so that its endpoints land at the min/max projected values.
	// The line in 3D space is: L(t) = avg + t*v
	// Projection of L(t) onto v is: dot(avg + t*v, v) = dot(avg, v) + t*dot(v, v)
	// Since v is normalized, dot(v, v) = 1. Projection is: dot(avg, v) + t
	// t_min = minProj - dot(avg, v)
	// t_max = maxProj - dot(avg, v)

	avgProj := dot(avg, v)
	tMin := minProj - avgProj
	tMax := maxProj - avgProj

	// Endpoint vectors: L(t_max) and L(t_min)
	end0 := add(avg, scale(v, tMax)) // Corresponds to max projection
	end1 := add(avg, scale(v, tMin)) // Corresponds to min projection

	// The results are clamped and converted to 565 colors
	c0 := rgbTo565f(end0[0], end0[1], end0[2])
	c1 := rgbTo565f(end1[0], end1[1], end1[2])

	// Ensure c0 > c1 to use the 4-color palette mode
	if c0 < c1 {
		c0, c1 = c1, c0
		end0, end1 = end1, end0
	}

	// 6. Generate the color palette
	col0 := decode565(c0)
	col1 := decode565(c1)

	var palette [4][3]uint8
	palette[0] = col0
	palette[1] = col1

	for i := 0; i < 3; i++ {
		// DXT1 interpolation: c2 = (2*c0 + c1) / 3, c3 = (c0 + 2*c1) / 3
		// +1 is for rounding: (a+b+c)/n = floor((a+b+c+n/2)/n)
		palette[2][i] = uint8((2*uint16(col0[i]) + uint16(col1[i]) + 1) / 3)
		palette[3][i] = uint8((uint16(col0[i]) + 2*uint16(col1[i]) + 1) / 3)
	}

	// 7. Find the best index for each pixel (same as before)
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

	// 8. Pack and return
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

// --- PCA Helper Functions ---

// dot product of two 3D vectors
func dot(a, b Color) float64 {
	return a[0]*b[0] + a[1]*b[1] + a[2]*b[2]
}

// normalize a 3D vector
func normalize(v Color) Color {
	len := math.Sqrt(dot(v, v))
	if len == 0 {
		return Color{}
	}
	return Color{v[0] / len, v[1] / len, v[2] / len}
}

// scale a 3D vector by a scalar
func scale(v Color, s float64) Color {
	return Color{v[0] * s, v[1] * s, v[2] * s}
}

// add two 3D vectors
func add(a, b Color) Color {
	return Color{a[0] + b[0], a[1] + b[1], a[2] + b[2]}
}

// simplifyPowerIteration estimates the principal eigenvector of the 3x3 covariance matrix S.
func simplifyPowerIteration(S [3][3]float64) Color {
	// Start with a guess vector (1, 1, 1) normalized
	v := normalize(Color{1.0, 1.0, 1.0})

	// Run a few iterations of matrix-vector multiplication (S * v)
	// A few iterations is usually sufficient for texture compression
	for i := 0; i < 5; i++ {
		var nextV Color
		// S * v
		nextV[0] = S[0][0]*v[0] + S[0][1]*v[1] + S[0][2]*v[2]
		nextV[1] = S[1][0]*v[0] + S[1][1]*v[1] + S[1][2]*v[2]
		nextV[2] = S[2][0]*v[0] + S[2][1]*v[1] + S[2][2]*v[2]

		v = normalize(nextV)
	}
	return v
}

// rgbTo565 converts float RGB [0, 255] to a 5-bit R, 6-bit G, 5-bit B uint16.
func rgbTo565f(r, g, b float64) uint16 {
	// Clamp to [0, 255] and round
	fr := math.Round(math.Max(0, math.Min(255, r)))
	fg := math.Round(math.Max(0, math.Min(255, g)))
	fb := math.Round(math.Max(0, math.Min(255, b)))

	return uint16((uint32(fr)>>3)<<11 | (uint32(fg)>>2)<<5 | (uint32(fb) >> 3))
}

// Original 565 conversion (retained for palette generation)
func rgbTo565(r, g, b uint8) uint16 {
	return uint16((uint32(r)>>3)<<11 | (uint32(g)>>2)<<5 | (uint32(b) >> 3))
}

func decode565(v uint16) [3]uint8 {
	return [3]uint8{
		// To get back to 8-bit, the 5-bit value is shifted left by 3 and the
		// 5 MSBs are duplicated into the 3 LSBs to improve color distribution
		// (e.g., 0b11111 becomes 0b11111111, not 0b11111000).
		// Simplified is just << 3, which is often used.
		uint8(((v >> 11) & 0x1F) << 3), // R (5-bit)
		uint8(((v >> 5) & 0x3F) << 2),  // G (6-bit)
		uint8((v & 0x1F) << 3),         // B (5-bit)
	}
}
