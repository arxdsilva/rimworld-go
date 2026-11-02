package game

import (
	"strings"
	"testing"

	"rimworld-go/internal/application/game/axview"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/research"
)

// TestAxContractPlannedKindsCannotBeBuilt (AX-0): a Planned kind and an
// Unbuildable one place no blueprint, while a live kind on the same cell
// does; every AX research project unlocks only AX kinds (each Planned, so
// unbuildable, until its owner makes it real).
func TestAxContractPlannedKindsCannotBeBuilt(t *testing.T) {
	t.Parallel()
	g := NewColony(Config{Seed: 7, Width: 40, Height: 40})
	x, y := -1, -1
	for cy := 2; cy < 38 && x < 0; cy++ {
		for cx := 2; cx < 38; cx++ {
			if ok, _ := g.CanPlaceAt("stool", "wood", "south", cx, cy); ok {
				x, y = cx, cy
				break
			}
		}
	}
	if x < 0 {
		t.Fatal("no free cell")
	}
	n := len(g.frameIDs)
	for k := construction.Kind(0); k.Valid(); k++ { // owners unplan their kinds as they go live (AX-2 on)
		if !k.Planned() && !construction.DefOf(k).Unbuildable {
			continue
		}
		g.placeFrameStuff(k, construction.DefaultStuff(k), construction.South, x, y)
		if len(g.frameIDs) != n {
			t.Fatalf("%s placed a blueprint while planned", k)
		}
	}
	g.placeFrameStuff(construction.Stool, item.Wood, construction.South, x, y)
	if len(g.frameIDs) != n+1 {
		t.Fatal("a live kind places its blueprint")
	}
	for p := research.WaterPower; p < research.NumProjects; p++ {
		for _, u := range research.DefOf(p).Unlocks {
			if !strings.HasPrefix(u, "build:") {
				continue // AX-5: Fire foam also unlocks the foam shell recipe
			}
			k, ok := construction.Parse(u[len("build:"):])
			if !ok || k <= construction.MultiAnalyzer {
				t.Errorf("%s unlocks %q, not an AX kind", research.DefOf(p).Name, u)
			}
		}
	}
	if (BuildingView{}).AX != (axview.Building{}) {
		t.Fatal("the AX building view starts zero")
	}
	if SaveVersion != 11 {
		t.Fatalf("save version %d, want 11 (AX-0)", SaveVersion)
	}
}
