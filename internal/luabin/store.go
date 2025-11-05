package luabin

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"os"
)

// Matches OpenMW SerializationType enum
const (
	TypeNil      = 0
	TypeBoolean  = 1
	TypeInteger  = 2
	TypeNumber   = 3
	TypeString   = 4
	TypeTable    = 5
	TypeUserdata = 6
)

// StorageSection is a map of string keys to raw serialized Lua data.
type StorageSection map[string][]byte

// StorageFile holds all sections from player_storage.bin or global_storage.bin.
type StorageFile struct {
	Sections map[string]StorageSection
}

// ReadStorageFile parses a full player_storage.bin/global_storage.bin file.
func ReadStorageFile(data []byte) (*StorageFile, error) {
	r := bytes.NewReader(data)
	readUint32 := func() (uint32, error) {
		var v uint32
		if err := binary.Read(r, binary.LittleEndian, &v); err != nil {
			return 0, err
		}
		return v, nil
	}

	readString := func() (string, error) {
		length, err := readUint32()
		if err != nil {
			return "", err
		}
		buf := make([]byte, length)
		if _, err := io.ReadFull(r, buf); err != nil {
			return "", err
		}
		return string(buf), nil
	}

	readBytes := func() ([]byte, error) {
		length, err := readUint32()
		if err != nil {
			return nil, err
		}
		buf := make([]byte, length)
		if _, err := io.ReadFull(r, buf); err != nil {
			return nil, err
		}
		return buf, nil
	}

	sectionCount, err := readUint32()
	if err != nil {
		return nil, fmt.Errorf("read sectionCount: %w", err)
	}

	result := &StorageFile{Sections: make(map[string]StorageSection)}

	for i := 0; i < int(sectionCount); i++ {
		sectionName, err := readString()
		if err != nil {
			return nil, fmt.Errorf("read section name: %w", err)
		}

		entryCount, err := readUint32()
		if err != nil {
			return nil, fmt.Errorf("read entryCount for section %s: %w", sectionName, err)
		}

		section := make(StorageSection)
		for j := 0; j < int(entryCount); j++ {
			key, err := readString()
			if err != nil {
				return nil, fmt.Errorf("read key: %w", err)
			}
			value, err := readBytes()
			if err != nil {
				return nil, fmt.Errorf("read value: %w", err)
			}
			section[key] = value
		}
		result.Sections[sectionName] = section
	}

	return result, nil
}

// GetSection returns a specific section by name.
func (sf *StorageFile) GetSection(name string) (StorageSection, bool) {
	sec, ok := sf.Sections[name]
	return sec, ok
}

// LoadStorageFile reads from disk and parses.
func LoadStorageFile(path string) (*StorageFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return ReadStorageFile(data)
}

type Deserializer struct {
	r *bytes.Reader
}

func NewDeserializer(data []byte) *Deserializer {
	return &Deserializer{r: bytes.NewReader(data)}
}

func (d *Deserializer) readByte() (byte, error) {
	b, err := d.r.ReadByte()
	if err != nil {
		return 0, fmt.Errorf("readByte: %w", err)
	}
	return b, nil
}

func (d *Deserializer) readUint32() (uint32, error) {
	var v uint32
	if err := binary.Read(d.r, binary.LittleEndian, &v); err != nil {
		return 0, fmt.Errorf("readUint32: %w", err)
	}
	return v, nil
}

func (d *Deserializer) readInt64() (int64, error) {
	var v int64
	if err := binary.Read(d.r, binary.LittleEndian, &v); err != nil {
		return 0, fmt.Errorf("readInt64: %w", err)
	}
	return v, nil
}

func (d *Deserializer) readFloat64() (float64, error) {
	var v float64
	if err := binary.Read(d.r, binary.LittleEndian, &v); err != nil {
		return 0, fmt.Errorf("readFloat64: %w", err)
	}
	return v, nil
}

func (d *Deserializer) readString() (string, error) {
	length, err := d.readUint32()
	if err != nil {
		return "", err
	}
	buf := make([]byte, length)
	if _, err := d.r.Read(buf); err != nil {
		return "", fmt.Errorf("readString: %w", err)
	}
	return string(buf), nil
}

func (d *Deserializer) readValue() (any, error) {
	tagByte, err := d.readByte()
	if err != nil {
		return nil, err
	}

	switch tagByte {
	case TypeNil:
		return nil, nil
	case TypeBoolean:
		b, err := d.readByte()
		if err != nil {
			return nil, err
		}
		return b != 0, nil
	case TypeInteger:
		v, err := d.readInt64()
		return v, err
	case TypeNumber:
		v, err := d.readFloat64()
		return v, err
	case TypeString:
		return d.readString()
	case TypeTable:
		return d.readTable()
	case TypeUserdata:
		// Userdata is stored as a string blob with a type tag before it
		typeName, err := d.readString()
		if err != nil {
			return nil, err
		}
		data, err := d.readString()
		if err != nil {
			return nil, err
		}
		return map[string]any{
			"type": typeName,
			"data": data,
		}, nil
	default:
		return nil, fmt.Errorf("unknown type tag: %d", tagByte)
	}
}

func (d *Deserializer) readTable() (map[any]any, error) {
	result := make(map[any]any)
	for {
		keyTag, err := d.readByte()
		if err != nil {
			return nil, err
		}
		if keyTag == TypeNil {
			break
		}
		d.r.UnreadByte() // push tag back for readValue()
		key, err := d.readValue()
		if err != nil {
			return nil, err
		}
		val, err := d.readValue()
		if err != nil {
			return nil, err
		}
		result[key] = val
	}
	return result, nil
}

// Deserialize parses a LuaUtil::serialize() blob into Go values.
func Deserialize(data []byte) (any, error) {
	d := NewDeserializer(data)
	return d.readValue()
}
