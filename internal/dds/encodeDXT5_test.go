package dds

import (
	"bytes"
	"image"
	"image/color"
	"testing"
)

func TestEncodeProducesHeaderAndBlocks(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 8, 8))

	// Fill with gradient for deterministic output
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			img.SetRGBA(x, y, color.RGBA{
				R: uint8(x * 30),
				G: uint8(y * 30),
				B: 0,
				A: 255,
			})
		}
	}

	var buf bytes.Buffer
	if err := EncodeDXT5(&buf, img); err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	out := buf.Bytes()

	if len(out) < 128 {
		t.Fatalf("DDS too small, got %d bytes", len(out))
	}

	// Magic
	if string(out[:4]) != "DDS " {
		t.Fatalf("Missing DDS magic, got %q", out[:4])
	}

	// Number of 4x4 blocks for 8x8 image = 4 blocks
	// Each DXT5 block = 16 bytes
	expectedSize := 4*16 + 128 // header(128) + blocks(64)
	if len(out) != expectedSize {
		t.Fatalf("Unexpected DDS size: got %d, want %d", len(out), expectedSize)
	}

	// Check fourCC
	if string(out[84:88]) != "DXT5" {
		t.Fatalf("FourCC is %q, expected DXT5", out[84:88])
	}
}
