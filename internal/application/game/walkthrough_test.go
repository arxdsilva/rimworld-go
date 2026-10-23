package game

import (
	"testing"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/ai/pathing"
	"rimworld-go/internal/domain/sim/entity"
)

// Walk-through (PLAY-010 decision 7): colonists and tame animals pass
// through each other; wild animals and raiders still block; a colonist in
// a fight never ends a walk on another colonist's tile.

// wildAt spawns a wild dustbuck standing at (x, y).
func wildAt(t *testing.T, g *Game, x, y int) entity.EntityID {
	t.Helper()
	return animalAt(t, g, "dustbuck", x, y, false)
}

func animalAt(t *testing.T, g *Game, species string, x, y int, tame bool) entity.EntityID {
	t.Helper()
	id, err := g.SpawnAnimal(species, x, y)
	if err != nil {
		t.Fatal(err)
	}
	a, _ := g.animals.Get(id)
	a.Tame = tame
	g.animals.Set(id, a)
	return id
}

// animalWalk gives animal id path, ending at its last cell.
func animalWalk(g *Game, id entity.EntityID, path ...pathing.Point) {
	a, _ := g.animals.Get(id)
	last := path[len(path)-1]
	a.Path, a.PathI, a.GoalX, a.GoalY = path, 0, last.X, last.Y
	g.animals.Set(id, a)
}

// corridorGame is a 14x12 map with a 1-wide corridor along y=5.
func corridorGame() *Game {
	g := occTestGame()
	for x := 0; x < 14; x++ {
		g.grid.SetWalkable(x, 4, false)
		g.grid.SetWalkable(x, 6, false)
	}
	return g
}

// runMoves runs only the movement phase for n ticks and reports the
// longest any of ids was held up.
func runMoves(g *Game, n int, ids ...entity.EntityID) (maxBlocked int) {
	for range n {
		g.phaseMovement()
		g.clock.Advance(1)
		for _, id := range ids {
			if b, _ := g.motionBlocked(id); b > maxBlocked {
				maxBlocked = b
			}
		}
	}
	return maxBlocked
}

func TestPartnersStepThroughEachOther(t *testing.T) {
	g := occTestGame()
	a := mustPawn(t, g, 2, 5)
	b := mustPawn(t, g, 3, 5)
	dog := animalAt(t, g, "husky", 2, 8, true)
	cat := animalAt(t, g, "husky", 3, 8, true)
	walkTo(g, a, job.Haul, straight(2, 8, 5)...)
	g.jobs.Set(b, job.Record{Kind: job.Construct, State: job.Do, TX: 3, TY: 4})
	animalWalk(g, dog, straight(2, 8, 8)...)
	rebuild(g)
	// a colonist walks into a working colonist's cell without waiting
	if v, alt := g.enter(a, Position{2, 5}, Position{3, 5}); v != stepGo || alt != (Position{3, 5}) {
		t.Fatalf("colonist through a worker = %v %v, want go", v, alt)
	}
	if b2, _ := g.jobs.Get(b); b2.Kind != job.Construct || b2.State != job.Do {
		t.Fatalf("the worker was disturbed: %+v", b2)
	}
	// a tame animal walks into another tame animal's cell, then a colonist's
	if v, _ := g.enter(dog, Position{2, 8}, Position{3, 8}); v != stepGo {
		t.Fatalf("tame through tame = %v, want go", v)
	}
	if v, _ := g.enter(dog, Position{3, 8}, Position{3, 5}); v != stepGo {
		t.Fatalf("tame through colonists = %v, want go", v)
	}
	// and a colonist into a tame animal's cell
	if v, _ := g.enter(b, Position{3, 5}, Position{3, 8}); v != stepGo {
		t.Fatalf("colonist through a tame animal = %v, want go", v)
	}
	_ = cat
	// head-on colonists both go
	h := occTestGame()
	c := mustPawn(t, h, 3, 5)
	d := mustPawn(t, h, 4, 5)
	walkTo(h, c, job.Move, straight(3, 9, 5)...)
	walkTo(h, d, job.Move, straight(4, 0, 5)...)
	rebuild(h)
	if v, _ := h.enter(c, Position{3, 5}, Position{4, 5}); v != stepGo {
		t.Fatalf("head-on lower ID = %v, want go", v)
	}
	if v, _ := h.enter(d, Position{4, 5}, Position{3, 5}); v != stepGo {
		t.Fatalf("head-on higher ID = %v, want go", v)
	}
}

