package game

import (
	"bytes"
	"reflect"
	"testing"

	"rimworld-go/internal/domain/sim/command"
)

// TestApplyPendingMatchesStep: applying queued commands while paused, then
// stepping, reaches exactly the state of stepping with them still queued.
func TestApplyPendingMatchesStep(t *testing.T) {
	cmds := func(g *Game) {
		p := g.Snapshot()[0]
		// zones skip rock: paint by the colonist, on open ground
		g.Enqueue(command.Command{Kind: command.DesignateStockpile, X: p.X - 2, Y: p.Y + 2, X2: p.X + 1, Y2: p.Y + 4})
		g.Enqueue(command.Command{Kind: command.DesignateBuild, Build: 1, X: 10, Y: 10})
		g.Enqueue(command.Command{Kind: command.SetWorkPriority, Pawn: uint64(p.ID), Work: command.WorkHaul, Count: 1})
		g.Enqueue(command.Command{Kind: command.MovePawn, Pawn: uint64(p.ID), X: p.X + 3, Y: p.Y})
	}
	a := NewColony(Config{Seed: 3, Width: 40, Height: 30})
	b := NewColony(Config{Seed: 3, Width: 40, Height: 30})
	cmds(a)
	cmds(b)
	a.ApplyPending()
	if len(zonesOf(a.World(), ZoneStockpile)) != 1 {
		t.Fatal("a stockpile applied while paused must show at once")
	}
	a.Step(300)
	b.Step(300)
	if !reflect.DeepEqual(a.World(), b.World()) || !reflect.DeepEqual(a.Snapshot(), b.Snapshot()) {
		t.Fatal("ApplyPending then Step must equal Step")
	}
	// and they stay equal through a save, a load and more play (gob map
	// order makes save bytes differ run to run, so compare loaded games)
	reload := func(g *Game) *Game {
		var buf bytes.Buffer
		if err := g.Save(&buf, "x"); err != nil {
			t.Fatal(err)
		}
		l, _, err := Load(&buf)
		if err != nil {
			t.Fatal(err)
		}
		l.Step(600)
		return l
	}
	la, lb := reload(a), reload(b)
	if !reflect.DeepEqual(la.World(), lb.World()) || !reflect.DeepEqual(la.Snapshot(), lb.Snapshot()) {
		t.Fatal("loaded games diverge")
	}
}
