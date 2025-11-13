// Package hdmap creates high-definition global maps.
//
// See also:
// https://github.com/OpenMW/openmw/blob/429305401ee7486f160cb1bbd2196fc80d33dc3a/apps/openmw/mwrender/globalmap.cpp#L137
package hdmap

import (
	"context"
	"encoding/hex"
	"fmt"
	"image"
	"iter"
	"math"
	"slices"

	"github.com/ernmw/omwpacker/esm"
	"github.com/ernmw/omwpacker/esm/record/land"
	"golang.org/x/sync/errgroup"
)

var fallbackHeights [][]float32
var fallbackNormals [][]land.VertexField

func init() {
	fallbackHeights = make([][]float32, 65)
	for i := range fallbackHeights {
		fallbackHeights[i] = make([]float32, 65)
		for b := range 65 {
			fallbackHeights[i][b] = -128 * 10
		}
	}

	fallbackNormals = make([][]land.VertexField, 65)
	for i := range fallbackNormals {
		fallbackNormals[i] = make([]land.VertexField, 65)
		for b := range 65 {
			fallbackNormals[i][b] = land.VertexField{
				X: 0,
				Y: math.MaxInt8,
				Z: 0,
			}
		}
	}
}

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
	Plugins             []string
	MapExtents          MapCoords
	NormalHeightHandler NormalHeightRenderer
	ColorHandler        ColorRenderer
}

func NewCellMapper(plugins []string, nhr NormalHeightRenderer, ch ColorRenderer) *CellMapper {
	if nhr == nil {
		nhr = &defaultNormalHeightRenderer{}
	}
	return &CellMapper{
		Plugins:             plugins,
		NormalHeightHandler: nhr,
	}
}

func (h *CellMapper) Generate(ctx context.Context) (<-chan *CellInfo, error) {
	return h.recordsToCellInfo(ctx, h.loadLANDs(h.Plugins))
}

func (h *CellMapper) loadLANDs(plugins []string) iter.Seq2[*esm.Record, error] {
	LANDs := make(map[string]*esm.Record)

	return func(yield func(*esm.Record, error) bool) {

		for _, p := range slices.Backward(plugins) {
			fmt.Printf("Parsing %q\n", p)
			records, err := esm.ParsePluginFile(p)
			if err != nil {
				if !yield(nil, fmt.Errorf("parse plugin %q: %w", p, err)) {
					return
				}
			}
			// iterate through records; later plugins override earlier ones
			for _, rec := range records {
				// Only interested in LAND records
				if rec.Tag != land.LAND {
					continue
				}
				fmt.Printf("\n")
				var intv *esm.Subrecord
				var vhgt *esm.Subrecord
				for _, s := range rec.Subrecords {
					fmt.Printf("%s: len=%d bytes\n", s.Tag, len(s.Data))
					if s.Tag == land.INTV {
						intv = s
					} else if s.Tag == land.VHGT && s != nil {
						vhgt = s
						fmt.Printf("found vhgt: first 32 bytes:\n\t%s\n", hex.EncodeToString(vhgt.Data[:min(32, len(vhgt.Data))]))
					}
				}
				if intv == nil || len(intv.Data) < 8 {
					// no coordinates — skip this LAND record
					fmt.Printf("skipping LAND because INTV is bad\n")
					continue
				}
				key := string(intv.Data)

				if vhgt == nil {
					// no texture height data, skip.
					fmt.Printf("skipping LAND %q because VHGT is empty\n", key)
					continue
				} else if len(vhgt.Data) == 0 {
					// bad height data, skip.
					fmt.Printf("skipping LAND %q because VHGT is bad:\n\t%s\n", key, hex.EncodeToString(vhgt.Data))
					continue
				}

				if _, filled := LANDs[key]; filled {
					// alread filled out. skip.
					fmt.Printf("skipping LAND %q because it was already seen\n", key)
					continue
				}
				LANDs[key] = rec
				if !yield(rec, nil) {
					return
				}
			}
		}
	}
}

type ParsedLandRecord struct {
	x       int32
	y       int32
	heights [][]float32
	normals [][]land.VertexField
}

func (h *CellMapper) parseLAND(rec *esm.Record) (*ParsedLandRecord, error) {
	out := &ParsedLandRecord{}
	for _, subrec := range rec.Subrecords {
		switch subrec.Tag {
		case land.INTV:
			parsed := land.INTVField{}
			if err := parsed.Unmarshal(subrec); err != nil {
				return nil, fmt.Errorf("parse land/intv: %q", err)
			}
			out.x = parsed.X
			out.y = parsed.Y
		case land.VHGT:
			parsed := land.VHGTField{}
			if err := parsed.Unmarshal(subrec); err != nil {
				out.heights = fallbackHeights
			} else {
				out.heights = parsed.ComputeAbsoluteHeights()
			}
		case land.VNML:
			normals := land.VNMLField{}
			if err := normals.Unmarshal(subrec); err != nil {
				out.normals = fallbackNormals
			} else {
				out.normals = normals.Vertices
			}
		}
	}
	return out, nil
}

type CellInfo struct {
	X               int32
	Y               int32
	NormalHeightMap image.Image
	Color           image.Image
}

func (h *CellMapper) recordsToCellInfo(ctx context.Context, recs iter.Seq2[*esm.Record, error]) (<-chan *CellInfo, error) {

	// Do first pass on records to find extents.
	minHeight := float32(math.Inf(1))
	maxHeight := float32(math.Inf(-1))

	parsedLANDs := []*ParsedLandRecord{}
	for lnd, err := range recs {
		if err != nil {
			return nil, fmt.Errorf("range over records iter: %w", err)
		}
		if lnd.Tag != land.LAND {
			continue
		}

		parsed, err := h.parseLAND(lnd)
		if err != nil {
			return nil, fmt.Errorf("parse land record: %w", err)
		}
		parsedLANDs = append(parsedLANDs, parsed)
		// calc XY extents
		h.MapExtents.Left = min(h.MapExtents.Left, parsed.x)
		h.MapExtents.Right = max(h.MapExtents.Right, parsed.x)
		h.MapExtents.Top = min(h.MapExtents.Top, parsed.y)
		h.MapExtents.Bottom = max(h.MapExtents.Bottom, parsed.y)
		// calc Z extents
		for x := range parsed.heights {
			for y := range parsed.heights[x] {
				minHeight = min(minHeight, parsed.heights[x][y])
				maxHeight = max(maxHeight, parsed.heights[x][y])
			}
		}
	}

	// todo: use real value for water height
	h.NormalHeightHandler.SetHeightExtents(minHeight, maxHeight, minHeight/2)

	// Fork out image rendering
	out := make(chan *CellInfo)
	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(4)

	for _, parsed := range parsedLANDs {
		g.Go(func() error {
			outCell := &CellInfo{
				X:               parsed.x,
				Y:               parsed.y,
				NormalHeightMap: h.NormalHeightHandler.RenderNormalHeightMap(parsed),
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			case out <- outCell:
				return nil
			}
		})
	}

	// make sure we close the channel once errgroup resolves.
	go func() {
		g.Wait()
		close(out)
	}()

	return out, nil
}
