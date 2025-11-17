package hue

import (
	"image"
	"image/color"
	"math"
)

// HSL represents a color in HSL color space.
type HSL struct {
	H, S, L float64 // Hue (0–360), Saturation (0–1), Lightness (0–1)
}

// RGBToHSL converts a color.Color to HSL.
func RGBToHSL(c color.Color) HSL {
	r16, g16, b16, _ := c.RGBA()

	// Convert 16-bit RGBA from image to normalized 0–1 floats
	r := float64(r16) / 65535.0
	g := float64(g16) / 65535.0
	b := float64(b16) / 65535.0

	max := math.Max(r, math.Max(g, b))
	min := math.Min(r, math.Min(g, b))
	delta := max - min

	l := (max + min) / 2
	h := 0.0
	s := 0.0

	if delta != 0 {
		// Hue
		switch max {
		case r:
			h = math.Mod((g-b)/delta, 6)
		case g:
			h = (b-r)/delta + 2
		case b:
			h = (r-g)/delta + 4
		}
		h *= 60
		if h < 0 {
			h += 360
		}

		// Saturation
		if l > 0.5 {
			s = delta / (2 - max - min)
		} else {
			s = delta / (max + min)
		}
	}

	return HSL{H: h, S: s, L: l}
}

// GetAverageHue calculates the average hue of an image.
// Uses vector averaging to handle circular wrap-around at 360°.
func GetAverageHue(img image.Image) float64 {
	var sumX, sumY float64
	count := 0

	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			hsl := RGBToHSL(img.At(x, y))

			rad := hsl.H * math.Pi / 180
			sumX += math.Cos(rad)
			sumY += math.Sin(rad)
			count++
		}
	}

	if count == 0 {
		return 0
	}

	avgX := sumX / float64(count)
	avgY := sumY / float64(count)

	angle := math.Atan2(avgY, avgX) * 180 / math.Pi
	if angle < 0 {
		angle += 360
	}

	return angle
}

// HSLToRGB converts an HSL value to an RGBA color.
func HSLToRGB(hsl HSL) color.RGBA {
	h := hsl.H / 360
	s := hsl.S
	l := hsl.L

	if s == 0 {
		val := uint8(l * 255)
		return color.RGBA{val, val, val, 255}
	}

	var hue2rgb func(p, q, t float64) float64
	hue2rgb = func(p, q, t float64) float64 {
		if t < 0 {
			t += 1
		}
		if t > 1 {
			t -= 1
		}
		if t < 1.0/6 {
			return p + (q-p)*6*t
		}
		if t < 1.0/2 {
			return q
		}
		if t < 2.0/3 {
			return p + (q-p)*(2.0/3-t)*6
		}
		return p
	}

	var q float64
	if l < 0.5 {
		q = l * (1 + s)
	} else {
		q = l + s - l*s
	}
	p := 2*l - q

	r := hue2rgb(p, q, h+1.0/3)
	g := hue2rgb(p, q, h)
	b := hue2rgb(p, q, h-1.0/3)

	return color.RGBA{
		R: uint8(r * 255),
		G: uint8(g * 255),
		B: uint8(b * 255),
		A: 255,
	}
}

func MulColor(a, b color.Color) color.RGBA {
	ar, ag, ab, _ := a.RGBA()
	br, bg, bb, _ := b.RGBA()

	// Convert to non-premultiplied 0–255
	ar8 := float64(ar >> 8)
	ag8 := float64(ag >> 8)
	ab8 := float64(ab >> 8)

	br8 := float64(br >> 8)
	bg8 := float64(bg >> 8)
	bb8 := float64(bb >> 8)

	return color.RGBA{
		R: uint8((ar8 * br8 / 255.0) + 0.5),
		G: uint8((ag8 * bg8 / 255.0) + 0.5),
		B: uint8((ab8 * bb8 / 255.0) + 0.5),
		A: 255,
	}
}
