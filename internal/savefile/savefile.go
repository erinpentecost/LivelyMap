package savefile

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
)

const magic_prefix = "!!LivelyMap!!STARTOFENTRY!!"
const magic_suffix = "!!LivelyMap!!ENDOFENTRY!!"

type PathEntry struct {
	TimeStamp uint64 `json:"t"`
	Duration  uint64 `json:"d"`
	Xposition int64  `json:"x"`
	Yposition int64  `json:"y"`
	CellID    string `json:"id"`
}

func ExtractSaveData(savePath string) ([]byte, error) {
	f, err := os.Open(savePath)
	if err != nil {
		return nil, fmt.Errorf("open save file: %w", err)
	}
	defer f.Close()

	raw, err := extractBetween(f, []byte(magic_prefix), []byte(magic_suffix))
	if err != nil {
		return nil, fmt.Errorf("extract subslice: %w", err)
	}
	return raw, nil
}

func Unmarshal(raw []byte) ([]PathEntry, error) {
	var paths []PathEntry
	if err := json.Unmarshal(raw, &paths); err != nil {
		return nil, fmt.Errorf("unmarshal subslice: %w", err)
	}
	return paths, nil
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
