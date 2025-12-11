package hdmap

import (
	"testing"
)

// ---------------- HELPERS ----------------

func isPowerOfTwo(n int32) bool {
	return n > 0 && (n&(n-1)) == 0
}

func sideLength(mc MapCoords) int32 {
	w := mc.Right - mc.Left + 1
	h := mc.Top - mc.Bottom + 1
	if w != h {
		return -1
	}
	return w
}

func coversHorizontally(parent, child MapCoords) bool {
	return child.Left <= parent.Left && child.Right >= parent.Right
}

func coversVertically(parent, child MapCoords) bool {
	return child.Bottom <= parent.Bottom && child.Top >= parent.Top
}

// ---------------- TESTS ----------------

func TestPartition2_SimpleSquare(t *testing.T) {
	parent := MapCoords{Left: 0, Bottom: 0, Right: 15, Top: 15} // 16x16

	tiles := Partition2(parent)
	if len(tiles) == 0 {
		t.Fatal("expected at least one tile")
	}

	for _, tile := range tiles {
		side := sideLength(tile.Extents)
		if side == -1 {
			t.Fatalf("tile is not square: %+v", tile.Extents)
		}
		if !isPowerOfTwo(side) {
			t.Fatalf("tile side %d is not power of two: %+v", side, tile.Extents)
		}
	}
}

func TestPartition2_TallMap(t *testing.T) {
	parent := MapCoords{Left: 0, Bottom: 0, Right: 9, Top: 29} // 10 wide, 30 tall

	tiles := Partition2(parent)
	if len(tiles) == 0 {
		t.Fatal("expected non-empty result")
	}
	if len(tiles) > 8 {
		t.Fatalf("expected at most 8 tiles, got %d", len(tiles))
	}

	// All tiles should be square and power-of-two sized
	for _, tile := range tiles {
		side := sideLength(tile.Extents)
		if side == -1 {
			t.Fatalf("tile not square: %+v", tile.Extents)
		}
		if !isPowerOfTwo(side) {
			t.Fatalf("tile side %d not power of two: %+v", side, tile.Extents)
		}
	}

	// Ensure vertical coverage
	topCovered := int32(-1 << 31)
	bottomCovered := int32(1<<31 - 1)
	for _, tile := range tiles {
		if tile.Extents.Top > topCovered {
			topCovered = tile.Extents.Top
		}
		if tile.Extents.Bottom < bottomCovered {
			bottomCovered = tile.Extents.Bottom
		}
	}
	if topCovered < parent.Top || bottomCovered > parent.Bottom {
		t.Fatalf("tiles do not fully cover vertical range: topCovered=%d, bottomCovered=%d", topCovered, bottomCovered)
	}
}

func TestPartition2_WideMap(t *testing.T) {
	parent := MapCoords{Left: 0, Bottom: 0, Right: 29, Top: 9} // 30 wide, 10 tall

	tiles := Partition2(parent)
	if len(tiles) == 0 {
		t.Fatal("expected non-empty result")
	}
	if len(tiles) > 8 {
		t.Fatalf("expected at most 8 tiles, got %d", len(tiles))
	}

	// All tiles square and power-of-two
	for _, tile := range tiles {
		side := sideLength(tile.Extents)
		if side == -1 {
			t.Fatalf("tile not square: %+v", tile.Extents)
		}
		if !isPowerOfTwo(side) {
			t.Fatalf("tile side %d not power of two: %+v", side, tile.Extents)
		}
	}

	// Ensure horizontal coverage
	horizontalCovered := false
	for _, tile := range tiles {
		if coversHorizontally(parent, tile.Extents) {
			horizontalCovered = true
			break
		}
	}
	if !horizontalCovered {
		t.Fatal("expected at least one tile to cover full width")
	}
}

func TestPartition2_OddDimensions(t *testing.T) {
	parent := MapCoords{Left: 0, Bottom: 0, Right: 14, Top: 20} // 15x21

	tiles := Partition2(parent)
	if len(tiles) == 0 {
		t.Fatal("expected non-empty result")
	}

	for _, tile := range tiles {
		side := sideLength(tile.Extents)
		if side == -1 {
			t.Fatalf("tile not square: %+v", tile.Extents)
		}
		if !isPowerOfTwo(side) {
			t.Fatalf("tile side %d not power of two: %+v", side, tile.Extents)
		}
	}
}

func TestPartition2_MaxTilesEight(t *testing.T) {
	parent := MapCoords{Left: 0, Bottom: 0, Right: 63, Top: 63} // large square

	tiles := Partition2(parent)
	if len(tiles) > 8 {
		t.Fatalf("expected at most 8 tiles, got %d", len(tiles))
	}
}
