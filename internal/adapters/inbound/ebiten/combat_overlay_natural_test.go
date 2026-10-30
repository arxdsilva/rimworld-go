package ebiten

import (
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/render/overlay"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/world/terrain"
)

// TestNaturalCoverCells (CBT-5): trees, rock and chunks join the cover
// overlay with the cover domain's percent; saplings and far plants do not.
func TestNaturalCoverCells(t *testing.T) {
	t.Parallel()
	w := game.WorldView{TerrainW: 4, Terrain: make([]uint8, 16)}
	w.Terrain[1*4+1] = uint8(terrain.Rock)
	w.Plants = []game.PlantView{{X: 2, Y: 2, Species: "tree", Growth: 100}, {X: 3, Y: 3, Species: "tree", Growth: 10}, {X: 90, Y: 90, Species: "tree", Growth: 100}}
	w.Items = []game.ItemView{{X: 0, Y: 3, Kind: "granite chunk", Qty: 1}, {X: 0, Y: 2, Kind: "steel", Qty: 1}}
	got := naturalCoverCells(nil, &w, 2, 2)
	want := map[overlay.CoverCell]bool{{X: 1, Y: 1, Value: 75}: true, {X: 2, Y: 2, Value: 25}: true, {X: 0, Y: 3, Value: 50}: true}
	if len(got) != len(want) {
		t.Fatalf("cells %+v", got)
	}
	for _, c := range got {
		if !want[c] {
			t.Fatalf("unexpected %+v in %+v", c, got)
		}
	}
}

// TestAimCoverIsDirectional: a wall beside the target counts in full only
// for a shot straight along it, and not at all from behind it.
func TestAimCoverIsDirectional(t *testing.T) {
	t.Parallel()
	mk := func() []overlay.CoverCell {
		return []overlay.CoverCell{{X: 11, Y: 10, Value: 75}, {X: 50, Y: 50, Value: 75}}
	}
	front := mk()
	aimCover(front, 20, 10, 10, 10) // shooter east of the target, wall east of it
	if front[0].Value != 75 || front[1].Value != 75 {
		t.Fatalf("front %+v", front)
	}
	behind := mk()
	aimCover(behind, 0, 10, 10, 10)
	if behind[0].Value != 0 || behind[1].Value != 75 {
		t.Fatalf("behind %+v", behind)
	}
}
