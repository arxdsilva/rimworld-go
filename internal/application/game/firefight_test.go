package game

import (
	"testing"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/ai/work"
	"rimworld-go/internal/domain/pawn/schedule"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// burn sets the fire on (x, y) to size (adding it when it isn't burning):
// a fixture standing in for ignition, re-applied by tests that need a fire
// to keep burning whatever the fire simulation does to it.
func burn(g *Game, x, y, size int) {
	if i := g.fireIndex(x, y); i >= 0 {
		g.fire.fires[i].Size = uint8(size)
		return
	}
	g.startFire(x, y, size) // fire.go
}

// firefighter spawns a colonist who only fights fires, in work hours.
func firefighter(t testing.TB, g *Game, x, y int, name string) entity.EntityID {
	t.Helper()
	id, err := g.SpawnNamedPawn(x, y, name)
	if err != nil {
		t.Fatal(err)
	}
	for wt := work.Type(0); wt < work.NumTypes; wt++ {
		g.setPriority(id, wt, 0)
	}
	g.setPriority(id, work.Firefight, work.DefaultPriority)
	scheduleAll(g, schedule.Work)
	return id
}

func TestFirefightOnByDefault(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 5, Width: 20, Height: 20})
	id, err := g.SpawnNamedPawn(3, 3, "Ada")
	if err != nil {
		t.Fatal(err)
	}
	if p := g.prioOf(id); p[work.Firefight] != work.DefaultPriority {
		t.Fatalf("new colonist's firefighting priority %d", p[work.Firefight])
	}
	if work.Order[0] != work.Firefight {
		t.Fatal("firefighting should come first within a priority")
	}
}

// TestFireEligibilityMatrix: a colonist fights a fire on its own only
// inside Home AND its allowed area (unrestricted: Home only); the
// Extinguish order reaches any fire.
func TestFireEligibilityMatrix(t *testing.T) {
	t.Parallel()
	g, id := areaGame(t) // area: x 0..9
	g.paintHome(0, 0, 29, 19, false)
	g.paintHome(0, 0, 14, 10, true) // Home: x 0..14, y 0..10
	cells := map[string][2]int{
		"home+area":   {5, 5},
		"home only":   {12, 5},
		"area only":   {5, 15},
		"neither":     {20, 15},
		"off the map": {-1, 5},
	}
	restricted := map[string]bool{"home+area": true}
	free := map[string]bool{"home+area": true, "home only": true}
	for name, c := range cells {
		if got := g.fireEligible(id, c[0], c[1]); got != restricted[name] {
			t.Errorf("restricted, %s: eligible %v", name, got)
		}
	}
	g.Enqueue(command.Command{Kind: command.AssignArea, Pawn: uint64(id), Index: command.AreaUnrestricted})
	g.ApplyPending()
	for name, c := range cells {
		if got := g.fireEligible(id, c[0], c[1]); got != free[name] {
			t.Errorf("unrestricted, %s: eligible %v", name, got)
		}
	}
}

func TestFirefighterIgnoresFireOutsideHomeOrArea(t *testing.T) {
	t.Parallel()
	g, id := areaGame(t) // area: x 0..9
	scheduleAll(g, schedule.Work)
	for wt := work.Type(0); wt < work.NumTypes; wt++ {
		g.setPriority(id, wt, 0)
	}
	g.setPriority(id, work.Firefight, 1)
	g.paintHome(0, 0, 29, 19, false)
	g.paintHome(0, 0, 14, 10, true)
	burn(g, 12, 5, 50) // Home, outside the area
	burn(g, 5, 15, 50) // the area, outside Home
	if g.assignFightFire(id) {
		t.Fatal("fought a fire outside Home or its area")
	}
	burn(g, 6, 5, 50)
	if !g.assignFightFire(id) {
		t.Fatal("ignored a fire in Home and its area")
	}
	if j, _ := g.jobs.Get(id); j.Kind != job.FightFire || j.DestX != 6 || j.DestY != 5 || g.fireAt(j.TX, j.TY) != 0 ||
		max(abs(j.TX-6), abs(j.TY-5)) != 1 {
		t.Fatalf("job %+v: should stand beside (6, 5)", j)
	}
}

func TestFirefighterPutsOutFire(t *testing.T) {
	t.Parallel()
	g := dirtGame(t)
	id := firefighter(t, g, 3, 6, "Ada")
	g.paintHome(0, 0, 29, 13, true)
	burn(g, 10, 6, 30)
	fought := false
	for range 600 {
		g.Step(1)
		if j, _ := g.jobs.Get(id); j.Kind == job.FightFire && j.State == job.Do {
			fought = true
		}
		if g.fireAt(10, 6) == 0 {
			break
		}
	}
	if !fought {
		t.Fatal("never beat at the fire")
	}
	if g.fireAt(10, 6) != 0 {
		t.Fatalf("fire still at size %d", g.fireAt(10, 6))
	}
}

