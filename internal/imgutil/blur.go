package imgutil

import (
	"image"
	"image/color"
	"math"
)

// Simple Gaussian kernel generator
func gaussianKernel(radius int, sigma float64) []float64 {
	size := radius*2 + 1
	kernel := make([]float64, size)

	var sum float64
	for i := -radius; i <= radius; i++ {
		v := math.Exp(-(float64(i * i)) / (2 * sigma * sigma))
		kernel[i+radius] = v
		sum += v
	}

	for i := range kernel {
		kernel[i] /= sum
	}
	return kernel
}

func BlurRGBIgnoreTransparent(src *image.RGBA, radius int, sigma float64) *image.RGBA {
	bounds := src.Bounds()
	dst := image.NewRGBA(bounds)

	kernel := gaussianKernel(radius, sigma)

	w := bounds.Dx()
	h := bounds.Dy()

	// Horizontal pass
	tmp := image.NewRGBA(bounds)

	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			var r, g, b, wsum float64
			srcPx := src.RGBAAt(x, y)

			for k := -radius; k <= radius; k++ {
				nx := x + k
				if nx < 0 || nx >= w {
					continue
				}

				c := src.RGBAAt(nx, y)
				if c.A == 0 {
					continue
				}

				weight := kernel[k+radius]
				r += float64(c.R) * weight
				g += float64(c.G) * weight
				b += float64(c.B) * weight
				wsum += weight
			}

			if wsum > 0 {
				r /= wsum
				g /= wsum
				b /= wsum
				tmp.SetRGBA(x, y, color.RGBA{
					R: uint8(clamp(r)),
					G: uint8(clamp(g)),
					B: uint8(clamp(b)),
					A: srcPx.A, // alpha untouched
				})
			} else {
				// IMPORTANT: preserve original pixel
				tmp.SetRGBA(x, y, srcPx)
			}
		}
	}

	// Vertical pass
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			var r, g, b, wsum float64
			srcPx := tmp.RGBAAt(x, y)

			for k := -radius; k <= radius; k++ {
				ny := y + k
				if ny < 0 || ny >= h {
					continue
				}

				c := tmp.RGBAAt(x, ny)
				if c.A == 0 {
					continue
				}

				weight := kernel[k+radius]
				r += float64(c.R) * weight
				g += float64(c.G) * weight
				b += float64(c.B) * weight
				wsum += weight
			}

			if wsum > 0 {
				r /= wsum
				g /= wsum
				b /= wsum
				dst.SetRGBA(x, y, color.RGBA{
					R: uint8(clamp(r)),
					G: uint8(clamp(g)),
					B: uint8(clamp(b)),
					A: srcPx.A,
				})
			} else {
				// IMPORTANT: preserve original pixel
				dst.SetRGBA(x, y, srcPx)
			}
		}
	}

	return dst
}

func clamp(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return v
}
