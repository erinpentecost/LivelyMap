package hdmap

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"io"
	"math"
	"os"

	_ "embed"

	"golang.org/x/image/bmp"
)

//go:embed ramp.bmp
var classicRampFile []byte

func LoadRamp(rawBmp io.Reader) ([256]color.RGBA, error) {
	// height of 0.000 should be x=128
	var ramp [256]color.RGBA
	rampImg, err := bmp.Decode(rawBmp)
	if err != nil {
		return ramp, fmt.Errorf("failed to decode color ramp BMP: %w", err)
	}
	b := rampImg.Bounds()
	if b.Dy() != 1 || b.Dx() < 256 {
		return ramp, fmt.Errorf("invalid color ramp dimensions (expected 1x256, got %dx%d)", b.Dx(), b.Dy())
	}
	for x := range 256 {
		r, g, b, a := rampImg.At(x, 0).RGBA()
		ramp[x] = color.RGBA{
			R: uint8(r >> 8),
			G: uint8(g >> 8),
			B: uint8(b >> 8),
			A: uint8(a >> 8),
		}
	}
	return ramp, nil
}

type ClassicRenderer struct {
	minHeight   float32
	maxHeight   float32
	waterHeight float32
	ramp        [256]color.RGBA
}

func NewClassicRenderer(rampFilePath string) (*ClassicRenderer, error) {
	if len(rampFilePath) == 0 {
		rmp, err := LoadRamp(bytes.NewReader(classicRampFile))
		if err != nil {
			return nil, fmt.Errorf("loading default ramp: %w", err)
		}
		return &ClassicRenderer{ramp: rmp}, nil
	}
	file, err := os.Open(rampFilePath)
	if err != nil {
		return nil, fmt.Errorf("loading ramp file %q: %w", rampFilePath, err)
	}
	rmp, err := LoadRamp(file)
	if err != nil {
		return nil, fmt.Errorf("loading default ramp: %w", err)
	}
	return &ClassicRenderer{ramp: rmp}, nil
}

func (d *ClassicRenderer) GetCellResolution() (x uint32, y uint32) {
	return gridSize, gridSize
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
			img.SetRGBA(x, iy, d.ramp[d.transformHeight(p.heights[y][x])])
		}
	}
	return img
}

func (d *ClassicRenderer) transformHeight(v float32) byte {
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