func TestWildBoarAndRaiderStillBlock(t *testing.T) {
	g := occTestGame()
	col := mustPawn(t, g, 2, 5)
	boar := animalAt(t, g, "wild_boar", 3, 5, false)
	dog := animalAt(t, g, "husky", 2, 7, true)
	g.raidWith(kitOfOne(g), planAssault)
	if len(g.raiderIDs) != 1 {
		t.Fatal("no raider")
	}
	r := g.raiderIDs[0]
	g.positions.Set(r, Position{3, 7})
	g.jobs.Set(r, job.Record{})
	rebuild(g)
	// partners into a boar's or a raider's cell
	walkTo(g, col, job.Haul, straight(2, 8, 5)...)
	if v, _ := g.enter(col, Position{2, 5}, Position{3, 5}); v == stepGo {
		t.Fatal("a colonist walked into a wild boar")
	}
	animalWalk(g, dog, straight(2, 8, 7)...)
	if v, _ := g.enter(dog, Position{2, 7}, Position{3, 7}); v == stepGo {
		t.Fatal("a tame animal walked into a raider")
	}
	// the boar and the raider into a partner's cell
	animalWalk(g, boar, pathing.Point{X: 2, Y: 5}, pathing.Point{X: 1, Y: 5})
	if v, _ := g.enter(boar, Position{3, 5}, Position{2, 5}); v == stepGo {
		t.Fatal("a wild boar walked into a colonist")
	}
	walkTo(g, r, job.Move, pathing.Point{X: 2, Y: 7}, pathing.Point{X: 1, Y: 7})
	if v, _ := g.enter(r, Position{3, 7}, Position{2, 7}); v == stepGo {
		t.Fatal("a raider walked into a tame animal")
	}
	// and into each other
	g.positions.Set(r, Position{4, 5})
	rebuild(g)
	walkTo(g, r, job.Move, pathing.Point{X: 3, Y: 5}, pathing.Point{X: 2, Y: 5})
	if v, _ := g.enter(r, Position{4, 5}, Position{3, 5}); v == stepGo {
		t.Fatal("a raider walked into a wild boar")
	}
}

