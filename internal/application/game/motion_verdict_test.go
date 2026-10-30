package game

import (
	"testing"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/ai/pathing"
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// verdicts makes the next calls of enter for id answer v (alt for
// sidestep / arrive-near), then go back to stepGo. It counts the calls.
func verdicts(g *Game, id entity.EntityID, n int, v stepVerdict, alt Position) *int {
	calls := 0
	g.enterHook = func(who entity.EntityID, from, to Position) (stepVerdict, Position) {
		if who != id || calls >= n {
			return stepGo, to
		}
		calls++
		return v, alt
	}
	return &calls
}

func TestWaitVerdictHoldsThenStepsNextTick(t *testing.T) {
	t.Parallel()
	g := corridor(12)
	id := orderMove(t, g, 1, 1, 6, 1)
	calls := verdicts(g, id, 5, stepWait, Position{})
	g.Step(4) // takes the order and tries to step at once: 4 refusals
	if p := pawn(t, g, id); p.X != 1 || *calls != 4 {
		t.Fatalf("waiting pawn at x=%d after %d refusals, want x=1 after 4", p.X, *calls)
	}
	if m := g.motionOf(id); m.BlockedTicks != 4 {
		t.Fatalf("BlockedTicks %d, want 4", m.BlockedTicks)
	}
	g.Step(1) // the fifth refusal
	before := uint64(g.Tick())
	g.Step(1) // free again: steps at once, not a full step later
	p := pawn(t, g, id)
	if p.X != 2 || p.StepTick != before {
		t.Fatalf("after the wait: x=%d StepTick=%d, want x=2 at %d", p.X, p.StepTick, before)
	}
	if m := g.motionOf(id); m.BlockedTicks != 0 || m.BlockedBy != 0 {
		t.Fatalf("a step clears the blocked state: %+v", m)
	}
}

func TestBlockedTooLongFailsJob(t *testing.T) {
	t.Parallel()
	g := corridor(12)
	id := orderMove(t, g, 1, 1, 6, 1)
	verdicts(g, id, 1<<30, stepWait, Position{})
	g.Step(blockedFailTicks) // 600 refusals: still waiting
	if j, _ := g.jobs.Get(id); j.State != job.Goto {
		t.Fatalf("after %d blocked ticks the walk is %v, want still walking", blockedFailTicks, j.State)
	}
	g.Step(1)
	if j, _ := g.jobs.Get(id); j.State != job.Fail || j.Fail != job.Blocked {
		t.Fatalf("job %v/%v, want failed: blocked", j.State, j.Fail)
	}
	if m := g.motionOf(id); m.BlockedTicks != 0 {
		t.Fatalf("failing resets BlockedTicks, got %d", m.BlockedTicks)
	}
	if p := pawn(t, g, id); p.X != 1 {
		t.Fatalf("blocked pawn moved to x=%d", p.X)
	}
}

func TestRepathVerdictReplansAndKeepsCounting(t *testing.T) {
	t.Parallel()
	g := corridor(12)
	id := orderMove(t, g, 1, 1, 6, 1)
	verdicts(g, id, 2, stepRepath, Position{})
	g.Step(2)
	if p := pawn(t, g, id); p.X != 1 {
		t.Fatalf("repathing pawn moved to x=%d", p.X)
	}
	if m := g.motionOf(id); m.BlockedTicks != 2 {
		t.Fatalf("BlockedTicks %d, want 2 (a repath does not reset it)", m.BlockedTicks)
	}
	g.Step(6 * TicksPerCell)
	if p := pawn(t, g, id); p.X != 6 {
		t.Fatalf("pawn at x=%d after repathing, want 6", p.X)
	}
}

func TestSidestepVerdictMovesAsideAndReplans(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 10, Height: 6})
	id := orderMove(t, g, 1, 1, 6, 1)
	verdicts(g, id, 1, stepSidestep, Position{2, 2})
	g.Step(2)
	p := pawn(t, g, id)
	if p.X != 2 || p.Y != 2 || p.StepTicks != TicksPerCellDiag || !p.HasNext {
		t.Fatalf("after sidestep %+v, want (2,2), a diagonal step, a new path", p)
	}
	g.Step(8 * TicksPerCellDiag)
	if p := pawn(t, g, id); p.X != 6 || p.Y != 1 {
		t.Fatalf("pawn at (%d,%d) after sidestep, want (6,1)", p.X, p.Y)
	}
}

