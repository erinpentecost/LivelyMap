package hdmap

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// Helper to check if two MapCoords are equal
func coordsEqual(a, b MapCoords) bool {
	return a.Top == b.Top &&
		a.Bottom == b.Bottom &&
		a.Left == b.Left &&
		a.Right == b.Right
}

// Helper to calculate size for verification
func getSize(c MapCoords) (width, height int32) {
	return c.Right - c.Left + 1, c.Top - c.Bottom + 1
}

func TestFindSquares_ChoppingCase(t *testing.T) {
	// Map size: 100 wide, 200 tall. (Top: 100, Bottom: -99, Left: -50, Right: 49)
	tallMap := MapCoords{Top: 100, Bottom: -99, Left: -50, Right: 49}
	// W=100, H=200. Square size will be 100x100.
	// Expected: Top Square (100, 1) and Bottom Square (0, -99).
	expectedTall := []MapCoords{
		{Top: 100, Bottom: 1, Left: -50, Right: 49}, // Top 100x100
		{Top: 0, Bottom: -99, Left: -50, Right: 49}, // Bottom 100x100
	}

	// Map size: 200 wide, 100 tall. (Top: 50, Bottom: -49, Left: -100, Right: 99)
	wideMap := MapCoords{Top: 50, Bottom: -49, Left: -100, Right: 99}
	// W=200, H=100. Square size will be 100x100.
	// Expected: Left Square (-100, -1) and Right Square (0, 99).
	expectedWide := []MapCoords{
		{Top: 50, Bottom: -49, Left: -100, Right: -1}, // Left 100x100
		{Top: 50, Bottom: -49, Left: 0, Right: 99},    // Right 100x100
	}

	tests := []struct {
		name     string
		input    MapCoords
		expected []MapCoords
	}{
		{"Tall Map (200x100)", tallMap, expectedTall},
		{"Wide Map (100x200)", wideMap, expectedWide},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := findSquares(tt.input)
			if len(result) != 2 {
				t.Fatalf("Expected 2 results for large map, got %d", len(result))
			}
			require.Equal(t, tt.expected, result)

			// Verify that both results are squares
			w0, h0 := getSize(result[0])
			w1, h1 := getSize(result[1])

			if w0 != h0 || w1 != h1 {
				t.Errorf("One or both results are not squares: (%dx%d) and (%dx%d)", w0, h0, w1, h1)
			}
		})
	}
}

func TestFindSquares_PerfectSquare(t *testing.T) {
	// Scenario 3: Large perfect square (100x100)
	perfectSquareMap := MapCoords{Top: 50, Bottom: -49, Left: -50, Right: 49}

	t.Run("Perfect Square Map (100x100)", func(t *testing.T) {
		result := findSquares(perfectSquareMap)
		if len(result) != 1 {
			t.Fatalf("Expected 1 result for perfect square map, got %d", len(result))
		}
		if !coordsEqual(result[0], perfectSquareMap) {
			t.Errorf("Expected %v, got %v", perfectSquareMap, result[0])
		}
	})
}

func TestConnectMapCoords(t *testing.T) {
	// Index layout (Top increases upward):
	//
	//       [0] North
	// [2] Left   [4] Right
	//       [3] South
	//
	coords := []MapCoords{
		{Top: 20, Bottom: 10, Left: -5, Right: 5},  // 0 = North
		{Top: 10, Bottom: 0, Left: -5, Right: 5},   // 1 = Center
		{Top: 10, Bottom: 0, Left: -15, Right: -5}, // 2 = Left
		{Top: 0, Bottom: -10, Left: -5, Right: 5},  // 3 = South
		{Top: 10, Bottom: 0, Left: 5, Right: 15},   // 4 = Right
	}

	nodes := connectMapCoords(coords)

	// Check center connections
	center := nodes[1]

	check := func(dir Direction, want SubmapID) {
		n, ok := center.ConnectedTo[dir]
		if !ok {
			t.Fatalf("missing %v connection", dir)
		}
		if n != want {
			t.Fatalf("wrong %v connection: got %d want %d", dir, n, want)
		}
	}

	check(North, 0)
	check(South, 3)
	check(West, 2)
	check(East, 4)
}

func TestMapCoordsUtilities(t *testing.T) {
	m := MapCoords{
		Top:    10,
		Bottom: 0,
		Left:   -5,
		Right:  5,
	}

	// String formatting
	if got := m.String(); got != "0_-5_10_5" {
		t.Fatalf("String() wrong: got %q want %q", got, "0_-5_10_5")
	}

	// Center
	x, y := m.Center()
	if x != 0 || y != 5 {
		t.Fatalf("Center() wrong: got (%d,%d), want (0,5)", x, y)
	}

	// Containment
	if m.NotContainsPoint(0, 5) {
		t.Fatalf("NotContainsPoint incorrectly returned true for interior point")
	}
	if !m.NotContainsPoint(100, 5) {
		t.Fatalf("NotContainsPoint incorrectly returned false for exterior point")
	}

	// Superset
	small := MapCoords{
		Top:    8,
		Bottom: 2,
		Left:   -3,
		Right:  3,
	}
	if !m.SupersetOf(small) {
		t.Fatalf("SupersetOf: expected large to contain small")
	}

	// Negative case
	overhang := MapCoords{
		Top:    12, // sticks out above
		Bottom: 2,
		Left:   -3,
		Right:  3,
	}
	if m.SupersetOf(overhang) {
		t.Fatalf("SupersetOf: incorrectly returned true for non-contained region")
	}
}

func TestPartitionAndConnectivity(t *testing.T) {
	// A deliberately uneven rectangle forcing findSquares() to split it.
	// Width = 20, height = 10 → two vertical squares each size 10.
	m := MapCoords{
		Top:    10,
		Bottom: 0,
		Left:   0,
		Right:  19,
	}

	parts := Partition(m)

	if len(parts) == 0 {
		t.Fatalf("Partition returned zero parts")
	}

	// Every returned node should:
	// 1. Be a square
	// 2. Have correct 1:1 IDs
	for _, node := range parts {
		w := 1 + node.Extents.Right - node.Extents.Left
		h := 1 + node.Extents.Top - node.Extents.Bottom
		if w != h {
			t.Fatalf("Partition returned non-square region: %v (w=%d h=%d)", node.Extents, w, h)
		}
	}

	// Connectivity sanity: each node should have at least one connection
	// unless it's an isolated single-node partition (not possible here).
	missing := 0
	for _, node := range parts {
		if len(node.ConnectedTo) == 0 {
			missing++
		}
	}
	if missing > 0 {
		t.Fatalf("Expected all partition nodes to have at least one connection, missing=%d", missing)
	}
}
