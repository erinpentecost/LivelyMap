package hdmap

type MapCoords struct {
	// Positive Y is North in cell grid coordinates.
	Top    int32
	Bottom int32
	Left   int32
	Right  int32
}
