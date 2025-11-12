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
						// fmt.Printf("found vhgt:\n\t%s\n", hex.EncodeToString(vhgt.Data))
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
				} else if len(vhgt.Data) < 100 {
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

func init() {
	heights := make([][]float32, 65)
	for i := range heights {
		heights[i] = make([]float32, 65)
		for b := range 65 {
			heights[i][b] = -128 * 10
		}
	}
}

type parsedLand struct {
	x       int32
	y       int32
	heights [][]float32
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
				//return nil, fmt.Errorf("parse land/vhgt: %q", err)
			} else {
				out.heights = parsed.ComputeAbsoluteHeights()
			}
		}
	}
	return out, nil
}

const gridSize = 65

// normalHeightMap generates a *_nh (normal height map) texture for openmw.
// The RGB channels of the normal map are used to store XYZ components of
// tangent space normals and the alpha channel of the normal map may be used
// to store a height map used for parallax.
func (p *parsedLand) normalHeightMap(heightTransform func(float32) byte) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, gridSize, gridSize))

	for x := range gridSize {
		for y := range gridSize {
			img.SetRGBA(
				x,
				y,
				color.RGBA{
					// TODO: finish normals
					R: heightTransform(p.heights[x][y]),
				})
		}
	}
	return img
}

func transformHeight(v float32, min float32, max float32) byte {
	if v < min {
		return 0
	}
	if v > max {
		return math.MaxUint8
	}

	return byte(((v - min) / max) * math.MaxUint8)
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
	var minHeight, maxHeight float32

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
	fmt.Printf("Top: %d, Bottom: %d, Left: %d, Right: %d, MinHeight: %.1f, MaxHeight: %.1f\n",
		top, bottom, left, right, minHeight, maxHeight,
	)

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
