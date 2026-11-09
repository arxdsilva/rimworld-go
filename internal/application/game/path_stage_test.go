package game

import (
	"testing"

	"rimworld-go/internal/application/game/pathstage"
	"rimworld-go/internal/domain/ai/pathing"
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/sim/entity"
)

// The movement phase's path stage (paths.go runRepaths, GAR-035): an
// animal's search around a creature is queued while creatures step and
// answered after the loop, against the occupancy the loop left.

// stageJam is a wild wanderer at (9, 1) walking east to (19, 1) with a
// wild blocker at (10, 1), in the 1-wide corridor or, with way set, in a
// strip whose row 2 is open (a way round the blocker).
func stageJam(t *testing.T, way bool) (*Game, entity.EntityID) {
	t.Helper()
	g := corridor(20)
	if way {
		for x := range 20 {
			g.grid.SetWalkable(x, 2, true)
		}
	}
	wildAt(t, g, 10, 1)
	d := wildAt(t, g, 9, 1)
	var path []pathing.Point
	for x := 10; x < 20; x++ {
		path = append(path, pathing.Point{X: x, Y: 1})
	}
	animalWalk(g, d, path...)
	a, _ := g.animals.Get(d)
	a.Mode, a.Errand = animal.Wander, false
	g.animals.Set(d, a)
	return g, d
}

// queueJam opens the stage and queues d's search as repathEvery would.
func queueJam(t *testing.T, g *Game, d entity.EntityID) {
	t.Helper()
	g.buildOccupancy()
	g.regs.paths.Open()
	a, _ := g.animals.Get(d)
	from := pathing.Point{X: 9, Y: 1}
	if !g.regs.paths.Queue(pathstage.Request{ID: d, From: from, To: pathing.Point{X: 10, Y: 1},
		Goal: a.Path[len(a.Path)-1], Token: pathstage.TokenOf(a.Path, a.PathI, from)}) {
		t.Fatal("an open stage refused a request")
	}
}

// TestRepathStageApplies: an answer lands on the walk it was asked for: a
// route round the blocker (first step free, not into the blocked cell),
// or on a failed search a doubled wait and, for a wild wanderer, no path.
func TestRepathStageApplies(t *testing.T) {
	t.Parallel()
	g, d := stageJam(t, true)
	queueJam(t, g, d)
	runRepaths(g)
	a, _ := g.animals.Get(d)
	if a.PathI != 0 || len(a.Path) == 0 || a.Path[0] == (pathing.Point{X: 10, Y: 1}) || a.Path[len(a.Path)-1] != (pathing.Point{X: 19, Y: 1}) {
		t.Fatalf("no route round the blocker applied: %v at %d", a.Path, a.PathI)
	}
	if m := g.motionOf(d); m.RepathWait != 0 {
		t.Fatalf("a found route raised the wait to %d", m.RepathWait)
	}

	g, d = stageJam(t, false)
	queueJam(t, g, d)
	runRepaths(g)
	if a, _ := g.animals.Get(d); len(a.Path) != 0 {
		t.Fatalf("a wild wanderer with no way round kept its path %v", a.Path)
	}
	if m := g.motionOf(d); m.RepathWait != 2*repathAfter {
		t.Fatalf("RepathWait %d after a failed search, want %d", m.RepathWait, 2*repathAfter)
	}
}

// TestRepathStageKeepsARewrittenWalk: a walk rewritten after its search
// was queued (makeWay, a give-up) keeps the new walk, whether the search
// found a route or not; a failed one still doubles the wait.
func TestRepathStageKeepsARewrittenWalk(t *testing.T) {
	t.Parallel()
	for _, way := range []bool{true, false} {
		g, d := stageJam(t, way)
		queueJam(t, g, d)
		animalWalk(g, d, pathing.Point{X: 8, Y: 1}, pathing.Point{X: 7, Y: 1})
		runRepaths(g)
		a, _ := g.animals.Get(d)
		if len(a.Path) != 2 || a.Path[0] != (pathing.Point{X: 8, Y: 1}) || a.PathI != 0 {
			t.Fatalf("way %v: the rewritten walk became %v at %d", way, a.Path, a.PathI)
		}
		if want := map[bool]int{true: 0, false: 2 * repathAfter}[way]; g.motionOf(d).RepathWait != want {
			t.Fatalf("way %v: RepathWait %d, want %d", way, g.motionOf(d).RepathWait, want)
		}
	}
}

// TestRepathStageQueuesOnlyInThePhase: outside a movement phase a search
// runs at once (as before the stage); the stage refuses requests, and
// takes them only in ascending ID.
func TestRepathStageQueuesOnlyInThePhase(t *testing.T) {
	t.Parallel()
	var s pathstage.Stage
	if s.Queue(pathstage.Request{ID: 2}) {
		t.Fatal("a closed stage queued a request")
	}
	s.Open()
	if !s.Queue(pathstage.Request{ID: 2}) || len(s.Requests()) != 1 {
		t.Fatal("an open stage refused a request")
	}
	defer func() {
		if recover() == nil {
			t.Fatal("a request out of ID order was queued")
		}
	}()
	s.Queue(pathstage.Request{ID: 2})
}

// TestCheckIndexWritesNothingOff: with the self-check off a lookup's
// check costs no write (path searches on several lanes share the game).
// Not parallel: it switches the package's self-check.
func TestCheckIndexWritesNothingOff(t *testing.T) {
	SetIndexSelfCheck(false)
	defer SetIndexSelfCheck(true)
	g, d := stageJam(t, true)
	before := g.idx.checkN
	queueJam(t, g, d)
	runRepaths(g)
	g.plantAt(3, 1)
	if g.idx.checkN != before {
		t.Fatalf("checkN moved %d -> %d with the self-check off", before, g.idx.checkN)
	}
}
