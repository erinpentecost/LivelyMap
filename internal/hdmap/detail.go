package hdmap

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"math"
	"os"

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
	ramp [256]color.RGBA
}

func NewDetailRenderer(rampFilePath string, textures map[uint16]image.Image) (*DetailRenderer, error) {
	out := &DetailRenderer{}

	// load rampfile
	if len(rampFilePath) == 0 {
		rmp, err := LoadRamp(bytes.NewReader(classicRampFile))
		if err != nil {
			return nil, fmt.Errorf("loading default ramp: %w", err)
		}
		out.ramp = rmp
	} else {
		file, err := os.Open(rampFilePath)
		if err != nil {
			return nil, fmt.Errorf("loading ramp file %q: %w", rampFilePath, err)
		}
		rmp, err := LoadRamp(file)
		if err != nil {
			return nil, fmt.Errorf("loading default ramp: %w", err)
		}
		out.ramp = rmp
	}

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
			baseColor := d.ramp[d.transformHeight(p.heights[y][x])]
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

func (d *DetailRenderer) transformHeight(v float32) byte {
	// clamp extremes
	if v <= d.minHeight {
		return 0
	}
	if v >= d.maxHeight {
		return 0xFF // 255
	}

	// exact water line -> the first value of the upper half
	if v == d.waterHeight {
		return 128
	}

	if v < d.waterHeight {
		denom := d.waterHeight - d.minHeight
		if denom == 0 {
			return 0
		}
		normalized := float64((v - d.minHeight) / denom) // 0..1
		// map to 0..127 (128 values)
		val := math.Round(normalized * 127.0)
		if val < 0 {
			val = 0
		}
		if val > 127 {
			val = 127
		}
		return byte(uint8(val))
	}

	// v > waterHeight -> map to 128..255 (128 values)
	denom := d.maxHeight - d.waterHeight
	if denom == 0 {
		return 0xFF
	}
	normalized := float64((v - d.waterHeight) / denom) // 0..1
	val := math.Round(normalized * 127.0)              // 0..127
	if val < 0 {
		val = 0
	}
	if val > 127 {
		val = 127
	}
	return byte(uint8(128 + int(val)))
}
