package heightmap

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"image"
	"image/color"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	_ "embed"

	"golang.org/x/image/bmp"
)

//go:embed ramp.bmp
var rampFile []byte

// --- Public API types ---

// PluginEntry preserves plugin order: first entry overrides earlier ones (same as openmw.cfg order).
type PluginEntry struct {
	Name string // lowercased basename
	Path string // full path
}

type MapMaker interface {
	// PluginsToBMP consumes plugins in the order provided by the slice.
	PluginsToBMP(plugins []PluginEntry, bmpDir string) (int, error)

	// Helpers that read config files and return ordered plugin lists
	OpenMWPlugins(cfgpath string, esmOnly bool) ([]PluginEntry, error)
	MWPlugins(iniPath string, esmOnly bool) ([]PluginEntry, error)
}

type concrete struct{}

func (c *concrete) PluginsToBMP(plugins []PluginEntry, bmpDir string) (int, error) {
	return PluginsToBMP(plugins, bmpDir)
}

func (c *concrete) OpenMWPlugins(cfgpath string, esmOnly bool) ([]PluginEntry, error) {
	return OpenMWPlugins(cfgpath, esmOnly)
}

func (c *concrete) MWPlugins(iniPath string, esmOnly bool) ([]PluginEntry, error) {
	return MWPlugins(iniPath, esmOnly)
}

func NewMapMaker() MapMaker { return &concrete{} }

// --- Internal types used for parsing ---

type Subrecord struct {
	Tag  string
	Data []byte
}

type Record struct {
	Tag          string
	Size         uint32
	Flags        uint32
	Subrecords   []*Subrecord
	PluginName   string
	PluginOffset int64
	Passed       bool
	Name         string
	Id           string
}

// --- binary helpers ---

func readUint32LE(b []byte) uint32 {
	return binary.LittleEndian.Uint32(b)
}

func readInt32LE(b []byte) int32 {
	return int32(binary.LittleEndian.Uint32(b))
}

// --- record parsing ---

func readRecord(f *os.File, tags map[string]bool) (*Record, error) {
	start, _ := f.Seek(0, io.SeekCurrent)
	hdr := make([]byte, 16)
	n, err := io.ReadFull(f, hdr)
	if err == io.EOF || (err == io.ErrUnexpectedEOF && n == 0) {
		return nil, nil // end of file
	}
	if err != nil {
		return nil, err
	}
	rec := &Record{
		Subrecords:   []*Subrecord{},
		PluginOffset: start,
	}
	rec.Tag = string(hdr[0:4])
	rec.Size = readUint32LE(hdr[4:8])
	// hdr[8:12] are padding ('4x' in Python)
	rec.Flags = readUint32LE(hdr[12:16])
	rec.PluginName = filepath.Base(f.Name())

	// If tags filtered and not in tags, skip and mark passed
	if tags != nil && !tags[rec.Tag] {
		_, _ = f.Seek(int64(rec.Size), io.SeekCurrent)
		rec.Passed = true
		return rec, nil
	}

	limit := start + 16 + int64(rec.Size)
	for {
		pos, _ := f.Seek(0, io.SeekCurrent)
		if pos >= limit {
			break
		}
		// read subrecord header
		subhdr := make([]byte, 8)
		if _, err := io.ReadFull(f, subhdr); err != nil {
			return nil, err
		}
		tag := string(subhdr[0:4])
		size := readUint32LE(subhdr[4:8])
		data := make([]byte, size)
		if size > 0 {
			if _, err := io.ReadFull(f, data); err != nil {
				return nil, err
			}
		}
		rec.Subrecords = append(rec.Subrecords, &Subrecord{Tag: tag, Data: data})
	}
	rec.setIdAndName()
	return rec, nil
}

func (r *Record) getSubrecord(tag string) *Subrecord {
	for _, s := range r.Subrecords {
		if s.Tag == tag {
			return s
		}
	}
	return nil
}

func (r *Record) setSubrecord(s *Subrecord) {
	// replace first occurrence or append
	for i, ex := range r.Subrecords {
		if ex.Tag == s.Tag {
			r.Subrecords[i] = s
			return
		}
	}
	r.Subrecords = append(r.Subrecords, s)
}

func (r *Record) delSubrecord(tag string) {
	for i, ex := range r.Subrecords {
		if ex.Tag == tag {
			r.Subrecords = append(r.Subrecords[:i], r.Subrecords[i+1:]...)
			return
		}
	}
}

func (r *Record) setIdAndName() {
	// Implement LAND case: INTV subrecord -> two ints
	if r.Tag == "LAND" {
		intv := r.getSubrecord("INTV")
		if intv != nil && len(intv.Data) >= 8 {
			x := int(readInt32LE(intv.Data[0:4]))
			y := int(readInt32LE(intv.Data[4:8]))
			r.Id = fmt.Sprintf("%d,%d", x, y)
			r.Name = r.Id
		} else {
			r.Id = ""
			r.Name = filepath.Base(r.PluginName)
		}
	} else {
		r.Name = filepath.Base(r.PluginName)
	}
}

