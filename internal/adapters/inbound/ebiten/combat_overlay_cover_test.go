package ebiten

import (
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/render/overlay"
	"rimworld-go/internal/application/game"
)

// TestCoverCellsUseCoverDomain (DEF-3): the cover overlay shows the cover
// domain's values in percent: wall 75, sandbags and barricade 55, bed 40;
// floors and far buildings none.
func TestCoverCellsUseCoverDomain(t *testing.T) {
	t.Parallel()
	bs := []game.BuildingView{
		{Kind: "wall", X: 1, Y: 1}, {Kind: "sandbags", X: 2, Y: 1}, {Kind: "barricade", X: 3, Y: 1},
		{Kind: "bed", X: 4, Y: 1}, {Kind: "powered door", X: 5, Y: 1}, {Kind: "floor", X: 6, Y: 1, Floor: true},
		{Kind: "wall", X: 90, Y: 90}, {Kind: "spike trap", X: 1, Y: 2},
	}
	got := coverCells(nil, bs, 2, 2)
	want := []overlay.CoverCell{{X: 1, Y: 1, Value: 75}, {X: 2, Y: 1, Value: 55}, {X: 3, Y: 1, Value: 55}, {X: 4, Y: 1, Value: 40}, {X: 5, Y: 1, Value: 75}}
	if len(got) != len(want) {
		t.Fatalf("cells %+v, want %+v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("cell %d %+v, want %+v", i, got[i], want[i])
		}
	}
}
