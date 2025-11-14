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

type CellRenderer interface {
	// Render turns a ParsedLandRecord into an image.
	Render(p *ParsedLandRecord) *image.RGBA
	SetHeightExtents(heightStats Stats, waterHeight float32)
	GetCellResolution() (x uint32, y uint32)
}

type CellMapper struct {
	LP *LandParser

	Renderer CellRenderer

	mux   sync.Mutex
	Cells []*CellInfo
}

func NewCellMapper(lp *LandParser, renderer CellRenderer) *CellMapper {
	if renderer == nil {
		renderer = &NormalHeightRenderer{}
	}
	return &CellMapper{
		LP:       lp,
		Renderer: renderer,
		Cells:    []*CellInfo{},
	}
}

func (h *CellMapper) Generate(ctx context.Context) ([]*CellInfo, error) {
	h.Renderer.SetHeightExtents(h.LP.Heights, 0)

	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(4)

	for _, parsed := range h.LP.Lands {
		g.Go(func() error {
			//fmt.Printf("Rendering cell %d,%d\n", parsed.x, parsed.y)
			outCell := &CellInfo{
				X:     parsed.x,
				Y:     parsed.y,
				Image: h.Renderer.Render(parsed),
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
	X     int32
	Y     int32
	Image *image.RGBA
}
