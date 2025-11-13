package hdmap

import (
	"encoding/hex"
	"fmt"
	"iter"
	"math"
	"slices"

	"github.com/ernmw/omwpacker/esm"
	"github.com/ernmw/omwpacker/esm/record/land"
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

type LandParser struct {
	MapExtents MapCoords
	MinHeight  float32
	MaxHeight  float32
	Plugins    []string
	Lands      []*ParsedLandRecord
}

type ParsedLandRecord struct {
	x       int32
	y       int32
	heights [][]float32
	normals [][]land.VertexField
}

func NewLandParser(plugins []string) *LandParser {
	return &LandParser{Plugins: plugins}
}

func (l *LandParser) ParsePlugins() error {
	records := l.loadPlugins()
	if err := l.recordsToCellInfo(records); err != nil {
		return fmt.Errorf("records to cell info: %w", err)
	}
	return nil
}

func (l *LandParser) loadPlugins() iter.Seq2[*esm.Record, error] {
	LANDs := make(map[string]*esm.Record)

	return func(yield func(*esm.Record, error) bool) {

		for _, p := range slices.Backward(l.Plugins) {
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
				var intv *esm.Subrecord
				var vhgt *esm.Subrecord
				for _, s := range rec.Subrecords {
					if s.Tag == land.INTV {
						intv = s
					} else if s.Tag == land.VHGT && s != nil {
						vhgt = s
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

func (l *LandParser) recordsToCellInfo(recs iter.Seq2[*esm.Record, error]) error {
	// Do first pass on records to find extents.
	l.MinHeight = float32(math.Inf(1))
	l.MaxHeight = float32(math.Inf(-1))

	for lnd, err := range recs {
		if err != nil {
			return fmt.Errorf("range over records iter: %w", err)
		}
		if lnd.Tag != land.LAND {
			continue
		}

		parsed, err := l.parseLandRecord(lnd)
		if err != nil {
			return fmt.Errorf("parse land record: %w", err)
		}
		l.Lands = append(l.Lands, parsed)
		// calc XY extents
		l.MapExtents.Left = min(l.MapExtents.Left, parsed.x)
		l.MapExtents.Right = max(l.MapExtents.Right, parsed.x)
		l.MapExtents.Top = max(l.MapExtents.Top, parsed.y)
		l.MapExtents.Bottom = min(l.MapExtents.Bottom, parsed.y)
		// calc Z extents
		for x := range parsed.heights {
			for y := range parsed.heights[x] {
				l.MinHeight = min(l.MinHeight, parsed.heights[x][y])
				l.MaxHeight = max(l.MaxHeight, parsed.heights[x][y])
			}
		}
	}
	return nil
}

func (l *LandParser) parseLandRecord(rec *esm.Record) (*ParsedLandRecord, error) {
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
