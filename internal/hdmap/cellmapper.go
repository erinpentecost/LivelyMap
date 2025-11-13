// Package hdmap creates high-definition global maps.
//
// See also:
// https://github.com/OpenMW/openmw/blob/429305401ee7486f160cb1bbd2196fc80d33dc3a/apps/openmw/mwrender/globalmap.cpp#L137
package hdmap

import (
	"context"
	"fmt"
	"image"
	"sync"

	"golang.org/x/sync/errgroup"
)

type NormalHeightRenderer interface {
	RenderNormalHeightMap(p *ParsedLandRecord) *image.RGBA
	SetHeightExtents(minHeight float32, maxHeight float32, waterHeight float32)
	GetCellResolution() (x uint32, y uint32)
}
type ColorRenderer interface {
	RenderColorMap(p *ParsedLandRecord) *image.RGBA
	GetCellResolution() (x uint32, y uint32)
}

type CellMapper struct {
	LP *LandParser

	NormalHeightHandler NormalHeightRenderer
	ColorHandler        ColorRenderer

	mux   sync.Mutex
	Cells []*CellInfo
}

func NewCellMapper(lp *LandParser, nhr NormalHeightRenderer, ch ColorRenderer) *CellMapper {
	if nhr == nil {
		nhr = &defaultNormalHeightRenderer{}
	}
	return &CellMapper{
		LP:                  lp,
		NormalHeightHandler: nhr,
		Cells:               []*CellInfo{},
	}
}

func (h *CellMapper) Generate(ctx context.Context) ([]*CellInfo, error) {
	h.NormalHeightHandler.SetHeightExtents(h.LP.MinHeight, h.LP.MaxHeight, 0)

	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(4)

	for _, parsed := range h.LP.Lands {
		g.Go(func() error {
			//fmt.Printf("Rendering cell %d,%d\n", parsed.x, parsed.y)
			outCell := &CellInfo{
				X:               parsed.x,
				Y:               parsed.y,
				NormalHeightMap: h.NormalHeightHandler.RenderNormalHeightMap(parsed),
			}
			h.mux.Lock()
			defer h.mux.Unlock()
			h.Cells = append(h.Cells, outCell)
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return nil, fmt.Errorf("render cell: %w", err)
	}
	return h.Cells, nil
}

type CellInfo struct {
	X               int32
	Y               int32
	NormalHeightMap image.Image
	Color           image.Image
}
