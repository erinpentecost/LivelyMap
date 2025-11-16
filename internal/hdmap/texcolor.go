package hdmap

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"math"
	"os"

	_ "embed"
)

type TexRenderer struct {
	minHeight   float32
	maxHeight   float32
	waterHeight float32
	// ramp is still used for water
	ramp            [256]color.RGBA
	texIndexToColor map[uint16]color.RGBA
}

func NewTexRenderer(rampFilePath string, texIndexToColor map[uint16]color.RGBA) (*TexRenderer, error) {
	if len(rampFilePath) == 0 {
		rmp, err := LoadRamp(bytes.NewReader(classicRampFile))
		if err != nil {
			return nil, fmt.Errorf("loading default ramp: %w", err)
		}
		return &TexRenderer{ramp: rmp}, nil
	}
	file, err := os.Open(rampFilePath)
	if err != nil {
		return nil, fmt.Errorf("loading ramp file %q: %w", rampFilePath, err)
	}
	rmp, err := LoadRamp(file)
	if err != nil {
		return nil, fmt.Errorf("loading default ramp: %w", err)
	}
	return &TexRenderer{ramp: rmp, texIndexToColor: map[uint16]color.RGBA{}}, nil
}

func (d *TexRenderer) GetCellResolution() (x uint32, y uint32) {
	return gridSize, gridSize
}

func (d *TexRenderer) SetHeightExtents(heightStats Stats, waterHeight float32) {
	d.maxHeight = float32(heightStats.Max())
	d.waterHeight = waterHeight

	// Throw away extreme low values that are underwater.
	// We are raising the "floor" here.
	potentialMin := float32(heightStats.Min())
	if potentialMin < d.waterHeight {
		d.minHeight = min(float32(heightStats.Quantile(0.1)), d.waterHeight)
	}
}

func (d *TexRenderer) Render(p *ParsedLandRecord) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, gridSize, gridSize))

	// Throw away the last column and row.
	// This is how I'm sampling a quad into a single pixel.
	for y := range gridSize {
		for x := range gridSize {
			// Need to invert y
			iy := gridSize - y - 1
			if p.heights[y][x] >= d.waterHeight {
				// get color from texture map
			} else {
				// use water ramp color
				img.SetRGBA(x, iy, d.ramp[d.transformHeight(p.heights[y][x])])
			}
		}
	}
	return img
}

func (d *TexRenderer) transformHeight(v float32) byte {
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
