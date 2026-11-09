package combat

import (
	"testing"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/combat/weapon"
	"rimworld-go/internal/domain/sim/entity"
)

// EAI-006, EAI-012 and EAI-022 unit tests.

// sparkRig is a fake with sentry turret 50 at (10, 10) whose fuse burns.
func sparkRig(t *testing.T) (*fakeHost, *Combat) {
	t.Helper()
	f := newFake(40, 30)
	f.build(50, construction.SentryTurret, 10, 10)
	c := f.combat()
	c.Turrets.sparks = append(c.Turrets.sparks, Fuse{ID: 50, Kind: construction.SentryTurret, X: 10, Y: 10, At: f.now + TurretFuseTicks})
	return f, c
}

// TestTurretDangerStepNoFlap (EAI-012): a colonist inside a sparking
// turret's blast walks out to reach+2; in the band between the blast and
// reach+2 it is never sent off again, idle or working; past the band the
// turret is nothing to it. Danger tells the raiders of the same blast.
func TestTurretDangerStepNoFlap(t *testing.T) {
	t.Parallel()
	f, c := sparkRig(t)
	g, tu := c.Guard, c.Turrets
	reach := turretBlast(f.bldgs[50].Kind).Reach()
	f.pawn(1, 11, 10, weapon.Fists)
	if !g.Defend(1, job.Record{}) || len(f.started) != 1 || f.started[0].Kind != job.Move {
		t.Fatalf("inside the blast: step out, started %+v", f.started)
	}
	out := f.started[0]
	if _, _, _, bad := tu.sparkAt(out.TX, out.TY); bad || weapon.Distance(out.TX, out.TY, 10, 10) < reach+1 {
		t.Fatalf("stepped to (%d, %d), reach %d", out.TX, out.TY, reach)
	}
	out.State = job.Do
	if !g.Defend(1, out) || len(f.started) != 1 {
		t.Fatal("on its way out: carry on")
	}
	if lx, ly, r, ok := c.Tactics.Danger(11, 10); !ok || lx != 10 || ly != 10 || r != reach+1 {
		t.Fatalf("Danger: (%d, %d) reach %d %v", lx, ly, r, ok)
	}
	// walk back and forth across the band: starts only inside the blast
	starts := len(f.started)
	for i := 0; i < 40; i++ {
		x := 10 + reach + 1 + i%2 // reach+1, reach+2: the band
		f.pos[1] = [2]int{x, 10}
		if _, _, _, ok := c.Tactics.Danger(x, 10); ok {
			t.Fatalf("the band (%d) is no danger", x)
		}
		if !g.Defend(1, job.Record{}) {
			t.Fatalf("idle in the band at %d: it holds", x)
		}
		if g.Defend(1, job.Record{Kind: job.Haul, State: job.Do}) {
			t.Fatal("working in the band: it carries on")
		}
	}
	if len(f.started) != starts {
		t.Fatalf("the band sent it off %d times", len(f.started)-starts)
	}
	interr := f.interr
	if !g.Defend(1, job.Record{Kind: job.Haul, TX: 10, TY: 11, State: job.Do}) || f.interr != interr+1 || len(f.started) != starts {
		t.Fatal("in the band, a job into the blast is dropped and it holds")
	}
	f.pos[1] = [2]int{10 + reach + 3, 10}
	if g.Defend(1, job.Record{}) {
		t.Fatal("past the band: nothing to do")
	}
	f.pos[1] = [2]int{12, 10}
	if !g.Defend(1, job.Record{Kind: job.Haul, State: job.Do}) || len(f.started) != starts+1 || f.interr == 0 {
		t.Fatal("back in the blast: out again, its work dropped")
	}
	tu.sparks = nil
	if g.Defend(1, job.Record{}) {
		t.Fatal("no fuse: nothing")
	}
}

// TestTurretTargetsTameManhunter (EAI-006): a tame animal gone manhunter
// is a turret's target; a calm tame one never is.
func TestTurretTargetsTameManhunter(t *testing.T) {
	t.Parallel()
	f, tu := turretRig(t)
	f.beast(3, 10, 12, animal.State{Species: animal.TimberWolf, Tame: true, Manhunter: f.now + 100})
	if !tu.Targetable(3, f.now) {
		t.Fatal("a tame manhunter is a target")
	}
	if tu.Targetable(3, f.now+100) {
		t.Fatal("its rage over: no target")
	}
	f.beast(4, 10, 13, animal.State{Species: animal.TimberWolf, Tame: true})
	if tu.Targetable(4, f.now) {
		t.Fatal("a calm tame animal is never a target")
	}
}

// TestNearestThreatIndexMatchesScan (EAI-022): the raider part of
// NearestThreat, read from the creature index, picks what a scan of
// every raider picks (nearest, ties to the lower id, an animal keeping a
// tie), over many layouts.
func TestNearestThreatIndexMatchesScan(t *testing.T) {
	t.Parallel()
	seed := uint64(7)
	next := func(n int) int {
		seed = seed*6364136223846793005 + 1442695040888963407
		return int(seed>>33) % n
	}
	for round := 0; round < 200; round++ {
		f := newFake(60, 40)
		g := f.combat().Guard
		f.pawn(1, 30, 20, weapon.Fists)
		for i := 0; i < 12; i++ {
			id := entity.EntityID(100 + next(50))
			if _, ok := f.pos[id]; ok {
				continue
			}
			f.raider(id, 30+next(30)-15, 20+next(20)-10, weapon.Rifle)
			f.raiders[id] = next(5) > 0
			if next(3) > 0 {
				f.jobs[id] = job.Record{Kind: job.Attack, Target: 1, State: job.Do}
			}
		}
		if next(2) == 0 {
			f.beast(5, 30+next(10)-5, 20+next(10)-5, animal.State{Species: animal.TimberWolf, Mode: animal.Attack, Target: 1})
		}
		th, tx, ty, d := g.NearestThreat(1, 30, 20, AnimalReach, RaiderReach)
		wth, wx, wy, wd := scanThreat(g, f, 1, 30, 20)
		if th != wth || tx != wx || ty != wy || d != wd {
			t.Fatalf("round %d: index %d (%d,%d) %d, scan %d (%d,%d) %d", round, th, tx, ty, d, wth, wx, wy, wd)
		}
	}
}

// scanThreat is NearestThreat by a scan of every animal and raider.
func scanThreat(g *Guard, f *fakeHost, id entity.EntityID, x, y int) (threat entity.EntityID, tx, ty, best int) {
	for _, a := range f.animalIDs {
		if px, py, d, ok := g.ThreatTo(id, a, x, y, AnimalReach, RaiderReach); ok && (threat == 0 || d < best || d == best && a < threat) {
			threat, tx, ty, best = a, px, py, d
		}
	}
	raider := false
	for _, r := range f.raiderIDs {
		if px, py, d, ok := g.ThreatTo(id, r, x, y, AnimalReach, RaiderReach); ok && (threat == 0 || d < best || d == best && raider && r < threat) {
			threat, tx, ty, best, raider = r, px, py, d, true
		}
	}
	return threat, tx, ty, best
}
