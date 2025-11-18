package hue

import (
	"image"
	"image/color"
	"math"
	"testing"
)

func absDiff(a, b uint8) int {
	d := int(a) - int(b)
	if d < 0 {
		return -d
	}
	return d
}

func TestRGBToHSL_Red(t *testing.T) {
	hsl := RGBToHSL(color.RGBA{255, 0, 0, 255})

	if math.Abs(hsl.H-0) > 0.01 {
		t.Errorf("expected hue ~0, got %f", hsl.H)
	}
	if hsl.S < 0.99 {
		t.Errorf("expected high saturation, got %f", hsl.S)
	}
	if hsl.L < 0.49 || hsl.L > 0.51 {
		t.Errorf("expected lightness ~0.5, got %f", hsl.L)
	}
}

func TestHSLToRGB_RoundTrip(t *testing.T) {
	orig := color.RGBA{10, 200, 30, 255}

	hsl := RGBToHSL(orig)
	rgb := HSLToRGB(hsl)

	if absDiff(rgb.R, orig.R) > 2 ||
		absDiff(rgb.G, orig.G) > 2 ||
		absDiff(rgb.B, orig.B) > 2 {
		t.Errorf("round trip mismatch: start=%v end=%v", orig, rgb)
	}
}

func TestGetAverageHue(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 2, 1))
	img.Set(0, 0, color.RGBA{255, 0, 0, 255})   // 0°
	img.Set(1, 0, color.RGBA{255, 255, 0, 255}) // 60°

	avg := GetAverageHue(img)

	if avg < 29 || avg > 31 {
		t.Errorf("expected ~30°, got %f", avg)
	}
}