func TestArriveNearVerdictEndsWalk(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 10, Height: 6})
	id := orderMove(t, g, 1, 1, 3, 1)
	verdicts(g, id, 1, stepArriveNear, Position{2, 2})
	g.Step(2)
	p := pawn(t, g, id)
	if p.X != 2 || p.Y != 2 || p.HasNext {
		t.Fatalf("after arrive-near %+v, want at (2,2) with no next step", p)
	}
	if j, _ := g.jobs.Get(id); j.State == job.Goto || j.State == job.Fail {
		t.Fatalf("walk not ended: %v", j.State)
	}

	// arrive-near on the current cell ends the walk without moving
	h := New(Config{Seed: 1, Width: 10, Height: 6})
	hid := orderMove(t, h, 1, 1, 3, 1)
	verdicts(h, hid, 1, stepArriveNear, Position{1, 1})
	h.Step(2)
	if p := pawn(t, h, hid); p.X != 1 || p.Y != 1 || p.HasNext || p.StepTick != 0 {
		t.Fatalf("arrive-near in place %+v, want still at (1,1), never stepped", p)
	}
}

func TestAnimalVerdicts(t *testing.T) {
	t.Parallel()
	path := []pathing.Point{{X: 2, Y: 1}, {X: 3, Y: 1}, {X: 4, Y: 1}}

	// wait: holds, counts, shows the next cell; then steps on the next tick
	g := New(Config{Seed: 1, Width: 10, Height: 6})
	id := walkAnimal(t, g, "dustbuck", 1, 1, path)
	verdicts(g, id, 3, stepWait, Position{})
	g.Step(3)
	if av := animalView(t, g, id); av.X != 1 || !av.HasNext || av.NextX != 2 {
		t.Fatalf("waiting animal %+v", av)
	}
	if m := g.motionOf(id); m.BlockedTicks != 3 {
		t.Fatalf("animal BlockedTicks %d, want 3", m.BlockedTicks)
	}
	g.Step(1)
	if av := animalView(t, g, id); av.X != 2 || av.BlockedBy != 0 {
		t.Fatalf("animal did not step after the wait: %+v", av)
	}

	// blocked too long: drops the path
	g = New(Config{Seed: 1, Width: 10, Height: 6})
	id = walkAnimal(t, g, "dustbuck", 1, 1, path)
	verdicts(g, id, 1<<30, stepWait, Position{})
	g.Step(blockedFailTicks + 1)
	if a, _ := g.animals.Get(id); a.PathI < len(a.Path) {
		t.Fatalf("animal still on its path after %d blocked ticks", blockedFailTicks+1)
	}

	// repath: replans to its goal and walks on
	g = New(Config{Seed: 1, Width: 10, Height: 6})
	id = walkAnimal(t, g, "dustbuck", 1, 1, path)
	verdicts(g, id, 1, stepRepath, Position{})
	g.Step(1)
	if m := g.motionOf(id); m.BlockedTicks != 1 {
		t.Fatalf("repath BlockedTicks %d, want 1", m.BlockedTicks)
	}
	g.Step(4 * 25)
	if av := animalView(t, g, id); av.X != 4 || av.Y != 1 {
		t.Fatalf("animal at (%d,%d) after repath, want (4,1)", av.X, av.Y)
	}

	// sidestep: moves aside, replans to the goal
	g = New(Config{Seed: 1, Width: 10, Height: 6})
	id = walkAnimal(t, g, "dustbuck", 1, 1, path)
	verdicts(g, id, 1, stepSidestep, Position{2, 2})
	g.Step(1)
	if av := animalView(t, g, id); av.X != 2 || av.Y != 2 || av.StepTicks != (animal.State{Species: animal.Deer}.StepAt(animal.GaitAmble)*1414+500)/1000 || !av.HasNext { // PLAY-022: a wanderer ambles
		t.Fatalf("sidestepped animal %+v", av)
	}
	g.Step(4 * 25)
	if av := animalView(t, g, id); av.X != 4 || av.Y != 1 {
		t.Fatalf("animal at (%d,%d) after sidestep, want (4,1)", av.X, av.Y)
	}

	// arrive-near: steps to alt and stops; in place it just stops
	for _, alt := range []Position{{2, 2}, {1, 1}} {
		g = New(Config{Seed: 1, Width: 10, Height: 6})
		id = walkAnimal(t, g, "dustbuck", 1, 1, path)
		verdicts(g, id, 1, stepArriveNear, alt)
		g.Step(1)
		if av := animalView(t, g, id); av.X != alt.X || av.Y != alt.Y || av.HasNext {
			t.Fatalf("arrive-near %+v: animal %+v", alt, av)
		}
	}
}

