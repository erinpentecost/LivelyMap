package ramp

import (
	"bytes"
	"fmt"
	"image/color"
	"io"
	"math"
	"os"

	_ "embed"

	"golang.org/x/image/bmp"
)

//go:embed ramp_hemaris.bmp
var classicRampFile []byte

type ColorRamp struct {
	ramp [512]color.RGBA
}

func LoadRamp(rampFilePath string) (*ColorRamp, error) {
	var reader io.Reader
	if len(rampFilePath) != 0 {
		var err error
		reader, err = os.Open(rampFilePath)
		if err != nil {
			return nil, fmt.Errorf("loading ramp file %q: %w", rampFilePath, err)
		}
	} else {
		reader = bytes.NewReader(classicRampFile)
	}

	var ramp [512]color.RGBA
	rampImg, err := bmp.Decode(reader)
	if err != nil {
		return &ColorRamp{ramp: ramp}, fmt.Errorf("failed to decode color ramp BMP: %w", err)
	}
	b := rampImg.Bounds()
	if b.Dy() != 1 || b.Dx() < 512 {
		return &ColorRamp{ramp: ramp}, fmt.Errorf("invalid color ramp dimensions (expected 1x512, got %dx%d)", b.Dx(), b.Dy())
	}
	for x := 0; x < 512; x++ {
		r, g, bb, a := rampImg.At(x, 0).RGBA()
		ramp[x] = color.RGBA{
			R: uint8(r >> 8),
			G: uint8(g >> 8),
			B: uint8(bb >> 8),
			A: uint8(a >> 8),
		}
	}
	return &ColorRamp{ramp: ramp}, nil
}

func (c *ColorRamp) Color(v, min, max, midpoint float32) color.RGBA {
	// clamp extremes
	if v <= min {
		return c.ramp[0]
	}
	if v >= max {
		return c.ramp[511]
	}

	// exact midpoint → first value of upper half
	if v == midpoint {
		return c.ramp[256]
	}

	if v < midpoint {
		den := midpoint - min
		if den == 0 {
			return c.ramp[0]
		}
		normalized := float64((v - min) / den) // 0..1
		idx := int(math.Round(normalized * 255.0))
		if idx < 0 {
			idx = 0
		}
		if idx > 255 {
			idx = 255
		}
		return c.ramp[idx]
	}

	// v > midpoint
	den := max - midpoint
	if den == 0 {
		return c.ramp[511]
	}
	normalized := float64((v - midpoint) / den) // 0..1
	idx := int(math.Round(normalized * 255.0))  // 0..255
	if idx < 0 {
		idx = 0
	}
	if idx > 255 {
		idx = 255
	}
	return c.ramp[256+idx]
}
