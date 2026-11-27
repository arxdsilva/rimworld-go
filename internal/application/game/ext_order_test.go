// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"slices"
	"testing"

	"rimworld-go/internal/application/scripting"
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/ai/work"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/terrain"
)

// orderYard is a 24x12 yard with a stockpile, a rock at (7, 2), a wood
// stack at (4, 6) and one hauler-miner whose id is 1 mod 60, so its
// cadence visits fall on tick%60 == 59 (the order is applied on the next
// tick, in the next minute: the boundary of lead note N1). f is its host,
// sending the orders order makes at every visit.
func orderYard(t *testing.T, order func(ev scripting.Event) []command.Command) (*Game, *fakeExt, entity.EntityID) {
	t.Helper()
	f := &fakeExt{uses: []scripting.Hook{scripting.OnPawn}, orders: order}
	g := New(Config{Seed: 1, Width: 24, Height: 12, Scripts: f})
	for g.entities.Peek()%scripting.PawnEvery != 1 {
		g.entities.Next()
	}
	ada, err := g.SpawnNamedPawn(1, 1, "Ada")
	if err != nil || ada%scripting.PawnEvery != 1 {
		t.Fatalf("pawn %d: %v", ada, err)
	}
	onlyWork(g, ada, work.Haul, work.Mine)
	g.Enqueue(command.Command{Kind: command.DesignateStockpile, X: 16, Y: 1, X2: 22, Y2: 10})
	g.setTerrain(7, 2, terrain.Rock)
	g.Step(1)
	wood, err := g.SpawnStack(item.Wood, 20, 4, 6)
	if err != nil {
		t.Fatal(err)
	}
	g.Enqueue(command.Command{Kind: command.SetForbidden, Target: uint64(wood), Enabled: true}) // only an order hauls it
	g.Step(1)
	UseExtensions(g, f)
	return g, f, ada
}

// orderAt is a ScriptOrder of o at (x, y) for each due pawn of ev.
func orderAt(o command.Order, x, y int) func(ev scripting.Event) []command.Command {
	return func(ev scripting.Event) []command.Command {
		var out []command.Command
		for _, p := range ev.Pawns {
			out = append(out, command.Command{Kind: command.ScriptOrder, Pawn: p.ID, Order: o, X: x, Y: y})
		}
		return out
	}
}

// forcedStarts steps g n ticks (id kept fed and rested) and returns the
// ticks where id started a forced job: a new forced job, or the same one
// with its work reset (a restart).
func forcedStarts(g *Game, id entity.EntityID, n int) []uint64 {
	var out []uint64
	prev, _ := g.jobs.Get(id)
	for range n {
		g.SetNeeds(id, needs.Full, needs.Full)
		g.Step(1)
		j, _ := g.jobs.Get(id)
		running := j.Forced && j.State != job.Fail && j.State != job.Complete
		same := prev.Kind == j.Kind && prev.Target == j.Target && prev.DestX == j.DestX && prev.DestY == j.DestY && prev.Forced
		if running && (!same || j.WorkLeft > prev.WorkLeft) {
			out = append(out, uint64(g.clock.Now()))
		}
		prev = j
	}
	return out
}

