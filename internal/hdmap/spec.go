package hdmap

import (
	"image"
	"image/color"
	"math"

	_ "embed"
)

type SpecularRenderer struct {
	waterHeight float32
	ramp        [256]color.RGBA
}

func NewSpecularRenderer() (*SpecularRenderer, error) {
	return &SpecularRenderer{}, nil
}

func (d *SpecularRenderer) SetHeightExtents(heightStats Stats, waterHeight float32) {
	d.waterHeight = waterHeight
}

func (d *SpecularRenderer) Render(p *ParsedLandRecord) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, gridSize, gridSize))

	// Throw away the last column and row.
	// This is how I'm sampling a quad into a single pixel.
	for y := range gridSize {
		for x := range gridSize {
			// Need to invert y
			iy := gridSize - y - 1
			img.SetRGBA(x, iy, d.transformHeight(p.heights[y][x]))
		}
	}
	return img
}

func (d *SpecularRenderer) transformHeight(v float32) color.RGBA {
	if v < d.waterHeight {
		return color.RGBA{
			R: math.MaxUint8,
			G: math.MaxUint8,
			B: math.MaxUint8,
			A: math.MaxUint8 / 8,
		}
	} else {
		return color.RGBA{
			R: math.MaxUint8,
			G: math.MaxUint8,
			B: math.MaxUint8,
			A: 0,
		}
	}
}
