package save

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"fmt"
	"io"
	"os"
)

// StorageSectionData holds the extracted section.
type StorageSectionData struct {
	SectionName string
	Data        []byte
}

// ExtractPlayerStorageSection reads an OpenMW 0.49 save file at savePath
// and finds the playerStorage section with name sectionName.
func ExtractPlayerStorageSection(savePath string, sectionName string) (*StorageSectionData, error) {
	f, err := os.Open(savePath)
	if err != nil {
		return nil, fmt.Errorf("open save file: %w", err)
	}
	defer f.Close()

	// 1. Read save header → check magic & version
	var magic [4]byte
	if _, err := io.ReadFull(f, magic[:]); err != nil {
		return nil, fmt.Errorf("read magic: %w", err)
	}
	if string(magic[:]) != "TES3" {
		return nil, fmt.Errorf("unexpected magic: %q", magic)
	}
	// Read version (major/minor) from header
	var versionMajor, versionMinor uint16
	if err := binary.Read(f, binary.LittleEndian, &versionMajor); err != nil {
		return nil, fmt.Errorf("read versionMajor: %w", err)
	}
	if err := binary.Read(f, binary.LittleEndian, &versionMinor); err != nil {
		return nil, fmt.Errorf("read versionMinor: %w", err)
	}
	// Optionally verify versionMajor/minor match 0.49

	// 2. Loop through records until we find the “playerStorage” record
	for {
		// Each record: uint32 type, uint32 size, then data (size bytes).
		var recType uint32
		if err := binary.Read(f, binary.LittleEndian, &recType); err != nil {
			if err == io.EOF {
				break
			}
			return nil, fmt.Errorf("reading recType: %w", err)
		}
		var recSize uint32
		if err := binary.Read(f, binary.LittleEndian, &recSize); err != nil {
			return nil, fmt.Errorf("reading recSize: %w", err)
		}

		data := make([]byte, recSize)
		if _, err := io.ReadFull(f, data); err != nil {
			return nil, fmt.Errorf("reading record data for type %d: %w", recType, err)
		}

		// Compare recType against the constant for playerStorage
		// this is calculated from running the fourcc macro on "LUAM":
		// https://github.com/OpenMW/openmw/blob/master/components/esm/fourcc.hpp#L6
		const REC_PlayerStorage = 0x4D41554C
		if recType == REC_PlayerStorage {
			// Found the playerStorage block
			// Possibly compressed — check header inside data
			reader := bytes.NewReader(data)
			// Example: read a bool or uint8 indicating compression
			var compressedFlag uint8
			if err := binary.Read(reader, binary.LittleEndian, &compressedFlag); err != nil {
				return nil, fmt.Errorf("reading compressedFlag: %w", err)
			}
			var raw []byte
			if compressedFlag != 0 {
				// decompress using zlib
				zr, err := zlib.NewReader(reader)
				if err != nil {
					return nil, fmt.Errorf("zlib.NewReader: %w", err)
				}
				defer zr.Close()
				raw, err = io.ReadAll(zr)
				if err != nil {
					return nil, fmt.Errorf("reading decompressed data: %w", err)
				}
			} else {
				raw, err = io.ReadAll(reader)
				if err != nil {
					return nil, fmt.Errorf("reading uncompressed data: %w", err)
				}
			}

			// Now raw contains the storage sections. Parse them:
			br := bytes.NewReader(raw)
			var numSections uint32
			if err := binary.Read(br, binary.LittleEndian, &numSections); err != nil {
				return nil, fmt.Errorf("reading numSections: %w", err)
			}
			for i := uint32(0); i < numSections; i++ {
				// Read section name length (uint16)
				var nameLen uint16
				if err := binary.Read(br, binary.LittleEndian, &nameLen); err != nil {
					return nil, fmt.Errorf("reading nameLen section #%d: %w", i, err)
				}
				nameBytes := make([]byte, nameLen)
				if _, err := io.ReadFull(br, nameBytes); err != nil {
					return nil, fmt.Errorf("reading sectionName bytes #%d: %w", i, err)
				}
				secName := string(nameBytes)

				// Read data length (uint32)
				var secDataLen uint32
				if err := binary.Read(br, binary.LittleEndian, &secDataLen); err != nil {
					return nil, fmt.Errorf("reading secDataLen for %q: %w", secName, err)
				}
				secData := make([]byte, secDataLen)
				if _, err := io.ReadFull(br, secData); err != nil {
					return nil, fmt.Errorf("reading secData for %q: %w", secName, err)
				}

				if secName == sectionName {
					return &StorageSectionData{
						SectionName: secName,
						Data:        secData,
					}, nil
				}
			}

			// If we reach here, sectionName not found in this block
			return nil, fmt.Errorf("section %q not found inside playerStorage block", sectionName)
		}

		// Move on to next record
	}

	return nil, fmt.Errorf("playerStorage record not found")
}