func TestCombatFinalStepArrivesNear(t *testing.T) {
	g := occTestGame()
	m := mustPawn(t, g, 4, 5)
	w := mustPawn(t, g, 5, 5)
	g.setDraft(m, true)
	g.jobs.Set(w, job.Record{Kind: job.Construct, State: job.Do, TX: 6, TY: 5})
	walkTo(g, m, job.Move, pathing.Point{X: 5, Y: 5})
	rebuild(g)
	// a drafted colonist's last step onto a colonist ends next to it
	v, alt := g.enter(m, Position{4, 5}, Position{5, 5})
	if v != stepArriveNear || alt == (Position{5, 5}) || pathing8(alt, Position{5, 5}) != 1 {
		t.Fatalf("drafted last step = %v %v, want arrive next to (5,5)", v, alt)
	}
	// mid-walk it passes through
	walkTo(g, m, job.Move, pathing.Point{X: 5, Y: 5}, pathing.Point{X: 6, Y: 5})
	g.positions.Set(m, Position{4, 5})
	rebuild(g)
	if v, _ := g.enter(m, Position{4, 5}, Position{5, 5}); v != stepGo {
		t.Fatalf("drafted mid-walk = %v, want go", v)
	}

	// a colonist out of combat ends on a drafted colonist's cell: no
	h := occTestGame()
	a := mustPawn(t, h, 4, 5)
	d := mustPawn(t, h, 5, 5)
	h.setDraft(d, true)
	walkTo(h, a, job.Cut, pathing.Point{X: 5, Y: 5})
	rebuild(h)
	if v, alt := h.enter(a, Position{4, 5}, Position{5, 5}); v != stepArriveNear || alt == (Position{5, 5}) {
		t.Fatalf("onto a drafted colonist = %v %v, want arrive near", v, alt)
	}
	// an exact-cell job waits for it
	walkTo(h, a, job.Sleep, pathing.Point{X: 5, Y: 5})
	if v, _ := h.enter(a, Position{4, 5}, Position{5, 5}); v != stepWait {
		t.Fatalf("exact cell onto a drafted colonist = %v, want wait", v)
	}
	// with both out of combat it ends on the shared cell, exact or not
	h.setDraft(d, false)
	rebuild(h)
	if v, _ := h.enter(a, Position{4, 5}, Position{5, 5}); v != stepGo {
		t.Fatalf("exact cell, no fight = %v, want go", v)
	}

	// a drafted colonist passing through someone never stops on that cell
	k := occTestGame()
	x := mustPawn(t, k, 4, 5)
	y := mustPawn(t, k, 5, 5)
	p := mustPawn(t, k, 4, 5) // standing in x's cell mid-walk
	k.setDraft(p, true)
	walkTo(k, p, job.Move, pathing.Point{X: 5, Y: 5})
	rebuild(k)
	v, alt = k.enter(p, Position{4, 5}, Position{5, 5})
	if v != stepArriveNear || alt != (Position{5, 4}) {
		t.Fatalf("passing drafted = %v %v, want arrive at the free (5,4)", v, alt)
	}
	_, _ = x, y
}

func TestNonCombatRingAllGo(t *testing.T) {
	g := occTestGame()
	a := mustPawn(t, g, 5, 5)
	b := mustPawn(t, g, 6, 5)
	c := mustPawn(t, g, 6, 6)
	walkTo(g, a, job.Move, pathing.Point{X: 6, Y: 5})
	walkTo(g, b, job.Move, pathing.Point{X: 6, Y: 6})
	walkTo(g, c, job.Move, pathing.Point{X: 5, Y: 5})
	rebuild(g)
	for _, s := range []struct {
		id       entity.EntityID
		from, to Position
	}{{a, Position{5, 5}, Position{6, 5}}, {b, Position{6, 5}, Position{6, 6}}, {c, Position{6, 6}, Position{5, 5}}} {
		if v, _ := g.enter(s.id, s.from, s.to); v != stepGo {
			t.Fatalf("%d: %v, want go", s.id, v)
		}
	}
}

func TestWalkThroughWorkingColonistNoWait(t *testing.T) {
	run := func(blocking bool) int {
		g := corridorGame()
		a := mustPawn(t, g, 1, 5)
		wx, wy := 6, 5
		if !blocking {
			wx, wy = 6, 8 // out of the corridor
		}
		w := mustPawn(t, g, wx, wy)
		g.jobs.Set(w, job.Record{Kind: job.Construct, State: job.Do, TX: wx, TY: wy - 1, WorkLeft: 100000})
		walkTo(g, a, job.Haul, straight(1, 11, 5)...)
		for tick := 1; tick <= 400; tick++ {
			g.phaseMovement()
			g.clock.Advance(1)
			if b, _ := g.motionBlocked(a); b != 0 {
				t.Fatalf("blocking=%v: walker held up at tick %d", blocking, tick)
			}
			if pos(g, a) == (Position{11, 5}) {
				return tick
			}
		}
		return -1
	}
	alone, through := run(false), run(true)
	if alone < 0 || through != alone {
		t.Fatalf("through a worker: arrived at %d, alone at %d", through, alone)
	}
}

