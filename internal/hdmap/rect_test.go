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
