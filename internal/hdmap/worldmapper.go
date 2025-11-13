package hdmap

import (
	"context"
	"fmt"
	"image"
	"image/draw"
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

func (w *WorldMapper) Write(ctx context.Context, mapExtents MapCoords, cells <-chan *CellInfo, imageSelector ImageSelector, path string) error {
	w.mapExtents = mapExtents
	w.imageSelector = imageSelector

	out, err := os.Create(path)
	if err != nil {
		return err
	}
	defer out.Close()

	// Handle all cell sub-images
outer:
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case cell, ok := <-cells:
			if !ok {
				// Done handling all cells
				break outer
			}
			if err := w.handleCell(cell); err != nil {
				return fmt.Errorf("handleCell: %w", err)
			}
		}
	}

	return bmp.Encode(out, w.outImage)
}

func (w *WorldMapper) handleCell(cell *CellInfo) error {
	// create the output image, if we haven't yet.
	// we wait to do this until we get our first cell image so we know how big each cell will be.
	if w.outImage == nil {
		mapWidth := 1 + w.mapExtents.Right - w.mapExtents.Left
		mapHeight := 1 + w.mapExtents.Top - w.mapExtents.Bottom
		w.outImage = image.NewRGBA(
			image.Rect(0, 0, int(mapWidth)*int(w.cellWidth), int(mapHeight)*int(w.cellHeight)),
		)
	}

	// Draw the cell image into the world image
	draw.Draw(
		w.outImage,
		image.Rect(
			int(w.cellWidth)*int(cell.X), int(w.cellWidth)*int(cell.Y),
			int(w.cellWidth)*int(cell.X+1), int(w.cellWidth)*int(cell.Y+1),
		),
		w.imageSelector.Select(cell),
		image.Pt(0, 0),
		draw.Src,
	)

	return nil
}
