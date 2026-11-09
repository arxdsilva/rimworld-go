package game

import (
	"testing"

	"rimworld-go/internal/domain/sim/entity"
)

// S5: an animal held up with no way round backs off its searches
// (motionRec.RepathWait doubles per failed search up to repathCap, a step
// clears it). The tests run the movement phase alone, as
// TestCorridorBlockerHeadOnBothArrive does, so nothing but the jam moves.

// repathJam is a 1-wide corridor (corridor(w): row 1) with a wild
// dustbuck standing at x=blockX and tame dustbucks at the given cells
// walking east to the far end (each one cell short of the one ahead). It
// counts every search around a creature each walker runs: a stepRepath
// verdict, or a failed one (rpMiss).
func repathJam(t *testing.T, w, blockX int, xs ...int) (g *Game, blocker entity.EntityID, walkers []entity.EntityID, searches map[entity.EntityID]int) {
	t.Helper()
	g = corridor(w)
	searches = map[entity.EntityID]int{}
	blocker = wildAt(t, g, blockX, 1)
	for i, x := range xs {
		id := animalAt(t, g, "dustbuck", x, 1, true)
		a, _ := g.animals.Get(id)
		g.animalGoTo(id, &a, w-1-i, 1)
		g.animals.Set(id, a)
		walkers = append(walkers, id)
	}
	g.enterHook = func(id entity.EntityID, from, to Position) (stepVerdict, Position) {
		g.occ.rpMiss = 0 // a colonist's is never read
		v, alt := g.enter(id, from, to)
		if v == stepRepath || g.occ.rpMiss == id {
			searches[id]++
		}
		return v, alt
	}
	return g, blocker, walkers, searches
}

// moveTick runs one movement phase and advances the clock.
func moveTick(g *Game) {
	g.phaseMovement()
	g.clock.Advance(1)
}

// TestAnimalRepathBackoff: an animal jammed with no way round searches
// at blocked ticks 45, 90, 180 and 360 (at most 5 over 600 blocked ticks,
// where the old every-45 rule ran 13), and keeps its path; a colonist in
// the same jam keeps the eager rule (a search at once, then every 45).
func TestAnimalRepathBackoff(t *testing.T) {
	t.Parallel()
	g, _, ws, searches := repathJam(t, 20, 10, 9)
	d := ws[0]
	for range blockedFailTicks {
		moveTick(g)
	}
	m := g.motionOf(d)
	if m.BlockedTicks != blockedFailTicks || pos(g, d) != (Position{9, 1}) {
		t.Fatalf("not jammed for %d ticks: %+v at %v", blockedFailTicks, m, pos(g, d))
	}
	if n := searches[d]; n == 0 || n > 5 {
		t.Fatalf("%d searches over %d blocked ticks, want 1..5", n, blockedFailTicks)
	}
	if m.RepathWait != repathCap {
		t.Fatalf("RepathWait %d, want the cap %d", m.RepathWait, repathCap)
	}
	if a, _ := g.animals.Get(d); a.PathI >= len(a.Path) {
		t.Fatal("a tame walker dropped its path: only a wild wanderer does")
	}
	if repathCap%repathAfter != 0 || repathCap <= blockedFailTicks {
		t.Fatalf("repathCap %d must be a multiple of %d past %d", repathCap, repathAfter, blockedFailTicks)
	}

	// a colonist behind the same blocker: eager, and never backs off
	h, _, _, hs := repathJam(t, 20, 10)
	col := draftedPawn(t, h, 9, 1)
	h.cmdMove(col, 19, 1)
	for range 2*repathAfter + 1 {
		moveTick(h)
	}
	if hs[col] != 3 || h.motionOf(col).RepathWait != 0 {
		t.Fatalf("colonist: %d searches in %d blocked ticks (want 3: at 0, 45, 90), RepathWait %d",
			hs[col], 2*repathAfter, h.motionOf(col).RepathWait)
	}
}

