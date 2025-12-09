package postprocessors

import (
	"image"
	"image/color"
	"math"
)

// MinimumEdgeTransparencyProcessor applies a target minimum alpha value (p.Minimum,
// likely 255 for full opacity) to the outer 32 pixels, vignetting into the interior.
type MinimumEdgeTransparencyProcessor struct {
	Minimum uint8 // This is the target full opacity value (e.g., 255)
}

const vignetteDistance = 32.0

func (p *MinimumEdgeTransparencyProcessor) Process(src *image.RGBA) (*image.RGBA, error) {
	// TODO: don't make it linear. the seam with 0 should be smooth.

	bounds := src.Bounds()
	width := float64(bounds.Dx())
	height := float64(bounds.Dy())

	// Target alpha at the edge (p.Minimum, e.g., 255)
	targetAlpha := float64(p.Minimum)

	// Alpha at the interior edge of the vignette zone (e.g., 0)
	interiorAlpha := 0.0

	// Difference between the edge target and the interior start (e.g., 255 - 0 = 255)
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

				// Calculate the interpolation factor based on distance:
				// factor goes from 0.0 at the interior limit to 1.0 at the edge (minDist = 0)
				// To get the ramp from 0 (interior) to 255 (edge), we use the distance factor
				// in reverse (1 - (distance / max distance))

				// Distance factor goes from 1.0 at the edge (minDist=0) to 0.0 at the interior limit (minDist=32)
				factor := 1.0 - (minDist / effectiveVignetteDistance)

				// Calculate the *required* alpha at this location:
				// RequiredAlpha = InteriorAlpha + (AlphaRange * factor)
				// E.g., RequiredAlpha = 0 + (255 * factor)
				requiredAlpha := uint8(math.Round(interiorAlpha + (alphaRange * factor)))

				// --- The Crucial Change ---
				// The final alpha is the MAXIMUM of the original alpha and the new required alpha.
				// This ensures that even if the original pixel was transparent (e.g., 10), it
				// is forced up to the higher requiredAlpha (e.g., 200), enforcing the opaque vignette.
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
