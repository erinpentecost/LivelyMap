package hdmap

import "fmt"

type Direction string

const (
	North Direction = "north"
	South Direction = "south"
	West  Direction = "west"
	East  Direction = "east"
)

type SubmapID int

type SubmapNode struct {
	Extents     MapCoords
	ID          SubmapID
	ConnectedTo map[Direction]SubmapID
	CenterX     int32
	CenterY     int32
}

func connectMapCoords(coords []MapCoords) []SubmapNode {
	nodes := make([]SubmapNode, len(coords))

	// Precompute centers and initialize nodes
	for i := range coords {
		cx, cy := coords[i].Center()

		nodes[i] = SubmapNode{
			Extents:     coords[i],
			ID:          SubmapID(i),
			ConnectedTo: make(map[Direction]SubmapID),
			CenterX:     cx,
			CenterY:     cy,
		}
	}

	sq := func(a int32) int64 { return int64(a) * int64(a) }

	for i, c := range nodes {
		cx, cy := c.CenterX, c.CenterY

		type best struct {
			id    int
			dist2 int64
			found bool
		}

		var north, south, left, right best

		for j, nn := range nodes {
			if j == i {
				continue
			}

			ox := nn.CenterX
			oy := nn.CenterY

			dx := ox - cx
			dy := oy - cy
			dist2 := sq(dx) + sq(dy)

			// NORTH
			if oy > cy {
				if !north.found || dist2 < north.dist2 {
					north = best{id: j, dist2: dist2, found: true}
				}
			}

			// SOUTH
			if oy < cy {
				if !south.found || dist2 < south.dist2 {
					south = best{id: j, dist2: dist2, found: true}
				}
			}

			// RIGHT
			if ox > cx {
				if !right.found || dist2 < right.dist2 {
					right = best{id: j, dist2: dist2, found: true}
				}
			}

			// LEFT
			if ox < cx {
				if !left.found || dist2 < left.dist2 {
					left = best{id: j, dist2: dist2, found: true}
				}
			}
		}

		if north.found {
			nodes[i].ConnectedTo[North] = SubmapID(north.id)
		}
		if south.found {
			nodes[i].ConnectedTo[South] = SubmapID(south.id)
		}
		if left.found {
			nodes[i].ConnectedTo[West] = SubmapID(left.id)
		}
		if right.found {
			nodes[i].ConnectedTo[East] = SubmapID(right.id)
		}
	}

	return nodes
}

type MapCoords struct {
	// Positive direction, North. Inclusive.
	Top int32
	// Negative direction, South. Inclusive.
	Bottom int32
	// Negative direction, West. Inclusive.
	Left int32
	// Positive direction, East. Inclusive.
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
func (m MapCoords) Center() (x int32, y int32) {
	return (m.Left + m.Right) / 2, (m.Bottom + m.Top) / 2
}

func (m MapCoords) NotContainsPoint(x int32, y int32) bool {
	return x < m.Left || x > m.Right || y < m.Bottom || y > m.Top
}

func (a MapCoords) SupersetOf(b MapCoords) bool {
	// Vertical Containment (Top/Bottom):
	// B's northern edge (Top) must be south of or equal to A's northern edge.
	// B's southern edge (Bottom) must be north of or equal to A's southern edge.
	verticalContained := b.Top <= a.Top && b.Bottom >= a.Bottom

	// Horizontal Containment (Left/Right):
	// B's western edge (Left) must be east of or equal to A's western edge.
	// B's eastern edge (Right) must be west of or equal to A's eastern edge.
	horizontalContained := b.Left >= a.Left && b.Right <= a.Right

	return verticalContained && horizontalContained
}

func (a MapCoords) quadrants() []MapCoords {
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
	childSide := (2*side + 2) / 3 // integer

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
		// Vvardenfell is 43x41.
		if width*height <= 43*41 {
			partitions = append(partitions, square)
			continue
		}
		// It's a big square, so we're going to chunk it up
		// into overlapping sub-squares.
		for _, quad := range square.quadrants() {
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
