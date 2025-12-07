package hdmap

import (
	"fmt"
	"image"
	"image/color"
	"math"

	"github.com/erinpentecost/LivelyMap/internal/hdmap/ramp"
	"github.com/erinpentecost/LivelyMap/internal/hue"

	_ "embed"
)

type colorSampler struct {
	source image.Image
	avgHue float64
	dx     int
	dy     int
}

func newColorSampler(source image.Image) *colorSampler {
	x := source.Bounds().Dx()
	y := source.Bounds().Dy()
	if x == 0 || y == 0 {
		return nil
	}

	return &colorSampler{
		source: source,
		avgHue: hue.GetAverageHue(source),
		dx:     x,
		dy:     y,
	}
}

func (c *colorSampler) Sample(x, y int) color.Color {
	return c.source.At(
		x%c.dx,
		y%c.dy,
	)
}

type DetailRenderer struct {
	minHeight   float32
	maxHeight   float32
	waterHeight float32
	// ramp is still used for water and as a fallback
	ramp *ramp.ColorRamp
}

func NewDetailRenderer(rampFilePath string, textures map[uint16]image.Image) (*DetailRenderer, error) {
	out := &DetailRenderer{}

	// load rampfile
	rmp, err := ramp.LoadRamp(rampFilePath)
	if err != nil {
		return nil, fmt.Errorf("loading default ramp: %w", err)
	}
	out.ramp = rmp

	return out, nil
}

func (d *DetailRenderer) SetHeightExtents(heightStats Stats, waterHeight float32) {
	d.maxHeight = float32(heightStats.Max())
	d.waterHeight = waterHeight

	// Throw away extreme low values that are underwater.
	// We are raising the "floor" here.
	potentialMin := float32(heightStats.Min())
	if potentialMin < d.waterHeight {
		d.minHeight = min(float32(heightStats.Quantile(0.1)), d.waterHeight)
	}
}

func (d *DetailRenderer) Render(p *ParsedLandRecord) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, gridSize, gridSize))

	// Throw away the last column and row.
	// This is how I'm sampling a quad into a single pixel.
	for y := range gridSize {
		for x := range gridSize {
			// Need to invert y
			iy := gridSize - y - 1
			baseColor := d.ramp.Color(p.heights[y][x], d.minHeight, d.maxHeight, d.waterHeight)
			if p.heights[y][x] >= d.waterHeight {
				// multiply vertex color onto the heightmap color
				if len(p.colors) == 65 && len(p.colors[y]) == 65 {
					baseColor = hue.MulColor(baseColor, color.RGBA{
						R: p.colors[y][x].R,
						G: p.colors[y][x].G,
						B: p.colors[y][x].B,
						A: math.MaxUint8,
					})
				}
			}

			img.SetRGBA(x, iy, baseColor)
		}
	}
	return img
}
