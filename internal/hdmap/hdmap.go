// Package hdmap creates high-definition global maps.
//
// See also:
// https://github.com/OpenMW/openmw/blob/429305401ee7486f160cb1bbd2196fc80d33dc3a/apps/openmw/mwrender/globalmap.cpp#L137
package hdmap

import (
	"encoding/hex"
	"fmt"
	"image"
	"image/color"
	"iter"
	"math"
	"slices"

	"github.com/ernmw/omwpacker/esm"
	"github.com/ernmw/omwpacker/esm/record/land"
)

// go run . read  "/home/ern/tes3/Morrowind/Data Files/Morrowind.esm" -r LAND -s VHGT

func LoadLANDs(plugins []string) iter.Seq2[*esm.Record, error] {
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

type parsedLand struct {
	x       int32
	y       int32
	heights [][]float32
	normals [][]land.VertexField
}

func parseLAND(rec *esm.Record) (*parsedLand, error) {
	out := &parsedLand{}
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

// The vertex grid is 65x65 because it uses one vertex per point on a quad.
// A cell is 8192 units along one dimension, which is 128 game units per quad.
//
// I could make each quad a 2x2 pixel array, which would result in each cell
// being 512x512 in resolution.
// Vvardenfell is about 47x42 cells, resulting in an image that is 24064x21504, or 2 GB.
//
// If I make each just 1 pixel, then the island resolution is 6016x5376, or 129 MB.
// Considering the size of Project Tamriel, I think this is the way to go.
// But how do smash down a quad into just one pixel? Just pick one of them.
// This results in a 64x64 pixel grid.
const gridSize = 64

// normalHeightMap generates a *_nh (normal height map) texture for openmw.
// The RGB channels of the normal map are used to store XYZ components of
// tangent space normals and the alpha channel of the normal map may be used
// to store a height map used for parallax.
func (p *parsedLand) normalHeightMap(heightTransform func(float32) byte) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, gridSize, gridSize))

	// Throw away the last column and row.
	// This is how I'm sampling a quad into a single pixel.

	// Normal mapping in wikipedia has this spec:
	// X: -1 to +1 :  Red:     0 to 255
	// Y: -1 to +1 :  Green:   0 to 255
	// Z:  0 to -1 :  Blue:  128 to 255

	for y := range gridSize {
		for x := range gridSize {
			img.SetRGBA(x, y, color.RGBA{
				R: normalTransform(p.normals[y][x].X),
				G: normalTransform(p.normals[y][x].Y),
				B: normalTransform(p.normals[y][x].Z),
				A: heightTransform(p.heights[y][x]),
			})
		}
	}
	return img
}

func normalTransform(v int8) uint8 {
	return uint8(v) ^ 0x80
}

func transformHeight(v float32, min float32, max float32) byte {
	if v < min {
		return 0
	}
	if v > max {
		return math.MaxUint8
	}
	denom := max - min
	if denom == 0 {
		return 0
	}
	normalized := (v - min) / denom
	return byte(normalized * math.MaxUint8)
}

type CellInfo struct {
	X               int32
	Y               int32
	NormalHeightMap image.Image
	Color           image.Image
}

func RecordsToCellInfo(recs iter.Seq2[*esm.Record, error]) (iter.Seq[*CellInfo], error) {

	// Do first pass on records to find extents.

	var left, right, top, bottom int32
	minHeight := float32(math.Inf(1))
	maxHeight := float32(math.Inf(-1))

	parsedLANDs := []*parsedLand{}
	for lnd, err := range recs {
		if err != nil {
			return nil, fmt.Errorf("range over records iter: %w", err)
		}
		if lnd.Tag != land.LAND {
			continue
		}

		parsed, err := parseLAND(lnd)
		if err != nil {
			return nil, fmt.Errorf("parse land record: %w", err)
		}
		parsedLANDs = append(parsedLANDs, parsed)
		// calc XY extents
		left = min(left, parsed.x)
		right = max(right, parsed.x)
		top = min(top, parsed.y)
		bottom = max(bottom, parsed.y)
		// calc Z extents
		for x := range parsed.heights {
			for y := range parsed.heights[x] {
				minHeight = min(minHeight, parsed.heights[x][y])
				maxHeight = max(maxHeight, parsed.heights[x][y])
			}
		}
	}

	heightTransform := func(v float32) byte {
		return transformHeight(v, minHeight, maxHeight)
	}

	// Generate images for each cell
	return func(yield func(*CellInfo) bool) {
		for _, parsed := range parsedLANDs {
			out := &CellInfo{
				X:               parsed.x,
				Y:               parsed.y,
				NormalHeightMap: parsed.normalHeightMap(heightTransform),
			}
			if !yield(out) {
				return
			}
		}
	}, nil

}