func TestDraftedCrowdEndsOnThreeCells(t *testing.T) {
	g := New(Config{Seed: 1, Width: 20, Height: 20})
	target := Position{10, 10}
	ids := []entity.EntityID{draftedPawn(t, g, 3, 10), draftedPawn(t, g, 17, 10), draftedPawn(t, g, 10, 3)}
	for _, id := range ids {
		path, ok := g.pathTo(id, target.X, target.Y)
		if !ok {
			t.Fatal("no path")
		}
		g.jobs.Set(id, job.Record{Kind: job.Move, State: job.Goto, TX: target.X, TY: target.Y, Path: path})
	}
	for tick := 1; tick <= 400; tick++ {
		g.phaseMovement()
		g.clock.Advance(1)
		assertNoSharedCell(t, g, tick)
	}
	seen := map[Position]bool{}
	for _, id := range ids {
		if j, _ := g.jobs.Get(id); j.Active() {
			t.Fatalf("pawn %d still walking: %s", id, j.Line())
		}
		p := pos(g, id)
		if seen[p] {
			t.Fatalf("two drafted colonists on %v", p)
		}
		seen[p] = true
		if pathing8(p, target) > 1 {
			t.Fatalf("pawn %d settled at %v, away from the target", id, p)
		}
	}
	if !seen[target] {
		t.Fatal("nobody took the target cell")
	}
}

func TestFleeingColonistDoesNotStopOnColonist(t *testing.T) {
	run := func(fleeing bool) (Position, int) {
		g := corridorGame()
		g.grid.SetWalkable(8, 4, true) // a bay beside the goal
		f := mustPawn(t, g, 2, 5)
		mid := mustPawn(t, g, 4, 5)
		end := mustPawn(t, g, 8, 5)
		for _, w := range []entity.EntityID{mid, end} {
			p := pos(g, w)
			g.jobs.Set(w, job.Record{Kind: job.Construct, State: job.Do, TX: p.X, TY: p.Y + 1, WorkLeft: 100000})
		}
		if fleeing {
			g.calm[f] = uint64(g.clock.Now()) + 5000
		}
		walkTo(g, f, job.Move, straight(2, 8, 5)...)
		blocked := runMoves(g, 300, f)
		return pos(g, f), blocked
	}
	if p, _ := run(false); p != (Position{8, 5}) {
		t.Fatalf("out of combat the walk ends on the worker's cell: at %v", p)
	}
	p, blocked := run(true)
	if p == (Position{8, 5}) || pathing8(p, Position{8, 5}) != 1 || p == (Position{4, 5}) {
		t.Fatalf("fleeing colonist stopped at %v, want next to the worker at (8,5)", p)
	}
	if blocked != 0 {
		t.Fatalf("fleeing colonist waited %d ticks passing through", blocked)
	}
}

func TestDogAndColonistPassThrough(t *testing.T) {
	g := corridorGame()
	c := mustPawn(t, g, 2, 5)
	dog := animalAt(t, g, "husky", 10, 5, true)
	walkTo(g, c, job.Move, straight(2, 10, 5)...)
	animalWalk(g, dog, straight(10, 2, 5)...)
	if blocked := runMoves(g, 300, c, dog); blocked != 0 {
		t.Fatalf("held up %d ticks", blocked)
	}
	if pos(g, c) != (Position{10, 5}) || pos(g, dog) != (Position{2, 5}) {
		t.Fatalf("colonist at %v, dog at %v", pos(g, c), pos(g, dog))
	}
}