func TestBlockedStateClearsWhenWalkEnds(t *testing.T) {
	t.Parallel()
	g := corridor(12)
	id := orderMove(t, g, 1, 1, 6, 1)
	verdicts(g, id, 1<<30, stepWait, Position{})
	g.Step(3)
	if m := g.motionOf(id); m.BlockedTicks != 3 {
		t.Fatalf("BlockedTicks %d, want 3", m.BlockedTicks)
	}
	g.jobs.Set(id, job.Record{}) // the walk is called off
	g.Step(1)
	if m := g.motionOf(id); m.BlockedTicks != 0 || m.BlockedBy != 0 {
		t.Fatalf("an idle pawn keeps blocked state %+v", m)
	}
}

func TestWallInTheWayReplansOrFails(t *testing.T) {
	t.Parallel()
	// open ground: a wall dropped on the next cell is walked around
	g := New(Config{Seed: 1, Width: 10, Height: 6})
	id := orderMove(t, g, 1, 2, 6, 2)
	g.Step(1)
	j, _ := g.jobs.Get(id)
	next := j.Path[j.PathI]
	g.grid.SetWalkable(next.X, next.Y, false)
	g.Step(10 * TicksPerCellDiag)
	if p := pawn(t, g, id); p.X != 6 || p.Y != 2 {
		t.Fatalf("pawn at (%d,%d), want around the wall to (6,2)", p.X, p.Y)
	}

	// a corridor: walled shut, the walk fails as unreachable
	h := corridor(12)
	hid := orderMove(t, h, 1, 1, 6, 1)
	h.Step(1)
	h.grid.SetWalkable(4, 1, false)
	failed := false
	for i := 0; i < 4*TicksPerCell && !failed; i++ {
		h.Step(1)
		j, _ := h.jobs.Get(hid)
		failed = j.State == job.Fail && j.Fail == job.Unreachable
	}
	if !failed {
		t.Fatal("walk into a sealed corridor should fail as unreachable")
	}

	// the target itself walled, next to it: the replan ends the walk there
	k := corridor(12)
	kid := orderMove(t, k, 1, 1, 3, 1)
	k.Step(1) // now at (2,1), next (3,1)
	k.grid.SetWalkable(3, 1, false)
	k.Step(TicksPerCell)
	if j, _ := k.jobs.Get(kid); j.State == job.Goto {
		t.Fatalf("walk to a walled target next door still going: %+v", j)
	}
}

func TestSidestepEdgeCases(t *testing.T) {
	t.Parallel()
	// sidestep onto the goal ends the walk
	g := New(Config{Seed: 1, Width: 10, Height: 6})
	id := orderMove(t, g, 1, 1, 2, 2)
	verdicts(g, id, 1, stepSidestep, Position{2, 2})
	g.Step(2)
	if j, _ := g.jobs.Get(id); j.State == job.Goto {
		t.Fatalf("sidestep onto the goal left the walk going: %+v", j)
	}

	// sidestep into a pocket with no way on fails the walk
	h := New(Config{Seed: 1, Width: 10, Height: 6})
	hid := orderMove(t, h, 1, 1, 6, 1)
	h.enterHook = func(who entity.EntityID, from, to Position) (stepVerdict, Position) {
		for _, c := range [][2]int{{1, 3}, {2, 3}, {3, 3}, {3, 2}, {1, 1}, {2, 1}, {3, 1}, {0, 1}, {0, 2}, {0, 3}} {
			h.grid.SetWalkable(c[0], c[1], false) // seal (2,2) in
		}
		h.enterHook = nil
		return stepSidestep, Position{2, 2}
	}
	h.Step(1)
	if j, _ := h.jobs.Get(hid); j.State != job.Fail || j.Fail != job.Unreachable {
		t.Fatalf("sealed-in sidestep: job %v/%v, want failed: unreachable", j.State, j.Fail)
	}
}

