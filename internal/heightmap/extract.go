package heightmap

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"image"
	"image/color"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	_ "embed"

	"golang.org/x/image/bmp"

	"github.com/ernmw/omwpacker/esm"
)

//go:embed ramp.bmp
var rampFile []byte

var defaultWNAM []byte

func init() {
	// default WNAM bytes: 81 bytes of signed -128 -> byte 0x80
	defaultWNAM = make([]byte, 81)
	for i := 0; i < 81; i++ {
		defaultWNAM[i] = 0x80
	}
}

func padLength(length, pad int) int {
	return ((length + pad - 1) / pad) * pad
}

// --- PixelArray ---

type PixelArray struct {
	Value    []byte
	Width    int
	Height   int
	PadWidth int
	Size     int
}

func NewPixelArrayFromBytes(b []byte, width, height, padWidth int) *PixelArray {
	return &PixelArray{
		Value:    b,
		Width:    width,
		Height:   height,
		PadWidth: padWidth,
		Size:     len(b),
	}
}

func (p *PixelArray) getRow(x, y, length int) []byte {
	if length == 0 {
		x = 0
		length = p.Width
	}
	baseRow := y * p.PadWidth
	baseColumn := baseRow + x
	return p.Value[baseColumn : baseColumn+length]
}

func (p *PixelArray) setRow(x, y int, b []byte) {
	if x >= p.Width {
		return
	}
	if len(b) > p.Width-x {
		b = b[:p.Width-x]
	}
	baseRow := y * p.PadWidth
	baseColumn := baseRow + x
	copy(p.Value[baseColumn:baseColumn+len(b)], b)
}

func (p *PixelArray) impose(pixelArray *PixelArray, x, y int) {
	for h := 0; h < pixelArray.Height; h++ {
		row := pixelArray.getRow(0, h, 0)
		p.setRow(x, y+h, row)
	}
}

// --- plugin file parsing helpers using esm.ParsePluginFile ---

// WNAMsFromPlugins accepts ordered plugins and returns a map of coords -> WNAM,
// using later plugins to override earlier ones.
func WNAMsFromPlugins(plugins []string) (map[string]*esm.Subrecord, error) {
	WNAMs := make(map[string]*esm.Subrecord)

	for _, p := range plugins {
		records, err := esm.ParsePluginFile(p)
		if err != nil {
			return nil, fmt.Errorf("parse plugin %q: %w", p, err)
		}
		// iterate through records; later plugins override earlier ones
		for _, rec := range records {
			// Only interested in LAND records
			if string(rec.Tag) != "LAND" {
				continue
			}
			// Find INTV subrecord to compute coords
			var intv *esm.Subrecord
			var wnam *esm.Subrecord
			for _, s := range rec.Subrecords {
				if string(s.Tag) == "INTV" {
					intv = s
				}
				if string(s.Tag) == "WNAM" {
					wnam = s
				}
			}
			if intv == nil || len(intv.Data) < 8 {
				// no coordinates — skip this LAND record
				continue
			}
			x := int32(binary.LittleEndian.Uint32(intv.Data[0:4]))
			y := int32(binary.LittleEndian.Uint32(intv.Data[4:8]))
			key := fmt.Sprintf("%d,%d", x, y)

			// If WNAM missing or too short, provide default WNAM bytes
			if wnam == nil || len(wnam.Data) < 81 {
				tmp := make([]byte, len(defaultWNAM))
				copy(tmp, defaultWNAM)
				wnam = &esm.Subrecord{Tag: esm.SubrecordTag("WNAM"), Data: tmp}
			}
			WNAMs[key] = wnam
		}
	}

	return WNAMs, nil
}

