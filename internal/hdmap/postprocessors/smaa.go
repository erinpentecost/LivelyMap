package postprocessors

import (
	"fmt"
	"image"
	"image/color"
	"math"
)

/*
SMAA implementation overview (CPU-friendly):

1. Edge detection
   - Luma-based horizontal & vertical edge detection

2. Blend weight calculation
   - Detect simple edge spans
   - Compute blending weights based on span length

3. Neighborhood blending
   - Blend neighboring pixels using computed weights
*/

const (
	lumaThreshold = 0.1
	maxSearch     = 8
)

type SMAA struct{}

// Apply applies SMAA to the input image and returns a new image.
// This is very subtle and probably not worth it.
func (s *SMAA) Process(src *image.RGBA) (*image.RGBA, error) {
	fmt.Printf("Anti-aliasing...\n")
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()

	edgesH := make([]float32, w*h)
	edgesV := make([]float32, w*h)

	// 1. Edge detection
	s.detectEdges(src, edgesH, edgesV, w, h)

	// 2. Blend weight computation
	weightsH := make([]float32, w*h)
	weightsV := make([]float32, w*h)
	s.computeWeights(edgesH, edgesV, weightsH, weightsV, w, h)

	// 3. Neighborhood blending
	dst := image.NewRGBA(b)
	s.blend(src, dst, weightsH, weightsV, w, h)

	return dst, nil
}

func (s *SMAA) detectEdges(img *image.RGBA, edgesH, edgesV []float32, w, h int) {
	for y := 1; y < h-1; y++ {
		for x := 1; x < w-1; x++ {
			i := y*w + x

			l := s.lumaAt(img, x-1, y)
			r := s.lumaAt(img, x+1, y)
			u := s.lumaAt(img, x, y-1)
			d := s.lumaAt(img, x, y+1)

			if math.Abs(float64(l-r)) > lumaThreshold {
				edgesV[i] = 1
			}
			if math.Abs(float64(u-d)) > lumaThreshold {
				edgesH[i] = 1
			}
		}
	}
}

func (s *SMAA) computeWeights(edgesH, edgesV, weightsH, weightsV []float32, w, h int) {
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			i := y*w + x

			if edgesH[i] > 0 {
				span := s.searchSpan(edgesH, x, y, 0, 1, w, h)
				weightsH[i] = s.spanWeight(span)
			}
			if edgesV[i] > 0 {
				span := s.searchSpan(edgesV, x, y, 1, 0, w, h)
				weightsV[i] = s.spanWeight(span)
			}
		}
	}
}

func (s *SMAA) searchSpan(edges []float32, x, y, dx, dy, w, h int) int {
	span := 0
	for i := 1; i <= maxSearch; i++ {
		nx := x + dx*i
		ny := y + dy*i
		if nx < 0 || ny < 0 || nx >= w || ny >= h {
			break
		}
		if edges[ny*w+nx] == 0 {
			break
		}
		span++
	}
	return span
}

func (s *SMAA) spanWeight(span int) float32 {
	if span == 0 {
		return 0
	}
	w := float32(span) / float32(maxSearch)
	if w > 1 {
		return 1
	}
	return w
}

func (s *SMAA) blend(src, dst *image.RGBA, weightsH, weightsV []float32, w, h int) {
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			i := y*w + x

			c := src.RGBAAt(x, y)

			var r, g, b, a float32
			r += float32(c.R)
			g += float32(c.G)
			b += float32(c.B)
			a += float32(c.A)

			var total float32 = 1

			if weightsH[i] > 0 && y+1 < h {
				c2 := src.RGBAAt(x, y+1)
				w := weightsH[i]
				r += float32(c2.R) * w
				g += float32(c2.G) * w
				b += float32(c2.B) * w
				a += float32(c2.A) * w
				total += w
			}

			if weightsV[i] > 0 && x+1 < w {
				c2 := src.RGBAAt(x+1, y)
				w := weightsV[i]
				r += float32(c2.R) * w
				g += float32(c2.G) * w
				b += float32(c2.B) * w
				a += float32(c2.A) * w
				total += w
			}

			dst.SetRGBA(x, y, color.RGBA{
				R: uint8(s.clamp(r / total)),
				G: uint8(s.clamp(g / total)),
				B: uint8(s.clamp(b / total)),
				A: uint8(s.clamp(a / total)),
			})
		}
	}
}

func (s *SMAA) lumaAt(img *image.RGBA, x, y int) float32 {
	c := img.RGBAAt(x, y)
	return (0.2126*float32(c.R) +
		0.7152*float32(c.G) +
		0.0722*float32(c.B)) / 255
}

func (s *SMAA) clamp(v float32) float32 {
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return v
}
