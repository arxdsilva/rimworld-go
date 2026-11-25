// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"slices"
	"testing"

	"rimworld-go/internal/domain/ai/work"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// Command ordering (#8, AC-2): every command entering through Apply is
// stamped (Tick, Seq) and the command phase applies due ones in that order.

func growPrio(id uint64, v int) command.Command {
	return command.Command{Kind: command.SetWorkPriority, Pawn: id, Work: command.WorkGrow, Count: v}
}

// TestCommandOrderEqualTick: two conflicting priority sets for one pawn at
// one tick resolve in submit order, the later Seq winning, whichever value
// comes first, and two equal runs end in the same state.
func TestCommandOrderEqualTick(t *testing.T) {
	t.Parallel()
	run := func(first, second int) (int, uint64) {
		g, id := choiceGame(t)
		g.Apply(growPrio(id, first))
		g.Apply(growPrio(id, second))
		if it := g.cmds.Items(); it[len(it)-2].Tick != it[len(it)-1].Tick || it[len(it)-2].Seq+1 != it[len(it)-1].Seq {
			t.Fatalf("envelopes %+v", it)
		}
		g.Step(1)
		return int(g.prioOf(uint64ToID(id))[work.Grow]), g.StateHash()
	}
	if got, _ := run(1, 4); got != 4 {
		t.Fatalf("priority %d after 1 then 4, want 4", got)
	}
	if got, _ := run(4, 1); got != 1 {
		t.Fatalf("priority %d after 4 then 1, want 1", got)
	}
	if _, a := run(2, 3); func() bool { _, b := run(2, 3); return a != b }() {
		t.Fatal("two equal runs differ")
	}
}

// TestCommandOrderFutureTick: a command for a later tick waits until that
// tick's command phase, not one tick before; a past target tick applies at
// the next phase.
func TestCommandOrderFutureTick(t *testing.T) {
	t.Parallel()
	g, id := choiceGame(t)
	pid := uint64ToID(id)
	g.Step(1)
	start := g.prioOf(pid)[work.Grow]
	at := uint64(g.Tick()) + 5
	c := growPrio(id, 1)
	c.Tick = at
	g.Apply(c)
	g.Step(5)
	if got := g.prioOf(pid)[work.Grow]; got != start || g.cmds.Len() != 1 || uint64(g.Tick()) != at {
		t.Fatalf("applied before its tick: priority %d (was %d), queued %d", got, start, g.cmds.Len())
	}
	g.Step(1)
	if got := g.prioOf(pid)[work.Grow]; got != 1 || g.cmds.Len() != 0 {
		t.Fatalf("not applied at its tick: priority %d, queued %d", got, g.cmds.Len())
	}
	past := growPrio(id, 2)
	past.Tick = 1
	g.Apply(past)
	g.Step(1)
	if got := g.prioOf(pid)[work.Grow]; got != 2 {
		t.Fatalf("a past target tick applies at the next phase: priority %d", got)
	}
}

// capableWork is a work type every one of ids can take a priority for.
func capableWork(t *testing.T, g *Game, ids ...entity.EntityID) (command.WorkType, work.Type) {
	t.Helper()
	for _, c := range []command.WorkType{command.WorkHaul, command.WorkClean, command.WorkGrow, command.WorkCook} {
		if w, ok := work.FromCommand(c); ok && !slices.ContainsFunc(ids, func(id entity.EntityID) bool { return g.incapableWork(id).Enabled(w) }) {
			return c, w
		}
	}
	t.Fatalf("pawns %v share no work type", ids)
	return 0, 0
}

