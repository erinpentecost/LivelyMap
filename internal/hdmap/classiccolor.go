package hdmap

import (
	"fmt"
	"image"

	_ "embed"

	"github.com/erinpentecost/LivelyMap/internal/hdmap/ramp"
)

type ClassicRenderer struct {
	minHeight   float32
	maxHeight   float32
	waterHeight float32
	ramp        *ramp.ColorRamp
}

func NewClassicRenderer(rampFilePath string) (*ClassicRenderer, error) {
	rmp, err := ramp.LoadRamp(rampFilePath)
	if err != nil {
		return nil, fmt.Errorf("loading default ramp: %w", err)
	}
	return &ClassicRenderer{ramp: rmp}, nil
}

func (d *ClassicRenderer) SetHeightExtents(heightStats Stats, waterHeight float32) {
	d.maxHeight = float32(heightStats.Max())
	d.waterHeight = waterHeight

	// Throw away extreme low values that are underwater.
	// We are raising the "floor" here.
	potentialMin := float32(heightStats.Min())
	if potentialMin < d.waterHeight {
		d.minHeight = min(float32(heightStats.Quantile(0.1)), d.waterHeight)
	}
}

// normalHeightMap generates a *_nh (normal height map) texture for openmw.
// The RGB channels of the normal map are used to store XYZ components of
// tangent space normals and the alpha channel of the normal map may be used
// to store a height map used for parallax.
func (d *ClassicRenderer) Render(p *ParsedLandRecord) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, gridSize, gridSize))

	// Throw away the last column and row.
	// This is how I'm sampling a quad into a single pixel.
	for y := range gridSize {
		for x := range gridSize {
			// Need to invert y
			iy := gridSize - y - 1
			img.SetRGBA(x, iy, d.ramp.Color(p.heights[y][x], d.minHeight, d.maxHeight, d.waterHeight))
		}
	}
	return img
}