func TestUnstackOnlyInCombat(t *testing.T) {
	work := func(g *Game, id entity.EntityID) {
		g.jobs.Set(id, job.Record{Kind: job.Construct, State: job.Do, TX: 6, TY: 5, WorkLeft: 50})
	}
	// two colonists at work on one cell: shared cells are fine
	g := occTestGame()
	a := mustPawn(t, g, 5, 5)
	b := mustPawn(t, g, 5, 5)
	work(g, a)
	work(g, b)
	rebuild(g)
	for _, id := range []entity.EntityID{a, b} {
		if j, _ := g.jobs.Get(id); j.State != job.Do {
			t.Fatalf("%d moved off a shared cell out of combat: %+v", id, j)
		}
	}
	// the higher ID drafted: it steps off
	g.setDraft(b, true)
	rebuild(g)
	if j, _ := g.jobs.Get(b); j.State != job.Goto || len(j.Path) != 1 {
		t.Fatalf("drafted stacked colonist got no step off: %+v", j)
	}
	if j, _ := g.jobs.Get(a); j.State != job.Do {
		t.Fatalf("the lower ID must stay: %+v", j)
	}
	// a drafted colonist passing through moves nobody
	h := occTestGame()
	c := mustPawn(t, h, 5, 5)
	work(h, c)
	d := draftedPawn(t, h, 5, 5)
	walkTo(h, d, job.Move, pathing.Point{X: 6, Y: 6})
	rebuild(h)
	if j, _ := h.jobs.Get(c); j.State != job.Do {
		t.Fatalf("worker pushed off by a passer-by: %+v", j)
	}
	// a colonist and a wild animal on one cell: the higher ID moves off
	k := occTestGame()
	e := mustPawn(t, k, 5, 5)
	work(k, e)
	wild := wildAt(t, k, 5, 5)
	rebuild(k)
	if an, _ := k.animals.Get(wild); len(an.Path) != 1 {
		t.Fatal("the wild animal on a colonist's cell got no step off")
	}
	if j, _ := k.jobs.Get(e); j.State != job.Do {
		t.Fatalf("the lower-ID colonist must stay: %+v", j)
	}
	m := occTestGame()
	wild = wildAt(t, m, 5, 5)
	e = mustPawn(t, m, 5, 5)
	work(m, e)
	rebuild(m)
	if j, _ := m.jobs.Get(e); j.State != job.Goto || len(j.Path) != 1 {
		t.Fatalf("the higher-ID colonist on a wild animal's cell got no step off: %+v", j)
	}
	if an, _ := m.animals.Get(wild); len(an.Path) != 0 {
		t.Fatal("the lower-ID animal must stay")
	}
}

func TestPartnerMeleeAndCombatOnSharedCell(t *testing.T) {
	// a colonist fighting another stops next to it, even mid-walk
	g := occTestGame()
	a := mustPawn(t, g, 2, 5)
	b := mustPawn(t, g, 3, 5)
	walkTo(g, a, job.Attack, straight(2, 5, 5)...)
	j, _ := g.jobs.Get(a)
	j.Target = b
	g.jobs.Set(a, j)
	rebuild(g)
	if v, _ := g.enter(a, Position{2, 5}, Position{3, 5}); v != stepWait {
		t.Fatalf("attacker onto its target = %v, want wait", v)
	}
	// ...but walks through anyone else
	j.Target = 999
	g.jobs.Set(a, j)
	if v, _ := g.enter(a, Position{2, 5}, Position{3, 5}); v != stepGo {
		t.Fatalf("attacker through a bystander = %v, want go", v)
	}

	// a drafted colonist standing on a cell a lower-ID colonist passes
	// through still keeps a walker's last step off that cell
	h := occTestGame()
	x := mustPawn(t, h, 5, 5)
	draftedPawn(t, h, 5, 5)
	w := mustPawn(t, h, 4, 5)
	walkTo(h, x, job.Move, pathing.Point{X: 6, Y: 6}) // passing through
	walkTo(h, w, job.Move, pathing.Point{X: 5, Y: 5})
	rebuild(h)
	if v, alt := h.enter(w, Position{4, 5}, Position{5, 5}); v != stepArriveNear || alt == (Position{5, 5}) {
		t.Fatalf("onto a cell with a drafted colonist = %v %v, want arrive near", v, alt)
	}
}
