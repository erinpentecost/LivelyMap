package savefile

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/ernmw/omwpacker/esm"
	"github.com/ernmw/omwpacker/esm/record/lua"
)

const magic_prefix = "!!LivelyMap!!STARTOFENTRY!!"
const magic_suffix = "!!LivelyMap!!ENDOFENTRY!!"

type SaveData struct {
	Player string       `json:"id"`
	Paths  []*PathEntry `json:"paths"`
}

type PathEntry struct {
	// TimeStamp the player entered the cell.
	TimeStamp uint64 `json:"t"`
	// Xposition is an exterior cell X position.
	Xposition int64 `json:"x"`
	// Yposition is an exterior cell Y position.
	Yposition int64 `json:"y"`
	// CellID is an interior cell ID.
	CellID string `json:"c"`
}

// Merge: keep the prefix of a.Paths with TimeStamp < earliest(B),
// then append all of b.Paths. If b entirely precedes a, return b+a.
// Handles nil/empty and mismatched player IDs.
func Merge(a *SaveData, b *SaveData) (*SaveData, error) {
	if a == nil && b == nil {
		return nil, fmt.Errorf("nil savedatas")
	}
	if a == nil {
		return b, nil
	}
	if b == nil {
		return a, nil
	}
	if !strings.EqualFold(a.Player, b.Player) {
		return nil, fmt.Errorf("mismatched player id: %q and %q", a.Player, b.Player)
	}

	// If either has no paths, return the other.
	if len(a.Paths) == 0 {
		return b, nil
	}
	if len(b.Paths) == 0 {
		return a, nil
	}

	// If b entirely precedes a, return b + a (no truncation of a).
	if b.Paths[len(b.Paths)-1].TimeStamp < a.Paths[0].TimeStamp {
		out := make([]*PathEntry, 0, len(b.Paths)+len(a.Paths))
		out = append(out, b.Paths...)
		out = append(out, a.Paths...)
		return &SaveData{Player: a.Player, Paths: out}, nil
	}

	earliestB := b.Paths[0].TimeStamp

	// Find the prefix length of A where TimeStamp < earliestB.
	prefixLen := 0
	for ; prefixLen < len(a.Paths); prefixLen++ {
		if a.Paths[prefixLen].TimeStamp >= earliestB {
			break
		}
	}

	// Keep a.Paths[:prefixLen] and append all of b.Paths
	merged := make([]*PathEntry, 0, prefixLen+len(b.Paths))
	if prefixLen > 0 {
		merged = append(merged, a.Paths[:prefixLen]...)
	}
	merged = append(merged, b.Paths...)

	return &SaveData{Player: a.Player, Paths: merged}, nil
}

func ExtractData(savePath string) (*SaveData, error) {
	// open file and back it up
	src, err := os.OpenFile(savePath, os.O_RDWR, 0666)
	if err != nil {
		return nil, fmt.Errorf("read %q: %w", savePath, err)
	}
	defer src.Close()
	backupPath := strings.TrimSuffix(savePath, filepath.Ext(savePath)) + ".bak"
	backup, err := os.Create(backupPath)
	if err != nil {
		return nil, fmt.Errorf("create %q: %w", backupPath, err)
	}
	if _, err := io.Copy(backup, src); err != nil {
		return nil, fmt.Errorf("back up %q to %q: %w", savePath, backupPath, err)
	}
	// parse file
	if _, err := src.Seek(0, io.SeekStart); err != nil {
		return nil, fmt.Errorf("seek to start of %q: %w", savePath, err)
	}
	records, err := esm.ParsePluginData("savefile", src)
	if err != nil {
		return nil, fmt.Errorf("parse %q: %w", savePath, err)
	}
	// find the data we are interested in
	// as a side-effect, records will be mutated to drop the records.
	raw, err := extractRecord(records)
	if err != nil {
		return nil, fmt.Errorf("extract data from %q: %w", savePath, err)
	}
	data, err := Unmarshal(raw)
	if err != nil {
		return nil, fmt.Errorf("unmarshal extracted data from %q: %w", savePath, err)
	}
	// overwrite the file with one that has the data removed.
	if err := src.Truncate(0); err != nil {
		return nil, fmt.Errorf("truncate save file: %w", err)
	}
	if _, err := src.Seek(0, io.SeekStart); err != nil {
		return nil, fmt.Errorf("seek save file: %w", err)
	}
	if err := esm.WriteRecords(src, slices.Values(records)); err != nil {
		return nil, fmt.Errorf("rewrite save file %q: %w", savePath, err)
	}

	return data, nil
}

