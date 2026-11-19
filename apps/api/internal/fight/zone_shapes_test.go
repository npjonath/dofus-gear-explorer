package fight

import (
	"slices"
	"testing"

	"dofusdb/api/internal/game"
)

func TestDirectionalZoneFootprintsOnAstrub(t *testing.T) {
	b := Board{Width: 10, Height: 15}
	for _, tc := range []struct {
		shape                        string
		size, second, caster, target int
		want                         []int
	}{
		{"U", 1, 0, 63, 62, []int{53, 62, 73}},
		{"U", 1, 0, 63, 73, []int{62, 64, 73}},
		{"U", 2, 0, 63, 64, []int{42, 53, 64, 73, 82}},
		{"F", 1, 0, 63, 64, []int{55, 64, 65, 75}},
		{"R", 1, 3, 63, 64, []int{54, 55, 56, 57, 64, 65, 66, 67, 74, 75, 76, 77}},
		{"U", 1, 0, 11, 0, []int{0, 1, 10}},
	} {
		got := b.zone(game.Zone{Shape: tc.shape, Size: tc.size, Min: tc.second}, tc.caster, tc.target)
		slices.Sort(got)
		if !slices.Equal(got, tc.want) {
			t.Errorf("%s %d→%d: got %v, want %v", tc.shape, tc.caster, tc.target, got, tc.want)
		}
	}
}
