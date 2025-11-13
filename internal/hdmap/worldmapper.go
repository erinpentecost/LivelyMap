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
	defer func() {
		// Close out the file
		out.Close()
		// Drain remaining cells to let producers finish if we cancelled early.
		for range cells {
		}
	}()

	// Handle all cell sub-images
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case cell, ok := <-cells:
			if !ok {
				fmt.Printf("Writing map to %q\n", path)
				// Channel closed, all done
				return bmp.Encode(out, w.outImage)
			}

			if err := w.handleCell(cell); err != nil {
				return fmt.Errorf("handleCell: %w", err)
			}
		}
	}
}

func (w *WorldMapper) handleCellbroke(cell *CellInfo) error {
	//fmt.Printf("Combining cell %d,%d\n", cell.X, cell.Y)
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

func (w *WorldMapper) handleCellUpsideDown(cell *CellInfo) error {
	if cell == nil {
		return nil
	}

	// Initialize the output image lazily once we know cell size
	if w.outImage == nil {
		sample := w.imageSelector.Select(cell)
		bounds := sample.Bounds()
		w.cellWidth = uint32(bounds.Dx())
		w.cellHeight = uint32(bounds.Dy())

		mapWidth := 1 + w.mapExtents.Right - w.mapExtents.Left
		mapHeight := 1 + w.mapExtents.Bottom - w.mapExtents.Top // bottom > top
		//mapHeight := 1 + w.mapExtents.Top - w.mapExtents.Bottom
		w.outImage = image.NewRGBA(
			image.Rect(0, 0, int(mapWidth)*int(w.cellWidth), int(mapHeight)*int(w.cellHeight)),
		)
	}

	// Normalize coordinates (shift so top-left cell = 0,0)
	px := int(cell.X - w.mapExtents.Left)
	py := int(w.mapExtents.Bottom - cell.Y) // invert Y so north is up
	//py := -1 * int(cell.Y-w.mapExtents.Bottom)

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

func (w *WorldMapper) handleCell(cell *CellInfo) error {
	if cell == nil {
		return nil
	}

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
