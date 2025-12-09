package postprocessors

import (
	"fmt"
	"image"
	"image/color"
	"math"
)

// MinimumEdgeTransparencyProcessor applies a target minimum alpha value (p.Minimum,
// likely 255 for full opacity) to the outer 32 pixels, vignetting into the interior.
type MinimumEdgeTransparencyProcessor struct {
	Minimum uint8 // This is the target full opacity value (e.g., 255)
}

const vignetteDistance = 128.0

func (p *MinimumEdgeTransparencyProcessor) Process(src *image.RGBA) (*image.RGBA, error) {
	fmt.Printf("Applying vignette...\n")
	bounds := src.Bounds()
	width := float64(bounds.Dx())
	height := float64(bounds.Dy())

	// Target alpha at the edge (p.Minimum, e.g., 255)
	targetAlpha := float64(p.Minimum)

	// Alpha at the interior edge of the vignette zone (e.g., 0)
	interiorAlpha := 0.0

	alphaRange := targetAlpha - interiorAlpha

	effectiveVignetteDistance := math.Min(vignetteDistance, math.Min(width/2.0, height/2.0))
	if effectiveVignetteDistance < 1 {
		return src, nil
	}

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {

			distX := math.Min(float64(x-bounds.Min.X), float64(bounds.Max.X-1-x))
			distY := math.Min(float64(y-bounds.Min.Y), float64(bounds.Max.Y-1-y))
			minDist := math.Min(distX, distY)

			if minDist < effectiveVignetteDistance {
				r, g, b, a := src.At(x, y).RGBA()

				currentAlpha := uint8(a >> 8)

				factor := 1.0 - (minDist / effectiveVignetteDistance)
				factor = factor * factor * factor

				requiredAlpha := uint8(math.Round(interiorAlpha + (alphaRange * factor)))

				finalAlpha := currentAlpha
				if requiredAlpha > currentAlpha {
					finalAlpha = requiredAlpha
				}

				if finalAlpha != currentAlpha {
					src.Set(x, y, color.RGBA{
						R: uint8(r >> 8), G: uint8(g >> 8), B: uint8(b >> 8),
						A: finalAlpha})
				}
			}
		}
	}
	return src, nil
}
