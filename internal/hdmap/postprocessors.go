package hdmap

import (
	"image"
)

type PostProcessor interface {
	Process(src *image.RGBA) (*image.RGBA, error)
}
