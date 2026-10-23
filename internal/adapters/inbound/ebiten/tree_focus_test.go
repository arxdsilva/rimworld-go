package ebiten

import (
	"slices"
	"testing"
)

// TestTreeFocusCells: the hovered tile, the selected thing's tile and the
// valid Architect ghost tiles all count as focus (canopies over them turn
// see-through, ART §9.2); an off-map hover adds nothing; the ghost is
// capped so a huge drag stays cheap.
func TestTreeFocusCells(t *testing.T) {
	t.Parallel()
	got := treeFocusCells(nil, [2]int{4, 5}, true, [][2]int{{9, 9}},
		[]ghostCell{{x: 1, y: 1, ok: true}, {x: 2, y: 1}})
	want := [][2]int{{4, 5}, {9, 9}, {1, 1}, {2, 1}}
	if !slices.Equal(got, want) {
		t.Fatalf("focus = %v, want %v", got, want)
	}
	if got := treeFocusCells(nil, [2]int{}, false, nil, nil); len(got) != 0 {
		t.Fatalf("no hover, selection or ghost: %v", got)
	}
	big := make([]ghostCell, 500)
	if got := treeFocusCells(nil, [2]int{}, false, nil, big); len(got) != treeFocusMaxGhost {
		t.Fatalf("ghost focus not capped: %d", len(got))
	}
}