// TestFightFireBeats: a beat every fireBeatTicks takes fireBeat off, the
// colonist turns to a burning neighbour when its cell is out, and is done
// when nothing beside it burns.
func TestFightFireBeats(t *testing.T) {
	t.Parallel()
	g := dirtGame(t)
	id := firefighter(t, g, 5, 5, "Ada")
	g.paintHome(0, 0, 29, 13, true)
	burn(g, 6, 5, 15)
	burn(g, 6, 6, 40)
	g.jobs.Set(id, job.Record{Kind: job.FightFire, State: job.Do, TX: 5, TY: 5, DestX: 6, DestY: 5, WorkLeft: fireBeatTicks})
	step := func(n int) {
		for range n {
			j, _ := g.jobs.Get(id)
			if j.State != job.Do {
				return
			}
			g.doFightFire(id, j)
		}
	}
	step(fireBeatTicks - 1)
	if g.fireAt(6, 5) != 15 {
		t.Fatalf("beat early: size %d", g.fireAt(6, 5))
	}
	step(1)
	if g.fireAt(6, 5) != 15-fireBeat {
		t.Fatalf("one beat: size %d", g.fireAt(6, 5))
	}
	step(fireBeatTicks)
	if g.fireAt(6, 5) != 0 {
		t.Fatalf("second beat left size %d", g.fireAt(6, 5))
	}
	step(1)
	if j, _ := g.jobs.Get(id); j.DestX != 6 || j.DestY != 6 || j.State != job.Do {
		t.Fatalf("job %+v: should turn to (6, 6)", j)
	}
	step(4 * fireBeatTicks)
	step(1)
	if g.fireAt(6, 6) != 0 {
		t.Fatalf("neighbour left at %d", g.fireAt(6, 6))
	}
	if j, _ := g.jobs.Get(id); j.State != job.Complete {
		t.Fatalf("job %+v: nothing left beside it", j)
	}
}

// TestDraftedExtinguishAnywhere: a drafted colonist's Extinguish order
// reaches a fire outside Home and its area, as a forced job; undrafted
// colonists aren't offered it.
func TestDraftedExtinguishAnywhere(t *testing.T) {
	t.Parallel()
	g, id := areaGame(t) // area: x 0..9
	g.paintHome(0, 0, 29, 19, false)
	burn(g, 20, 10, 40)
	hasOrder := func() (OrderOption, bool) {
		for _, o := range g.OrdersAt(id, 20, 10) {
			if o.Cmd.Order == command.OrderExtinguish {
				return o, true
			}
		}
		return OrderOption{}, false
	}
	if _, ok := hasOrder(); ok {
		t.Fatal("an undrafted colonist is offered Extinguish")
	}
	g.setDraft(id, true)
	o, ok := hasOrder()
	if !ok || o.Disabled {
		t.Fatalf("drafted: Extinguish %+v offered %v", o, ok)
	}
	g.Enqueue(o.Cmd)
	g.ApplyPending()
	j, _ := g.jobs.Get(id)
	if j.Kind != job.FightFire || !j.Forced || j.DestX != 20 || j.DestY != 10 {
		t.Fatalf("job %+v", j)
	}
	for range 400 {
		g.Step(1)
		burn(g, 20, 10, max(1, g.fireAt(20, 10))) // keep it from burning out alone
		if j, _ := g.jobs.Get(id); j.Kind == job.FightFire && j.State == job.Do {
			return
		}
	}
	t.Fatal("never reached the fire")
}

func TestExtinguishNoFire(t *testing.T) {
	t.Parallel()
	g, id := areaGame(t)
	if _, why := g.extinguishJob(id, 20, 10); why != ReasonGone {
		t.Fatalf("no fire: %q", why)
	}
	// a fire walled in on every side can't be reached
	burn(g, 20, 10, 40)
	for dy := -1; dy <= 1; dy++ {
		for dx := -1; dx <= 1; dx++ {
			if dx != 0 || dy != 0 {
				burn(g, 20+dx, 10+dy, 40)
			}
		}
	}
	if _, why := g.extinguishJob(id, 20, 10); why != ReasonUnreachable {
		t.Fatalf("surrounded fire: %q", why)
	}
}

// TestFirefightWorkCommand: the Work tab's Firefight column sets the
// firefighting priority and switches it off and on.
func TestFirefightWorkCommand(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 5, Width: 20, Height: 20})
	id, err := g.SpawnNamedPawn(3, 3, "Ada")
	if err != nil {
		t.Fatal(err)
	}
	if !pawn(t, g, id).WorkFirefight {
		t.Fatal("view: firefighting should be on")
	}
	g.Enqueue(command.Command{Kind: command.SetWorkPriority, Pawn: uint64(id), Work: command.WorkFirefight, Count: 1})
	g.ApplyPending()
	if p := g.prioOf(id); p[work.Firefight] != 1 {
		t.Fatalf("firefighting priority %d", p[work.Firefight])
	}
	g.Enqueue(command.Command{Kind: command.SetWorkEnabled, Pawn: uint64(id), Work: command.WorkFirefight, Enabled: false})
	g.ApplyPending()
	if g.prioOf(id)[work.Firefight] != 0 || pawn(t, g, id).WorkFirefight {
		t.Fatal("firefighting not switched off")
	}
	g.Enqueue(command.Command{Kind: command.SetWorkEnabled, Pawn: uint64(id), Work: command.WorkFirefight, Enabled: true})
	g.ApplyPending()
	if g.prioOf(id)[work.Firefight] != work.DefaultPriority {
		t.Fatal("firefighting not switched back on")
	}
}
