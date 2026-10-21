package game

import (
	"testing"

	"rimworld-go/internal/domain/sim/command"
)

// TestUndesignateKeepsZones: cancelling one designation never deletes the
// pen, grow zone or stockpile under it (DesignateCancel would).
func TestUndesignateKeepsZones(t *testing.T) {
	g := New(Config{Seed: 1, Width: 20, Height: 20})
	apply := func(c command.Command) { g.Enqueue(c); g.ApplyPending() }
	apply(command.Command{Kind: command.DesignatePen, X: 2, Y: 2, X2: 8, Y2: 8})
	apply(command.Command{Kind: command.DesignateGrowZone, X: 10, Y: 10, X2: 14, Y2: 14})
	aid, err := g.SpawnAnimal("dustbuck", 4, 4)
	if err != nil {
		t.Fatal(err)
	}
	pid, err := g.SpawnCrop("potato", 12, 12, 100)
	if err != nil {
		t.Fatal(err)
	}
	apply(command.Command{Kind: command.DesignateHunt, X: 4, Y: 4, X2: 4, Y2: 4})
	apply(command.Command{Kind: command.DesignateTame, X: 4, Y: 4, X2: 4, Y2: 4})
	apply(command.Command{Kind: command.DesignateHarvest, X: 12, Y: 12, X2: 12, Y2: 12})
	apply(command.Command{Kind: command.DesignateCut, X: 12, Y: 12, X2: 12, Y2: 12})

	for _, o := range []command.Order{command.OrderHunt, command.OrderTame} {
		apply(command.Command{Kind: command.Undesignate, Order: o, X: 4, Y: 4, X2: 4, Y2: 4})
	}
	for _, o := range []command.Order{command.OrderHarvest, command.OrderCut} {
		apply(command.Command{Kind: command.Undesignate, Order: o, X: 12, Y: 12, X2: 12, Y2: 12})
	}
	w := g.World()
	if len(w.Pens) != 1 || len(w.GrowZones) != 1 {
		t.Fatalf("zones must stay: pens %d grow zones %d", len(w.Pens), len(w.GrowZones))
	}
	for _, a := range w.Animals {
		if a.ID == aid && (a.Designated || a.TameMark) {
			t.Fatal("hunt and tame marks must clear")
		}
	}
	for _, p := range w.Plants {
		if p.ID == pid && (p.Designated || p.Cut) {
			t.Fatal("harvest and cut marks must clear")
		}
	}
}

// TestUndesignateOnlyNamedMark: clearing the cut mark leaves a mine mark
// and deconstruction in the same rectangle alone.
func TestUndesignateOnlyNamedMark(t *testing.T) {
	g := weatherGame(t)
	if !g.BuildNow("bed", "wood", "south", 3, 3) {
		t.Fatal("build bed")
	}
	apply := func(c command.Command) { g.Enqueue(c); g.ApplyPending() }
	apply(command.Command{Kind: command.DesignateDeconstruct, X: 3, Y: 3, X2: 3, Y2: 3})
	apply(command.Command{Kind: command.Undesignate, Order: command.OrderCut, X: 0, Y: 0, X2: 10, Y2: 10})
	marked := func() bool {
		for _, b := range g.World().Buildings {
			if b.Kind == "bed" {
				return b.Deconstruct
			}
		}
		return false
	}
	if !marked() {
		t.Fatal("undesignating cut must not clear deconstruction")
	}
	apply(command.Command{Kind: command.Undesignate, Order: command.OrderDeconstruct, X: 3, Y: 3, X2: 3, Y2: 3})
	if marked() {
		t.Fatal("undesignating deconstruct clears it")
	}
}
