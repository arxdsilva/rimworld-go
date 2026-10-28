package creature

import (
	"math"
	"testing"

	"rimworld-go/internal/application/game"
)

func TestCorpseFliesWhenRotting(t *testing.T) {
	t.Parallel()
	fresh := game.ItemView{ID: 9, Kind: "human corpse", X: 3, Y: 3}
	if len(CorpseFlies(fresh, 10)) != 0 {
		t.Fatal("a fresh body draws no flies")
	}
	bones := game.ItemView{ID: 9, Kind: "bones", Rotten: true}
	if len(CorpseFlies(bones, 10)) != 0 {
		t.Fatal("bones draw no flies")
	}
	rot := fresh
	rot.Rotten = true
	a := CorpseFlies(rot, 10)
	if len(a) < 2 || len(a) > 4 {
		t.Fatalf("a few flies: %d", len(a))
	}
	b := CorpseFlies(rot, 10)
	for i := range a {
		if a[i] != b[i] {
			t.Fatal("deterministic in item ID and frame")
		}
		if math.Hypot(a[i][0]-3.5, a[i][1]-3.5) > 0.7 {
			t.Fatalf("flies circle the body: %v", a[i])
		}
	}
	if c := CorpseFlies(rot, 40); c[0] == a[0] {
		t.Fatal("flies move")
	}
	other := rot
	other.ID = 10
	if CorpseFlies(other, 10)[0] == a[0] {
		t.Fatal("seeded by the item ID")
	}
}

func TestHeadstoneVariant(t *testing.T) {
	t.Parallel()
	if HeadstoneSprite(game.BuildingView{Kind: "grave"}) != "" {
		t.Fatal("no headstone on an empty grave")
	}
	for i, want := range []string{"furniture/headstone_00", "furniture/headstone_01", "furniture/headstone_02", "furniture/headstone_03"} {
		if got := HeadstoneSprite(game.BuildingView{Occupant: "Ada", Headstone: i}); got != want {
			t.Fatalf("style %d: %s", i, got)
		}
	}
	if HeadstoneSprite(game.BuildingView{Occupant: "A", Headstone: 9}) != "furniture/headstone_03" ||
		HeadstoneSprite(game.BuildingView{Occupant: "A", Headstone: -1}) != "furniture/headstone_00" {
		t.Fatal("style clamps to 0..3")
	}
	// the head end: top of a south grave, left end of an east grave
	for _, f := range []string{"", "south", "east"} {
		b := game.BuildingView{X: 4, Y: 7, Facing: f}
		if x, y := HeadstoneTile(b); x != 4 || y != 7 {
			t.Fatalf("%q: head end at %v,%v", f, x, y)
		}
	}
	if x, y := HeadstoneTile(game.BuildingView{X: 4, Y: 7, W: 2, H: 1, Facing: "west"}); x != 5 || y != 7 {
		t.Fatalf("west grave: head at the east end, got %v,%v", x, y)
	}
	if x, y := HeadstoneTile(game.BuildingView{X: 4, Y: 7, W: 1, H: 2, Facing: "north"}); x != 4 || y != 8 {
		t.Fatalf("north grave: head at the south end, got %v,%v", x, y)
	}
}
