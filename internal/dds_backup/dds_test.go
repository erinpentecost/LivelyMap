package dds

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"testing"
)

func TestDDSRGBASetGet(t *testing.T) {
	img := NewDDSRGBA(image.Rect(0, 0, 2, 2))

	img.SetRGBA(0, 0, color.RGBA{10, 20, 30, 40})
	img.SetRGBA(1, 1, color.RGBA{50, 60, 70, 80})

	c1 := img.RGBAAt(0, 0)
	c2 := img.RGBAAt(1, 1)

	if c1 != (color.RGBA{10, 20, 30, 40}) {
		t.Fatalf("pixel 0,0 wrong: %v", c1)
	}
	if c2 != (color.RGBA{50, 60, 70, 80}) {
		t.Fatalf("pixel 1,1 wrong: %v", c2)
	}
}

func TestEncodeHeader(t *testing.T) {
	img := NewDDSRGBA(image.Rect(0, 0, 4, 4))
	var buf bytes.Buffer

	if err := Encode(&buf, img); err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	data := buf.Bytes()

	if len(data) < 128 {
		t.Fatalf("DDS too small: %d bytes", len(data))
	}

	// Check magic
	magic := binary.LittleEndian.Uint32(data[0:4])
	if magic != ddsMagic {
		t.Fatalf("bad DDS magic %08x", magic)
	}

	// Check header size
	hsize := binary.LittleEndian.Uint32(data[4:8])
	if hsize != ddsHeaderSize {
		t.Fatalf("bad header size %d", hsize)
	}

	// Check width/height
	h := binary.LittleEndian.Uint32(data[4+8 : 4+12])
	w := binary.LittleEndian.Uint32(data[4+12 : 4+16])
	if w != 4 || h != 4 {
		t.Fatalf("bad dimensions: %dx%d", w, h)
	}
}
