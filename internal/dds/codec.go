package dds

type Codec int

const (
	// DXT1 doesn't support alpha.
	DXT1 Codec = iota
	// DXT5 supports alpha.
	DXT5
	// Lossless is basically a bmp.
	Lossless
)
