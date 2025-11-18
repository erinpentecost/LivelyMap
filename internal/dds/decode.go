package dds

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"

	"github.com/mauserzjeh/dxt"
)

const (
	ddsMagicLen   = 4
	ddsHdrLen     = 124
	totalHdrLen   = ddsMagicLen + ddsHdrLen // 128
	pfOffsetInHdr = 76                      // pixel format start inside the 124-byte header
)

// Decode parses a DDS file (given as bytes) and returns an image.Image.
// Supports DXT1, DXT3, DXT5 and simple uncompressed 24/32-bit RGB(A).
func Decode(dds []byte) (image.Image, error) {
	if len(dds) < totalHdrLen {
		return nil, fmt.Errorf("dds: data too short for header: %d < %d", len(dds), totalHdrLen)
	}
	if string(dds[0:4]) != "DDS " {
		return nil, fmt.Errorf("dds: missing magic 'DDS '")
	}

	// hdr is the 124-byte header (bytes 4..127)
	hdr := dds[4 : 4+ddsHdrLen]

	// Read canonical header fields by exact byte offsets (little-endian)
	height := binary.LittleEndian.Uint32(hdr[8:12])
	width := binary.LittleEndian.Uint32(hdr[12:16])
	// pitchOrLinear := binary.LittleEndian.Uint32(hdr[16:20]) // unused here

	// Ensure pixel-format block exists
	if len(hdr) < pfOffsetInHdr+32 {
		return nil, fmt.Errorf("dds: header missing pixel format")
	}
	pf := hdr[pfOffsetInHdr : pfOffsetInHdr+32]

	// Read canonical pf fields by offset
	pfSize := binary.LittleEndian.Uint32(pf[0:4])
	_ = pfSize // read it for potential validation; not strictly required

	// Some broken exporters set fields oddly. We'll be defensive about locating FourCC.
	// Try pf[4:8] first (many broken files put ASCII FourCC there), then pf[8:12] (canonical),
	// then scan the header as a final fallback.
	var fourCC string
	// helper to test ASCII-like FourCC (DXT1/3/5)
	isDXT := func(b []byte) bool {
		if len(b) < 3 {
			return false
		}
		return (b[0] == 'D' && b[1] == 'X' && b[2] == 'T')
	}

	if isDXT(pf[4:8]) {
		fourCC = string(pf[4:8])
	} else if isDXT(pf[8:12]) {
		fourCC = string(pf[8:12])
	} else {
		// final fallback: scan the 124-byte header for known FourCCs
		for _, s := range []string{"DXT1", "DXT3", "DXT5", "DX10"} {
			if bytes.Index(hdr, []byte(s)) >= 0 {
				fourCC = s
				break
			}
		}
	}

	// Read rgbBitCount (canonical pf[12:16]). Might be garbage for compressed formats -
	// we'll ignore it when a valid FourCC is found.
	rgbBitCount := binary.LittleEndian.Uint32(pf[12:16])

	// Data begins immediately after the 128-byte header
	data := dds[totalHdrLen:]
	if len(data) == 0 {
		return nil, fmt.Errorf("dds: no image data")
	}

	// If we have a DXT FourCC, prefer that (ignore rgbBitCount garbage).
	var rgbaBytes []byte
	var err error

	switch fourCC {
	case "DXT1":
		rgbaBytes, err = dxt.DecodeDXT1(data, uint(width), uint(height))
	case "DXT3":
		rgbaBytes, err = dxt.DecodeDXT3(data, uint(width), uint(height))
	case "DXT5":
		rgbaBytes, err = dxt.DecodeDXT5(data, uint(width), uint(height))
	case "DX10":
		return nil, fmt.Errorf("dds: DX10 header not supported")
	default:
		// No DXT detected: attempt simple uncompressed RGB(A) fallback.
		// Accept rgbBitCount == 24 or 32. (Some headers have garbage values; reject those.)
		if rgbBitCount == 24 || rgbBitCount == 32 {
			rgbaBytes, err = decodeUncompressedRGB(data, uint(width), uint(height), rgbBitCount, pf)
		} else {
			return nil, fmt.Errorf("dds: unsupported FourCC %q and rgbBits=%d", fourCC, rgbBitCount)
		}
	}
	if err != nil {
		return nil, fmt.Errorf("dds: decode error: %w", err)
	}

	// Build image.RGBA
	expected := int(width * height * 4)
	if len(rgbaBytes) != expected {
		return nil, fmt.Errorf("dds: unexpected decoded byte length %d, want %d", len(rgbaBytes), expected)
	}
	img := image.NewRGBA(image.Rect(0, 0, int(width), int(height)))
	copy(img.Pix, rgbaBytes)

	return img, nil
}

// decodeUncompressedRGB decodes a simple uncompressed DDS pixel buffer into RGBA bytes.
// This handles contiguous scanlines in BGR/BGRA order. pf is the 32-byte pixel-format block.
func decodeUncompressedRGB(data []byte, width, height uint, bits uint32, pf []byte) ([]byte, error) {
	bytesPerPixel := int(bits / 8)
	expected := int(width) * int(height) * bytesPerPixel
	if len(data) < expected {
		return nil, fmt.Errorf("dds uncompressed: data too small (%d < %d)", len(data), expected)
	}

	out := make([]byte, int(width*height*4))
	srcIdx := 0
	dstIdx := 0

	// Read masks in case we need to detect BGRA vs RGBA
	rMask := binary.LittleEndian.Uint32(pf[16:20])
	//gMask := binary.LittleEndian.Uint32(pf[20:24])
	bMask := binary.LittleEndian.Uint32(pf[24:28])
	//aMask := binary.LittleEndian.Uint32(pf[28:32])

	// Common mask patterns:
	// BGRA: RMask = 0x00FF0000, GMask = 0x0000FF00, BMask = 0x000000FF
	// RGBA: RMask = 0x000000FF, GMask = 0x0000FF00, BMask = 0x00FF0000
	isBGRA := rMask == 0x00FF0000 && bMask == 0x000000FF

	for y := 0; y < int(height); y++ {
		for x := 0; x < int(width); x++ {
			if bytesPerPixel == 3 {
				// assume B G R
				b := data[srcIdx+0]
				g := data[srcIdx+1]
				r := data[srcIdx+2]
				out[dstIdx+0] = r
				out[dstIdx+1] = g
				out[dstIdx+2] = b
				out[dstIdx+3] = 255
				srcIdx += 3
				dstIdx += 4
			} else if bytesPerPixel == 4 {
				// read four bytes; order depends on masks
				a := byte(255)
				b0 := data[srcIdx+0]
				b1 := data[srcIdx+1]
				b2 := data[srcIdx+2]
				b3 := data[srcIdx+3]
				srcIdx += 4

				var r, g, b byte
				if isBGRA {
					b = b0
					g = b1
					r = b2
					a = b3
				} else {
					// assume order is R G B A
					r = b0
					g = b1
					b = b2
					a = b3
				}
				out[dstIdx+0] = r
				out[dstIdx+1] = g
				out[dstIdx+2] = b
				out[dstIdx+3] = a
				dstIdx += 4
			} else {
				return nil, fmt.Errorf("dds uncompressed: unsupported bytesPerPixel %d", bytesPerPixel)
			}
		}
		// Note: if scanlines are padded in the file, this simple approach will mis-align.
		// For most DDS files used here scanlines are contiguous.
	}

	return out, nil
}