// TestAnimalRepathWildWandererRepicks: a wild wanderer with no way round
// drops its path on its first failed search (it picks again at its next
// think) instead of standing in the jam.
func TestAnimalRepathWildWandererRepicks(t *testing.T) {
	t.Parallel()
	g, _, _, searches := repathJam(t, 20, 10)
	d := wildAt(t, g, 9, 1)
	a, _ := g.animals.Get(d)
	g.animalGoTo(d, &a, 19, 1)
	g.animals.Set(d, a)
	for range repathAfter + 1 {
		moveTick(g)
	}
	if a, _ := g.animals.Get(d); a.PathI < len(a.Path) || searches[d] != 1 {
		t.Fatalf("wanderer still on its path after %d searches", searches[d])
	}
	if m := g.motionOf(d); m.RepathWait != 2*repathAfter {
		t.Fatalf("RepathWait %d, want %d", m.RepathWait, 2*repathAfter)
	}
}

// TestAnimalRepathBackoffNoFlap: with the way blocked and freed in turn
// (the blocker shuffles one cell east every period), the wait rises only
// on a tick with a failed search and falls only on a tick the animal
// stepped; spells shorter than repathAfter never search, and longer ones
// never search more often than the old every-45 rule.
func TestAnimalRepathBackoffNoFlap(t *testing.T) {
	t.Parallel()
	for _, period := range []int{20, 100, 400} {
		g, b, ws, searches := repathJam(t, 40, 6, 5)
		d := ws[0]
		blocked := 0
		for tick := 1; tick <= 3000 && pos(g, b).X < 36; tick++ {
			if tick%period == 0 {
				p := pos(g, b)
				g.positions.Set(b, Position{X: p.X + 1, Y: 1})
			}
			before, at, n := g.motionOf(d), pos(g, d), searches[d]
			moveTick(g)
			after := g.motionOf(d)
			stepped, searched := pos(g, d) != at, searches[d] > n
			if after.RepathWait > before.RepathWait && !searched {
				t.Fatalf("period %d tick %d: the wait rose %d -> %d with no search", period, tick, before.RepathWait, after.RepathWait)
			}
			if after.RepathWait < before.RepathWait && !stepped {
				t.Fatalf("period %d tick %d: the wait fell %d -> %d with no step", period, tick, before.RepathWait, after.RepathWait)
			}
			if after.BlockedTicks > 0 {
				blocked++
			}
		}
		n := searches[d]
		if period < repathAfter && n != 0 {
			t.Fatalf("period %d: %d searches in spells shorter than %d ticks", period, n, repathAfter)
		}
		if n > blocked/repathAfter {
			t.Fatalf("period %d: %d searches in %d blocked ticks, more than the old rule's %d", period, n, blocked, blocked/repathAfter)
		}
		if period > repathAfter && n == 0 {
			t.Fatalf("period %d: never searched: the test proves nothing", period)
		}
	}
}

// TestAnimalRepathJamClears: a queue of animals behind a blocker in a
// 1-wide corridor, all backed off to the cap, clears once the blocker is
// gone: each walker steps the tick its cell frees (no wait on its
// back-off), its wait resets, and all reach their goals in the time an
// empty corridor takes plus a step each.
func TestAnimalRepathJamClears(t *testing.T) {
	t.Parallel()
	g, b, ws, _ := repathJam(t, 24, 10, 9, 8, 7)
	for range 500 {
		moveTick(g)
	}
	for _, id := range ws {
		if m := g.motionOf(id); m.BlockedTicks < 400 || m.RepathWait < 4*repathAfter {
			t.Fatalf("walker %d not backed off in the jam: %+v", id, m)
		}
	}
	g.removeAnimal(b)
	lead := ws[0]
	moveTick(g)
	if pos(g, lead) != (Position{10, 1}) || g.motionOf(lead).RepathWait != 0 {
		t.Fatalf("the lead did not step at once: at %v, %+v", pos(g, lead), g.motionOf(lead))
	}
	slowest := 0
	for _, id := range ws {
		a, _ := g.animals.Get(id)
		slowest = max(slowest, a.StepAt(a.GaitAt(uint64(g.clock.Now()))))
	}
	budget := (23-9+len(ws))*slowest + len(ws)
	tick := 1
	for ; tick <= budget; tick++ {
		done := true
		for i, id := range ws {
			done = done && pos(g, id) == (Position{23 - i, 1})
		}
		if done {
			break
		}
		moveTick(g)
	}
	for i, id := range ws {
		if pos(g, id) != (Position{23 - i, 1}) {
			t.Fatalf("walker %d at %v after %d ticks, want x=%d", id, pos(g, id), budget, 23-i)
		}
	}
	t.Logf("queue cleared in %d ticks (budget %d)", tick, budget)
}