// TestCommandOrderCrossMap: on the home map's pass a command for another
// open map stays queued, in order and with its envelope; that map's pass
// applies it, the later Seq winning. A command for a closed map is dropped.
func TestCommandOrderCrossMap(t *testing.T) {
	t.Parallel()
	g := worldGame(t)
	ada, bo := g.pawnIDs[0], g.pawnIDs[1]
	m := founded(t, g, landable(t, g, 1)[0], ada)
	home := g.maps[0]
	if m == home || onMap(g, ada) != m || onMap(g, bo) != home {
		t.Fatal("setup: Ada on the new colony, Bo at home")
	}
	cw, w := capableWork(t, g, ada, bo)
	set := func(id entity.EntityID, v int, on uint32) command.Command {
		return command.Command{Kind: command.SetWorkPriority, Pawn: uint64(id), Work: cw, Count: v, Map: on, MapSet: true}
	}
	g.Apply(set(ada, 1, m.id))
	g.Apply(set(bo, 2, home.id))
	g.Apply(set(ada, 4, m.id))
	g.Apply(set(bo, 3, 99)) // no such map
	now := uint64(g.Tick())

	g.mapState = home
	g.phaseCommands()
	type env struct{ pawn, tick, seq uint64 }
	var left []env
	for _, c := range g.cmds.Items() {
		left = append(left, env{c.Pawn, c.Tick, c.Seq})
	}
	last := g.cmds.Last()
	if want := []env{{uint64(ada), now, last - 3}, {uint64(ada), now, last - 1}}; !slices.Equal(left, want) {
		t.Fatalf("after the home pass %v, want %v", left, want)
	}
	if g.prioOf(bo)[w] != 2 {
		t.Fatalf("home command not applied on the home pass: %d", g.prioOf(bo)[w])
	}
	g.mapState = m
	g.phaseCommands()
	g.mapState = g.view
	if g.cmds.Len() != 0 || g.prioOf(ada)[w] != 4 {
		t.Fatalf("after the colony pass: queued %d, priority %d, want 0 and 4", g.cmds.Len(), g.prioOf(ada)[w])
	}
	if g.prioOf(bo)[w] != 2 {
		t.Fatalf("the closed map's command was applied: %d", g.prioOf(bo)[w])
	}
}

// TestCommandSeqSurvivesSave: a pending future command keeps its (Tick,
// Seq) and the counter through a save; the next command is stamped
// Last+1, the loaded state hashes the same, and both games apply the
// command at its tick.
func TestCommandSeqSurvivesSave(t *testing.T) {
	t.Parallel()
	g, id := choiceGame(t)
	g.Step(1)
	c := growPrio(id, 1)
	c.Tick = uint64(g.Tick()) + 3
	g.Apply(c)
	h := saveLoad(t, g)
	if !slices.Equal(h.cmds.Items(), g.cmds.Items()) || h.cmds.Last() != g.cmds.Last() || h.cmds.Len() != 1 {
		t.Fatalf("pending %+v last %d, want %+v last %d", h.cmds.Items(), h.cmds.Last(), g.cmds.Items(), g.cmds.Last())
	}
	if g.StateHash() != h.StateHash() {
		t.Fatal("the round trip changed the state hash")
	}
	h.Apply(growPrio(id, 4))
	if it := h.cmds.Items(); it[0].Seq != g.cmds.Last()+1 {
		t.Fatalf("next Seq %d, want %d", it[0].Seq, g.cmds.Last()+1)
	}
	g.Apply(growPrio(id, 4))
	g.Step(4)
	h.Step(4)
	if g.prioOf(uint64ToID(id))[work.Grow] != 1 || g.StateHash() != h.StateHash() {
		t.Fatalf("priority %d: the future command lands last; loaded game diverged %v", g.prioOf(uint64ToID(id))[work.Grow], g.StateHash() != h.StateHash())
	}
}

// TestCommandOldSavePendingOrder: a save from before #8 has unstamped
// (Tick 0, Seq 0) commands; loaded, they are stamped in slice order and
// apply at the next phase in that order.
func TestCommandOldSavePendingOrder(t *testing.T) {
	t.Parallel()
	g, id := choiceGame(t)
	g.Step(1)
	g.cmds.Restore([]command.Command{growPrio(id, 4), growPrio(id, 2)}, 0) // as an old save loads
	if it := g.cmds.Items(); it[0].Seq != 1 || it[1].Seq != 2 || g.cmds.Last() != 2 {
		t.Fatalf("stamped %+v", it)
	}
	g.Step(1)
	if got := g.prioOf(uint64ToID(id))[work.Grow]; got != 2 || g.cmds.Len() != 0 {
		t.Fatalf("priority %d, queued %d: the old commands apply in their saved order", got, g.cmds.Len())
	}
}

// TestCommandPhaseEmptyAllocs: the command phase runs on every map pass;
// with nothing queued it allocates nothing.
func TestCommandPhaseEmptyAllocs(t *testing.T) {
	g := New(Config{Seed: 3, Width: 16, Height: 16})
	g.Step(1)
	if g.cmds.Len() != 0 {
		t.Fatalf("setup: %d queued", g.cmds.Len())
	}
	if n := testing.AllocsPerRun(100, g.phaseCommands); n != 0 {
		t.Fatalf("empty command phase: %v allocs", n)
	}
}
