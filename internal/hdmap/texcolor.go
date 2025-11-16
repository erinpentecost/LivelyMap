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

type colorSampler struct {
	source image.Image
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

type TexRenderer struct {
	minHeight   float32
	maxHeight   float32
	waterHeight float32
	// ramp is still used for water and as a fallback
	ramp     [256]color.RGBA
	textures map[uint16]*colorSampler
}

func NewTexRenderer(rampFilePath string, textures map[uint16]image.Image) (*TexRenderer, error) {
	out := &TexRenderer{}

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

	// textures
	out.textures = map[uint16]*colorSampler{}
	for idx, img := range textures {
		if idx != math.MaxUint16 {
			sampler := newColorSampler(img)
			if sampler != nil {
				out.textures[idx] = sampler
			}
		}
	}

	return out, nil
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
				ty := iy / 4
				tx := x / 4
				// get color from texture map if available
				if len(p.vtex) < 16 || len(p.vtex[ty]) < 16 {
					//fmt.Printf("Mangled VTEX record for cell %d,%d\n", p.x, p.y)
					img.SetRGBA(x, iy, d.ramp[d.transformHeight(p.heights[y][x])])
					continue
				}
				texIndex := p.vtex[ty][tx]
				tex, ok := d.textures[texIndex]
				if !ok {
					//fmt.Printf("Unknown texture %d in VTEX record for cell %d,%d\n", texIndex, p.x, p.y)
					continue
					//img.SetRGBA(x, iy, d.ramp[d.transformHeight(p.heights[y][x])])
				} else {
					// sample color from tex
					// todo: pick a random color instead
					img.Set(x, iy, tex.Sample(x, iy))
				}
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
