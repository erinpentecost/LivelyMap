package hdmap

// Partition2 returns square sub-extents (power-of-two side lengths) that
// cover m. Tiles overlap by at least 2 cells. We return at most 8 tiles.
func Partition2(m MapCoords) []SubmapNode {
	const overlap = int32(2) // required overlap in cells

	W := int(m.Width())
	H := int(m.Height())

	// quick degenerate case: tiny map -> single tile sized to next power of two >= max(W,H)
	if W <= 0 || H <= 0 {
		return connectMapCoords([]MapCoords{m})
	}

	// list all candidate powers-of-two S (descending) to try
	// maxDim = max(W,H)
	maxDim := W
	if H > maxDim {
		maxDim = H
	}

	// build power-of-two list up to maxDim
	pows := []int{}
	for p := 1; p <= maxDim*2; p <<= 1 { // allow slightly larger powers to reduce tile count
		pows = append(pows, p)
	}
	// reverse so we try largest first
	for i, j := 0, len(pows)-1; i < j; i, j = i+1, j-1 {
		pows[i], pows[j] = pows[j], pows[i]
	}

	var chosenS int
	var chosenNX, chosenNY int

tryPows:
	for _, S := range pows {
		// step (stride) must be at least 1
		step := S - int(overlap)
		if step < 1 {
			step = 1
		}

		nx := intCeilDiv(W-S, step) + 1
		ny := intCeilDiv(H-S, step) + 1
		if nx <= 0 {
			nx = 1
		}
		if ny <= 0 {
			ny = 1
		}
		if nx*ny <= 8 {
			chosenS = S
			chosenNX = nx
			chosenNY = ny
			break tryPows
		}
	}

	// If nothing chosen (very unlikely), fall back to S=1
	if chosenS == 0 {
		chosenS = 1
		chosenNX = intCeilDiv(W-chosenS, 1) + 1
		chosenNY = intCeilDiv(H-chosenS, 1) + 1
		if chosenNX*chosenNY > 8 {
			// cap to 8 by increasing step (reduce count)
			// naive reduction: make nx = min(nx,8), ny = 1 (best-effort)
			if chosenNX > 8 {
				chosenNX = 8
			}
			if chosenNX*chosenNY > 8 {
				chosenNY = 1
			}
		}
	}

	S := int32(chosenS)
	step := int32(chosenS) - overlap
	if step < 1 {
		step = 1
	}

	// Compute starting origin for X and Y such that the tiles cover [Left,Right] and [Bottom,Top].
	// We compute an initial start that will place (count-1) steps before the final tile ends at
	// start + (count-1)*step + S - 1 >= Right (inclusive). Solve for start:
	// start = Right - (S-1) - (count-1)*step
	startX := m.Right - (S - 1) - int32(chosenNX-1)*step
	if startX > m.Left {
		// ok
	} else {
		startX = m.Left
	}

	startY := m.Top - (S - 1) - int32(chosenNY-1)*step // bottoms grow upward; we'll use bottoms
	if startY < m.Bottom {
		startY = m.Bottom
	}

	tiles := make([]MapCoords, 0, chosenNX*chosenNY)
	for ix := 0; ix < chosenNX; ix++ {
		left := startX + int32(ix)*step
		right := left + S - 1
		for iy := 0; iy < chosenNY; iy++ {
			bottom := startY + int32(iy)*step
			top := bottom + S - 1

			tile := MapCoords{
				Left:   left,
				Right:  right,
				Bottom: bottom,
				Top:    top,
			}
			tiles = append(tiles, tile)
		}
	}

	// Remove tiles that don't intersect the original map at all (defensive).
	intersecting := []MapCoords{}
	for _, t := range tiles {
		if !(t.Right < m.Left || t.Left > m.Right || t.Top < m.Bottom || t.Bottom > m.Top) {
			intersecting = append(intersecting, t)
		}
	}

	// Remove tiles wholly contained inside another tile (dedupe)
	final := []MapCoords{}
	for i, a := range intersecting {
		contained := false
		for j, b := range intersecting {
			if i == j {
				continue
			}
			if b.SupersetOf(a) {
				contained = true
				break
			}
		}
		if !contained {
			final = append(final, a)
		}
	}

	// Safety cap to 8 if still >8 (trim smallest-area tiles last)
	if len(final) > 8 {
		// sort by area descending (prefer bigger tiles) - simple selection
		type kv struct {
			m    MapCoords
			area int64
		}
		kvs := make([]kv, 0, len(final))
		for _, mm := range final {
			area := int64(mm.Width()) * int64(mm.Height())
			kvs = append(kvs, kv{m: mm, area: area})
		}
		// simple selection sort to pick top 8 by area
		selected := make([]MapCoords, 0, 8)
		for k := 0; k < 8; k++ {
			bestIdx := -1
			var bestArea int64
			for i := range kvs {
				if kvs[i].area == 0 {
					continue
				}
				if bestIdx == -1 || kvs[i].area > bestArea {
					bestIdx = i
					bestArea = kvs[i].area
				}
			}
			if bestIdx == -1 {
				break
			}
			selected = append(selected, kvs[bestIdx].m)
			kvs[bestIdx].area = 0 // mark used
		}
		final = selected
	}

	return connectMapCoords(final)
}

func intCeilDiv(a, b int) int {
	if b <= 0 {
		return 0
	}
	if a <= 0 {
		return 1
	}
	return (a + b - 1) / b
}
