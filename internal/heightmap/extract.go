package heightmap

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type MapMaker interface {
	PluginsToBMP(pluginList map[string]string, bmpDir string) (int, error)
	OpenMWPlugins(cfgpath string, esmOnly bool) (map[string]string, error)
	MWPlugins(iniPath string, esmOnly bool) (map[string]string, error)
}

type concrete struct{}

func (c *concrete) PluginsToBMP(pluginList map[string]string, bmpDir string) (int, error) {
	return PluginsToBMP(pluginList, bmpDir)
}

func (c *concrete) OpenMWPlugins(cfgpath string, esmOnly bool) (map[string]string, error) {
	return OpenMWPlugins(cfgpath, esmOnly)
}

func (c *concrete) MWPlugins(iniPath string, esmOnly bool) (map[string]string, error) {
	return MWPlugins(iniPath, esmOnly)
}

func NewMapMaker() MapMaker {
	return &concrete{}
}

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

func readUint32LE(b []byte) uint32 {
	return binary.LittleEndian.Uint32(b)
}

func readInt32LE(b []byte) int32 {
	return int32(binary.LittleEndian.Uint32(b))
}

func readFloat32LE(b []byte) float32 {
	var v float32
	binary.Read(bytes.NewReader(b), binary.LittleEndian, &v)
	return v
}

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

var defaultWNAM []byte

func init() {
	// default WNAM bytes: 81 bytes of signed -128 -> byte 0x80
	defaultWNAM = make([]byte, 81)
	for i := range 81 {
		defaultWNAM[i] = 0x80
	}
}

func padLength(length, pad int) int {
	return ((length + pad - 1) / pad) * pad
}

type PixelArray struct {
	Value    []byte
	Width    int
	Height   int
	PadWidth int
	Size     int
}

func newPixelArrayFromRows(rows [][]byte, width, height, padWidth int) *PixelArray {
	b := make([]byte, 0, height*padWidth)
	for _, row := range rows {
		// row length may be width
		rowBytes := make([]byte, len(row))
		copy(rowBytes, row)
		b = append(b, rowBytes...)
		// pad remainder
		for i := 0; i < padWidth-width; i++ {
			b = append(b, 0)
		}
	}
	return &PixelArray{
		Value:    b,
		Width:    width,
		Height:   height,
		PadWidth: padWidth,
		Size:     len(b),
	}
}

func newPixelArrayFromBytes(b []byte, width, height, padWidth int) *PixelArray {
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
	for h := range height {
		r := p.getRow(x, y+h, width)
		c := make([]byte, len(r))
		copy(c, r)
		cropped = append(cropped, c...)
	}
	return newPixelArrayFromBytes(cropped, width, height, width)
}

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

func recordsFromPlugins(pluginDict map[string]string, recordTags map[string]bool) (map[string]map[string]*Record, error) {
	all := make(map[string]map[string]*Record)
	all["TES3"] = make(map[string]*Record)
	for _, pluginPath := range pluginDict {
		f, err := os.Open(pluginPath)
		if err != nil {
			return nil, err
		}
		f.Close()
		// parse only requested tags
		records := make(map[string]*Record)
		land, err := parsePluginFile(pluginPath, recordTags)
		if err != nil {
			return nil, err
		}
		maps.Copy(records, land)
		all["LAND"] = mergeMaps(all["LAND"], records)
	}
	return all, nil
}

