package postprocessors

import (
	"fmt"
	"image"
	"math/bits"

	"golang.org/x/image/draw"
)

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

func nextPoT(n uint64) uint64 {
	if n == 0 {
		return 1
	}
	// If n is already a power of two, return n
	if n&(n-1) == 0 {
		return n
	}
	// bits.Len64 gives position of highest bit + 1
	return 1 << bits.Len64(n)
}
