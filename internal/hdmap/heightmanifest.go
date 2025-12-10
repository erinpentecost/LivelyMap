package hdmap

import (
	"context"
	"fmt"
	"image"
	"math"
)

type HeightManifest struct{}

func NewHeightManifest() *HeightManifest {
	return &HeightManifest{}
}

func (h *HeightManifest) GetHeights(
	ctx context.Context,
	mapExtents MapCoords,
	heightMap *image.RGBA,
) (map[string]float32, error) {

	heights := map[string]float32{}
	const minHeight = float32(1) / 255
	for _, cell := range h.SplitIntoCells(heightMap, mapExtents) {
		avgHeight := h.avgPositiveAlpha(heightMap, cell.Rect) / math.MaxUint8
		if avgHeight >= minHeight {
			key := fmt.Sprintf("%d,%d", cell.X, cell.Y)
			heights[key] = avgHeight
		}
	}

	return heights, nil
}

// avgPositiveAlpha computes the average alpha (0–255)
// for all pixels in the rectangle whose alpha > 0.
func (h *HeightManifest) avgPositiveAlpha(img *image.RGBA, rect image.Rectangle) float32 {
	r := rect.Intersect(img.Bounds())
	if r.Empty() {
		return 0
	}

	var sum uint64
	var count uint64

	stride := img.Stride
	pix := img.Pix

	for y := r.Min.Y; y < r.Max.Y; y++ {
		rowStart := y * stride
		for x := r.Min.X; x < r.Max.X; x++ {
			i := rowStart + x*4
			a := pix[i+3]
			if a > 0 {
				sum += uint64(a)
				count++
			}
		}
	}

	if count == 0 {
		return 0
	}

	return float32(sum) / float32(count)
}

type cellRect struct {
	X, Y int32
	Rect image.Rectangle
}

// SplitIntoCells respects inclusive extents.
// X loops: Left..Right   (inclusive)
// Y loops: Top..Bottom   (inclusive, downward)
func (h *HeightManifest) SplitIntoCells(heightMap *image.RGBA, ext MapCoords) []*cellRect {
	cells := []*cellRect{}

	numX := int(ext.Right - ext.Left + 1)
	numY := int(ext.Top - ext.Bottom + 1)

	cellWidth := heightMap.Bounds().Dx() / numX
	cellHeight := heightMap.Bounds().Dy() / numY

	for y := ext.Top; y >= ext.Bottom; y-- { // inclusive
		for x := ext.Left; x <= ext.Right; x++ { // inclusive

			r := h.RectForCell(int(x), int(y), cellWidth, cellHeight, ext)

			cells = append(cells, &cellRect{
				X:    x,
				Y:    y,
				Rect: r,
			})
		}
	}

	return cells
}

// RectForCell computes the rectangle of a given world cell coordinate,
// correctly handling inclusive map ranges.
func (h *HeightManifest) RectForCell(
	x, y, cellWidth, cellHeight int,
	ext MapCoords,
) image.Rectangle {

	gx := x - int(ext.Left)
	gy := int(ext.Top) - y // invert Y → world Y up, image Y down

	minX := gx * cellWidth
	minY := gy * cellHeight

	return image.Rect(
		minX,
		minY,
		minX+cellWidth,
		minY+cellHeight,
	)
}
