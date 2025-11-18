package hdmap

import (
	"context"
	"fmt"
	"image"
	"image/png"
	"iter"
	"os"
	"path/filepath"
	"strings"

	"github.com/erinpentecost/LivelyMap/internal/dds"
	"golang.org/x/image/draw"
)

type WorldMapper struct {
	// extents holds map size, in number of cells.
	mapExtents MapCoords

	// cellWidth is the width of one rendered cell in pixels.
	cellWidth uint32
	// cellHeight is the height of one rendered cell in pixels.
	cellHeight uint32

	outImage *image.RGBA
}

func NewWorldMapper() *WorldMapper {
	return &WorldMapper{}
}

func (w *WorldMapper) Write(
	ctx context.Context,
	mapExtents MapCoords,
	cells iter.Seq[*CellInfo],
	path string,
	downScaleFactor int,
) error {
	w.outImage = nil
	w.mapExtents = mapExtents
	fmt.Printf("Map extents: %s\n", mapExtents)

	if downScaleFactor < 1 {
		return fmt.Errorf("downScaleFactor must be at least 1.")
	}

	if w.mapExtents.Bottom > w.mapExtents.Top || w.mapExtents.Left > w.mapExtents.Right {
		return fmt.Errorf("invalid extents: %s", w.mapExtents)
	}

	for cell := range cells {
		if err := w.handleCell(cell); err != nil {
			return fmt.Errorf("handleCell: %w", err)
		}
	}

	if downScaleFactor > 1 {
		fmt.Printf("Scaling down image...")
		sourceBounds := w.outImage.Bounds()
		downSize := image.NewRGBA(image.Rect(0, 0, sourceBounds.Dx()/2, sourceBounds.Dy()/2))
		draw.CatmullRom.Scale(downSize, downSize.Bounds(), w.outImage, w.outImage.Bounds(), draw.Over, nil)
		w.outImage = downSize
	}

	fmt.Printf("Writing map to %q\n", path)
	out, err := os.Create(path)
	if err != nil {
		return err
	}
	defer out.Close()
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".dds":
		return dds.Encode(out, w.outImage)
	case ".png":
		return png.Encode(out, w.outImage)
	default:
		return fmt.Errorf("bad extension %q", ext)
	}

}

// handleCell copies cell.Image (must be *image.RGBA) into w.outImage.
// It does a raw byte copy row-by-row (preserves alpha exactly) and
// performs no heap allocations per blit.
func (w *WorldMapper) handleCell(cell *CellInfo) error {
	if cell == nil {
		return nil
	}
	src := cell.Image
	if src == nil {
		return fmt.Errorf("cell.Image is nil")
	}

	// Lazily initialize once we know the cell tile size
	if w.outImage == nil {
		b := src.Bounds()
		w.cellWidth = uint32(b.Dx())
		w.cellHeight = uint32(b.Dy())

		mapWidth := 1 + w.mapExtents.Right - w.mapExtents.Left
		mapHeight := 1 + w.mapExtents.Top - w.mapExtents.Bottom

		w.outImage = image.NewRGBA(
			image.Rect(
				0,
				0,
				int(mapWidth)*int(w.cellWidth),
				int(mapHeight)*int(w.cellHeight),
			),
		)
	}

	if w.mapExtents.NotContainsPoint(cell.X, cell.Y) {
		return nil
	}

	// Compute tile destination pixel coordinates (in pixel units)
	px := int(cell.X - w.mapExtents.Left)
	py := int(w.mapExtents.Top - cell.Y) // flip world Y -> image Y (top-left image origin)

	dstX0 := px * int(w.cellWidth)
	dstY0 := py * int(w.cellHeight)

	// fast refs
	dst := w.outImage
	srcPix := src.Pix
	dstPix := dst.Pix
	srcStride := src.Stride
	dstStride := dst.Stride

	// Account for possible non-zero src.Bounds().Min
	srcMin := src.Bounds().Min
	tw := src.Bounds().Dx()
	th := src.Bounds().Dy()

	// For each row of the source, copy the row bytes into the destination.
	// Each pixel is 4 bytes (RGBA).
	for row := range th {
		// source index for the beginning of this row
		si := (row+srcMin.Y-srcMin.Y)*srcStride + (0+srcMin.X-srcMin.X)*4
		// destination index: (dstY0 + row) * dstStride + dstX0*4
		di := (dstY0+row)*dstStride + dstX0*4

		// boundaries safety (shouldn't be needed if inputs correct)
		if di < 0 || di+tw*4 > len(dstPix) || si < 0 || si+tw*4 > len(srcPix) {
			return fmt.Errorf("cell (%d,%d) blit out of bounds (di=%d si=%d tw=%d th=%d dstLen=%d srcLen=%d)",
				cell.X, cell.Y,
				di, si, tw, th, len(dstPix), len(srcPix))
		}
		copy(dstPix[di:di+tw*4], srcPix[si:si+tw*4])
	}

	return nil
}
