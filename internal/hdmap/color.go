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

// ColorRenderer is still broken.
type ColorRenderer struct {
	minHeight   float32
	maxHeight   float32
	waterHeight float32
	// ramp is still used for water and as a fallback
	ramp     *ramp.ColorRamp
	textures map[uint16]*colorSampler
}

func NewColorRenderer(rampFilePath string, textures map[uint16]image.Image) (*ColorRenderer, error) {
	out := &ColorRenderer{}

	// load rampfile
	rmp, err := ramp.LoadRamp(rampFilePath)
	if err != nil {
		return nil, fmt.Errorf("loading default ramp: %w", err)
	}
	out.ramp = rmp

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

func (d *ColorRenderer) GetCellResolution() (x uint32, y uint32) {
	return gridSize, gridSize
}

func (d *ColorRenderer) SetHeightExtents(heightStats Stats, waterHeight float32) {
	d.maxHeight = float32(heightStats.Max())
	d.waterHeight = waterHeight

	// Throw away extreme low values that are underwater.
	// We are raising the "floor" here.
	potentialMin := float32(heightStats.Min())
	if potentialMin < d.waterHeight {
		d.minHeight = min(float32(heightStats.Quantile(0.1)), d.waterHeight)
	}
}

func (d *ColorRenderer) Render(p *ParsedLandRecord) *image.RGBA {
	/* Notes from Qlonever:
	* VTEX 0 indicates the default ground texture, and plugin-defined textures come after
	* though it would be more convenient if LTEX just started at 1
	* the way VTEX correspond to the actual positions of textures in the cell:
	* the first 16 indices in the VTEX correspond to a 4x4 square of textures in the cell, then the next 16 indices make another 4x4 square, etc
	 */
	img := image.NewRGBA(image.Rect(0, 0, gridSize, gridSize))

	// Throw away the last column and row.
	// This is how I'm sampling a quad into a single pixel.
	for y := range gridSize {
		for x := range gridSize {
			// Need to invert y
			iy := gridSize - y - 1
			baseColor := d.ramp.Color(p.heights[y][x], d.minHeight, d.maxHeight, d.waterHeight)
			if p.heights[y][x] >= d.waterHeight {
				gx := x / 4
				gy := y / 4

				dx := 4*(gy%4) + (gx % 4)
				dy := 4*(gy/4) + (gx / 4)

				if len(p.vtex) == 16 && len(p.vtex[dy]) == 16 {
					texIndex := p.vtex[dy][dx]
					tex, ok := d.textures[texIndex]
					if ok {
						// hue and saturation from texture.
						baseHSL := hue.RGBToHSL(baseColor)
						baseHSL.H = tex.avgHSL.H
						baseHSL.S = tex.avgHSL.S
						baseHSL.L = baseHSL.L*.8 + .1
						/*
						* normalizedHeight := (p.heights[y][x] - d.waterHeight) / (d.maxHeight - d.waterHeight)
						* reclampedHeight := float64(1-normalizedHeight)*.3 + .1
						* baseHSL.L = -(math.Cos(math.Pi*reclampedHeight) - 1) / 2
						 */
						baseColor = hue.HSLToRGB(baseHSL)
					}
				}

				// multiply by vertex color
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
