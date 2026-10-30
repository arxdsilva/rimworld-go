package game

import (
	"slices"
	"strings"
	"testing"

	"rimworld-go/internal/application/game/raid"
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/pawn/mood"
	"rimworld-go/internal/domain/pawn/social"
	"rimworld-go/internal/domain/sim/entity"
)

// kidnapGame is Ada downed at (5, 10), her husband Bo drafted far off at
// (25, 3), and a fleeing raider beside her at (6, 10): the west edge is
// the nearest way out.
func kidnapGame(t *testing.T) (g *Game, ada, bo, rid entity.EntityID) {
	t.Helper()
	g = New(Config{Seed: 3, Width: 30, Height: 20})
	ada, _ = g.SpawnNamedPawn(5, 10, "Ada")
	bo, _ = g.SpawnNamedPawn(25, 3, "Bo")
	g.draft[bo] = true
	fed(g, ada, bo)
	g.ensurePerson(ada)
	g.ensurePerson(bo)
	now := uint64(g.clock.Now())
	g.setPair(ada, bo, social.Pair{Kind: social.PairSpouse, Since: now, LastChange: now})
	g.hurt(ada, health.DownedInjury+10, 0)
	if v, _ := g.vital.Get(ada); v.Status != health.Downed {
		t.Fatalf("Ada should be downed: %+v", v)
	}
	rid = spawnRaiderAt(t, g, 6, 10, 0)
	r := g.raiders[rid]
	r.Mode = raid.Fleeing
	g.raiders[rid] = r
	return g, ada, bo, rid
}

// stepUntil steps until cond holds, at most n ticks; it reports whether it did.
func stepUntil(g *Game, n int, cond func() bool) bool {
	for i := 0; i < n; i++ {
		if cond() {
			return true
		}
		g.Step(1)
	}
	return cond()
}

func TestKidnapCarriesAColonistOffTheMap(t *testing.T) {
	t.Parallel()
	g, ada, bo, rid := kidnapGame(t)
	if !stepUntil(g, 200, func() bool { return g.care.Carry.Patient(rid) == ada }) {
		j, _ := g.jobs.Get(rid)
		t.Fatalf("the raider never picked Ada up: job %+v", j)
	}
	carried := false
	for _, p := range g.Snapshot() {
		carried = carried || p.ID == ada && p.CarriedBy == rid
	}
	if !carried {
		t.Fatal("the read model does not show Ada carried by the raider")
	}
	if !stepUntil(g, 2000, func() bool { return !slices.Contains(g.pawnIDs, ada) }) {
		t.Fatal("Ada was never carried off")
	}
	if !g.isStashed(ada) || g.personOf(ada).Where != whereKidnapped || g.onMap(ada) {
		t.Fatalf("kidnapped Ada: stashed %v, registry %+v", g.isStashed(ada), g.personOf(ada))
	}
	if slices.Contains(g.raiderIDs, rid) || g.personOf(rid).Where != whereOffMap {
		t.Fatalf("the kidnapper is still here: %+v", g.personOf(rid))
	}
	if g.edgeOf(ada, bo) == nil || g.pairOf(ada, bo).Kind != social.PairSpouse {
		t.Fatal("the marriage was lost with her")
	}
	grief, lost := false, false
	for _, m := range g.memories[bo] {
		grief = grief || m.Kind == mood.Grief && m.Subject == uint64(ada)
		lost = lost || m.Kind == mood.ColonistLost
	}
	if !grief || !lost {
		t.Fatalf("Bo does not miss Ada: grief %v, colonist lost %v", grief, lost)
	}
	letter := false
	for _, m := range g.World().Messages {
		letter = letter || m.Letter && strings.Contains(m.Text, "Ada was kidnapped by raiders")
	}
	if !letter {
		t.Fatalf("no kidnap letter: %+v", g.World().Messages)
	}
	hist := false
	for _, h := range g.History() {
		hist = hist || strings.Contains(h.Text, "was kidnapped")
	}
	if !hist {
		t.Fatal("no history line")
	}
	g.checkLosses(uint64(g.clock.Now()))
	if slices.Contains(g.ng.dead, ada) || g.ng.lost {
		t.Fatal("a kidnapped colonist counted as a death")
	}
	g.pruneStrangers()
	if g.personOf(ada) == nil {
		t.Fatal("the registry forgot the kidnapped colonist")
	}
}

