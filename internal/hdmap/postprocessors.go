package hdmap

import (
	"fmt"
	"image"
	"image/color"

	"golang.org/x/image/draw"
)

type PostProcessor interface {
	Process(src *image.RGBA) (*image.RGBA, error)
}

type PowerOfTwoProcessor struct {
	DownScaleFactor int
}

func (p *PowerOfTwoProcessor) Process(src *image.RGBA) (*image.RGBA, error) {
	bounds := src.Bounds()
	fmt.Printf("Scaling down square image...")
	newLength := uint64(max(bounds.Dx(), bounds.Dy()) / p.DownScaleFactor)
	newLength = nextPoT(newLength)
	downSize := image.NewRGBA(image.Rect(0, 0, int(newLength), int(newLength)))
	draw.CatmullRom.Scale(downSize, downSize.Bounds(), src, src.Bounds(), draw.Over, nil)
	return downSize, nil
}

type MinimumEdgeTransparencyProcessor struct {
	Minimum uint8
}

func (p *MinimumEdgeTransparencyProcessor) Process(src *image.RGBA) (*image.RGBA, error) {
	bounds := src.Bounds()
	for x := bounds.Min.X; x < bounds.Max.X; x++ {
		for _, y := range []int{bounds.Min.Y, bounds.Max.Y - 1} {
			if r, g, b, a := src.At(x, y).RGBA(); uint8(a>>8) < p.Minimum {
				src.Set(x, y, color.RGBA{
					R: uint8(r >> 8), G: uint8(g >> 8), B: uint8(b >> 8),
					A: p.Minimum})
			}
		}
	}
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for _, x := range []int{bounds.Min.X, bounds.Max.X - 1} {
			if r, g, b, a := src.At(x, y).RGBA(); uint8(a>>8) < p.Minimum {
				src.Set(x, y, color.RGBA{
					R: uint8(r >> 8), G: uint8(g >> 8), B: uint8(b >> 8),
					A: p.Minimum})
			}
		}
	}
	return src, nil
}