// --- default WNAM ---

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

func (p *PixelArray) crop(x, y, width, height int) *PixelArray {
	cropped := make([]byte, 0, height*width)
	for h := 0; h < height; h++ {
		r := p.getRow(x, y+h, width)
		c := make([]byte, len(r))
		copy(c, r)
		cropped = append(cropped, c...)
	}
	return NewPixelArrayFromBytes(cropped, width, height, width)
}

// --- plugin file parsing helpers ---

// parsePluginFile: returns LAND records found in a single plugin file
func parsePluginFile(path string, tags map[string]bool) (map[string]*Record, error) {
	records := make(map[string]*Record)
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	for {
		rec, err := readRecord(f, tags)
		if err != nil {
			return nil, err
		}
		if rec == nil {
			break
		}
		if rec.Passed {
			continue
		}
		if rec.Tag == "LAND" {
			// set id & name already done in readRecord
			if rec.Id == "" {
				// fallback: use plugin name + offset
				rec.Name = fmt.Sprintf("%s %d", rec.PluginName, rec.PluginOffset)
			}
			records[rec.Name] = rec
		}
	}
	return records, nil
}

// recordsFromPlugins now accepts an ordered slice of PluginEntry and preserves that ordering
func recordsFromPlugins(plugins []PluginEntry, recordTags map[string]bool) (map[string]map[string]*Record, error) {
	all := make(map[string]map[string]*Record)
	all["TES3"] = make(map[string]*Record)
	allLAND := make(map[string]*Record)

	for _, p := range plugins {
		land, err := parsePluginFile(p.Path, recordTags)
		if err != nil {
			return nil, err
		}
		// IMPORTANT: iterate in plugin order and let later plugins overwrite earlier ones
		for name, rec := range land {
			allLAND[name] = rec
		}
	}

	all["LAND"] = allLAND
	return all, nil
}

func sanitizeLand(records map[string]*Record) map[string]*Record {
	for k, record := range records {
		if record.Tag == "LAND" {
			if record.getSubrecord("WNAM") == nil {
				// set DATA flags OR 1 if DATA exists
				data := record.getSubrecord("DATA")
				var flags uint32 = 0
				if data != nil && len(data.Data) >= 4 {
					flags = readUint32LE(data.Data[0:4])
				}
				flags = flags | 1
				flagBytes := make([]byte, 4)
				binary.LittleEndian.PutUint32(flagBytes, flags)
				record.setSubrecord(&Subrecord{Tag: "DATA", Data: flagBytes})
				// set default WNAM
				wn := make([]byte, len(defaultWNAM))
				copy(wn, defaultWNAM)
				record.setSubrecord(&Subrecord{Tag: "WNAM", Data: wn})
				records[k] = record
			}
		}
	}
	return records
}

// WNAMsFromPlugins accepts ordered plugins and returns a map of coords -> WNAM, where later plugins override earlier ones.
func WNAMsFromPlugins(plugins []PluginEntry) (map[string]*Subrecord, error) {
	all, err := recordsFromPlugins(plugins, map[string]bool{"LAND": true})
	if err != nil {
		return nil, err
	}
	landRecords := all["LAND"]
	landRecords = sanitizeLand(landRecords)
	WNAMs := make(map[string]*Subrecord)
	for coords, record := range landRecords {
		wnam := record.getSubrecord("WNAM")
		if wnam != nil {
			WNAMs[coords] = wnam
		}
	}
	return WNAMs, nil
}

// BMPFromPixelArray writes an 8-bit BMP preserving the original palette ordering
// used by the previous manual-writer (128..255 then 0..127). That palette is
// equivalent to mapping each stored byte b -> (b + 128) & 0xFF for display.
func BMPFromPixelArrayOldest(bmpPath string, pixelArray *PixelArray) error {
	img := image.NewGray(image.Rect(0, 0, pixelArray.Width, pixelArray.Height))

	// Copy pixels, flipping vertically so the image isn't upside-down.
	// Also remap palette index: displayed = (stored + 128) & 0xFF
	for y := 0; y < pixelArray.Height; y++ {
		srcY := pixelArray.Height - 1 - y // BMPs are bottom-up in pixel arrays we wrote
		row := pixelArray.getRow(0, srcY, 0)
		for x := 0; x < pixelArray.Width; x++ {
			stored := row[x]
			display := byte((int(stored) + 128) & 0xFF)
			img.SetGray(x, y, color.Gray{Y: display})
		}
	}

	out, err := os.Create(bmpPath)
	if err != nil {
		return err
	}
	defer out.Close()

	return bmp.Encode(out, img)
}

