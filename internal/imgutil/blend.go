package imgutil

import (
	"image"
	"image/color"
)

func Blend(src1 *image.RGBA, src2 *image.RGBA, blender func(c1 color.RGBA, c2 color.RGBA) color.RGBA) *image.RGBA {
	bounds := src1.Bounds()
	dst := image.NewRGBA(bounds)

	w := bounds.Dx()
	h := bounds.Dy()

	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			dst.SetRGBA(x, y, blender(src1.RGBAAt(x, y), src2.RGBAAt(x, y)))
		}
	}

	return dst
}
