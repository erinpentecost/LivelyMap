package hdmap

import (
	"fmt"
)

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

func (m MapCoords) Width() int32 {
	return 1 + m.Right - m.Left
}
func (m MapCoords) Height() int32 {
	return 1 + m.Top - m.Bottom
}

func (m MapCoords) Extend(height int32, width int32) MapCoords {
	return MapCoords{
		Top:    m.Top + height/2,
		Bottom: m.Bottom - height + height/2,
		Left:   m.Left - width + width/2,
		Right:  m.Right + width/2,
	}
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
