package hdmap

import "fmt"

type MapCoords struct {
	// Positive direction, North.
	Top int32
	// Negative direction, South.
	Bottom int32
	// Negative direction, West.
	Left int32
	// Positive direction, East.
	Right int32
}

func (m MapCoords) String() string {
	// this is formatted this way so lua can read it better.
	return fmt.Sprintf("%d_%d_%d_%d",
		m.Bottom,
		m.Left,
		m.Top,
		m.Right,
	)
}

func (m MapCoords) NotContains(x int32, y int32) bool {
	return x < m.Left || x > m.Right || y < m.Bottom || y > m.Top
}

// We're actually going to fit the two largest squares
// we can into the map, and make images for both.
// This way we can render them onto squar meshes and not worry about stretching.
func FindSquares(mapExtents MapCoords) []MapCoords {
	// Vvardenfell is 43x41.
	width := 1 + mapExtents.Right - mapExtents.Left
	height := 1 + mapExtents.Top - mapExtents.Bottom

	// the map is too big!
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

		return []MapCoords{topSquare, bottomSquare}

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

		return []MapCoords{leftSquare, rightSquare}
	}

	// If width == height, it is already a square. Return it.
	return []MapCoords{mapExtents}
}
