package dds

import (
	"fmt"
	"image"
	"io"
)

// Encode writes m encoded as DDS into w.
func Encode(w io.Writer, m image.Image, codec Codec) error {
	switch codec {
	case DXT1:
		return EncodeDXT1(w, m)
	case DXT5:
		return EncodeDXT5(w, m)
	case Lossless:
		return EncodeLossless(w, m)
	default:
		return fmt.Errorf("unknown codec %v", codec)
	}
}