// BMPFromPixelArray writes a BMP from a PixelArray, optionally using a color ramp BMP.
//
// If rampPath is empty, it uses the original grayscale mapping (b -> (b+128)&0xFF).
// If rampPath points to a 1x256 BMP, its pixels are used as a color gradient where
// the leftmost pixel is the lowest elevation and the rightmost is the highest.
func BMPFromPixelArrayOld(bmpPath string, pixelArray *PixelArray, recolor bool) error {
	var ramp [256]color.Color

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
		// Default grayscale ramp (mirroring the old 128..255 then 0..127 palette)
		for i := 0; i < 256; i++ {
			v := byte((i + 128) & 0xFF)
			ramp[i] = color.Gray{Y: v}
		}
	}

	img := image.NewRGBA(image.Rect(0, 0, pixelArray.Width, pixelArray.Height))

	// Copy pixels, flipping vertically
	for y := 0; y < pixelArray.Height; y++ {
		srcY := pixelArray.Height - 1 - y
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
func PluginsToBMP(plugins []PluginEntry, bmpDir string) (int, error) {
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

// --- Config parsing helpers that return ordered plugin slices ---

// OpenMWPlugins returns openmw plugins from openmw.cfg in the order they appear.
func OpenMWPlugins(cfgpath string, esmOnly bool) ([]PluginEntry, error) {
	type pending struct {
		Name string
		Raw  string
	}
	var dataFolders []string
	var pendingContents []pending

	f, err := os.Open(cfgpath)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	b, err := io.ReadAll(f)
	if err != nil {
		return nil, err
	}
	lines := strings.Split(string(b), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(parts[0]))
		val := strings.TrimSpace(parts[1])
		if key == "data" {
			p := verifyPath(cfgpath, val)
			if p != "" {
				if info, err := os.Stat(p); err == nil && info.IsDir() {
					dataFolders = append(dataFolders, p)
				}
			}
		} else if key == "content" {
			ext := strings.ToLower(filepath.Ext(val))
			validExts := map[string]bool{".esm": true}
			if !esmOnly {
				validExts[".esp"] = true
				validExts[".omwaddon"] = true
			}
			if validExts[ext] {
				pendingContents = append(pendingContents, pending{Name: strings.ToLower(filepath.Base(val)), Raw: val})
			}
		}
	}

	// resolve pendingContents against dataFolders preserving order
	var out []PluginEntry
	for _, pc := range pendingContents {
		// search folders in order for first match
		found := false
		for _, dataPath := range dataFolders {
			candidate := filepath.Join(dataPath, pc.Raw)
			if _, err := os.Stat(candidate); err == nil {
				out = append(out, PluginEntry{Name: strings.ToLower(filepath.Base(pc.Raw)), Path: candidate})
				found = true
				break
			}
			// also check lowercase matches in directory
			items, _ := os.ReadDir(dataPath)
			for _, item := range items {
				if strings.ToLower(item.Name()) == strings.ToLower(pc.Raw) {
					out = append(out, PluginEntry{Name: strings.ToLower(item.Name()), Path: filepath.Join(dataPath, item.Name())})
					found = true
					break
				}
			}
			if found {
				break
			}
		}
		// If not found, skip it (mirrors original behavior)
	}

	if len(out) == 0 {
		return nil, nil
	}
	return out, nil
}

func verifyPath(baseCfgPath, s string) string {
	s = strings.Trim(s, "\" ")
	if s == "" {
		return ""
	}
	if filepath.IsAbs(s) {
		return s
	}
	absPath, err := filepath.Abs(filepath.Join(filepath.Dir(baseCfgPath), s))
	if err != nil {
		return ""
	}
	return absPath
}

// MWPlugins returns plugins listed in morrowind.ini in the order they appear.
func MWPlugins(iniPath string, esmOnly bool) ([]PluginEntry, error) {
	masters := []PluginEntry{}
	plugins := []PluginEntry{}

	dataDir := filepath.Join(filepath.Dir(iniPath), "Data Files")
	b, err := os.ReadFile(iniPath)
	if err != nil {
		return nil, err
	}
	lines := strings.Split(string(b), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, ";") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(parts[0]))
		val := strings.TrimSpace(parts[1])
		if strings.HasPrefix(key, "gamefile") {
			pluginPath := filepath.Join(dataDir, val)
			if _, err := os.Stat(pluginPath); err == nil {
				ext := strings.ToLower(filepath.Ext(val))
				name := strings.ToLower(val)
				if ext == ".esm" {
					masters = append(masters, PluginEntry{Name: name, Path: pluginPath})
				} else if ext == ".esp" {
					plugins = append(plugins, PluginEntry{Name: name, Path: pluginPath})
				}
			}
		}
	}

	// combine masters then plugins (same semantic as original)
	out := make([]PluginEntry, 0, len(masters)+len(plugins))
	out = append(out, masters...)
	if !esmOnly {
		out = append(out, plugins...)
	}
	if len(out) == 0 {
		return nil, nil
	}
	return out, nil
}
