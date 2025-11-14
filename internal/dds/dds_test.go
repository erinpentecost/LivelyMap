package dds

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"testing"
)

func TestEncodeBasicDDS(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	img.SetRGBA(0, 0, color.RGBA{1, 2, 3, 4})
	img.SetRGBA(1, 0, color.RGBA{5, 6, 7, 8})
	img.SetRGBA(0, 1, color.RGBA{9, 10, 11, 12})
	img.SetRGBA(1, 1, color.RGBA{13, 14, 15, 16})

	var buf bytes.Buffer
	err := Encode(&buf, img)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	data := buf.Bytes()
	if len(data) < 128 {
		t.Fatalf("DDS too small: %d bytes", len(data))
	}

	// check magic
	magic := binary.LittleEndian.Uint32(data[0:4])
	if magic != ddsMagic {
		t.Fatalf("bad magic: %08x", magic)
	}

	// pixel checks
	p := data[128:] // pixel data starts after header

	exp := []byte{
		1, 2, 3, 4,
		5, 6, 7, 8,
		9, 10, 11, 12,
		13, 14, 15, 16,
	}

	if !bytes.Equal(p, exp) {
		t.Fatalf("pixels wrong:\n got %v\nwant %v", p, exp)
	}
}
