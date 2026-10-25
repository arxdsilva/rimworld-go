package game

import (
	"testing"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/ai/work"
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/pawn/schedule"
	"rimworld-go/internal/domain/sim/entity"
)

// kingDist is the king-move distance between two cells.
func kingDist(ax, ay, bx, by int) int { return max(abs(ax-bx), abs(ay-by)) }

// nearFire reports a burning cell within r of (x, y).
func nearFire(g *Game, x, y, r int) bool {
	for dy := -r; dy <= r; dy++ {
		for dx := -r; dx <= r; dx++ {
			if g.fireAt(x+dx, y+dy) > 0 {
				return true
			}
		}
	}
	return false
}

func TestFireAvoidCost(t *testing.T) {
	t.Parallel()
	g := dirtGame(t)
	if g.fireAvoidCost(10, 6) != 0 {
		t.Fatal("cost with no fire")
	}
	burn(g, 10, 6, 30)
	if c := g.fireAvoidCost(10, 6); c != fireCostBurning {
		t.Fatalf("burning cell cost %d", c)
	}
	if c := g.fireAvoidCost(11, 7); c != fireCostBeside {
		t.Fatalf("cell beside cost %d", c)
	}
	if c := g.fireAvoidCost(12, 6); c != 0 {
		t.Fatalf("two cells off cost %d", c)
	}
}

// TestPathAvoidsFire: a walk across a burning line goes round it when it
// can.
func TestPathAvoidsFire(t *testing.T) {
	t.Parallel()
	g := dirtGame(t)
	id, err := g.SpawnNamedPawn(3, 6, "Ada")
	if err != nil {
		t.Fatal(err)
	}
	for y := 3; y <= 9; y++ {
		burn(g, 10, y, 60)
	}
	g.positions.Set(id, Position{X: 3, Y: 6})
	path, ok := g.pathTo(id, 17, 6)
	if !ok {
		t.Fatal("no path")
	}
	for _, p := range path {
		if g.fireAt(p.X, p.Y) > 0 {
			t.Fatalf("path walks through fire at %v: %v", p, path)
		}
	}
}

// TestIdleColonistStepsOutNoFlap: an idle colonist beside a fire walks
// clear of it and doesn't drift back while it burns.
func TestIdleColonistStepsOutNoFlap(t *testing.T) {
	t.Parallel()
	g := dirtGame(t)
	scheduleAll(g, schedule.Anything)
	id, err := g.SpawnNamedPawn(9, 6, "Ada")
	if err != nil {
		t.Fatal(err)
	}
	for wt := work.Type(0); wt < work.NumTypes; wt++ {
		g.setPriority(id, wt, 0) // nothing to do but idle
	}
	left, back := -1, 0
	near := true
	for tick := range 3000 {
		burn(g, 10, 6, 40) // keeps burning
		g.Step(1)
		p, _ := g.positions.Get(id)
		now := kingDist(p.X, p.Y, 10, 6) <= 1
		switch {
		case near && !now && left < 0:
			left = tick
		case !near && now:
			back++
		}
		near = now
	}
	if left < 0 || left > 200 {
		t.Fatalf("stepped out at tick %d", left)
	}
	if back > 0 {
		t.Fatalf("came back beside the fire %d times", back)
	}
}

// TestCaughtColonistLeavesFire: a colonist standing in fire runs out of
// it, whatever it was doing; a drafted one stays put.
func TestCaughtColonistLeavesFire(t *testing.T) {
	t.Parallel()
	for _, drafted := range []bool{false, true} {
		g := dirtGame(t)
		scheduleAll(g, schedule.Work)
		id, err := g.SpawnNamedPawn(10, 6, "Ada")
		if err != nil {
			t.Fatal(err)
		}
		g.setDraft(id, drafted)
		for range 120 {
			burn(g, 10, 6, 40)
			g.Step(1)
		}
		p, _ := g.positions.Get(id)
		if out := g.fireAt(p.X, p.Y) == 0; out == drafted {
			t.Fatalf("drafted %v: at %v, out of the fire %v", drafted, p, out)
		}
	}
}

// TestFirefighterStaysInFire: a firefighter at work is not chased off.
func TestFirefighterStaysInFire(t *testing.T) {
	t.Parallel()
	g := dirtGame(t)
	id := firefighter(t, g, 10, 6, "Ada")
	burn(g, 10, 6, 40)
	j := job.Record{Kind: job.FightFire, State: job.Do, TX: 10, TY: 6, DestX: 11, DestY: 6, WorkLeft: fireBeatTicks}
	if g.fireDanger(id, j) {
		t.Fatal("a firefighter was sent out of the fire")
	}
}

// animalBy spawns a deer at (x, y).
func animalBy(t *testing.T, g *Game, x, y int) entity.EntityID {
	t.Helper()
	id, err := g.SpawnAnimal("deer", x, y)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

// TestAnimalFleesFireNoFlap: an animal within fireFleeR of a fire runs
// clear of it, and doesn't keep coming back to run again.
func TestAnimalFleesFireNoFlap(t *testing.T) {
	t.Parallel()
	g := dirtGame(t)
	aid := animalBy(t, g, 12, 6)
	flees, fled := 0, false
	for range 3000 {
		burn(g, 10, 6, 60)
		g.Step(1)
		a, ok := g.animals.Get(aid)
		if !ok {
			t.Fatal("deer gone")
		}
		if a.Mode == animal.Flee && !fled {
			flees++
		}
		fled = a.Mode == animal.Flee
	}
	p, _ := g.positions.Get(aid)
	if flees == 0 || kingDist(p.X, p.Y, 10, 6) <= fireFleeR {
		t.Fatalf("deer at %v after %d flights", p, flees)
	}
	if flees > 2 {
		t.Fatalf("deer fled %d times: flapping", flees)
	}
}

func TestIdleWalkAvoidsFire(t *testing.T) {
	t.Parallel()
	g := dirtGame(t)
	if g.idleFireBlocked(5, 5) {
		t.Fatal("blocked with no fire")
	}
	burn(g, 10, 6, 40)
	if !g.idleFireBlocked(12, 6) || g.idleFireBlocked(13, 6) {
		t.Fatal("idle goals within fireSafeDist-1 of a fire are blocked, farther ones not")
	}
	a := animal.State{Mode: animal.Wander}
	if !g.animalFireBlocked(&a, 15, 6) || g.animalFireBlocked(&a, 16, 6) {
		t.Fatal("animal goals within fireFleeR+1 are blocked")
	}
	a.Mode = animal.Flee
	if g.animalFireBlocked(&a, 11, 6) {
		t.Fatal("a fleeing animal's run is never blocked")
	}
	if !nearFire(g, 11, 6, 1) {
		t.Fatal("fixture")
	}
}
