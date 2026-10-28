package game

import (
	"testing"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// Full-simulation blocking scenarios (SIM-014): creatures that block each
// other never share a cell, and every conflict resolves without deadlock.
// PLAY-010: colonists and tame animals (partners) walk through each other;
// only two colonists settled on one cell while either is in combat
// conflict.

// standingCreatures lists every creature that owns its cell right now.
func standingCreatures(g *Game) []entity.EntityID {
	var out []entity.EntityID
	for _, ids := range [3][]entity.EntityID{g.pawnIDs, g.raiderIDs, g.animalIDs} {
		for _, id := range ids {
			g.occ.carried = g.occ.carried[:0]
			for _, p := range g.carrying {
				g.occ.carried = append(g.occ.carried, p)
			}
			if g.standing(id) {
				out = append(out, id)
			}
		}
	}
	return out
}

// sharedCells maps each cell holding two or more standing creatures to
// them.
func sharedCells(g *Game) map[Position][]entity.EntityID {
	at := map[Position][]entity.EntityID{}
	for _, id := range standingCreatures(g) {
		p, _ := g.positions.Get(id)
		at[p] = append(at[p], id)
	}
	for p, ids := range at {
		if len(ids) < 2 {
			delete(at, p)
		}
	}
	return at
}

// blockingPair: a and b block each other (not both colonists or tame
// animals).
func blockingPair(g *Game, a, b entity.EntityID) bool { return !g.partner(a) || !g.partner(b) }

// combatPair: two colonists, either in combat, both standing still (not
// passing through): the pair decision 7 keeps apart.
func combatPair(g *Game, a, b entity.EntityID) bool {
	if !g.colonist(a) || !g.colonist(b) || !g.inCombat(a) && !g.inCombat(b) {
		return false
	}
	_, wa := g.walkNext(a)
	_, wb := g.walkNext(b)
	return !wa && !wb
}

// conflicts lists every pair sharing a cell that must not: blocking pairs
// and settled combat pairs, with their cell.
func conflicts(g *Game) map[[2]entity.EntityID]Position {
	out := map[[2]entity.EntityID]Position{}
	for p, ids := range sharedCells(g) {
		for i := 0; i < len(ids); i++ {
			for k := i + 1; k < len(ids); k++ {
				a, b := min(ids[i], ids[k]), max(ids[i], ids[k])
				if blockingPair(g, a, b) || combatPair(g, a, b) {
					out[[2]entity.EntityID{a, b}] = p
				}
			}
		}
	}
	return out
}

func assertNoSharedCell(t *testing.T, g *Game, tick int) {
	t.Helper()
	for pair, p := range conflicts(g) {
		t.Fatalf("tick %d: creatures %v share cell %v", tick, pair, p)
	}
}

// draftedPawn spawns a drafted colonist (it follows orders and nothing
// else, so the scenario stays about movement).
func draftedPawn(t *testing.T, g *Game, x, y int) entity.EntityID {
	t.Helper()
	id := mustPawn(t, g, x, y)
	g.setDraft(id, true)
	return id
}

func order(g *Game, id entity.EntityID, x, y int) {
	g.Enqueue(command.Command{Kind: command.MovePawn, Pawn: uint64(id), X: x, Y: y})
}

// runUntil steps up to limit ticks, checking the invariant every tick,
// and returns the tick done() first held (-1 if never).
func runUntil(t *testing.T, g *Game, limit int, done func() bool) int {
	t.Helper()
	for tick := 1; tick <= limit; tick++ {
		g.Step(1)
		assertNoSharedCell(t, g, tick)
		if done() {
			return tick
		}
	}
	return -1
}

// corridorScenario mirrors cmd/game -scenario corridor: a 1-wide corridor
// of walls, two drafted colonists at opposite ends. It orders those
// flagged across and returns each one's arrival tick after the orders (0
// when not ordered, -1 when it never arrived).
func corridorScenario(t *testing.T, goA, goB bool) (int, int) {
	g := New(Config{Seed: 1, Width: 32, Height: 9})
	cx, cy := 16, 4
	for x := cx - 6; x <= cx+6; x++ {
		if !g.BuildNow("wall", "wood", "south", x, cy-1) || !g.BuildNow("wall", "wood", "south", x, cy+1) {
			t.Fatal("walls not built")
		}
	}
	a := draftedPawn(t, g, cx-9, cy)
	b := draftedPawn(t, g, cx+9, cy)
	if goA {
		order(g, a, cx+8, cy)
	}
	if goB {
		order(g, b, cx-8, cy)
	}
	start := int(g.Tick())
	doneA, doneB := -1, -1
	if !goA {
		doneA = 0
	}
	if !goB {
		doneB = 0
	}
	runUntil(t, g, 17*TicksPerCell+300, func() bool {
		tick := int(g.Tick()) - start
		if doneA < 0 && pos(g, a) == (Position{cx + 8, cy}) {
			doneA = tick
		}
		if doneB < 0 && pos(g, b) == (Position{cx - 8, cy}) {
			doneB = tick
		}
		return doneA >= 0 && doneB >= 0
	})
	return doneA, doneB
}

func TestCorridorHeadOnBothArrive(t *testing.T) {
	// PLAY-010: the two colonists pass through each other, so each crosses
	// in the time it takes through an empty corridor (±1 tick)
	aloneA, _ := corridorScenario(t, true, false)
	_, aloneB := corridorScenario(t, false, true)
	if aloneA < 0 || aloneB < 0 {
		t.Fatalf("an empty corridor was not crossed: %d %d", aloneA, aloneB)
	}
	doneA, doneB := corridorScenario(t, true, true)
	if doneA < 0 || doneB < 0 {
		t.Fatalf("not both across: %d %d", doneA, doneB)
	}
	if abs(doneA-aloneA) > 1 || abs(doneB-aloneB) > 1 {
		t.Fatalf("crossing took %d and %d ticks, alone %d and %d", doneA, doneB, aloneA, aloneB)
	}
}

func TestCorridorBlockerHeadOnBothArrive(t *testing.T) {
	// the corridor scenario with a colonist and a wild animal, which still
	// block each other: the animal (the higher ID) backs out of the
	// corridor and both get across. Only the movement phase runs.
	g := New(Config{Seed: 1, Width: 32, Height: 9})
	cx, cy := 16, 4
	for x := cx - 6; x <= cx+6; x++ {
		if !g.BuildNow("wall", "wood", "south", x, cy-1) || !g.BuildNow("wall", "wood", "south", x, cy+1) {
			t.Fatal("walls not built")
		}
	}
	col := draftedPawn(t, g, cx-9, cy)
	d := wildAt(t, g, cx+9, cy)
	goalC, goalD := Position{cx + 8, cy}, Position{cx - 8, cy}
	g.cmdMove(col, goalC.X, goalC.Y)
	send := func() {
		an, _ := g.animals.Get(d)
		g.animalGoTo(d, &an, goalD.X, goalD.Y)
		g.animals.Set(d, an)
	}
	send()
	an, _ := g.animals.Get(d)
	// PLAY-022: the wanderer ambles, so its crossing takes twice the walking step a tile
	budget := 17*max(TicksPerCell, an.StepAt(an.GaitAt(uint64(g.clock.Now())))) + 400
	tick := 1
	for ; tick <= budget && (pos(g, col) != goalC || pos(g, d) != goalD); tick++ {
		g.phaseMovement()
		g.clock.Advance(1)
		assertNoSharedCell(t, g, tick)
		if an, _ := g.animals.Get(d); an.PathI >= len(an.Path) && pos(g, d) != goalD {
			send() // it stopped short (backed out, or gave up): send it on
		}
	}
	if pos(g, col) != goalC || pos(g, d) != goalD {
		t.Fatalf("not both across within %d ticks: colonist at %v, animal at %v", budget, pos(g, col), pos(g, d))
	}
	t.Logf("both across after %d ticks", tick)
}

func TestCrowdToOneCellSpreads(t *testing.T) {
	g := New(Config{Seed: 1, Width: 20, Height: 20})
	target := Position{10, 10}
	ids := []entity.EntityID{draftedPawn(t, g, 3, 3), draftedPawn(t, g, 3, 16), draftedPawn(t, g, 16, 3)}
	// each is ordered to the same cell once the one before has settled
	for _, id := range ids {
		order(g, id, target.X, target.Y)
		if runUntil(t, g, 400, func() bool {
			j, _ := g.jobs.Get(id)
			return !j.Active()
		}) < 0 {
			t.Fatalf("pawn %d never settled; at %v", id, pos(g, id))
		}
		if j, _ := g.jobs.Get(id); j.State == job.Fail {
			t.Fatalf("pawn %d order failed: %v", id, j.Fail)
		}
		// its tile reservation is released on the next AI pass
		runUntil(t, g, 2, func() bool { return false })
	}
	runUntil(t, g, 30, func() bool { return false })
	seen := map[Position]bool{}
	for _, id := range ids {
		p := pos(g, id)
		if seen[p] {
			t.Fatalf("two pawns on %v", p)
		}
		seen[p] = true
		if d := max(abs(p.X-target.X), abs(p.Y-target.Y)); d > 1 {
			t.Fatalf("pawn %d settled at %v, %d from the target", id, p, d)
		}
	}
	if !seen[target] {
		t.Fatal("nobody took the target cell")
	}
}

func TestDoorChokepointQueues(t *testing.T) {
	// mirrors cmd/game -scenario crowd: a 5x5 room with one door, every
	// colonist sent inside at once
	g := New(Config{Seed: 1, Width: 30, Height: 20})
	cx, cy := 18, 10
	for x := cx - 3; x <= cx+3; x++ {
		g.BuildNow("wall", "wood", "south", x, cy-3)
		g.BuildNow("wall", "wood", "south", x, cy+3)
	}
	for y := cy - 2; y <= cy+2; y++ {
		kind := "wall"
		if y == cy {
			kind = "door"
		}
		if !g.BuildNow(kind, "wood", "east", cx-3, y) || !g.BuildNow("wall", "wood", "south", cx+3, y) {
			t.Fatalf("room not built at y=%d", y)
		}
	}
	var ids []entity.EntityID
	for i := range 5 {
		ids = append(ids, draftedPawn(t, g, 5+i%2, 7+i))
	}
	for i, id := range ids {
		order(g, id, cx+1-i/3, cy-1+i%3)
	}
	at := runUntil(t, g, 900, func() bool {
		for _, id := range ids {
			if j, _ := g.jobs.Get(id); j.Active() {
				return false
			}
		}
		return true
	})
	if at < 0 {
		for _, id := range ids {
			j, _ := g.jobs.Get(id)
			t.Logf("pawn %d at %v job %s", id, pos(g, id), j.Line())
		}
		t.Fatal("the queue at the door never cleared")
	}
	for i, id := range ids {
		want := Position{cx + 1 - i/3, cy - 1 + i%3}
		if p := pos(g, id); p != want {
			t.Errorf("pawn %d at %v, want %v", id, p, want)
		}
	}
}

func TestThreeCycleNoDeadlock(t *testing.T) {
	g := New(Config{Seed: 1, Width: 14, Height: 14})
	a := draftedPawn(t, g, 6, 6)
	b := draftedPawn(t, g, 7, 6)
	c := draftedPawn(t, g, 7, 7)
	// each is sent onto the next one's cell: a ring of three
	order(g, a, 7, 6)
	order(g, b, 7, 7)
	order(g, c, 6, 6)
	at := runUntil(t, g, 200, func() bool {
		return pos(g, a) == (Position{7, 6}) && pos(g, b) == (Position{7, 7}) && pos(g, c) == (Position{6, 6})
	})
	if at < 0 {
		t.Fatalf("ring deadlocked: a %v b %v c %v", pos(g, a), pos(g, b), pos(g, c))
	}
}

func TestRaiderBlocksDoorway(t *testing.T) {
	// a wall across the map with a single gap; a raider stands in it.
	// Only the movement phase runs, so the raider holds still.
	g := New(Config{Seed: 1, Width: 16, Height: 12})
	for y := range 12 {
		if y != 5 {
			g.grid.SetWalkable(8, y, false)
		}
	}
	col := draftedPawn(t, g, 3, 5)
	g.raidWith(kitOfOne(g), planAssault)
	if len(g.raiderIDs) != 1 {
		t.Fatal("no raider")
	}
	r := g.raiderIDs[0]
	g.positions.Set(r, Position{8, 5})
	g.cmdMove(col, 13, 5)
	blockedSeen := false
	for tick := 1; tick <= blockedFailTicks+120; tick++ {
		g.phaseMovement()
		g.clock.Advance(1)
		assertNoSharedCell(t, g, tick)
		if p := pos(g, col); p.X > 8 {
			t.Fatalf("tick %d: colonist got past the raider to %v", tick, p)
		}
		if _, by := g.motionBlocked(col); by == r {
			blockedSeen = true
		}
	}
	if !blockedSeen {
		t.Fatal("the colonist was never held up by the raider")
	}
	if p := pos(g, col); p != (Position{7, 5}) && pathing8(p, Position{8, 5}) != 1 {
		t.Fatalf("colonist waits at %v, want next to the gap", p)
	}
	if j, _ := g.jobs.Get(col); j.State != job.Fail || j.Fail != job.Blocked {
		t.Fatalf("after %d blocked ticks the order should fail as blocked: %s", blockedFailTicks, j.Line())
	}
	if pos(g, r) != (Position{8, 5}) {
		t.Fatal("the raider was pushed out of the doorway")
	}
}

func pathing8(a, b Position) int { return max(abs(a.X-b.X), abs(a.Y-b.Y)) }

// kitOfOne is a one-raider kit.
func kitOfOne(g *Game) []item.Kind { return g.kitOf(1)[:1] }

func TestNoSharedCellInvariant(t *testing.T) {
	seeds, ticks := 5, 20000
	if testing.Short() {
		seeds, ticks = 2, 5000
	}
	for seed := 1; seed <= seeds; seed++ {
		g := NewColony(Config{Seed: uint64(seed), Width: 48, Height: 40})
		g.Raid(3)
		if len(g.animalIDs) == 0 {
			t.Fatalf("seed %d: no animals", seed)
		}
		// blocking stacks only come from outside movement (spawns,
		// standing up); a step never creates one. Colonists settled on one
		// cell with either in combat, and every blocking stack, clear
		// quickly.
		since := map[[2]entity.EntityID]int{}
		for tick := 1; tick <= ticks; tick++ {
			before := map[entity.EntityID]Position{}
			for _, id := range standingCreatures(g) {
				before[id], _ = g.positions.Get(id)
			}
			g.Step(1)
			now := map[[2]entity.EntityID]int{}
			for pair, p := range conflicts(g) {
				if blockingPair(g, pair[0], pair[1]) {
					for _, id := range pair {
						if was, ok := before[id]; ok && was != p {
							t.Fatalf("seed %d tick %d: %d walked onto %v held by %v", seed, tick, id, p, pair)
						}
					}
				}
				now[pair] = since[pair] + 1
				if now[pair] > 2*TicksPerCellDiag {
					t.Fatalf("seed %d tick %d: %v stacked on %v for %d ticks", seed, tick, pair, p, now[pair])
				}
			}
			since = now
		}
	}
}

func TestBlockingSameSeedSameRun(t *testing.T) {
	run := func() []Position {
		g := NewColony(Config{Seed: 7, Width: 48, Height: 40})
		g.Raid(3)
		g.Step(3000)
		var out []Position
		for _, ids := range [3][]entity.EntityID{g.pawnIDs, g.raiderIDs, g.animalIDs} {
			for _, id := range ids {
				out = append(out, pos(g, id))
			}
		}
		return out
	}
	a, b := run(), run()
	if len(a) != len(b) {
		t.Fatalf("creature counts differ: %d vs %d", len(a), len(b))
	}
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("creature %d at %v vs %v", i, a[i], b[i])
		}
	}
}
