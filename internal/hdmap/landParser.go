package hdmap

import (
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"image"
	"math"
	"path/filepath"
	"slices"
	"strings"

	"github.com/dblezek/tga"
	"github.com/ernmw/omwpacker/cfg"
	"github.com/ernmw/omwpacker/esm"

	"github.com/erinpentecost/LivelyMap/internal/dds"
	"github.com/erinpentecost/LivelyMap/internal/tdigest"
	"github.com/ernmw/omwpacker/esm/record/land"
	"github.com/ernmw/omwpacker/esm/record/ltex"
)

var fallbackNormals [][]land.VertexField
var fallbackVtex [][]uint16

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

	fallbackVtex = make([][]uint16, 16)
	for i := range fallbackVtex {
		fallbackVtex[i] = make([]uint16, 16)
		for b := range 16 {
			fallbackVtex[i][b] = math.MaxUint16
		}
	}
}

type Stats interface {
	Min() float64
	Max() float64
	Quantile(q float64) float64
}

type LandParser struct {
	Env          *cfg.Environment
	Heights      *tdigest.TDigest
	MapExtents   MapCoords
	Lands        []*ParsedLandRecord
	LandTextures map[uint16]image.Image
}

type ParsedLandRecord struct {
	x       int32
	y       int32
	heights [][]float32
	normals [][]land.VertexField
	vtex    [][]uint16
}

func NewLandParser(env *cfg.Environment) *LandParser {
	return &LandParser{
		Heights:      tdigest.New(),
		LandTextures: map[uint16]image.Image{},
		Env:          env,
	}
}

func (l *LandParser) readTexture(path string) (image.Image, error) {
	raw, err := l.Env.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read texture: %w", err)
	}
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".tga":
		return tga.Decode(bytes.NewReader(raw))
	case ".dds":
		return dds.Decode(raw)
	default:
		return nil, fmt.Errorf("don't know how to read %q", path)
	}
}

func (l *LandParser) ParsePlugins() error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	present := map[uint64]bool{}

	for rec := range l.loadPlugins(ctx) {
		switch rec.Tag {
		case ltex.LTEX:
			idx, path, err := parseLtex(rec)
			if err != nil {
				return fmt.Errorf("failed to parse LTEX record")
			}
			normalizedPath := strings.ToLower("textures/" + strings.ReplaceAll(path, "\\", "/"))
			if img, err := l.readTexture(normalizedPath); err != nil {
				// Lots of textures are missing; don't fail
				// the whole run because of it.
				err = fmt.Errorf("failed to load texture %q from disk: %w", normalizedPath, err)
				fmt.Printf("%v\n", err)
			} else {
				l.LandTextures[idx] = img
			}
		case land.LAND:
			parsed, err := l.parseLandRecord(rec)
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
					vtex:    fallbackVtex,
				})
			}
		}
	}

	return nil
}

func parseLtex(s *esm.Record) (index uint16, path string, err error) {
	for _, s := range s.Subrecords {
		switch s.Tag {
		case ltex.INTV:
			parsed := ltex.INTVField{}
			err = parsed.Unmarshal(s)
			if err != nil {
				return
			}
			index = uint16(parsed.Value)
		case ltex.DATA:
			parsed := ltex.DATAField{}
			err = parsed.Unmarshal(s)
			if err != nil {
				return
			}
			path = parsed.Value
		}
	}
	return
}

// loadPlugins reads in plugins and returns a filtered set of active records.
// These have been deduped, with overridden records dropped.
func (l *LandParser) loadPlugins(ctx context.Context) <-chan *esm.Record {
	LTEXs := make(map[uint16]*esm.Record)
	LANDs := make(map[string]*esm.Record)
	type pluginsResp struct {
		recs []*esm.Record
		err  error
	}

	pluginsChan := make(chan *pluginsResp, 2)
	go func() {
		defer close(pluginsChan)
		for _, p := range slices.Backward(l.Env.Plugins) {
			fmt.Printf("Parsing %q\n", p)
			records, err := esm.ParsePluginFile(p)
			if err != nil {
				err = fmt.Errorf("parse plugin %q: %w", p, err)
			}
			fmt.Printf("Done parsing %q\n", p)
			pluginsChan <- &pluginsResp{
				recs: records,
				err:  err,
			}
		}
	}()

	out := make(chan *esm.Record, 10)

	go func() {
		defer close(out)
		// iterate through records; later plugins override earlier ones
		for resp := range pluginsChan {
			if resp.err != nil {
				fmt.Printf("error parsing plugin: %v", resp.err)
				continue
			}
			for _, rec := range resp.recs {
				switch rec.Tag {
				case ltex.LTEX:
					idx, _, err := parseLtex(rec)
					if err != nil {
						fmt.Printf("failed to parse LTEX record")
					} else {
						if _, present := LTEXs[idx]; !present {
							LTEXs[idx] = rec
							select {
							case out <- rec:
							case <-ctx.Done():
								return
							}
						}
					}
				case land.LAND:
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
						continue
					} else if len(vhgt.Data) == 0 {
						// bad height data, skip.
						fmt.Printf("skipping LAND %q because VHGT is bad:\n\t%s\n", key, hex.EncodeToString(vhgt.Data))
						continue
					}

					if _, filled := LANDs[key]; filled {
						// alread filled out. skip.
						continue
					}
					LANDs[key] = rec
					select {
					case out <- rec:
					case <-ctx.Done():
						return
					}
				default:
					continue
				}
			}
		}
	}()
	return out
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
		case land.VTEX:
			texes := land.VTEXField{}
			if err := texes.Unmarshal(subrec); err != nil {
				out.vtex = fallbackVtex
			} else {
				out.vtex = texes.Vertices
			}
		}
	}
	return out, nil
}