// BMPFromPixelArray writes a BMP from a PixelArray.
// If rampPath is provided, it uses the 1×256 BMP there as a color ramp.
// If not, it uses the original grayscale mapping (b → (b+128)&0xFF, lowest=black).
func BMPFromPixelArray(bmpPath string, pixelArray *PixelArray, recolor bool) error {
	var ramp [256]color.Color

	// Load optional color ramp BMP
	if recolor {
		rampImg, err := bmp.Decode(bytes.NewReader(rampFile))
		if err != nil {
			return fmt.Errorf("failed to decode color ramp BMP: %w", err)
		}
		b := rampImg.Bounds()
		if b.Dy() != 1 || b.Dx() < 256 {
			return fmt.Errorf("invalid color ramp dimensions (expected 1x256, got %dx%d)", b.Dx(), b.Dy())
		}
		for x := 0; x < 256; x++ {
			ramp[x] = rampImg.At(x, 0)
		}
	} else {
		// Default grayscale ramp that preserves the original palette behavior:
		// The original wrote palette entries 128..255 (bright) then 0..127 (dark),
		// so we map stored byte b -> (b + 128) % 256 for display intensity.
		for i := 0; i < 256; i++ {
			//display := byte((i + 128) & 0xFF)
			ramp[i] = color.Gray{Y: byte(i)}
		}
	}

	img := image.NewRGBA(image.Rect(0, 0, pixelArray.Width, pixelArray.Height))

	// Copy pixels, flipping vertically, and applying palette mapping
	for y := 0; y < pixelArray.Height; y++ {
		srcY := pixelArray.Height - 1 - y // invert vertically to match original BMPs
		row := pixelArray.getRow(0, srcY, 0)
		for x := 0; x < pixelArray.Width; x++ {
			stored := int(row[x])
			displayIdx := (stored + 128) & 0xFF
			img.Set(x, y, ramp[displayIdx])
		}
	}

	out, err := os.Create(bmpPath)
	if err != nil {
		return err
	}
	defer out.Close()

	return bmp.Encode(out, img)
}

const resolution = 9

// PluginsToBMP consumes an ordered slice of PluginEntry. Later entries override earlier ones.
func PluginsToBMP(plugins []string, bmpDir string) (int, error) {
	if len(plugins) == 0 {
		return 0, errors.New("no plugins provided")
	}

	imageWNAMs, err := WNAMsFromPlugins(plugins)
	if err != nil {
		return 0, err
	}
	if len(imageWNAMs) == 0 {
		return 0, errors.New("Couldn't find any LAND records in the provided plugin(s).")
	}

	// Determine bounding rectangle
	var left, right, top, bottom *int
	for coords := range imageWNAMs {
		parts := strings.Split(coords, ",")
		if len(parts) != 2 {
			continue
		}
		x, _ := strconv.Atoi(parts[0])
		y, _ := strconv.Atoi(parts[1])
		if left == nil || x < *left {
			left = &x
		}
		if right == nil || x > *right {
			right = &x
		}
		if bottom == nil || y < *bottom {
			bottom = &y
		}
		if top == nil || y > *top {
			top = &y
		}
	}
	if left == nil || right == nil || top == nil || bottom == nil {
		return 0, errors.New("no valid LAND coordinates found")
	}
	cellWidth := *right - *left + 1
	cellHeight := *top - *bottom + 1
	width := cellWidth * resolution
	height := cellHeight * resolution
	padWidth := padLength(width, 4)

	// Initialize map array to -128 (0x80 bytes)
	row := make([]byte, padWidth)
	for i := 0; i < width; i++ {
		row[i] = 0x80
	}
	for i := width; i < padWidth; i++ {
		row[i] = 0 // padding zeros (as Python added pad bytes)
	}
	mapBytes := make([]byte, 0, padWidth*height)
	for i := 0; i < height; i++ {
		mapBytes = append(mapBytes, row...)
	}
	mapArray := NewPixelArrayFromBytes(mapBytes, width, height, padWidth)

	// For each cell, impose the WNAM (9x9)
	for x := 0; x < cellWidth; x++ {
		worldX := x + *left
		for y := 0; y < cellHeight; y++ {
			worldY := y + *bottom
			key := fmt.Sprintf("%d,%d", worldX, worldY)
			if sub, ok := imageWNAMs[key]; ok {
				data := sub.Data
				if len(data) < 81 {
					// pad to 81
					tmp := make([]byte, 81)
					copy(tmp, data)
					data = tmp
				}
				cellArray := NewPixelArrayFromBytes(data, resolution, resolution, resolution)
				mapArray.impose(cellArray, x*resolution, y*resolution)
			}
		}
	}

	bmpName := fmt.Sprintf("color_%d,%d_%d,%d.bmp", *left, *bottom, *right, *top)
	bmpPath := filepath.Join(bmpDir, bmpName)
	if err := BMPFromPixelArray(bmpPath, mapArray, true); err != nil {
		return 0, err
	}

	hbmpName := fmt.Sprintf("height_%d,%d_%d,%d.bmp", *left, *bottom, *right, *top)
	hbmpPath := filepath.Join(bmpDir, hbmpName)
	if err := BMPFromPixelArray(hbmpPath, mapArray, false); err != nil {
		return 0, err
	}

	return len(imageWNAMs), nil
}
