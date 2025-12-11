package hdmap

import "errors"

func Partition2(m MapCoords) []SubmapNode {
	// Step 1: ensure normal aspect ratio
	m = ensureAspectRatio(m)
	return Partition(m)
}

func ensureAspectRatio(m MapCoords) MapCoords {
	w := m.Width()
	h := m.Height()

	const shortSide = 4
	const longSide = 10

	if w*shortSide > h*longSide {
		// Too wide, extend vertically
		newHeight := max(h, (w*shortSide+2)/longSide) // integer division, round up
		return m.Extend(newHeight, 0)
	} else if h*shortSide > w*longSide {
		// Too tall, extend horizontally
		newWidth := max(w, (h*shortSide+2)/longSide)
		return m.Extend(0, newWidth)
	}
	return m
}

// powerOfTwoInRange returns a power of two p such that a <= p <= b.
// If no such power of two exists, it returns an error.
func powerOfTwoInRange(a, b int32) (int32, error) {
	if a > b {
		return 0, errors.New("invalid range: a > b")
	}
	if b < 1 {
		return 0, errors.New("no power of two in range")
	}

	// Find smallest power of two >= a.
	p := int32(1)
	for p < a {
		p <<= 1
		// Prevent infinite loop or overflow beyond int.
		if p <= 0 {
			return 0, errors.New("overflow while searching for power of two")
		}
	}

	if p > b {
		return 0, errors.New("no power of two in range")
	}
	return p, nil
}

func quadrants(a MapCoords) []MapCoords {
	// Textures used for NIFs need to be in powers of two.
	// Each cell is 64x64 pixels.
	// We should try to select cell region square sizes that will
	// result in a final image whose dimensions are a power of 2.

	width := 1 + a.Right - a.Left
	height := 1 + a.Top - a.Bottom

	// Require the parent to be a square.
	// If not, just return the original.
	if width != height {
		return []MapCoords{a}
	}

	side := width

	// Compute child square side length ≈ 2/3 side.
	// Bias upward for better coverage.
	// Can we find some length >= side/2 and <= side that is
	// a power of 2?
	childSide, err := powerOfTwoInRange(side/2, side)
	if err != nil {
		childSide = (2*side + 2) / 3
	}

	// How much the child window slides before falling off edge
	offset := side - childSide

	// Helper to create a square given origin (left,bottom)
	newSquare := func(left, bottom int32) MapCoords {
		return MapCoords{
			Left:   left,
			Right:  left + int32(childSide) - 1, // inclusive
			Bottom: bottom,
			Top:    bottom + int32(childSide) - 1, // inclusive
		}
	}

	// NW origin (x = Left, y = Top - childSide + 1)
	nwLeft := a.Left
	nwBottom := a.Top - int32(childSide) + 1

	// NE origin shifted by offset in X
	neLeft := a.Left + int32(offset)
	neBottom := nwBottom

	// SW origin
	swLeft := a.Left
	swBottom := a.Bottom

	// SE origin shifted by offset in X
	seLeft := a.Left + int32(offset)
	seBottom := a.Bottom

	return []MapCoords{
		newSquare(nwLeft, nwBottom),
		newSquare(neLeft, neBottom),
		newSquare(swLeft, swBottom),
		newSquare(seLeft, seBottom),
	}
}

// Partition will return between 1 and 8 partitions of m.
// All partitions will be squares, and may overlap.
func Partition(m MapCoords) []SubmapNode {
	partitions := []MapCoords{}
	for _, square := range findSquares(m) {
		// If the square is small enough, don't subdivide it.
		width := 1 + square.Right - square.Left
		height := 1 + square.Top - square.Bottom
		// Vvardenfell is 43x42.
		if width*height <= 43*42 {
			partitions = append(partitions, square)
			continue
		}
		// It's a big square, so we're going to chunk it up
		// into overlapping sub-squares.
		for _, quad := range quadrants(square) {
			// If the parent map is close to a square already,
			// then our initial subdivide is going to have two
			// big squares that overlap a ton. So when we do
			// our second chopping, we'll make sure to throw away
			// any square that are entirely contained by ones
			// we've already seen.
			contained := false
			for _, donePart := range partitions {
				if donePart.SupersetOf(quad) {
					contained = true
					break
				}
			}
			if !contained {
				partitions = append(partitions, quad)
			}
		}
	}
	return connectMapCoords(partitions)
}

// We're actually going to fit the two largest squares
// we can into the map, and make images for both.
// This way we can render them onto squar meshes and not worry about stretching.
func findSquares(extents ...MapCoords) []MapCoords {
	out := []MapCoords{}
	for _, mapExtents := range extents {
		width := 1 + mapExtents.Right - mapExtents.Left
		height := 1 + mapExtents.Top - mapExtents.Bottom

		// the map is not a square!
		if width < height {
			// Map is taller than it is wide. Chop into a top square and a bottom square.
			// The side length of the squares is the full width of the map.
			squareSize := width

			// Top square: uses the full width, extends down by squareSize
			topSquare := MapCoords{
				Top:    mapExtents.Top,
				Bottom: mapExtents.Top - squareSize + 1, // +1 for inclusive coordinate math
				Left:   mapExtents.Left,
				Right:  mapExtents.Right,
			}

			// Bottom square: uses the full width, extends up by squareSize
			bottomSquare := MapCoords{
				Top:    mapExtents.Bottom + squareSize - 1,
				Bottom: mapExtents.Bottom,
				Left:   mapExtents.Left,
				Right:  mapExtents.Right,
			}
			out = append(out, topSquare, bottomSquare)
		} else if width > height {
			// Map is wider than it is tall. Chop into a left square and a right square.
			// The side length of the squares is the full height of the map.
			squareSize := height

			// Left square: uses the full height, extends right by squareSize
			leftSquare := MapCoords{
				Top:    mapExtents.Top,
				Bottom: mapExtents.Bottom,
				Left:   mapExtents.Left,
				Right:  mapExtents.Left + squareSize - 1,
			}

			// Right square: uses the full height, extends left by squareSize
			rightSquare := MapCoords{
				Top:    mapExtents.Top,
				Bottom: mapExtents.Bottom,
				Left:   mapExtents.Right - squareSize + 1,
				Right:  mapExtents.Right,
			}
			out = append(out, leftSquare, rightSquare)
		} else {
			// If width == height, it is already a square. Return it.
			out = append(out, mapExtents)
		}
	}
	return out
}
