package postprocessors

import (
	"fmt"
	"image"
	"image/color"
)

type LocalToneMapAlpha struct {
	WindowRadiusDenom int
}

func (p *LocalToneMapAlpha) Process(src *image.RGBA) (*image.RGBA, error) {
	fmt.Printf("Exaggerating bumps...\n")
	b := src.Bounds()
	w := b.Dx()
	h := b.Dy()

	windowRadius := max(b.Max.X, b.Max.Y) / p.WindowRadiusDenom
	if windowRadius < 1 {
		return src, nil
	}

	// ---- Build Integral Image ----
	intImg := make([][]int, h+1)
	for i := range intImg {
		intImg[i] = make([]int, w+1)
	}

	for y := 0; y < h; y++ {
		rowSum := 0
		for x := 0; x < w; x++ {
			_, _, _, a := src.At(b.Min.X+x, b.Min.Y+y).RGBA()
			a8 := int(a >> 8)

			rowSum += a8
			intImg[y+1][x+1] = intImg[y][x+1] + rowSum
		}
	}

	// ---- Tone Mapping ----
	r := windowRadius
	dst := image.NewRGBA(b)

	eps := 1e-3
	targetMean := 64.0 // keep midrange stable

	for y := 0; y < h; y++ {
		y0 := max(0, y-r)
		y1 := min(h-1, y+r)

		for x := 0; x < w; x++ {
			x0 := max(0, x-r)
			x1 := min(w-1, x+r)

			sum := intImg[y1+1][x1+1] -
				intImg[y0][x1+1] -
				intImg[y1+1][x0] +
				intImg[y0][x0]

			area := float64((y1 - y0 + 1) * (x1 - x0 + 1))
			localMean := float64(sum) / area

			R, G, B, A := src.At(b.Min.X+x, b.Min.Y+y).RGBA()
			alpha := float64(A >> 8)

			// --- adaptive mean-based scaling ---
			scale := targetMean / (localMean + eps)
			norm := alpha * scale

			// --- soft-knee highlight compression (prevents 255 pileup) ---
			// reduces overshoot while keeping relative contrasts
			compressed := norm - (norm*norm)/255.0*0.25

			// average it with the actual values.
			compressed = (compressed + float64(alpha)) / 2

			// clamp
			if compressed < 0 {
				compressed = 0
			}
			if compressed > 255 {
				compressed = 255
			}

			dst.Set(b.Min.X+x, b.Min.Y+y, color.RGBA{
				R: uint8(R >> 8),
				G: uint8(G >> 8),
				B: uint8(B >> 8),
				A: uint8(compressed),
			})
		}
	}

	return dst, nil
}

// helpers
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
