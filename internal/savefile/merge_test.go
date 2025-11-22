package savefile

import (
	"testing"
)

func newPE(t uint64) *PathEntry {
	return &PathEntry{TimeStamp: t}
}

func TestMerge(t *testing.T) {
	tests := []struct {
		name      string
		a, b      *SaveData
		wantPaths []uint64
		wantErr   bool
	}{
		{
			name:    "both nil",
			a:       nil,
			b:       nil,
			wantErr: true,
		},
		{
			name:    "mismatched players",
			a:       &SaveData{Player: "A"},
			b:       &SaveData{Player: "B"},
			wantErr: true,
		},
		{
			name:      "A empty",
			a:         &SaveData{Player: "p", Paths: nil},
			b:         &SaveData{Player: "p", Paths: []*PathEntry{newPE(1), newPE(2)}},
			wantPaths: []uint64{1, 2},
		},
		{
			name:      "B empty",
			a:         &SaveData{Player: "p", Paths: []*PathEntry{newPE(3)}},
			b:         &SaveData{Player: "p"},
			wantPaths: []uint64{3},
		},
		{
			name:      "A ends before B starts → append A→B",
			a:         &SaveData{Player: "p", Paths: []*PathEntry{newPE(1), newPE(2)}},
			b:         &SaveData{Player: "p", Paths: []*PathEntry{newPE(3), newPE(4)}},
			wantPaths: []uint64{1, 2, 3, 4},
		},
		{
			name:      "B ends before A starts → append B→A",
			a:         &SaveData{Player: "p", Paths: []*PathEntry{newPE(5), newPE(6)}},
			b:         &SaveData{Player: "p", Paths: []*PathEntry{newPE(1), newPE(2)}},
			wantPaths: []uint64{1, 2, 5, 6},
		},
		{
			name: "Overlap → B minimally truncated",
			// A:       5, 10, 15
			// B:  1,  6, 11, 20
			// oldestA = 15
			// B entries <15 are [1,6,11] → index = 3
			// result = A + B[3:] = [5,10,15,20]
			a: &SaveData{
				Player: "p",
				Paths:  []*PathEntry{newPE(5), newPE(10), newPE(15)},
			},
			b: &SaveData{
				Player: "p",
				Paths:  []*PathEntry{newPE(1), newPE(6), newPE(11), newPE(20)},
			},
			wantPaths: []uint64{5, 10, 15, 20},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Merge(tt.a, tt.b)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got.Paths) != len(tt.wantPaths) {
				t.Fatalf("wrong path count: got %d want %d", len(got.Paths), len(tt.wantPaths))
			}
			for i, pe := range got.Paths {
				if pe.TimeStamp != tt.wantPaths[i] {
					t.Fatalf("paths[%d] = %d want %d", i, pe.TimeStamp, tt.wantPaths[i])
				}
			}
		})
	}
}
