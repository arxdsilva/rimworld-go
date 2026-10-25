package game

import (
	"testing"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/ai/pathing"
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/sim/entity"
)

// rebuild starts a movement phase and builds the index now.
func rebuild(g *Game) {
	g.buildOccupancy()
	g.occIndex()
}

func occTestGame() *Game { return New(Config{Seed: 1, Width: 14, Height: 12}) }

func mustPawn(t *testing.T, g *Game, x, y int) entity.EntityID {
	t.Helper()
	id, err := g.SpawnPawn(x, y)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

// walkTo gives id a job of kind k walking path to its last cell.
func walkTo(g *Game, id entity.EntityID, k job.Kind, path ...pathing.Point) {
	last := path[len(path)-1]
	g.jobs.Set(id, job.Record{Kind: k, State: job.Goto, TX: last.X, TY: last.Y, Path: path})
}

// straight is the cells from (x0, y) to (x1, y), excluding x0.
func straight(x0, x1, y int) []pathing.Point {
	var p []pathing.Point
	for x := x0; x != x1; {
		if x1 > x0 {
			x++
		} else {
			x--
		}
		p = append(p, pathing.Point{X: x, Y: y})
	}
	return p
}

func setBlocked(g *Game, id entity.EntityID, ticks int, by entity.EntityID) {
	m := g.motionOf(id)
	m.BlockedTicks, m.BlockedBy = ticks, by
	g.motion.Set(id, m)
}

func pos(g *Game, id entity.EntityID) Position {
	p, _ := g.positions.Get(id)
	return p
}

func TestEnterFreeCellGoesAndReindexes(t *testing.T) {
	g := occTestGame()
	a := mustPawn(t, g, 2, 2)
	walkTo(g, a, job.Move, straight(2, 5, 2)...)
	rebuild(g)
	if id, ok := g.occupantAt(2, 2); !ok || id != a {
		t.Fatalf("occupantAt(2,2) = %d,%v, want %d", id, ok, a)
	}
	v, alt := g.enter(a, Position{2, 2}, Position{3, 2})
	if v != stepGo || alt != (Position{3, 2}) {
		t.Fatalf("enter = %v %v, want go", v, alt)
	}
	if _, ok := g.occupantAt(2, 2); ok {
		t.Fatal("old cell still indexed")
	}
	if id, _ := g.occupantAt(3, 2); id != a {
		t.Fatal("new cell not indexed")
	}
}

func TestEnterBlockedByStandingPawn(t *testing.T) {
	// PLAY-010: a colonist no longer blocks another colonist; the walker
	// steps into the worker's cell at once and both stay indexed there
	g := occTestGame()
	a := mustPawn(t, g, 2, 5)
	b := mustPawn(t, g, 3, 5)
	walkTo(g, a, job.Move, straight(2, 8, 5)...)
	g.jobs.Set(b, job.Record{Kind: job.Construct, State: job.Do, TX: 3, TY: 4})
	rebuild(g)
	if v, alt := g.enter(a, Position{2, 5}, Position{3, 5}); v != stepGo || alt != (Position{3, 5}) {
		t.Fatalf("colonist into a working colonist = %v %v, want go", v, alt)
	}
	if i := 5*g.occ.w + 3; g.occ.pcount[i] != 2 || g.occ.pfirst[i] != a && g.occ.pfirst[i] != b {
		t.Fatalf("shared cell indexes %d partners (first %d), want both", g.occ.pcount[i], g.occ.pfirst[i])
	}

	// a standing wild animal still blocks: the colonist plans around it at once
	g = occTestGame()
	a = mustPawn(t, g, 2, 5)
	b = wildAt(t, g, 3, 5)
	walkTo(g, a, job.Move, straight(2, 8, 5)...)
	rebuild(g)
	v, _ := g.enter(a, Position{2, 5}, Position{3, 5})
	if v == stepGo {
		t.Fatal("walked into a standing wild animal")
	}
	if v != stepRepath {
		t.Fatalf("verdict = %v, want repath", v)
	}
	p, ok := g.pathAround(a, 8, 5)
	if !ok || len(p) == 0 {
		t.Fatal("no route around")
	}
	for _, s := range p {
		if s == (pathing.Point{X: 3, Y: 5}) {
			t.Fatalf("route %v goes through the animal", p)
		}
	}
	if p[len(p)-1] != (pathing.Point{X: 8, Y: 5}) {
		t.Fatalf("route %v does not end at the goal", p)
	}
	// once only: the next repath waits for the next 45-tick window
	setBlocked(g, a, 1, b)
	if v, _ := g.enter(a, Position{2, 5}, Position{3, 5}); v != stepWait {
		t.Fatalf("verdict at 1 blocked tick = %v, want wait", v)
	}

	// a walker moving on elsewhere is waited for, not planned around
	animalWalk(g, b, pathing.Point{X: 3, Y: 4})
	setBlocked(g, a, 0, 0)
	rebuild(g)
	if v, _ := g.enter(a, Position{2, 5}, Position{3, 5}); v != stepWait {
		t.Fatalf("behind a walker: %v, want wait", v)
	}
	if id, _ := g.occupantAt(3, 5); id != b {
		t.Fatal("blocker no longer indexed")
	}
}

func TestDownedPawnDoesNotBlock(t *testing.T) {
	g := occTestGame()
	a := mustPawn(t, g, 2, 5)
	b := mustPawn(t, g, 3, 5)
	c := mustPawn(t, g, 4, 5)
	walkTo(g, a, job.Move, straight(2, 6, 5)...)
	g.down(b, health.CauseInjury)
	g.jobs.Set(c, job.Record{Kind: job.Sleep, State: job.Do, TX: 4, TY: 5})
	rebuild(g)
	if v, _ := g.enter(a, Position{2, 5}, Position{3, 5}); v != stepGo {
		t.Fatalf("downed pawn blocked: %v", v)
	}
	if v, _ := g.enter(a, Position{3, 5}, Position{4, 5}); v != stepGo {
		t.Fatalf("sleeping pawn blocked: %v", v)
	}
}

func TestCarriedPawnDoesNotBlock(t *testing.T) {
	g := occTestGame()
	a := mustPawn(t, g, 2, 5)
	carrier := mustPawn(t, g, 3, 6)
	patient := mustPawn(t, g, 3, 5)
	g.carrying[carrier] = patient
	walkTo(g, a, job.Move, straight(2, 6, 5)...)
	rebuild(g)
	if v, _ := g.enter(a, Position{2, 5}, Position{3, 5}); v != stepGo {
		t.Fatalf("carried pawn blocked: %v", v)
	}
	if g.standing(patient) || !g.standing(carrier) {
		t.Fatal("standing: carried pawn must not stand, its carrier must")
	}
}

func TestIdlePawnStepsAside(t *testing.T) {
	// a 1-wide corridor along y=5 with a bay at (5,4)
	corridor := func() *Game {
		g := occTestGame()
		for x := 1; x < 12; x++ {
			g.grid.SetWalkable(x, 4, x == 5)
			g.grid.SetWalkable(x, 6, false)
		}
		return g
	}
	// PLAY-010: a colonist walks through an idle colonist; nobody moves aside
	g := corridor()
	a := mustPawn(t, g, 2, 5)
	b := mustPawn(t, g, 3, 5)
	walkTo(g, a, job.Move, straight(2, 10, 5)...)
	g.jobs.Set(b, job.Record{Kind: job.Wander, State: job.Do, TX: 3, TY: 5, WorkLeft: 400})
	rebuild(g)
	if v, _ := g.enter(a, Position{2, 5}, Position{3, 5}); v != stepGo {
		t.Fatalf("colonist past an idler = %v, want go", v)
	}
	if j, _ := g.jobs.Get(b); j.State != job.Do {
		t.Fatalf("idler moved for a colonist: %+v", j)
	}

	// a wild animal still waits, and the idle colonist makes way for it
	g = corridor()
	b = mustPawn(t, g, 3, 5)
	d := wildAt(t, g, 2, 5)
	animalWalk(g, d, straight(2, 10, 5)...)
	g.jobs.Set(b, job.Record{Kind: job.Wander, State: job.Do, TX: 3, TY: 5, WorkLeft: 400})
	rebuild(g)
	if v, _ := g.enter(d, Position{2, 5}, Position{3, 5}); v != stepWait {
		t.Fatalf("verdict = %v, want wait (no way around)", v)
	}
	if j, _ := g.jobs.Get(b); j.State != job.Do {
		t.Fatal("idler moved before the walker waited 30 ticks")
	}
	setBlocked(g, d, idleYieldAfter, b)
	if v, _ := g.enter(d, Position{2, 5}, Position{3, 5}); v != stepWait {
		t.Fatalf("verdict = %v, want wait while it makes way", v)
	}
	j, _ := g.jobs.Get(b)
	if j.Kind != job.Wander || j.State != job.Goto || len(j.Path) == 0 {
		t.Fatalf("idler was not sent aside: %+v", j)
	}
	if dest := j.Path[len(j.Path)-1]; dest != (pathing.Point{X: 5, Y: 4}) {
		t.Fatalf("idler goes to %v, want the bay (5,4) off the walker's path", dest)
	}

	// the full movement phase then clears the corridor
	for range 400 {
		g.phaseMovement()
		g.clock.Advance(1)
		if pos(g, d) == pos(g, b) {
			t.Fatal("walker and idler share a cell")
		}
	}
	if pos(g, d) != (Position{10, 5}) {
		t.Fatalf("walker at %v, want (10,5)", pos(g, d))
	}
}

func TestDraftedAndRaiderDoNotMakeWay(t *testing.T) {
	g := occTestGame()
	b := mustPawn(t, g, 3, 5)
	g.setDraft(b, true)
	if g.idleYieldable(b) {
		t.Fatal("a drafted pawn must hold its cell")
	}
	g.raiders[b] = raiderState{}
	g.draft[b] = false
	if g.idleYieldable(b) {
		t.Fatal("a raider never makes way")
	}
	delete(g.raiders, b)
	g.breaks[b] = breakState{}
	if g.idleYieldable(b) {
		t.Fatal("a pawn in a mental break does not take hints")
	}
}

func TestHeadOnSwapResolves(t *testing.T) {
	// PLAY-010: two colonists head-on simply pass through each other
	g := occTestGame()
	a := mustPawn(t, g, 3, 5)
	b := mustPawn(t, g, 4, 5)
	walkTo(g, a, job.Move, straight(3, 9, 5)...)
	walkTo(g, b, job.Move, straight(4, 0, 5)...)
	rebuild(g)
	if v, _ := g.enter(a, Position{3, 5}, Position{4, 5}); v != stepGo {
		t.Fatalf("colonist head-on, lower ID = %v, want go", v)
	}
	if v, _ := g.enter(b, Position{4, 5}, Position{3, 5}); v != stepGo {
		t.Fatalf("colonist head-on, higher ID = %v, want go", v)
	}

	// a colonist and a wild animal head-on: the higher ID (the animal) yields
	g = occTestGame()
	a = mustPawn(t, g, 3, 5) // lower ID: keeps its line
	b = wildAt(t, g, 4, 5)   // higher ID: yields
	walkTo(g, a, job.Move, straight(3, 9, 5)...)
	animalWalk(g, b, straight(4, 0, 5)...)
	rebuild(g)
	if v, _ := g.enter(a, Position{3, 5}, Position{4, 5}); v != stepWait {
		t.Fatalf("lower ID verdict %v, want wait", v)
	}
	v, alt := g.enter(b, Position{4, 5}, Position{3, 5})
	if v != stepSidestep {
		t.Fatalf("higher ID verdict %v, want sidestep", v)
	}
	if alt.Y == 5 || pathing.Dist(alt.X, alt.Y, 4, 5) != 1 {
		t.Fatalf("sidestep to %v: must leave row 5 by one step", alt)
	}
	// it chooses the side cell nearest its own goal (west), lower y first
	if alt != (Position{3, 4}) {
		t.Fatalf("sidestep to %v, want (3,4)", alt)
	}
	if id, _ := g.occupantAt(3, 4); id != b {
		t.Fatal("sidestep not indexed")
	}

	// in a 1-wide corridor the yielder backs off along the other's way
	h := corridorGame()
	a = mustPawn(t, h, 3, 5)
	b = wildAt(t, h, 4, 5)
	walkTo(h, a, job.Move, straight(3, 9, 5)...)
	animalWalk(h, b, straight(4, 0, 5)...)
	rebuild(h)
	v, alt = h.enter(b, Position{4, 5}, Position{3, 5})
	if v != stepSidestep || alt != (Position{5, 5}) {
		t.Fatalf("corridor yield = %v %v, want back off to (5,5)", v, alt)
	}
	// between retries (every 15 blocked ticks) it just waits
	rebuild(h)
	setBlocked(h, b, 7, a)
	if v, _ := h.enter(b, Position{4, 5}, Position{3, 5}); v != stepWait {
		t.Fatalf("off-retry verdict %v, want wait", v)
	}
}

func TestHostileTargetIsNotAYieldPartner(t *testing.T) {
	g := occTestGame()
	a := mustPawn(t, g, 2, 5)
	b := mustPawn(t, g, 3, 5)
	walkTo(g, a, job.Attack, pathing.Point{X: 3, Y: 5})
	j, _ := g.jobs.Get(a)
	j.Target = b
	g.jobs.Set(a, j)
	setBlocked(g, a, 60, b)
	rebuild(g)
	if v, _ := g.enter(a, Position{2, 5}, Position{3, 5}); v != stepWait {
		t.Fatalf("attacker verdict %v, want wait next to its target", v)
	}
	if j, _ := g.jobs.Get(b); j.Kind != job.None {
		t.Fatalf("target was pushed aside: %+v", j)
	}
}

func TestArriveNearWhenDestinationTaken(t *testing.T) {
	// the last cell is held by a blocker (a wild animal standing still);
	// colonists in one another's way are covered in walkthrough_test.go
	g := occTestGame()
	a := mustPawn(t, g, 4, 5)
	wildAt(t, g, 5, 5)

	// an adjacency job: 'from' already touches the work cell
	g.jobs.Set(a, job.Record{Kind: job.Cut, State: job.Goto, TX: 5, TY: 5, Path: []pathing.Point{{X: 5, Y: 5}}})
	rebuild(g)
	if v, alt := g.enter(a, Position{4, 5}, Position{5, 5}); v != stepArriveNear || alt != (Position{4, 5}) {
		t.Fatalf("adjacency job = %v %v, want arrive here", v, alt)
	}

	// mining works from any cell touching the rock
	g.positions.Set(a, Position{3, 3})
	g.jobs.Set(a, job.Record{Kind: job.Mine, State: job.Goto, TX: 4, TY: 4, DestX: 5, DestY: 4,
		Path: []pathing.Point{{X: 4, Y: 4}}})
	c := wildAt(t, g, 4, 4)
	rebuild(g)
	v, alt := g.enter(a, Position{3, 3}, Position{4, 4})
	if v != stepArriveNear || pathing.Dist(alt.X, alt.Y, 5, 4) != 1 || alt == (Position{4, 4}) {
		t.Fatalf("mine job = %v %v, want a free cell next to the rock", v, alt)
	}

	// a move order takes the nearest free cell
	g.positions.Set(a, Position{4, 6})
	g.positions.Set(c, Position{12, 10})
	g.jobs.Set(a, job.Record{Kind: job.Move, State: job.Goto, TX: 5, TY: 5, Path: []pathing.Point{{X: 5, Y: 5}}})
	rebuild(g)
	v, alt = g.enter(a, Position{4, 6}, Position{5, 5})
	if v != stepArriveNear || alt != (Position{4, 5}) {
		t.Fatalf("move order = %v %v, want nearest free (4,5)", v, alt)
	}

	// an animal ends at the nearest free cell
	d, err := g.SpawnAnimal("dustbuck", 6, 6)
	if err != nil {
		t.Fatal(err)
	}
	an, _ := g.animals.Get(d)
	an.Path, an.GoalX, an.GoalY = []pathing.Point{{X: 5, Y: 5}}, 5, 5
	g.animals.Set(d, an)
	rebuild(g)
	if v, alt := g.enter(d, Position{6, 6}, Position{5, 5}); v != stepArriveNear || alt != (Position{6, 5}) {
		t.Fatalf("animal = %v %v, want nearest free (6,5)", v, alt)
	}
	// with nothing nearer, it stops where it stands
	g.positions.Set(d, Position{4, 4})
	g.positions.Set(a, Position{5, 4})
	g.positions.Set(c, Position{4, 5})
	rebuild(g)
	if v, alt := g.enter(d, Position{4, 4}, Position{5, 5}); v != stepArriveNear || alt != (Position{4, 4}) {
		t.Fatalf("animal = %v %v, want stop at (4,4)", v, alt)
	}
}

func TestExactCellJobsWait(t *testing.T) {
	kinds := append([]job.Kind(nil), exactCellKinds...)
	for _, k := range append(kinds, job.Haul, job.Eat) {
		g := occTestGame()
		a := mustPawn(t, g, 4, 5)
		b := mustPawn(t, g, 5, 5)
		g.setDraft(b, true) // in combat: never shares its cell (PLAY-010)
		rec := job.Record{Kind: k, State: job.Goto, TX: 5, TY: 5, Path: []pathing.Point{{X: 5, Y: 5}}}
		if k == job.Haul || k == job.Eat {
			rec.Carry = 99 // carrying it to its cell
		}
		g.jobs.Set(a, rec)
		rebuild(g)
		if v, _ := g.enter(a, Position{4, 5}, Position{5, 5}); v != stepWait {
			t.Fatalf("%v: verdict %v, want wait for the exact cell", k, v)
		}
		if g.arrivalOf(a) != arriveExact {
			t.Fatalf("%v: arrival %v, want exact", k, g.arrivalOf(a))
		}
	}
	// an idling animal on a bed cell makes way after 30 ticks
	g := occTestGame()
	a := mustPawn(t, g, 4, 5)
	b := wildAt(t, g, 5, 5)
	g.jobs.Set(a, job.Record{Kind: job.Sleep, State: job.Goto, TX: 5, TY: 5, Path: []pathing.Point{{X: 5, Y: 5}}})
	setBlocked(g, a, idleYieldAfter, b)
	rebuild(g)
	if v, _ := g.enter(a, Position{4, 5}, Position{5, 5}); v != stepWait {
		t.Fatalf("verdict %v, want wait", v)
	}
	if an, _ := g.animals.Get(b); len(an.Path) == 0 {
		t.Fatalf("idler on the bed cell not sent aside: %+v", an)
	}
}

func TestThreeCycleHighestYields(t *testing.T) {
	// drafted: each last step lands on a colonist in combat, so the ring
	// resolves as before (out of combat they all just go: walkthrough_test.go)
	g := occTestGame()
	a := draftedPawn(t, g, 5, 5)
	b := draftedPawn(t, g, 6, 5)
	c := draftedPawn(t, g, 6, 6)
	walkTo(g, a, job.Move, pathing.Point{X: 6, Y: 5})
	walkTo(g, b, job.Move, pathing.Point{X: 6, Y: 6})
	walkTo(g, c, job.Move, pathing.Point{X: 5, Y: 5})
	rebuild(g)
	if v, _ := g.enter(a, Position{5, 5}, Position{6, 5}); v != stepWait {
		t.Fatalf("a: %v, want wait", v)
	}
	if v, _ := g.enter(b, Position{6, 5}, Position{6, 6}); v != stepWait {
		t.Fatalf("b: %v, want wait", v)
	}
	v, alt := g.enter(c, Position{6, 6}, Position{5, 5})
	if v != stepSidestep || alt == (Position{5, 5}) || alt == (Position{6, 5}) {
		t.Fatalf("c: %v %v, want sidestep off the ring", v, alt)
	}
}

func TestUnstackHigherIDMovesOff(t *testing.T) {
	g := occTestGame()
	a := mustPawn(t, g, 5, 5)
	b := mustPawn(t, g, 5, 5)
	g.setDraft(b, true)
	rebuild(g)
	if id, _ := g.occupantAt(5, 5); id != a {
		t.Fatalf("cell holder %d, want the lower ID %d", id, a)
	}
	j, _ := g.jobs.Get(b)
	if j.State != job.Goto || len(j.Path) != 1 {
		t.Fatalf("stacked pawn got no step off: %+v", j)
	}
	// a working pawn on a drafted pawn's cell keeps its job and steps next
	// to its work
	h := occTestGame()
	draftedPawn(t, h, 5, 5)
	w := mustPawn(t, h, 5, 5)
	h.jobs.Set(w, job.Record{Kind: job.Construct, State: job.Do, TX: 7, TY: 5, WorkLeft: 50})
	rebuild(h)
	j, _ = h.jobs.Get(w)
	if j.Kind != job.Construct || j.State != job.Goto || j.Path[0] != (pathing.Point{X: 6, Y: 5}) {
		t.Fatalf("working pawn detour = %+v, want (6,5) next to its work", j)
	}
	// once off, the walk goes back to work at the new cell
	for range 30 {
		h.phaseMovement()
		h.clock.Advance(1)
	}
	if j, _ := h.jobs.Get(w); j.State != job.Do || pos(h, w) != (Position{6, 5}) {
		t.Fatalf("after detour: %+v at %v", j, pos(h, w))
	}
	// animals too
	k := occTestGame()
	d1, _ := k.SpawnAnimal("dustbuck", 3, 3)
	d2, _ := k.SpawnAnimal("dustbuck", 3, 3)
	rebuild(k)
	if a1, _ := k.animals.Get(d1); len(a1.Path) != 0 {
		t.Fatal("the lower ID must stay")
	}
	if a2, _ := k.animals.Get(d2); len(a2.Path) != 1 {
		t.Fatal("stacked animal got no step off")
	}
}

func TestAnimalRepathAround(t *testing.T) {
	g := occTestGame()
	d, err := g.SpawnAnimal("dustbuck", 2, 5)
	if err != nil {
		t.Fatal(err)
	}
	p := mustPawn(t, g, 3, 5)
	g.jobs.Set(p, job.Record{Kind: job.Construct, State: job.Do, TX: 3, TY: 4})
	a, _ := g.animals.Get(d)
	a.Path, a.GoalX, a.GoalY = straight(2, 8, 5), 8, 5
	g.animals.Set(d, a)
	rebuild(g)
	if v, _ := g.enter(d, Position{2, 5}, Position{3, 5}); v != stepWait {
		t.Fatalf("animal verdict %v, want wait before %d blocked ticks", v, repathAfter)
	}
	setBlocked(g, d, repathAfter, p)
	v, _ := g.enter(d, Position{2, 5}, Position{3, 5})
	if v != stepRepath {
		t.Fatalf("animal verdict %v, want repath", v)
	}
	path, ok := g.pathAround(d, a.GoalX, a.GoalY)
	if !ok || path[len(path)-1] != (pathing.Point{X: 8, Y: 5}) {
		t.Fatalf("animal route %v", path)
	}
	for _, s := range path {
		if s == (pathing.Point{X: 3, Y: 5}) {
			t.Fatalf("animal route %v crosses the pawn", path)
		}
	}
}

func TestPathAroundUsesMoverQuery(t *testing.T) {
	g := occTestGame()
	// a wall across x=6 with one door at (6,5)
	for y := 0; y < 12; y++ {
		if y != 5 {
			g.grid.SetWalkable(6, y, false)
		}
	}
	if !g.BuildNow("door", "wood", "east", 6, 5) {
		t.Fatal("door not built")
	}
	wild, _ := g.SpawnAnimal("husky", 3, 5)
	rebuild(g)
	if _, ok := g.pathAround(wild, 9, 5); ok {
		t.Fatal("a wild animal must not plan through a door")
	}
	a, _ := g.animals.Get(wild)
	a.Tame = true
	g.animals.Set(wild, a)
	if p, ok := g.pathAround(wild, 9, 5); !ok || p[len(p)-1] != (pathing.Point{X: 9, Y: 5}) {
		t.Fatalf("a tame animal uses the door: %v %v", p, ok)
	}
	// PLAY-015: livestock (tame or not) keep out of doors unless led
	a.Species = animal.Cow
	g.animals.Set(wild, a)
	if _, ok := g.pathAround(wild, 9, 5); ok {
		t.Fatal("a tame cow must not plan through a door on its own")
	}
	col := mustPawn(t, g, 3, 7)
	rebuild(g)
	if p, ok := g.pathAround(col, 9, 5); !ok || len(p) == 0 {
		t.Fatal("a colonist uses the door")
	}
}

func TestPathAroundUnwalkableTarget(t *testing.T) {
	g := occTestGame()
	g.grid.SetWalkable(8, 5, false)
	a := mustPawn(t, g, 2, 5)
	wildAt(t, g, 7, 5) // a blocker (a colonist there would be walked through)
	rebuild(g)
	p, ok := g.pathAround(a, 8, 5)
	if !ok || len(p) == 0 {
		t.Fatal("expected a route next to the target")
	}
	end := p[len(p)-1]
	if !pathing.Adjacent(end.X, end.Y, 8, 5) || end == (pathing.Point{X: 7, Y: 5}) {
		t.Fatalf("route ends at %v, want a free cell beside (8,5)", end)
	}
	if p, ok := g.pathAround(a, 2, 5); !ok || len(p) != 0 {
		t.Fatal("already there")
	}
	g.positions.Set(a, Position{9, 5})
	if p, ok := g.pathAround(a, 8, 5); !ok || len(p) != 0 {
		t.Fatal("already next to an unwalkable target")
	}
}

func TestCrowdCostPricesCreatures(t *testing.T) {
	g := occTestGame()
	a := mustPawn(t, g, 2, 2)
	b := wildAt(t, g, 5, 5)
	w := wildAt(t, g, 8, 8)
	animalWalk(g, w, pathing.Point{X: 9, Y: 8})
	col := mustPawn(t, g, 5, 9)
	rebuild(g)
	c := g.costedQuery(a).(pathing.Coster)
	if got := c.ExtraCost(5, 5); got != occupiedCost {
		t.Fatalf("occupied = %d", got)
	}
	if got := c.ExtraCost(4, 5); got != besideCost {
		t.Fatalf("beside a stander = %d", got)
	}
	if got := c.ExtraCost(7, 8); got != 0 {
		t.Fatalf("beside a walker = %d, want 0", got)
	}
	if got := c.ExtraCost(2, 2); got != 0 {
		t.Fatalf("own cell = %d, want 0", got)
	}
	// a colonist is free ground for another colonist (PLAY-010)...
	if got := c.ExtraCost(5, 9); got != 0 {
		t.Fatalf("a colonist's cell for a colonist = %d, want 0", got)
	}
	if got := c.ExtraCost(4, 9); got != 0 {
		t.Fatalf("beside a colonist for a colonist = %d, want 0", got)
	}
	// ...but not for a wild animal
	wc := g.costedQuery(b).(pathing.Coster)
	if got := wc.ExtraCost(5, 9); got != occupiedCost {
		t.Fatalf("a colonist's cell for a wild animal = %d", got)
	}
	if got := wc.ExtraCost(4, 9); got != besideCost {
		t.Fatalf("beside a colonist for a wild animal = %d", got)
	}
	if got := wc.ExtraCost(5, 5); got != 0 {
		t.Fatalf("own cell = %d, want 0", got)
	}
	_ = col
}

func TestOccupancyIndexIsLazyAndBounded(t *testing.T) {
	g := occTestGame()
	a := mustPawn(t, g, 1, 1)
	// off the 15-tick sweep, starting a phase builds nothing
	for uint64(g.clock.Now())%yieldRetry == 0 {
		g.clock.Advance(1)
	}
	g.buildOccupancy()
	if g.occ.cells != nil || !g.occ.dirty {
		t.Fatal("index built eagerly off the sweep tick")
	}
	if id, ok := g.occupantAt(1, 1); !ok || id != a {
		t.Fatal("occupantAt must build the index on first use")
	}
	if _, ok := g.occupantAt(-1, 40); ok {
		t.Fatal("out of bounds is never occupied")
	}
	g.vital.Set(a, health.Record{Status: health.Dead})
	rebuild(g)
	if _, ok := g.occupantAt(1, 1); ok {
		t.Fatal("dead pawns do not block")
	}
	d, _ := g.SpawnAnimal("dustbuck", 4, 4)
	an, _ := g.animals.Get(d)
	an.Injury = animal.DefOf(an.Species).MaxHP
	g.animals.Set(d, an)
	if g.standing(d) {
		t.Fatal("a dead animal does not stand")
	}
}