func extractRecord(records []*esm.Record) ([]byte, error) {
	var found []byte
	for _, rec := range records {
		// there's only one LUAM per save game
		if rec.Tag != esm.RecordTag("PLAY") {
			continue
		}
		for i := 0; i < len(rec.Subrecords); i++ {
			sub := rec.Subrecords[i]
			if sub.Tag == lua.LUAS {
				lf := lua.LUASField{}
				if err := lf.Unmarshal(sub); err != nil {
					return nil, fmt.Errorf("parse LUAS subrecord: %w", err)
				}
				if lf.Value == "scripts/livelymap/player.lua" {
					// the next record should be LUAD
					if rec.Subrecords[i+1].Tag != lua.LUAD {
						return nil, fmt.Errorf("expected LUAD after LUAS")
					}
					// Extract our JSON data from it.
					var err error
					found, err = extractBetween(
						bytes.NewReader(rec.Subrecords[i+1].Data),
						[]byte(magic_prefix),
						[]byte(magic_suffix))
					if err != nil {
						return nil, fmt.Errorf("read JSON in LUAD: %w", err)
					}
					// Snip the current record and next one.
					rec.Subrecords = append(rec.Subrecords[:i], rec.Subrecords[i+2:]...)
					return found, nil
				}
			}
		}
	}
	return nil, fmt.Errorf("didn't find any data")
}

func Unmarshal(raw []byte) (*SaveData, error) {
	var data SaveData
	if err := json.Unmarshal(raw, &data); err != nil {
		return nil, fmt.Errorf("unmarshal save data: %w", err)
	}
	return &data, nil
}

// extractBetween reads from r and returns the first byte slice found between prefix and suffix.
func extractBetween(r io.Reader, prefix, suffix []byte) ([]byte, error) {
	const bufSize = 4096
	br := bufio.NewReader(r)

	window := make([]byte, 0, bufSize*2)
	prefixFound := false
	var data []byte

	for {
		// Read next chunk
		chunk := make([]byte, bufSize)
		n, err := br.Read(chunk)
		if n > 0 {
			window = append(window, chunk[:n]...)
		}

		// If prefix not yet found, search for it
		if !prefixFound {
			if idx := bytes.Index(window, prefix); idx >= 0 {
				prefixFound = true
				// keep only data after prefix in the window
				window = window[idx+len(prefix):]
			} else {
				// keep only possible prefix overlap to handle prefix across boundary
				if len(window) > len(prefix) {
					window = window[len(window)-len(prefix):]
				}
			}
		}

		// If prefix found, always check for suffix in the current window
		if prefixFound {
			if idx := bytes.Index(window, suffix); idx >= 0 {
				// found suffix — append bytes before suffix and return
				data = append(data, window[:idx]...)
				return data, nil
			}

			// No suffix yet — move safe bytes into data but keep a tail for overlap
			keep := max(0, len(suffix)-1)
			if len(window) > keep {
				data = append(data, window[:len(window)-keep]...)
				window = window[len(window)-keep:]
			}
		}

		// Handle errors / EOF
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read: %w", err)
		}
	}

	// After reading all input, determine proper error
	if !prefixFound {
		return nil, fmt.Errorf("prefix not found")
	}
	// If prefix was found but we never found the suffix
	return nil, fmt.Errorf("suffix not found after prefix")
}
