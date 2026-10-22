package game

import (
	"testing"

	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/world/terrain"
)

// buildFloorUnder places a floor blueprint on (x, y), gives a colonist the
// wood and waits for the floor to be built.
func buildFloorUnder(t *testing.T, g *Game, x, y int) {
	t.Helper()
	if ok, r := g.CanPlaceAt("floor", "wood", "south", x, y); !ok {
		t.Fatalf("floor at %d,%d refused: %q", x, y, r)
	}
	g.Enqueue(command.Command{Kind: command.DesignateBuild, Build: uint8(construction.Floor), X: x, Y: y})
	id, _ := g.SpawnNamedPawn(1, 1, "Ada")
	g.SpawnStack(item.Wood, 10, 1, 2)
	for i := 0; i < 400 && g.layerBldgAt(x, y, construction.LayerFloor) == 0; i++ {
		g.SetNeeds(id, needs.Full, needs.Full)
		g.Step(TicksPerCell * 10)
	}
	if g.layerBldgAt(x, y, construction.LayerFloor) == 0 {
		t.Fatalf("floor at %d,%d never built", x, y)
	}
}

// bothAt reports the World view lists a floor and kind on (x, y).
func bothAt(g *Game, kind string, x, y int) (floor, obj bool) {
	for _, b := range g.World().Buildings {
		if b.X == x && b.Y == y && b.Floor && b.Kind == "floor" {
			floor = true
		}
		if b.X == x && b.Y == y && b.Kind == kind {
			obj = true
		}
	}
	return floor, obj
}

func TestFloorUnderBed(t *testing.T) {
	g := New(Config{Seed: 1, Width: 14, Height: 8})
	placeBuilt(t, g, construction.Bed, construction.South, 7, 3)
	buildFloorUnder(t, g, 7, 3)
	if f, b := bothAt(g, "bed", 7, 3); !f || !b {
		t.Fatalf("floor %v, bed %v on 7,3", f, b)
	}
}

func TestFloorUnderWorkbench(t *testing.T) {
	g := New(Config{Seed: 1, Width: 14, Height: 8})
	placeBuilt(t, g, construction.Workbench, construction.South, 7, 3)
	if g.grid.Walkable(7, 3) {
		t.Fatal("a workbench blocks its tile")
	}
	buildFloorUnder(t, g, 7, 3)
	if f, b := bothAt(g, "workbench", 7, 3); !f || !b {
		t.Fatalf("floor %v, workbench %v on 7,3", f, b)
	}
	if g.grid.Walkable(7, 3) {
		t.Fatal("the bench still blocks its tile over the new floor")
	}
}

func TestNoFloorInWallOrRock(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 14, Height: 8})
	placeBuilt(t, g, construction.Wall, construction.South, 3, 3)
	g.placeFrameStuff(construction.Wall, item.Wood, construction.South, 4, 3)
	placeBuilt(t, g, construction.Fence, construction.South, 5, 3)
	g.placeFrameStuff(construction.FenceGate, item.Wood, construction.South, 6, 3)
	g.terr.Set(8, 3, terrain.Rock)
	for _, c := range []struct {
		x      int
		reason string
	}{{3, ReasonBlocked}, {4, ReasonBlocked}, {5, ReasonBlocked}, {6, ReasonBlocked}, {8, ReasonRock}} {
		if ok, r := g.CanPlaceAt("floor", "wood", "south", c.x, 3); ok || r != c.reason {
			t.Fatalf("floor at %d,3: %v %q, want %q", c.x, ok, r, c.reason)
		}
	}
	// doors and chairs take a floor
	placeBuilt(t, g, construction.Door, construction.South, 10, 3)
	placeBuilt(t, g, construction.Chair, construction.South, 11, 3)
	for _, x := range []int{10, 11} {
		if ok, r := g.CanPlaceAt("floor", "wood", "south", x, 3); !ok {
			t.Fatalf("floor at %d,3 refused: %q", x, r)
		}
	}
	// a non-floor still needs a walkable tile
	placeBuilt(t, g, construction.Workbench, construction.South, 2, 5)
	if ok, r := g.CanPlaceAt("chair", "wood", "south", 2, 5); ok || r != ReasonOccupied && r != ReasonBlocked {
		t.Fatalf("chair in a workbench: %v %q", ok, r)
	}
}
