package hdmap

import (
	"image"
	"image/color"
	"math"
)

const gridSize = 64

var waterNormalHeight = color.RGBA{R: math.MaxUint8 / 2, G: math.MaxUint8 / 2, B: math.MaxUint8, A: 0}

// The vertex grid is 65x65 because it uses one vertex per point on a quad.
// A cell is 8192 units along one dimension, which is 128 game units per quad.
//
// I could make each quad a 2x2 pixel array, which would result in each cell
// being 512x512 in resolution.
// Vvardenfell is about 47x42 cells, resulting in an image that is 24064x21504, or 2 GB.
//
// If I make each just 1 pixel, then the island resolution is 6016x5376, or 129 MB.
// Considering the size of Project Tamriel, I think this is the way to go.
// But how do smash down a quad into just one pixel? Just pick one of them.
// This results in a 64x64 pixel grid.
//
// Also note that the "image" package treats 0,0 as the top-left, so we need to invert Y.
type NormalHeightRenderer struct {
	minHeight   float32
	maxHeight   float32
	waterHeight float32
}

func (d *NormalHeightRenderer) SetHeightExtents(heightStats Stats, waterHeight float32) {
	d.maxHeight = float32(heightStats.Max())
	d.waterHeight = waterHeight

	// Throw away all values that are underwater.
	d.minHeight = max(d.waterHeight, float32(heightStats.Min()))
}

// normalHeightMap generates a *_nh (normal height map) texture for openmw.
// The RGB channels of the normal map are used to store XYZ components of
// tangent space normals and the alpha channel of the normal map may be used
// to store a height map used for parallax.
func (d *NormalHeightRenderer) Render(p *ParsedLandRecord) *image.RGBA {

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
			if p.heights[y][x] >= d.waterHeight {
				img.SetRGBA(x, iy, color.RGBA{
					R: normalTransform(p.normals[y][x].X),
					// Positive Y in the VNML file is toward the north.
					// This gets flipped when ultimately writing to the png
					// so, flip it here.
					// G: normalTransform(p.normals[y][x].Y), // original
					G: normalTransform(-1 * p.normals[y][x].Y),
					B: normalTransform(p.normals[y][x].Z),
					// setting A to 255 results in a correct normal map
					A: d.transformHeight(p.heights[y][x]),
				})
			} else {
				img.SetRGBA(x, iy, waterNormalHeight)
			}
		}
	}
	return img
}

func normalTransform(v int8) uint8 {
	return uint8(v) ^ 0x80
}

func (d *NormalHeightRenderer) transformHeight(v float32) byte {
	if v < d.minHeight {
		return 0
	}
	if v > d.maxHeight {
		return math.MaxUint8
	}
	denom := d.maxHeight - d.minHeight
	if denom == 0 {
		return 0
	}
	normalized := (v - d.minHeight) / denom
	return byte(normalized * math.MaxUint8)
}
