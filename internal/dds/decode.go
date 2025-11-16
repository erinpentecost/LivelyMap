package dds

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"

	"github.com/mauserzjeh/dxt"
)

// Decode parses a DDS file from the provided bytes and returns an image.Image.
// Supports DXT1, DXT3, DXT5 and simple uncompressed 24/32-bit RGB(A).
func Decode(dds []byte) (image.Image, error) {
	const (
		ddsMagicLen   = 4
		ddsHeaderLen  = 124
		totalHeader   = ddsMagicLen + ddsHeaderLen // 128
		pfOffsetInHdr = 76                         // pixel format start inside the 124-byte header
	)

	if len(dds) < totalHeader {
		return nil, fmt.Errorf("dds: data too short for header: %d < %d", len(dds), totalHeader)
	}

	if string(dds[0:4]) != "DDS " {
		return nil, fmt.Errorf("dds: missing magic 'DDS '")
	}

	// hdr is the 124-byte DDS header (immediately after the 4-byte magic)
	hdr := dds[4 : 4+ddsHeaderLen] // 124 bytes

	// Read header fields by exact byte offsets (do NOT rely on binary.Read into a Go struct)
	if len(hdr) < ddsHeaderLen {
		return nil, fmt.Errorf("dds: header too short")
	}

	// core header fields (offsets are relative to hdr)
	// dwSize       = hdr[0:4]
	// dwFlags      = hdr[4:8]
	// dwHeight     = hdr[8:12]
	// dwWidth      = hdr[12:16]
	// dwPitchOrLinearSize = hdr[16:20]
	height := binary.LittleEndian.Uint32(hdr[8:12])
	width := binary.LittleEndian.Uint32(hdr[12:16])
	// pitchOrLinear := binary.LittleEndian.Uint32(hdr[16:20]) // unused for now

	// PixelFormat (32 bytes) starts at hdr offset 76
	if len(hdr) < pfOffsetInHdr+32 {
		return nil, fmt.Errorf("dds: header missing pixel format")
	}
	pf := hdr[pfOffsetInHdr : pfOffsetInHdr+32]

	// Read the exact pixel format fields by offset
	pfSize := binary.LittleEndian.Uint32(pf[0:4])
	_ = pfSize // currently unused, but good to read/validate if needed
	// pfFlags := binary.LittleEndian.Uint32(pf[4:8])
	// canonical FourCC is pf[8:12]
	fourCC := string(pf[8:12])
	// canonical rgb bit count is pf[12:16]
	rgbBitCount := binary.LittleEndian.Uint32(pf[12:16])
	// masks (pf[16:20], pf[20:24], pf[24:28], pf[28:32])
	// rMask := binary.LittleEndian.Uint32(pf[16:20])
	// gMask := binary.LittleEndian.Uint32(pf[20:24])
	// bMask := binary.LittleEndian.Uint32(pf[24:28])
	// aMask := binary.LittleEndian.Uint32(pf[28:32])

	// Defensive FourCC detection:
	// Some exporters (e.g. some versions of GIMP) write FourCC at pf[4:8] or otherwise nonstandard fields.
	if fourCC == "\x00\x00\x00\x00" {
		// try pf[4:8]
		candidate := string(pf[4:8])
		if candidate != "\x00\x00\x00\x00" {
			fourCC = candidate
		} else {
			// final fallback: search the header for known FourCCs
			for _, s := range []string{"DXT1", "DXT3", "DXT5", "DX10"} {
				if idx := bytes.Index(hdr, []byte(s)); idx >= 0 {
					fourCC = s
					break
				}
			}
		}
	}

	// Defensive rgbBitCount recovery: if it's zero, try pf[16:20] (some broken exporters reorder),
	// otherwise fall back to the header's pitch/linear-size as last resort.
	if rgbBitCount == 0 {
		alt := binary.LittleEndian.Uint32(pf[16:20])
		if alt != 0 {
			rgbBitCount = alt
		} else {
			rgbBitCount = binary.LittleEndian.Uint32(hdr[16:20])
		}
	}

	// Data starts after the 128-byte header
	data := dds[totalHeader:]
	if len(data) == 0 {
		return nil, fmt.Errorf("dds: no image data")
	}

	// call appropriate decoder based on fourCC or uncompressed RGB
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
		// DX10 requires parsing a DDS_HEADER_DX10 extension; not implemented here.
		return nil, fmt.Errorf("dds: DX10 header not supported")
	default:
		// support simple uncompressed RGB(A) (common masks) as fallback
		if fourCC == "\x00\x00\x00\x00" && (rgbBitCount == 24 || rgbBitCount == 32) {
			rgbaBytes, err = decodeUncompressedRGB(data, uint(width), uint(height), rgbBitCount, pf)
		} else {
			return nil, fmt.Errorf("dds: unsupported FourCC %q or rgbBits=%d", fourCC, rgbBitCount)
		}
	}
	if err != nil {
		return nil, fmt.Errorf("dds: decode error: %w", err)
	}

	// Create image.RGBA and copy pixels
	img := image.NewRGBA(image.Rect(0, 0, int(width), int(height)))
	// our decoders produce RGBA byte slices in R,G,B,A per pixel order
	expectedLen := int(width * height * 4)
	if len(rgbaBytes) != expectedLen {
		return nil, fmt.Errorf("dds: unexpected decoded byte length %d, want %d", len(rgbaBytes), expectedLen)
	}
	copy(img.Pix, rgbaBytes)

	return img, nil
}

// decodeUncompressedRGB decodes a simple uncompressed DDS pixel buffer into RGBA bytes.
// This handles formats where pixel data is stored as contiguous scanlines in BGR/BGRA order.
// pf is the 32-byte pixel format block from the header (may be used to detect masks).
func decodeUncompressedRGB(data []byte, width, height uint, bits uint32, pf []byte) ([]byte, error) {
	// Very simple implementation: assume 24-bit BGR or 32-bit BGRA with row alignment equal to width*(bits/8).
	bytesPerPixel := int(bits / 8)
	expected := int(width) * int(height) * bytesPerPixel
	if len(data) < expected {
		return nil, fmt.Errorf("dds uncompressed: data too small (%d < %d)", len(data), expected)
	}
	out := make([]byte, int(width*height*4))
	srcIdx := 0
	dstIdx := 0
	for y := 0; y < int(height); y++ {
		for x := 0; x < int(width); x++ {
			if bytesPerPixel == 3 {
				// B G R
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
				// B G R A (or R G B A depending on masks — we assume BGRA)
				b := data[srcIdx+0]
				g := data[srcIdx+1]
				r := data[srcIdx+2]
				a := data[srcIdx+3]
				out[dstIdx+0] = r
				out[dstIdx+1] = g
				out[dstIdx+2] = b
				out[dstIdx+3] = a
				srcIdx += 4
				dstIdx += 4
			} else {
				return nil, fmt.Errorf("dds uncompressed: unsupported bytesPerPixel %d", bytesPerPixel)
			}
		}
		// Note: some DDS files pad each scanline to 4-byte boundary; this naive approach assumes none.
	}
	return out, nil
}
