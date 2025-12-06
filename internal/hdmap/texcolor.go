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

type TexRenderer struct {
	minHeight   float32
	maxHeight   float32
	waterHeight float32
	// ramp is still used for water and as a fallback
	ramp     *ramp.ColorRamp
	textures map[uint16]*colorSampler
}

func NewTexRenderer(rampFilePath string, textures map[uint16]image.Image) (*TexRenderer, error) {
	out := &TexRenderer{}

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
			baseColor := d.ramp.Color(p.heights[y][x], d.minHeight, d.maxHeight, d.waterHeight)
			if p.heights[y][x] >= d.waterHeight {
				// set the hue from the vertex color
				if len(p.colors) == 65 && len(p.colors[y]) == 65 {
					baseColor = hue.MulColor(baseColor, color.RGBA{
						R: p.colors[y][x].R,
						G: p.colors[y][x].G,
						B: p.colors[y][x].B,
						A: math.MaxUint8,
					})
					/*
						texIndex := p.vtex[ty][tx]
						tex, ok := d.textures[texIndex]
						if ok {
							baseHSL := hue.RGBToHSL(baseColor)
							baseHSL.H = tex.avgHue
							baseColor = hue.HSLToRGB(baseHSL)
						}*/
				}
			}

			img.SetRGBA(x, iy, baseColor)
		}
	}
	return img
}

func (d *TexRenderer) renderHueFromTex(p *ParsedLandRecord) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, gridSize, gridSize))

	// Throw away the last column and row.
	// This is how I'm sampling a quad into a single pixel.
	for y := range gridSize {
		for x := range gridSize {
			// Need to invert y
			iy := gridSize - y - 1
			baseColor := d.ramp.Color(p.heights[y][x], d.minHeight, d.maxHeight, d.waterHeight)
			if p.heights[y][x] >= d.waterHeight {
				// set the hue from the texture
				ty := iy / 4
				tx := x / 4
				// get color from texture map if available
				if len(p.vtex) == 16 && len(p.vtex[ty]) == 16 {
					texIndex := p.vtex[ty][tx]
					tex, ok := d.textures[texIndex]
					if ok {
						baseHSL := hue.RGBToHSL(baseColor)
						baseHSL.H = tex.avgHue
						baseColor = hue.HSLToRGB(baseHSL)
					}
				}
			}

			img.SetRGBA(x, iy, baseColor)
		}
	}
	return img
}