func TestRepathOntoGoalEndsWalk(t *testing.T) {
	t.Parallel()
	g := corridor(12)
	id := orderMove(t, g, 1, 1, 6, 1)
	g.Step(1)                           // at (2,1)
	g.positions.Set(id, Position{6, 1}) // pushed onto the goal
	verdicts(g, id, 1, stepRepath, Position{})
	g.Step(TicksPerCell)
	if j, _ := g.jobs.Get(id); j.State == job.Goto {
		t.Fatalf("repath at the goal left the walk going: %+v", j)
	}
}

// A fleeing animal whose flee cell is taken stops where it stands; it
// replans after a step's pause, not every tick (each replan is a search).
func TestAnimalStoppedShortDoesNotReplanEveryTick(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 30, Height: 30})
	pid, _ := g.SpawnPawn(12, 15)
	g.Enqueue(command.Command{Kind: command.DraftPawn, Pawn: uint64(pid), Enabled: true})
	aid, _ := g.SpawnAnimal("dustbuck", 15, 15)
	calls := 0
	g.enterHook = func(who entity.EntityID, from, to Position) (stepVerdict, Position) {
		if who != aid {
			return stepGo, to
		}
		calls++
		return stepArriveNear, from
	}
	g.Step(40)
	if p, _ := g.positions.Get(aid); p != (Position{15, 15}) {
		t.Fatalf("animal moved to %+v", p)
	}
	if a, _ := g.animals.Get(aid); a.Mode != animal.Flee {
		t.Fatalf("dustbuck mode %v, want fleeing", a.Mode)
	}
	if calls == 0 || calls > 8 {
		t.Fatalf("stopped-short animal tried %d steps in 40 ticks, want a few (one per pause)", calls)
	}
}

// carryingHauler is a pawn at (2,2) walking a load picked up at (2,2) to
// the drop cell (8,2): TX/TY is the pickup, DestX/DestY the drop.
func carryingHauler(t *testing.T) (*Game, entity.EntityID) {
	t.Helper()
	g := New(Config{Seed: 1, Width: 12, Height: 6})
	id, _ := g.SpawnPawn(2, 2)
	sid, _ := g.SpawnStack(item.Wood, 10, 2, 2)
	st, _ := g.stacks.Get(sid)
	st.Carrier = id
	g.putStack(sid, st)
	path, ok := g.pathTo(id, 8, 2)
	if !ok {
		t.Fatal("no path to the drop cell")
	}
	g.jobs.Set(id, job.Record{Kind: job.Haul, State: job.Goto, Target: sid, TX: 2, TY: 2,
		Carry: sid, DestX: 8, DestY: 2, Path: path})
	return g, id
}

func TestSidestepWhileCarryingReplansToDestination(t *testing.T) {
	t.Parallel()
	g, id := carryingHauler(t)
	verdicts(g, id, 1, stepSidestep, Position{3, 3})
	g.Step(1)
	j, _ := g.jobs.Get(id)
	if p := pawn(t, g, id); p.X != 3 || p.Y != 3 {
		t.Fatalf("hauler at (%d,%d), want sidestepped to (3,3)", p.X, p.Y)
	}
	if j.State != job.Goto || len(j.Path) == 0 || j.Path[len(j.Path)-1] != (pathing.Point{X: 8, Y: 2}) {
		t.Fatalf("after sidestep the load goes to %+v, want the drop cell (8,2)", j.Path)
	}
}

func TestRepathOrWallWhileCarryingKeepsDestination(t *testing.T) {
	t.Parallel()
	// a repath (no cached route) heads for the drop cell, not the pickup
	g, id := carryingHauler(t)
	verdicts(g, id, 1, stepRepath, Position{})
	g.Step(1)
	if j, _ := g.jobs.Get(id); j.State != job.Goto || len(j.Path) == 0 || j.Path[len(j.Path)-1] != (pathing.Point{X: 8, Y: 2}) {
		t.Fatalf("after a repath the load goes to %+v (%v), want (8,2)", j.Path, j.State)
	}

	// a wall on the next cell: the replan heads for the drop cell too
	h, hid := carryingHauler(t)
	j, _ := h.jobs.Get(hid)
	h.grid.SetWalkable(j.Path[0].X, j.Path[0].Y, false)
	h.Step(1)
	if j, _ := h.jobs.Get(hid); j.State != job.Goto || len(j.Path) == 0 || j.Path[len(j.Path)-1] != (pathing.Point{X: 8, Y: 2}) {
		t.Fatalf("after a wall the load goes to %+v (%v), want (8,2)", j.Path, j.State)
	}
}
