package hdmap

import (
	"context"
	"fmt"
	"image"
	"image/draw"
	"iter"
	"os"

	"golang.org/x/image/bmp"
)

type ImageSelector interface {
	Select(ci *CellInfo) image.Image
}

type NormalHeightImageSelector struct{}

func (s *NormalHeightImageSelector) Select(ci *CellInfo) image.Image { return ci.NormalHeightMap }

type ColorImageSelector struct{}

func (s *ColorImageSelector) Select(ci *CellInfo) image.Image { return ci.Color }

type WorldMapper struct {
	// extents holds map size, in number of cells.
	mapExtents MapCoords

	// cellWidth is the width of one rendered cell in pixels.
	cellWidth uint32
	// cellHeight is the height of one rendered cell in pixels.
	cellHeight uint32

	outImage *image.RGBA

	imageSelector ImageSelector
}

func NewWorldMapper() *WorldMapper {
	return &WorldMapper{}
}

func (w *WorldMapper) Write(ctx context.Context, mapExtents MapCoords, cells iter.Seq[*CellInfo], imageSelector ImageSelector, path string) error {
	w.mapExtents = mapExtents
	w.imageSelector = imageSelector

	for cell := range cells {
		if err := w.handleCell(cell); err != nil {
			return fmt.Errorf("handleCell: %w", err)
		}
	}

	fmt.Printf("Writing map to %q\n", path)
	out, err := os.Create(path)
	if err != nil {
		return err
	}
	defer out.Close()
	return bmp.Encode(out, w.outImage)
}

func (w *WorldMapper) handleCell(cell *CellInfo) error {
	if cell == nil {
		return nil
	}

	// TODO: ignore cells outside of mapExtents

	// Lazily initialize once we know cell size
	if w.outImage == nil {
		sample := w.imageSelector.Select(cell)
		bounds := sample.Bounds()
		w.cellWidth = uint32(bounds.Dx())
		w.cellHeight = uint32(bounds.Dy())

		mapWidth := 1 + w.mapExtents.Right - w.mapExtents.Left
		mapHeight := 1 + w.mapExtents.Top - w.mapExtents.Bottom // Y increases northward

		w.outImage = image.NewRGBA(
			image.Rect(0, 0, int(mapWidth)*int(w.cellWidth), int(mapHeight)*int(w.cellHeight)),
		)
	}

	// Normalize coordinates relative to Bottom/Left
	px := int(cell.X - w.mapExtents.Left)
	//py := int(cell.Y - w.mapExtents.Bottom) // no inversion — Y increases northward
	py := int(w.mapExtents.Top - cell.Y) // flip vertically

	dstRect := image.Rect(
		px*int(w.cellWidth),
		py*int(w.cellHeight),
		(px+1)*int(w.cellWidth),
		(py+1)*int(w.cellHeight),
	)

	src := w.imageSelector.Select(cell)
	draw.Draw(w.outImage, dstRect, src, image.Point{}, draw.Src)

	return nil
}
