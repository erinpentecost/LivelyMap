package hdmap

import (
	"image"
	"image/color"
	"image/draw"
	"testing"
)

// Helper to compare rectangles verbosely
func rectEquals(a, b image.Rectangle) bool {
	return a.Min.X == b.Min.X &&
		a.Min.Y == b.Min.Y &&
		a.Max.X == b.Max.X &&
		a.Max.Y == b.Max.Y
}

func TestSplitIntoCells_InclusiveExtents(t *testing.T) {

	tests := []struct {
		name      string
		ext       MapCoords
		imgWidth  int
		imgHeight int
	}{
		{
			name:      "3x2 grid (inclusive extents 0..2 by 0..1)",
			ext:       MapCoords{Top: 1, Bottom: 0, Left: 0, Right: 2},
			imgWidth:  300,
			imgHeight: 200,
		},
		{
			name:      "4x3 grid (inclusive extents 0..3 by 0..2)",
			ext:       MapCoords{Top: 2, Bottom: 0, Left: 0, Right: 3},
			imgWidth:  400,
			imgHeight: 300,
		},
		{
			name:      "1x1 grid",
			ext:       MapCoords{Top: 0, Bottom: 0, Left: 0, Right: 0},
			imgWidth:  64,
			imgHeight: 64,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			h := NewHeightManifest()

			img := image.NewRGBA(image.Rect(0, 0, tt.imgWidth, tt.imgHeight))
			draw.Draw(img, img.Bounds(), &image.Uniform{color.White},
				image.Point{}, draw.Src)

			cells := h.SplitIntoCells(img, tt.ext)

			// expected count
			numX := int(tt.ext.Right - tt.ext.Left + 1)
			numY := int(tt.ext.Top - tt.ext.Bottom + 1)
			wantCount := numX * numY

			if len(cells) != wantCount {
				t.Fatalf("expected %d cells, got %d", wantCount, len(cells))
			}

			// check rectangles
			cellWidth := tt.imgWidth / numX
			cellHeight := tt.imgHeight / numY

			index := 0
			for y := tt.ext.Top; y >= tt.ext.Bottom; y-- {
				for x := tt.ext.Left; x <= tt.ext.Right; x++ {

					wantRect := image.Rect(
						int(x-tt.ext.Left)*cellWidth,
						int(tt.ext.Top-y)*cellHeight,
						int(x-tt.ext.Left+1)*cellWidth,
						int(tt.ext.Top-y+1)*cellHeight,
					)

					got := cells[index]

					if got.X != x || got.Y != y {
						t.Fatalf("cell[%d] coords = (%d,%d) want (%d,%d)",
							index, got.X, got.Y, x, y)
					}

					if !rectEquals(got.Rect, wantRect) {
						t.Fatalf("cell[%d] rect = %v want %v",
							index, got.Rect, wantRect)
					}

					index++
				}
			}

			// check that all pixels are covered exactly once
			covered := make([]bool, tt.imgWidth*tt.imgHeight)

			for _, c := range cells {
				r := c.Rect
				for py := r.Min.Y; py < r.Max.Y; py++ {
					for px := r.Min.X; px < r.Max.X; px++ {
						i := py*tt.imgWidth + px
						if covered[i] {
							t.Fatalf("pixel (%d,%d) appears in multiple cells", px, py)
						}
						covered[i] = true
					}
				}
			}

			for i, ok := range covered {
				if !ok {
					x := i % tt.imgWidth
					y := i / tt.imgWidth
					t.Fatalf("pixel (%d,%d) not covered by any cell", x, y)
				}
			}
		})
	}
}

func TestRectForCell_Inclusive(t *testing.T) {
	ext := MapCoords{Top: 1, Bottom: 0, Left: 0, Right: 2}
	h := NewHeightManifest()

	numX := int(ext.Right - ext.Left + 1) // 3
	numY := int(ext.Top - ext.Bottom + 1) // 2

	cellWidth := 300 / numX  // 100
	cellHeight := 200 / numY // 100

	tests := []struct {
		x, y int
		want image.Rectangle
	}{
		{0, 1, image.Rect(0, 0, 100, 100)},
		{1, 1, image.Rect(100, 0, 200, 100)},
		{2, 1, image.Rect(200, 0, 300, 100)},

		{0, 0, image.Rect(0, 100, 100, 200)},
		{1, 0, image.Rect(100, 100, 200, 200)},
		{2, 0, image.Rect(200, 100, 300, 200)},
	}

	for _, tt := range tests {
		got := h.RectForCell(tt.x, tt.y, cellWidth, cellHeight, ext)
		if !rectEquals(got, tt.want) {
			t.Fatalf("RectForCell(%d,%d) = %v, want %v", tt.x, tt.y, got, tt.want)
		}
	}
}
