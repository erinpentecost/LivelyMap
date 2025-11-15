package hdmap

import (
	"image"
	"testing"
)

func TestHandleCellCopiesRGBA(t *testing.T) {
	// Create a WorldMapper with map extents Left=0,Right=1,Top=1,Bottom=0
	w := &WorldMapper{
		mapExtents: MapCoords{Left: 0, Right: 1, Top: 1, Bottom: 0},
	}

	// Helper for making a 1×1 RGBA tile with arbitrary RGBA values.
	makeTile := func(r, g, b, a uint8) *image.RGBA {
		img := image.NewRGBA(image.Rect(0, 0, 1, 1))
		img.Pix[0] = r
		img.Pix[1] = g
		img.Pix[2] = b
		img.Pix[3] = a
		return img
	}

	// Two-by-two world:
	// coords (x,y):
	// (0,1) (1,1)  <- top row, image y = 0
	// (0,0) (1,0)  <- bottom row, image y = 1

	cells := []*CellInfo{
		{X: 0, Y: 1, Image: makeTile(10, 20, 30, 40)},     // top-left (image x=0,y=0)
		{X: 1, Y: 1, Image: makeTile(50, 60, 70, 80)},     // top-right (image x=1,y=0)
		{X: 0, Y: 0, Image: makeTile(90, 100, 110, 120)},  // bottom-left (image x=0,y=1)
		{X: 1, Y: 0, Image: makeTile(130, 140, 150, 160)}, // bottom-right (image x=1,y=1)
	}

	for _, c := range cells {
		if err := w.handleCell(c); err != nil {
			t.Fatalf("HandleCell error: %v", err)
		}
	}

	out := w.outImage
	if out.Bounds().Dx() != 2 || out.Bounds().Dy() != 2 {
		t.Fatalf("unexpected outImage bounds: %v", out.Bounds())
	}

	at := func(x, y int) [4]uint8 {
		i := y*out.Stride + x*4
		p := out.Pix[i : i+4]
		return [4]uint8{p[0], p[1], p[2], p[3]}
	}

	// Expectation uses Go image coordinates (x rightwards, y downwards).
	// Top row (y=0) should hold Y==1 tiles:
	if got := at(0, 0); got != [4]uint8{10, 20, 30, 40} {
		t.Fatalf("pixel (0,0) = %v; want top-left tile (10,20,30,40)", got)
	}
	if got := at(1, 0); got != [4]uint8{50, 60, 70, 80} {
		t.Fatalf("pixel (1,0) = %v; want top-right tile (50,60,70,80)", got)
	}

	// Bottom row (y=1) should hold Y==0 tiles:
	if got := at(0, 1); got != [4]uint8{90, 100, 110, 120} {
		t.Fatalf("pixel (0,1) = %v; want bottom-left tile (90,100,110,120)", got)
	}
	if got := at(1, 1); got != [4]uint8{130, 140, 150, 160} {
		t.Fatalf("pixel (1,1) = %v; want bottom-right tile (130,140,150,160)", got)
	}
}
