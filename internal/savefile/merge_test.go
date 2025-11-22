package savefile

import (
	"testing"
)

func pe(t uint64) *PathEntry { return &PathEntry{TimeStamp: t} }

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
			a:         &SaveData{Player: "p"},
			b:         &SaveData{Player: "p", Paths: []*PathEntry{pe(1), pe(2)}},
			wantPaths: []uint64{1, 2},
		},
		{
			name:      "B empty",
			a:         &SaveData{Player: "p", Paths: []*PathEntry{pe(1)}},
			b:         &SaveData{Player: "p"},
			wantPaths: []uint64{1},
		},

		// *** NEW RULE TESTS ***

		{
			name: "No overlap → straight append",
			// A: 1, 2
			// B: 5, 6
			// earliestB=5 → keep all A (<5)
			a:         &SaveData{Player: "p", Paths: []*PathEntry{pe(1), pe(2)}},
			b:         &SaveData{Player: "p", Paths: []*PathEntry{pe(5), pe(6)}},
			wantPaths: []uint64{1, 2, 5, 6},
		},

		{
			name: "B entirely precedes A → B then A",
			// B: 1, 2
			// A: 5, 6
			// b.latest < a.first → B + A with no truncation
			a:         &SaveData{Player: "p", Paths: []*PathEntry{pe(5), pe(6)}},
			b:         &SaveData{Player: "p", Paths: []*PathEntry{pe(1), pe(2)}},
			wantPaths: []uint64{1, 2, 5, 6},
		},

		{
			name: "Overlap → drop only overlapping suffix of A",
			// A: 1, 2, 3, 14, 15
			// B: 12, 13, 20, 21
			// earliestB=12
			// keep prefix of A only while <12 → [1,2,3]
			// drop [14,15]
			a: &SaveData{
				Player: "p",
				Paths:  []*PathEntry{pe(1), pe(2), pe(3), pe(14), pe(15)},
			},
			b: &SaveData{
				Player: "p",
				Paths:  []*PathEntry{pe(12), pe(13), pe(20), pe(21)},
			},
			wantPaths: []uint64{1, 2, 3, 12, 13, 20, 21},
		},

		{
			name: "Partial overlap → keep only prefix of A",
			// A: 1, 4, 7
			// B: 5, 6, 10
			// earliestB=5
			// keep A entries <5 → [1,4]
			// drop 7
			a:         &SaveData{Player: "p", Paths: []*PathEntry{pe(1), pe(4), pe(7)}},
			b:         &SaveData{Player: "p", Paths: []*PathEntry{pe(5), pe(6), pe(10)}},
			wantPaths: []uint64{1, 4, 5, 6, 10},
		},

		{
			name: "All A entries overlap B → result is just B",
			// A: 3, 6, 9, 12
			// B: 10, 11, 20
			// earliestB=10
			// keep A entries <10 → only [3,6,9]
			// but 12 is dropped
			// merged = 3,6,9,10,11,20
			a:         &SaveData{Player: "p", Paths: []*PathEntry{pe(3), pe(6), pe(9), pe(12)}},
			b:         &SaveData{Player: "p", Paths: []*PathEntry{pe(10), pe(11), pe(20)}},
			wantPaths: []uint64{3, 6, 9, 10, 11, 20},
		},

		{
			name: "A already safe → keep all A",
			// A: 1,2,3
			// B: 4,5
			// earliestB=4 → keep all A (<4)
			a:         &SaveData{Player: "p", Paths: []*PathEntry{pe(1), pe(2), pe(3)}},
			b:         &SaveData{Player: "p", Paths: []*PathEntry{pe(4), pe(5)}},
			wantPaths: []uint64{1, 2, 3, 4, 5},
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
				t.Fatalf("unexpected length: got %d want %d", len(got.Paths), len(tt.wantPaths))
			}
			for i, p := range got.Paths {
				if p.TimeStamp != tt.wantPaths[i] {
					t.Fatalf("paths[%d] = %d want %d", i, p.TimeStamp, tt.wantPaths[i])
				}
			}
		})
	}
}
