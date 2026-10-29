package game

import (
	"testing"

	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/building/door"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/command"
)

// doorAtID is the building on the hut's door tile (5, 8).
func doorAtID(t *testing.T, g *Game) uint64 {
	t.Helper()
	bid := g.layerBldgAt(5, 8, construction.LayerObject)
	if b, ok := g.bldgs.Get(bid); !ok || !b.Kind.IsDoor() {
		t.Fatal("no door at (5, 8)")
	}
	return uint64(bid)
}

// TestDoorWaitOnTheWay (DEF-3): a colonist walking out of the hut waits at
// the shut wooden door for its opening time before stepping into it.
func TestDoorWaitOnTheWay(t *testing.T) {
	g := New(Config{Seed: 3, Width: 20, Height: 14})
	hutWalls(t, g, 2, 2, 8, 8)
	id, _ := g.SpawnPawn(5, 6)
	g.draft[id] = true
	g.Apply(command.Command{Kind: command.MovePawn, Pawn: uint64(id), X: 5, Y: 11})
	var opened, entered uint64
	for i := 0; i < 600 && entered == 0; i++ {
		g.Step(1)
		bid := g.layerBldgAt(5, 8, construction.LayerObject)
		if s := g.combat.Doors.State(bid); opened == 0 && s.OpenedAt != 0 {
			opened = s.OpenedAt
		}
		if p, _ := g.positions.Get(id); p.X == 5 && p.Y == 8 {
			entered = uint64(g.clock.Now())
		}
	}
	if opened == 0 || entered == 0 {
		t.Fatalf("door opened at %d, entered at %d", opened, entered)
	}
	if want := opened + uint64(door.OpenTicks(door.Speed(construction.Door, item.Wood, false))); entered < want {
		t.Fatalf("stepped into the door at %d, before it was open at %d", entered, want)
	}
}

// TestHeldDoorLetsHostilesThrough: a door held open is open to raiders'
// paths and steps (the region map agrees); let go, it shuts them out again.
func TestHeldDoorLetsHostilesThrough(t *testing.T) {
	g := New(Config{Seed: 3, Width: 20, Height: 12})
	hut(t, g, 2, 2, 8, 8)
	doorAtID(t, g)
	g.Apply(command.Command{Kind: command.SetDoorHoldOpen, X: 5, Y: 8, Enabled: true})
	g.Step(1)
	if !g.hostileMap().Walkable(5, 8) || g.doorAt(5, 8) {
		t.Fatal("a held door is open to hostiles")
	}
	if _, ok := g.raidPathTo(t, 5, 5); !ok {
		t.Fatal("a hostile path through the held door")
	}
	g.Apply(command.Command{Kind: command.SetDoorHoldOpen, X: 5, Y: 8})
	g.Step(1)
	if g.hostileMap().Walkable(5, 8) || !g.doorAt(5, 8) {
		t.Fatal("let go, the door shuts hostiles out")
	}
	if _, ok := g.raidPathTo(t, 5, 5); ok {
		t.Fatal("no hostile path into the closed hut")
	}
}

// TestForbiddenDoorBarsColonists: a forbidden door is shut to colonists
// (and stays shut to hostiles); allowed again, colonists pass.
func TestForbiddenDoorBarsColonists(t *testing.T) {
	g := New(Config{Seed: 3, Width: 20, Height: 12})
	hut(t, g, 2, 2, 8, 8)
	bid := doorAtID(t, g)
	id, _ := g.SpawnPawn(5, 5)
	g.Apply(command.Command{Kind: command.SetForbidden, Target: bid, Enabled: true})
	g.Step(1)
	if g.grid.Walkable(5, 8) || g.hostileMap().Walkable(5, 8) || g.reachable(id, 5, 11) {
		t.Fatal("a forbidden door bars colonists and hostiles")
	}
	if !g.combat.Doors.Barred(5, 8) {
		t.Fatal("barred")
	}
	g.Apply(command.Command{Kind: command.SetForbidden, Target: bid})
	g.Step(1)
	if !g.grid.Walkable(5, 8) || !g.reachable(id, 5, 11) {
		t.Fatal("allowed again, colonists pass")
	}
}

// TestAutodoorIsADoor: the powered door separates rooms, is shut to
// hostiles and lines up with its walls like the plain door.
func TestAutodoorIsADoor(t *testing.T) {
	g := New(Config{Seed: 3, Width: 20, Height: 12})
	for x := 2; x <= 8; x++ {
		kind := "wall"
		if x == 5 {
			kind = "powered door"
		}
		if !g.BuildNow(kind, "steel", "south", x, 6) {
			t.Fatalf("build %s at %d", kind, x)
		}
	}
	if !g.doorAt(5, 6) || g.hostileMap().Walkable(5, 6) || !g.grid.Walkable(5, 6) {
		t.Fatal("an autodoor is shut to hostiles, open to colonists")
	}
	b, _ := g.bldgs.Get(g.layerBldgAt(5, 6, construction.LayerObject))
	if !roomWall(b.Kind) || !autoOriented(b.Kind) || !orientPiece(b.Kind) || construction.Movable(b.Kind) {
		t.Fatal("an autodoor bounds rooms and lines up like a door")
	}
	if construction.MaxHP(b.Kind, item.Steel) != construction.MaxHP(construction.Door, item.Steel) {
		t.Fatal("door hit points")
	}
}
