package dds

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"

	"github.com/mauserzjeh/dxt"
)

// DecodeDDS parses a DDS file from the provided bytes and returns an image.Image.
// Supports DXT1, DXT3, DXT5.
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

	hdr := dds[4 : 4+ddsHeaderLen] // 124 bytes
	// within hdr:
	// [0:4]  dwSize
	// [4:8]  dwFlags
	// [8:12] dwHeight
	// [12:16] dwWidth
	//
	// PixelFormat structure at offset 76 of the header (32 bytes)
	if len(hdr) < pfOffsetInHdr+32 {
		return nil, fmt.Errorf("dds: header missing pixel format")
	}
	pf := hdr[pfOffsetInHdr : pfOffsetInHdr+32]
	// pf layout usually: [0:4] dwSize, [4:8] dwFlags, [8:12] dwFourCC, [12:16] dwRGBBitCount, ...
	// But some exporters put the FourCC in a nonstandard slot (e.g. GIMP puts "DXT1" at pf[4:8]),
	// and some headers have dwSize==4. Be defensive: check canonical slot, then pf[4:8], then scan header.
	fourCC := string(pf[8:12])
	if fourCC == "\x00\x00\x00\x00" {
		// try the adjacent slot (some exporters set dwFlags/fourcc oddly)
		candidate := string(pf[4:8])
		if candidate != "\x00\x00\x00\x00" {
			fourCC = candidate
		} else {
			// final fallback: search the 124-byte header for known FourCC strings
			// (DXT1/DXT3/DXT5 are the ones we support).
			for _, s := range []string{"DXT1", "DXT3", "DXT5"} {
				if idx := bytes.Index(hdr, []byte(s)); idx >= 0 {
					// found it somewhere in the header — use that
					fourCC = s
					break
				}
			}
		}
	}

	// rgbBitCount typically lives at pf[12:16]. If it's zero, try to recover:
	rgbBitCount := binary.LittleEndian.Uint32(pf[12:16])
	if rgbBitCount == 0 {
		// Try a nearby location (sometimes the exporter wrote fields in a different order)
		// check pf[16:20] as a fallback; otherwise read hdr's pitch/linear field too.
		alt := binary.LittleEndian.Uint32(pf[16:20])
		if alt != 0 {
			rgbBitCount = alt
		} else if len(hdr) >= 20 {
			// fallback: use dwPitchOrLinearSize from header (bytes 16..20 of hdr)
			rgbBitCount = binary.LittleEndian.Uint32(hdr[16:20])
		}
	}

	height := binary.LittleEndian.Uint32(hdr[8:12])
	width := binary.LittleEndian.Uint32(hdr[12:16])

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
	if len(rgbaBytes) != int(width*height*4) {
		// defensive: if strides differ, try to copy as much as possible
		return nil, fmt.Errorf("dds: unexpected decoded byte length %d, want %d", len(rgbaBytes), int(width*height*4))
	}
	copy(img.Pix, rgbaBytes)

	// optional: ensure opaque pixels have alpha==255 (depending on decoder)
	// not required, but safe: iterate and clamp
	for i := 0; i < len(img.Pix); i += 4 {
		if img.Pix[i+3] == 0 {
			// leave as-is; some formats legitimately have zero alpha
		}
	}

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
