package hdmap

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"math"

	_ "embed"

	"golang.org/x/image/bmp"
)

//go:embed ramp.bmp
var rampFile []byte

// height of 0.000 should be x=128
var ramp [256]color.RGBA

func init() {
	rampImg, err := bmp.Decode(bytes.NewReader(rampFile))
	if err != nil {
		panic(fmt.Errorf("failed to decode color ramp BMP: %w", err))
	}
	b := rampImg.Bounds()
	if b.Dy() != 1 || b.Dx() < 256 {
		panic(fmt.Errorf("invalid color ramp dimensions (expected 1x256, got %dx%d)", b.Dx(), b.Dy()))
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
}

type classicColorRenderer struct {
	minHeight   float32
	maxHeight   float32
	waterHeight float32
}

func (d *classicColorRenderer) GetCellResolution() (x uint32, y uint32) {
	return gridSize, gridSize
}

func (d *classicColorRenderer) SetHeightExtents(minHeight float32, maxHeight float32, waterHeight float32) {
	d.minHeight = minHeight
	d.maxHeight = maxHeight
	d.waterHeight = waterHeight
}

// normalHeightMap generates a *_nh (normal height map) texture for openmw.
// The RGB channels of the normal map are used to store XYZ components of
// tangent space normals and the alpha channel of the normal map may be used
// to store a height map used for parallax.
func (d *classicColorRenderer) RenderNormalHeightMap(p *ParsedLandRecord) *image.RGBA {

	img := image.NewRGBA(image.Rect(0, 0, gridSize, gridSize))

	// Throw away the last column and row.
	// This is how I'm sampling a quad into a single pixel.

	// Normal mapping in wikipedia has this spec:
	// X: -1 to +1 :  Red:     0 to 255
	// Y: -1 to +1 :  Green:   0 to 255
	// Z:  0 to -1 :  Blue:  128 to 255

	for y := range gridSize {
		for x := range gridSize {
			// Need to invert y
			iy := gridSize - y - 1
			img.SetRGBA(x, iy, ramp[d.transformHeight(p.heights[y][x])])
		}
	}
	return img
}

func (d *classicColorRenderer) transformHeight(v float32) int {
	if v < d.minHeight {
		return 0
	}
	if v > d.maxHeight {
		return math.MaxUint8
	}
	if v < 0 {
		normalized := (v - d.minHeight) / (-1 * d.minHeight)
		return int(normalized * math.MaxUint8)
	} else {
		normalized := v / d.maxHeight
		return int(normalized * math.MaxUint8)
	}
}
