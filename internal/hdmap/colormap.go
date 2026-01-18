package hdmap

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"math"
	"slices"

	"github.com/erinpentecost/LivelyMap/internal/hdmap/ramp"
	"github.com/erinpentecost/LivelyMap/internal/hue"
	"github.com/erinpentecost/LivelyMap/internal/imgutil"

	_ "embed"
)

// VecTexLayerRenderer is an image of just landscape textures.
// Water is fully white and transparent.
type VecTexLayerRenderer struct {
	minHeight   float32
	maxHeight   float32
	waterHeight float32
	// ramp is still used for water and as a fallback
	ramp     *ramp.ColorRamp
	textures map[uint16]*colorSampler
}

func NewVecTexLayerRenderer(rampFilePath string, textures map[uint16]image.Image) (*VecTexLayerRenderer, error) {
	out := &VecTexLayerRenderer{}

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

func (d *VecTexLayerRenderer) GetCellResolution() (x uint32, y uint32) {
	return gridSize, gridSize
}

func (d *VecTexLayerRenderer) SetHeightExtents(heightStats Stats, waterHeight float32) {
	d.maxHeight = float32(heightStats.Max())
	d.waterHeight = waterHeight

	// Throw away extreme low values that are underwater.
	// We are raising the "floor" here.
	potentialMin := float32(heightStats.Min())
	if potentialMin < d.waterHeight {
		d.minHeight = min(float32(heightStats.Quantile(0.1)), d.waterHeight)
	}
}

func (d *VecTexLayerRenderer) Render(p *ParsedLandRecord) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, gridSize, gridSize))

	waterColor := color.RGBA{
		R: 255,
		G: 255,
		B: 255,
		A: 0,
	}

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
				img.SetRGBA(x, iy, baseColor)
			} else {
				img.SetRGBA(x, iy, waterColor)
			}
		}
	}
	return img
}

// VecDetailLayerRenderer is an image with fully rendered water, and land vertex colors on white.
type VecDetailLayerRenderer struct {
	minHeight   float32
	maxHeight   float32
	waterHeight float32
	// ramp is still used for water and as a fallback
	ramp     *ramp.ColorRamp
	textures map[uint16]*colorSampler
}

func NewVecDetailLayerRenderer(rampFilePath string, textures map[uint16]image.Image) (*VecDetailLayerRenderer, error) {
	out := &VecDetailLayerRenderer{}

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

func (d *VecDetailLayerRenderer) GetCellResolution() (x uint32, y uint32) {
	return gridSize, gridSize
}

func (d *VecDetailLayerRenderer) SetHeightExtents(heightStats Stats, waterHeight float32) {
	d.maxHeight = float32(heightStats.Max())
	d.waterHeight = waterHeight

	// Throw away extreme low values that are underwater.
	// We are raising the "floor" here.
	potentialMin := float32(heightStats.Min())
	if potentialMin < d.waterHeight {
		d.minHeight = min(float32(heightStats.Quantile(0.1)), d.waterHeight)
	}
}

func (d *VecDetailLayerRenderer) Render(p *ParsedLandRecord) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, gridSize, gridSize))

	for y := range gridSize {
		for x := range gridSize {
			// Need to invert y
			iy := gridSize - y - 1
			baseColor := d.ramp.Color(p.heights[y][x], d.minHeight, d.maxHeight, d.waterHeight)
			if p.heights[y][x] >= d.waterHeight {
				// Overwrite land color
				if len(p.colors) == 65 && len(p.colors[y]) == 65 {
					// just get raw vertex color
					baseColor = color.RGBA{
						R: p.colors[y][x].R,
						G: p.colors[y][x].G,
						B: p.colors[y][x].B,
						A: math.MaxUint8,
					}
				}
			}

			img.SetRGBA(x, iy, baseColor)
		}
	}
	return img
}

type ColorMapGenerator struct {
	colorCells  *CellMapper
	detailCells *CellMapper
}

func NewColorMapGenerator(ctx context.Context, rampPath string, parsedLands *LandParser) (*ColorMapGenerator, error) {
	// Build up flat color image
	colorRenderer, err := NewVecTexLayerRenderer(rampPath, parsedLands.LandTextures)
	if err != nil {
		return nil, fmt.Errorf("new color renderer: %w", err)
	}
	colorCells := NewCellMapper(parsedLands, colorRenderer)
	if err := colorCells.Generate(ctx); err != nil {
		return nil, fmt.Errorf("generate cell maps: %w", err)
	}
	// Buld up detail layer
	detailRenderer, err := NewVecDetailLayerRenderer(rampPath, parsedLands.LandTextures)
	if err != nil {
		return nil, fmt.Errorf("new color renderer: %w", err)
	}
	detailCells := NewCellMapper(parsedLands, detailRenderer)
	if err := detailCells.Generate(ctx); err != nil {
		return nil, fmt.Errorf("generate cell maps: %w", err)
	}
	return &ColorMapGenerator{
		colorCells:  colorCells,
		detailCells: detailCells,
	}, nil
}

func (cmg *ColorMapGenerator) ProcessColorMapper(ctx context.Context, extents SubmapNode, postProcs []PostProcessor) (*WorldMapper, error) {
	colorWorldMapper := NewWorldMapper()
	err := colorWorldMapper.Process(ctx,
		extents.Extents,
		slices.Values(cmg.colorCells.Cells),
		postProcs,
	)
	if err != nil {
		return nil, fmt.Errorf("process world map %s %d: %w", extents.Extents, extents.ID, err)
	}
	colorWorldMapper.outImage = imgutil.BlurRGBIgnoreTransparent(colorWorldMapper.outImage, 8, 3)

	detailWorldMapper := NewWorldMapper()
	err = detailWorldMapper.Process(ctx,
		extents.Extents,
		slices.Values(cmg.detailCells.Cells),
		postProcs,
	)
	if err != nil {
		return nil, fmt.Errorf("process world map %s %d: %w", extents.Extents, extents.ID, err)
	}

	// Multiply them together
	//detailWorldMapper.outImage = blend.Multiply(colorWorldMapper.outImage, detailWorldMapper.outImage)
	detailWorldMapper.outImage = imgutil.Blend(detailWorldMapper.outImage, colorWorldMapper.outImage, hue.MulColor)

	return detailWorldMapper, nil
}
