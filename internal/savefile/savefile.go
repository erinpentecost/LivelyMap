package savefile

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/ernmw/omwpacker/esm"
	"github.com/ernmw/omwpacker/esm/record/lua"
)

const magic_prefix = "!!LivelyMap!!STARTOFENTRY!!"
const magic_suffix = "!!LivelyMap!!ENDOFENTRY!!"

type SaveData struct {
	Player string `json:"id"`
	Paths  []PathEntry
}

type PathEntry struct {
	TimeStamp uint64 `json:"t"`
	Duration  uint64 `json:"d"`
	// Xposition is an exterior cell X position.
	Xposition int64 `json:"x"`
	// Yposition is an exterior cell Y position.
	Yposition int64 `json:"y"`
	// CellID is an interior cell ID.
	CellID string `json:"id"`
}

func ExtractData(savePath string) (*SaveData, error) {
	// open file and back it up
	src, err := os.Open(savePath)
	if err != nil {
		return nil, fmt.Errorf("read %q: %w", savePath, err)
	}
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

	raw, err := extractRecord(records)
	if err != nil {
		return nil, fmt.Errorf("extract data from %q: %w", savePath, err)
	}
	data, err := Unmarshal(raw)
	if err != nil {
		return nil, fmt.Errorf("unmarshal extracted data from %q: %w", savePath, err)
	}
	return data, nil
}

func extractRecord(records []*esm.Record) ([]byte, error) {
	var found []byte
	for _, rec := range records {
		// there's only one LUAM per save game
		if rec.Tag != lua.LUAM {
			continue
		}
		for i := 0; i < len(rec.Subrecords); i++ {
			sub := rec.Subrecords[i]
			if sub.Tag == lua.LUAS {
				lf := lua.LUASField{}
				if err := lf.Unmarshal(sub); err != nil {
					return nil, fmt.Errorf("parse LUAS subrecord: %w", err)
				}
				if lf.Value == "scripts/LivelyMap/player.lua" {
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
					break
				}
			}
		}
	}
	return found, nil
}

func Unmarshal(raw []byte) (*SaveData, error) {
	var data SaveData
	if err := json.Unmarshal(raw, &data); err != nil {
		return nil, fmt.Errorf("unmarshal save data: %w", err)
	}
	return &data, nil
}

// ExtractBetween reads from r and returns the first byte slice found between prefix and suffix.
func extractBetween(r io.Reader, prefix, suffix []byte) ([]byte, error) {
	const bufSize = 4096
	br := bufio.NewReader(r)

	var (
		data        []byte
		buf         = make([]byte, bufSize)
		prefixFound bool
	)

	// A rolling buffer in case the pattern spans chunks
	var window []byte

	for {
		n, err := br.Read(buf)
		if n > 0 {
			window = append(window, buf[:n]...)

			if !prefixFound {
				// Search for prefix
				if idx := bytes.Index(window, prefix); idx >= 0 {
					prefixFound = true
					window = window[idx+len(prefix):]
				} else if len(window) > len(prefix) {
					// Keep only possible overlap
					window = window[len(window)-len(prefix):]
				}
			} else {
				// Already found prefix; search for suffix
				if idx := bytes.Index(window, suffix); idx >= 0 {
					// Found suffix, return data up to it
					data = append(data, window[:idx]...)
					return data, nil
				}

				// Otherwise, accumulate data and keep tail overlap
				keep := max(0, len(suffix)-1)
				if len(window) > keep {
					data = append(data, window[:len(window)-keep]...)
					window = window[len(window)-keep:]
				}
			}
		}

		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read: %w", err)
		}
	}

	if !prefixFound {
		return nil, fmt.Errorf("prefix not found")
	}
	return nil, fmt.Errorf("suffix not found after prefix")
}