// TestExtScriptOrderNoFlap: a script order acts on its pawn's order visit
// only (one visit in scripting.OrderEvery, checked against the visit's
// own tick even when it falls on tick%60 == 59), so re-sending a running
// job starts it once, alternating orders switch the job at most once per
// 600 ticks, and a refused order is checked at most once per 600 ticks.
func TestExtScriptOrderNoFlap(t *testing.T) {
	t.Parallel()
	const window = scripting.OrderEvery * scripting.PawnEvery
	t.Run("reissued", func(t *testing.T) {
		t.Parallel()
		for _, c := range []struct {
			name string
			o    command.Order
			x, y int
			kind job.Kind
		}{{"mine", command.OrderMine, 7, 2, job.Mine}, {"haul", command.OrderHaul, 4, 6, job.Haul}} {
			g, _, ada := orderYard(t, orderAt(c.o, c.x, c.y))
			var during int // order visits while the job ran
			starts := 0
			prev, _ := g.jobs.Get(ada)
			for range 3 * window {
				g.SetNeeds(ada, needs.Full, needs.Full)
				now := uint64(g.clock.Now())
				g.Step(1)
				j, _ := g.jobs.Get(ada)
				if j.Kind == c.kind && j.Forced && (prev.Kind != c.kind || !prev.Forced || j.WorkLeft > prev.WorkLeft) {
					starts++
				}
				if now/scripting.PawnEvery%scripting.OrderEvery == 0 && (now+uint64(ada))%scripting.PawnEvery == 0 && prev.Kind == c.kind && prev.Forced {
					during++
				}
				prev = j
			}
			if starts != 1 || c.kind == job.Mine && during == 0 {
				t.Fatalf("%s: %d starts, %d re-sends while it ran", c.name, starts, during)
			}
		}
		// a haul ends inside one window: re-send it on the next order visit
		// while it runs (prioritize's planGather rewrote the job)
		g, _, ada := orderYard(t, nil)
		next := func() uint64 {
			for at := uint64(g.clock.Now()); ; at++ {
				if at/scripting.PawnEvery%scripting.OrderEvery == 0 && (at+uint64(ada))%scripting.PawnEvery == 0 {
					return at
				}
			}
		}
		c := command.Command{Kind: command.ScriptOrder, Pawn: uint64(ada), Order: command.OrderHaul, X: 4, Y: 6, Target: next()}
		applyScript(g, c)
		for range 3 {
			g.SetNeeds(ada, needs.Full, needs.Full)
			g.Step(1)
		}
		was, _ := g.jobs.Get(ada)
		if was.Kind != job.Haul || !was.Forced || len(was.Path) == 0 {
			t.Fatalf("fixture: the order gave %+v", was)
		}
		c.Target = next() + window
		applyScript(g, c)
		if j, _ := g.jobs.Get(ada); j.State != was.State || j.PathI != was.PathI || &j.Path[0] != &was.Path[0] || j.Count != was.Count {
			t.Fatalf("a re-sent haul restarted: %+v, was %+v", j, was)
		}
	})
	t.Run("alternating", func(t *testing.T) {
		t.Parallel()
		mine, haul := orderAt(command.OrderMine, 7, 2), orderAt(command.OrderHaul, 4, 6)
		g, _, ada := orderYard(t, func(ev scripting.Event) []command.Command {
			if ev.Tick/window%2 == 0 {
				return mine(ev)
			}
			return haul(ev)
		})
		starts := forcedStarts(g, ada, 4*window)
		if len(starts) < 2 {
			t.Fatalf("starts %v: the orders never switched the job", starts)
		}
		for i := 1; i < len(starts); i++ {
			if starts[i]-starts[i-1] < window {
				t.Fatalf("starts %v: two within %d ticks", starts, window)
			}
		}
	})
	t.Run("refused", func(t *testing.T) {
		t.Parallel()
		g, f, ada := orderYard(t, orderAt(command.OrderMine, 2, 9)) // no rock there
		f.uses = append(f.uses, scripting.OnPawns)                  // two hooks, the same order each visit
		var checked []uint64
		last, had := uint64(0), false
		for range 4 * window {
			g.SetNeeds(ada, needs.Full, needs.Full)
			g.Step(1)
			if at := g.scripts.orderAt; slices.Contains(g.scripts.ordered, ada) && (!had || at != last) {
				checked, last, had = append(checked, uint64(g.clock.Now())), at, true
			}
			if len(g.scripts.ordered) > 1 {
				t.Fatalf("the visit list grew to %v", g.scripts.ordered)
			}
		}
		if len(checked) < 3 {
			t.Fatalf("checked at %v", checked)
		}
		for i := 1; i < len(checked); i++ {
			if checked[i]-checked[i-1] != window {
				t.Fatalf("checked at %v, want every %d ticks", checked, window)
			}
		}
		if j, _ := g.jobs.Get(ada); j.Forced || len(g.messageViews()) != 0 {
			t.Fatalf("a refusal is silent: job %+v, messages %+v", j, g.messageViews())
		}
	})
	t.Run("boundary", func(t *testing.T) {
		t.Parallel()
		g, _, ada := orderYard(t, nil)
		visit := uint64(20*window + scripting.PawnEvery - 1) // an order visit at tick%60 == 59
		if (visit+uint64(ada))%scripting.PawnEvery != 0 {
			t.Fatalf("fixture: %d is not a visit of %d", visit, ada)
		}
		c := command.Command{Kind: command.ScriptOrder, Pawn: uint64(ada), Order: command.OrderMine, X: 7, Y: 2, Target: visit}
		if !scriptOrderDue(g, c) {
			t.Fatal("the order visit at tick%60 == 59 refused")
		}
		for _, at := range []uint64{visit + 1, visit + scripting.PawnEvery, visit - window + scripting.PawnEvery} {
			c.Target = at
			if scriptOrderDue(g, c) {
				t.Fatalf("stamp %d (not this pawn's order visit) accepted", at)
			}
		}
		c.Target, c.Order = visit, command.OrderBuild
		if scriptOrderDue(g, c) {
			t.Fatal("an order outside the closed set accepted")
		}
		// two orders of one visit (two hooks or plugins): the first acts
		c.Order = command.OrderMine
		applyScript(g, c)
		applyScript(g, command.Command{Kind: command.ScriptOrder, Pawn: uint64(ada), Order: command.OrderHaul, X: 4, Y: 6, Target: visit})
		if j, _ := g.jobs.Get(ada); j.Kind != job.Mine || !j.Forced {
			t.Fatalf("the visit's second order replaced its first: %+v", j)
		}
		// the visit list holds one visit's pawns only: the next visit, of
		// another pawn, clears it (a departed pawn leaves nothing behind)
		bob := spawnColonists(t, g, 1, 8)[0]
		at := uint64(21*window) + (scripting.PawnEvery-uint64(bob)%scripting.PawnEvery)%scripting.PawnEvery // bob's next order visit
		applyScript(g, command.Command{Kind: command.ScriptOrder, Pawn: uint64(bob), Order: command.OrderMine, X: 7, Y: 2, Target: at})
		if !slices.Equal(g.scripts.ordered, []entity.EntityID{bob}) || g.scripts.orderAt != at {
			t.Fatalf("visit %d: list %v at %d, want only %d", at, g.scripts.ordered, g.scripts.orderAt, bob)
		}
	})
}
