package hdmap

import "testing"

func TestEnsureAspectRatio(t *testing.T) {
	tests := []struct {
		in   MapCoords
		want int32 // maximum allowed ratio <=2
	}{
		{MapCoords{Left: 0, Bottom: 0, Right: 9, Top: 4}, 10}, // width 10, height 5 -> fine
		{MapCoords{Left: 0, Bottom: 0, Right: 9, Top: 2}, 10}, // width 10, height 3 -> extend height
		{MapCoords{Left: 0, Bottom: 0, Right: 2, Top: 9}, 10}, // width 3, height 10 -> extend width
		{MapCoords{Left: 0, Bottom: 0, Right: 5, Top: 5}, 6},  // square -> unchanged
	}

	for _, tt := range tests {
		got := ensureAspectRatio(tt.in)
		w := got.Width()
		h := got.Height()
		if w > 2*h || h > 2*w {
			t.Errorf("ensureAspectRatio(%+v) = %+v has bad ratio w/h = %d/%d", tt.in, got, w, h)
		}
	}
}
