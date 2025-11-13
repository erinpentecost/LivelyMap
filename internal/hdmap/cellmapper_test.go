package hdmap

import (
	"testing"
)

func TestNormalToRGB(t *testing.T) {
	tests := []struct {
		name string
		in   [3]int8
		want [3]uint8
	}{
		{
			name: "zero vector",
			in:   [3]int8{0, 0, 0},
			want: [3]uint8{128, 128, 128},
		},
		{
			name: "positive extremes",
			in:   [3]int8{127, 127, 127},
			want: [3]uint8{255, 255, 255},
		},
		{
			name: "negative extremes",
			in:   [3]int8{-128, -128, -128},
			want: [3]uint8{0, 0, 0},
		},
		{
			name: "mixed signs",
			in:   [3]int8{-128, 0, 127},
			want: [3]uint8{0, 128, 255},
		},
		{
			name: "negative small values",
			in:   [3]int8{-1, -2, -3},
			want: [3]uint8{127, 126, 125},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for i := range tt.in {
				got := normalTransform(tt.in[i])
				if got != tt.want[i] {
					t.Errorf("NormalToRGB(%v) = %v, want %v", tt.in[i], got, tt.want[i])
				}
			}

		})
	}
}
