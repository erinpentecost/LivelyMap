package hdmap

import (
	"encoding/hex"
	"fmt"
	"iter"
	"math"
	"slices"

	"github.com/ernmw/omwpacker/esm"

	"github.com/erinpentecost/LivelyMap/internal/tdigest"
	"github.com/ernmw/omwpacker/esm/record/land"
)

var fallbackNormals [][]land.VertexField

func init() {
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

type Stats interface {
	Min() float64
	Max() float64
	Quantile(q float64) float64
}

type LandParser struct {
	Heights    *tdigest.TDigest
	MapExtents MapCoords
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
	return &LandParser{Plugins: plugins, Heights: tdigest.New()}
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

	type pluginsResp struct {
		recs []*esm.Record
		err  error
	}

	pluginsChan := make(chan *pluginsResp, 2)
	go func() {
		defer close(pluginsChan)
		for _, p := range slices.Backward(l.Plugins) {
			fmt.Printf("Parsing %q\n", p)
			records, err := esm.ParsePluginFile(p)
			if err != nil {
				err = fmt.Errorf("parse plugin %q: %w", p, err)
			}
			pluginsChan <- &pluginsResp{
				recs: records,
				err:  err,
			}
		}
	}()

	return func(yield func(*esm.Record, error) bool) {
		// iterate through records; later plugins override earlier ones
		for resp := range pluginsChan {
			if resp.err != nil {
				fmt.Printf("error parsing plugin: %v", resp.err)
				continue
			}
			for _, rec := range resp.recs {
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

	present := map[uint64]bool{}

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
		present[coordKey(parsed.x, parsed.y)] = true
		// calc XY extents
		l.MapExtents.Left = min(l.MapExtents.Left, parsed.x)
		l.MapExtents.Right = max(l.MapExtents.Right, parsed.x)
		l.MapExtents.Top = max(l.MapExtents.Top, parsed.y)
		l.MapExtents.Bottom = min(l.MapExtents.Bottom, parsed.y)
		// calc Z extents
		for x := range parsed.heights {
			for y := range parsed.heights[x] {
				l.Heights.Add(float64(parsed.heights[x][y]), 1)
			}
		}
	}

	// fill in empties
	nearBottom := float32(l.Heights.Quantile(0.1))
	fallbackHeights := make([][]float32, 65)
	for i := range fallbackHeights {
		fallbackHeights[i] = make([]float32, 65)
		for b := range 65 {
			fallbackHeights[i][b] = nearBottom
		}
	}
	fmt.Println("Faking records...")
	for x := l.MapExtents.Left; x <= l.MapExtents.Right; x++ {
		for y := l.MapExtents.Bottom; y <= l.MapExtents.Top; y++ {
			if !present[coordKey(x, y)] {
				l.Lands = append(l.Lands, &ParsedLandRecord{
					x:       x,
					y:       y,
					heights: fallbackHeights,
					normals: fallbackNormals,
				})
			}
		}
	}

	return nil
}

func coordKey(x int32, y int32) uint64 {
	return (uint64(y) << 32) ^ uint64(x)
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
				return nil, fmt.Errorf("bad VHGT entry for %d,%d", out.x, out.y)
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
