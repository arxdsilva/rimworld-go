package game

import (
	"testing"

	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// areaGame is a bare 30x20 map with one colonist at (3, 3) restricted to
// area 0, the strip x 0..9 (all rows).
func areaGame(t *testing.T) (*Game, entity.EntityID) {
	t.Helper()
	g := New(Config{Seed: 5, Width: 30, Height: 20})
	id, err := g.SpawnNamedPawn(3, 3, "Ada")
	if err != nil {
		t.Fatal(err)
	}
	g.Enqueue(command.Command{Kind: command.PaintArea, Index: 0, X: 0, Y: 0, X2: 9, Y2: 19, Enabled: true})
	g.Enqueue(command.Command{Kind: command.AssignArea, Pawn: uint64(id), Index: 0})
	g.ApplyPending()
	if g.areaFor(id) != 0 {
		t.Fatal("setup: pawn not restricted")
	}
	return g, id
}

func TestAreaCommandsAndViews(t *testing.T) {
	t.Parallel()
	g, id := areaGame(t)
	g.Enqueue(command.Command{Kind: command.RenameArea, Index: 0, Name: "  Inner yard  "})
	g.Enqueue(command.Command{Kind: command.RenameArea, Index: 1, Name: "Fields"})
	g.Enqueue(command.Command{Kind: command.RenameArea, Index: 5, Name: "skipped"})
	g.Enqueue(command.Command{Kind: command.PaintArea, Index: 1, X: 20, Y: 0, X2: 21, Y2: 1, Enabled: true})
	g.ApplyPending()
	v := g.Areas()
	if len(v) != 2 || v[0].Name != "Inner yard" || v[0].Cells != 200 || v[1].Name != "Fields" || v[1].Cells != 4 {
		t.Fatalf("areas %+v", v)
	}
	g.Enqueue(command.Command{Kind: command.AssignArea, Pawn: uint64(id), Index: -1})
	g.ApplyPending()
	if g.AreaOf(id) != -1 || !g.inArea(id, 25, 5) {
		t.Fatal("unassigned pawn still restricted")
	}
	for i := 2; i < 12; i++ {
		g.Enqueue(command.Command{Kind: command.PaintArea, Index: i, X: 0, Y: 0, X2: 0, Y2: 0, Enabled: true})
	}
	g.ApplyPending()
	if n := len(g.Areas()); n != 10 {
		t.Fatalf("%d areas, the cap is 10", n)
	}
}

func TestAreaRestrictedPawnNeverLeaves(t *testing.T) {
	t.Parallel()
	g, id := areaGame(t)
	if _, err := g.SpawnPlant(20, 10); err != nil {
		t.Fatal(err)
	}
	g.Enqueue(command.Command{Kind: command.DesignateHarvest, X: 20, Y: 10, X2: 20, Y2: 10})
	for i := 0; i < 3000; i++ {
		g.Step(1)
		if p, _ := g.positions.Get(id); p.X > 9 {
			t.Fatalf("tick %d: restricted pawn at %v, outside its area", i, p)
		}
	}
	// drafted or ordered, it may go anywhere
	g.Enqueue(command.Command{Kind: command.MovePawn, Pawn: uint64(id), X: 20, Y: 5})
	out := false
	for i := 0; i < 600 && !out; i++ {
		g.Step(1)
		out = posOf(g, id).X > 9
	}
	if !out {
		t.Fatalf("a direct order did not take the pawn out of its area: %v", posOf(g, id))
	}
}

func TestAreaPathStaysInside(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 5, Width: 30, Height: 20})
	id, _ := g.SpawnNamedPawn(1, 1, "Ada")
	// a U: down the left, along the bottom, up the right
	for _, r := range [][4]int{{0, 0, 2, 19}, {0, 17, 20, 19}, {18, 0, 20, 19}} {
		g.Enqueue(command.Command{Kind: command.PaintArea, Index: 0, X: r[0], Y: r[1], X2: r[2], Y2: r[3], Enabled: true})
	}
	g.Enqueue(command.Command{Kind: command.AssignArea, Pawn: uint64(id), Index: 0})
	g.ApplyPending()
	path, ok := g.pathTo(id, 19, 1)
	if !ok || len(path) < 40 {
		t.Fatalf("path round the U: ok %v, %d steps", ok, len(path))
	}
	for _, p := range path {
		if !g.areas.Contains(0, p.X, p.Y) {
			t.Fatalf("path leaves the area at %v", p)
		}
	}
	if _, ok := g.pathTo(id, 10, 5); ok {
		t.Fatal("a path to a cell outside the area")
	}
	g.setDraft(id, true)
	if path, ok := g.pathTo(id, 19, 1); !ok || len(path) > 25 {
		t.Fatalf("a drafted pawn should take the short way: ok %v, %d steps", ok, len(path))
	}
}

func TestAreaRejectionDoesNotFlap(t *testing.T) {
	t.Parallel()
	g, id := areaGame(t)
	for x := 12; x < 16; x++ {
		_, _ = g.SpawnPlant(x, 4)
	}
	g.Enqueue(command.Command{Kind: command.DesignateHarvest, X: 12, Y: 4, X2: 15, Y2: 4})
	g.Step(10)
	if c := kindChanges(g, id, 2000); c > 12 {
		t.Fatalf("job kind changed %d times in 2000 ticks", c)
	}
}

func TestAreaPawnOutsideWalksBack(t *testing.T) {
	t.Parallel()
	g, id := areaGame(t)
	g.positions.Set(id, Position{20, 10})
	for i := 0; i < 8 && !g.areas.Contains(0, posOf(g, id).X, posOf(g, id).Y); i++ {
		g.Step(250)
	}
	if p := posOf(g, id); !g.areas.Contains(0, p.X, p.Y) {
		t.Fatalf("pawn outside its area stayed out at %v", p)
	}
}

func posOf(g *Game, id entity.EntityID) Position {
	p, _ := g.positions.Get(id)
	return p
}

func TestAreaSaveLoad(t *testing.T) {
	t.Parallel()
	g, id := areaGame(t)
	g.Step(5) // a stepped game (a fresh one warms its indexes on load)
	h := saveLoad(t, g)
	if h.areaFor(id) != 0 || h.areas.Count(0) != 200 || h.StateHash() != g.StateHash() {
		t.Fatal("areas or assignments lost in the round trip")
	}
}