func TestKilledKidnapperDropsTheColonist(t *testing.T) {
	t.Parallel()
	for _, kill := range []bool{true, false} {
		g, ada, _, rid := kidnapGame(t)
		if !stepUntil(g, 200, func() bool { return g.care.Carry.Patient(rid) == ada }) {
			t.Fatal("the raider never picked Ada up")
		}
		g.Step(20) // on the way out
		rp, _ := g.positions.Get(rid)
		if kill {
			g.dieBy(rid, health.CauseInjury, 0)
		} else {
			g.hurt(rid, health.DownedInjury+10, 0)
			if v, _ := g.vital.Get(rid); v.Status != health.Downed {
				t.Fatalf("the kidnapper should be down: %+v", v)
			}
		}
		g.Step(1)
		if g.care.Carry.Patient(rid) != 0 || g.care.Carry.Carrier(ada) != 0 {
			t.Fatalf("kill %v: Ada still carried", kill)
		}
		if ap, _ := g.positions.Get(ada); ap != rp {
			t.Fatalf("kill %v: Ada at %+v, the kidnapper fell at %+v", kill, ap, rp)
		}
		g.Step(300)
		if !slices.Contains(g.pawnIDs, ada) || g.isStashed(ada) || g.personOf(ada).Where != whereColony {
			t.Fatalf("kill %v: Ada is not back with the colony: %+v", kill, g.personOf(ada))
		}
	}
}

// The kidnap target choice does not flap: a raider commits to the
// colonist it picked, and a guard who comes and goes beside a downed
// colonist does not make it start over and over.
func TestKidnapTargetDoesNotFlap(t *testing.T) {
	t.Parallel()
	g, ada, bo, rid := kidnapGame(t)
	cy, _ := g.SpawnNamedPawn(8, 11, "Cy")
	fed(g, cy)
	g.hurt(cy, health.DownedInjury+10, 0)
	if v, _ := g.vital.Get(cy); v.Status != health.Downed {
		t.Fatalf("Cy should be downed: %+v", v)
	}
	// Bo, drafted, steps in beside Ada and away again every 30 ticks
	targets := []entity.EntityID{}
	starts := 0
	last, _ := g.jobs.Get(rid)
	for i := 0; i < 1200 && slices.Contains(g.raiderIDs, rid); i++ {
		if i%30 == 0 && g.care.Carry.Patient(rid) == 0 {
			if (i/30)%2 == 0 {
				g.positions.Set(bo, Position{X: 4, Y: 10})
			} else {
				g.positions.Set(bo, Position{X: 25, Y: 3})
			}
			g.invalidateCreatures()
		}
		g.Step(1)
		j, _ := g.jobs.Get(rid)
		if j.Kind == job.Kidnap && last.Kind != job.Kidnap {
			starts++
		}
		if j.Kind == job.Kidnap && (len(targets) == 0 || targets[len(targets)-1] != j.Target) {
			targets = append(targets, j.Target)
		}
		last = j
	}
	if starts != 1 || len(targets) != 1 {
		t.Fatalf("kidnap started %d times, targets %v (want one, kept)", starts, targets)
	}
	if slices.Contains(g.pawnIDs, targets[0]) {
		t.Fatalf("the committed target %d was never carried off", targets[0])
	}
	_ = ada
	// a failed kidnap waits out the cooldown
	g2, ada2, _, rid2 := kidnapGame(t)
	g2.Step(1)
	now := uint64(g2.clock.Now())
	g2.endKidnap(rid2, now)
	for i := 0; i < kidnapScan*3; i++ {
		g2.Step(1)
		if j, _ := g2.jobs.Get(rid2); j.Kind == job.Kidnap {
			t.Fatalf("kidnap retried %d ticks after a failure", i)
		}
	}
	if g2.care.Carry.Patient(rid2) == ada2 {
		t.Fatal("picked up during the cooldown")
	}
}