func mergeMaps(a, b map[string]*Record) (out map[string]*Record) {
	out = make(map[string]*Record)
	maps.Copy(out, a)
	maps.Copy(out, b)
	return out
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

func wnamsFromPlugins(pluginDict map[string]string) (map[string]*Subrecord, error) {
	all, err := recordsFromPlugins(pluginDict, map[string]bool{"LAND": true})
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

func bmpFromPixelArray(bmpPath string, pixelArray *PixelArray) error {
	// base header values similar to Python baseBMPheader
	fileSize := uint32(0x436 + pixelArray.Height*pixelArray.PadWidth)
	dataOffset := uint32(0x436) // 1078
	infoSize := uint32(0x28)
	width := uint32(pixelArray.Width)
	height := uint32(pixelArray.Height)
	planes := uint16(1)
	bitsPerPixel := uint16(8)
	compression := uint32(0)
	imageSize := uint32(pixelArray.Height * pixelArray.PadWidth)
	xppm := uint32(0x0EC4)
	yppm := uint32(0x0EC4)
	colorsUsed := uint32(0x0100)
	importantColors := uint32(0x0100)

	out, err := os.Create(bmpPath)
	if err != nil {
		return err
	}
	defer out.Close()

	// BITMAPFILEHEADER (14 bytes)
	if _, err := out.Write([]byte("BM")); err != nil {
		return err
	}
	err = binary.Write(out, binary.LittleEndian, fileSize)
	if err != nil {
		return err
	}
	// Reserved
	err = binary.Write(out, binary.LittleEndian, uint32(0))
	if err != nil {
		return err
	}
	err = binary.Write(out, binary.LittleEndian, dataOffset)
	if err != nil {
		return err
	}

	// BITMAPINFOHEADER (40 bytes)
	err = binary.Write(out, binary.LittleEndian, infoSize)
	if err != nil {
		return err
	}
	err = binary.Write(out, binary.LittleEndian, width)
	if err != nil {
		return err
	}
	err = binary.Write(out, binary.LittleEndian, height)
	if err != nil {
		return err
	}
	err = binary.Write(out, binary.LittleEndian, planes)
	if err != nil {
		return err
	}
	err = binary.Write(out, binary.LittleEndian, bitsPerPixel)
	if err != nil {
		return err
	}
	err = binary.Write(out, binary.LittleEndian, compression)
	if err != nil {
		return err
	}
	err = binary.Write(out, binary.LittleEndian, imageSize)
	if err != nil {
		return err
	}
	err = binary.Write(out, binary.LittleEndian, xppm)
	if err != nil {
		return err
	}
	err = binary.Write(out, binary.LittleEndian, yppm)
	if err != nil {
		return err
	}
	err = binary.Write(out, binary.LittleEndian, colorsUsed)
	if err != nil {
		return err
	}
	err = binary.Write(out, binary.LittleEndian, importantColors)
	if err != nil {
		return err
	}

	// Palette: heightPalette as in Python: 128..255 then 0..127 each as RGBA (A=0)
	palette := make([]byte, 0, 256*4)
	for i := 128; i < 256; i++ {
		palette = append(palette, byte(i))
		palette = append(palette, byte(i))
		palette = append(palette, byte(i))
		palette = append(palette, 0)
	}
	for i := range 128 {
		palette = append(palette, byte(i))
		palette = append(palette, byte(i))
		palette = append(palette, byte(i))
		palette = append(palette, 0)
	}
	if len(palette) != 1024 {
		return fmt.Errorf("palette wrong length %d", len(palette))
	}
	if _, err := out.Write(palette); err != nil {
		return err
	}

	// Pixel data (already includes padding per row)
	if _, err := out.Write(pixelArray.Value); err != nil {
		return err
	}
	return nil
}

// PluginsToBMP builds a BMP from the provided plugins files.
func PluginsToBMP(pluginList map[string]string, bmpDir string) (int, error) {
	if len(pluginList) == 0 {
		return 0, errors.New("no plugins provided")
	}
	imageWNAMs, err := wnamsFromPlugins(pluginList)
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
	width := cellWidth * 9
	height := cellHeight * 9
	padWidth := padLength(width, 4)

	// Initialize map array to -128 (0x80 bytes)
	row := make([]byte, padWidth)
	for i := range width {
		row[i] = 0x80
	}
	for i := width; i < padWidth; i++ {
		row[i] = 0 // padding zeros (as Python added pad bytes)
	}
	mapBytes := make([]byte, 0, padWidth*height)
	for range height {
		mapBytes = append(mapBytes, row...)
	}
	mapArray := newPixelArrayFromBytes(mapBytes, width, height, padWidth)

	// For each cell, impose the WNAM (9x9)
	for x := range cellWidth {
		worldX := x + *left
		for y := range cellHeight {
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
				cellArray := newPixelArrayFromBytes(data, 9, 9, 9)
				mapArray.impose(cellArray, x*9, y*9)
			}
		}
	}

	bmpName := fmt.Sprintf("%d,%d.bmp", *left, *bottom)
	bmpPath := filepath.Join(bmpDir, bmpName)
	if err := bmpFromPixelArray(bmpPath, mapArray); err != nil {
		return 0, err
	}

	return len(imageWNAMs), nil
}

// OpenMWPlugins returns openmw plugins files from openmw.cfg.
func OpenMWPlugins(cfgpath string, esmOnly bool) (map[string]string, error) {
	contentFiles := make(map[string]string)
	dataFolders := []string{}

	f, err := os.Open(cfgpath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	scanner := io.NewSectionReader(f, 0, 1<<63-1)
	buf := make([]byte, 4096)
	var fileContent bytes.Buffer
	for {
		n, err := scanner.Read(buf)
		if n > 0 {
			fileContent.Write(buf[:n])
		}
		if err == io.EOF {
			break
		} else if err != nil {
			return nil, err
		}
	}
	for line := range strings.SplitSeq(fileContent.String(), "\n") {
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
			path := verifyPath(cfgpath, val)
			if path != "" {
				// if path is a dir
				if info, err := os.Stat(path); err == nil && info.IsDir() {
					dataFolders = append(dataFolders, path)
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
				contentFiles[strings.ToLower(val)] = ""
			}
		}
	}

	for _, dataPath := range dataFolders {
		items, _ := os.ReadDir(dataPath)
		for _, item := range items {
			lower := strings.ToLower(item.Name())
			if _, ok := contentFiles[lower]; ok {
				contentFiles[lower] = filepath.Join(dataPath, item.Name())
			}
		}
	}

	for k, v := range contentFiles {
		if v == "" {
			delete(contentFiles, k)
		}
	}
	if len(contentFiles) == 0 {
		return nil, nil
	}
	// normalize map key to plugin basename lower -> full path
	result := make(map[string]string)
	for k, v := range contentFiles {
		result[strings.ToLower(filepath.Base(k))] = v
	}
	return result, nil
}

func verifyPath(cfgPath string, s string) string {
	s = strings.Trim(s, "\" ")
	if s == "" {
		return ""
	}
	if filepath.IsAbs(s) {
		return s
	}
	absPath, err := filepath.Abs(filepath.Join(filepath.Dir(cfgPath), s))
	if err != nil {
		return ""
	}
	return absPath
}

// MWPlugins returns mw plugins files from morrowind.ini.
func MWPlugins(iniPath string, esmOnly bool) (map[string]string, error) {
	masters := make(map[string]string)
	plugins := make(map[string]string)
	masterDates := map[string]float64{}
	pluginDates := map[string]float64{}

	dataDir := filepath.Join(filepath.Dir(iniPath), "Data Files")
	contentFiles := map[string]string{}

	f, err := os.ReadFile(iniPath)
	if err != nil {
		return nil, err
	}
	for line := range strings.SplitSeq(string(f), "\n") {
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
				info, _ := os.Stat(pluginPath)
				if ext == ".esm" {
					masters[name] = pluginPath
					masterDates[name] = float64(info.ModTime().Unix())
				} else if ext == ".esp" {
					plugins[name] = pluginPath
					pluginDates[name] = float64(info.ModTime().Unix())
				}
			}
		}
	}
	// order by timestamps: we will just combine masters then plugins
	contentFiles = make(map[string]string)
	maps.Copy(contentFiles, masters)

	if !esmOnly {
		maps.Copy(contentFiles, plugins)
	}
	if len(contentFiles) == 0 {
		return nil, nil
	}
	return contentFiles, nil
}
