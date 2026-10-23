package game

import (
	"testing"

	"rimworld-go/internal/domain/ai/job"
)

// TestInCombat: a colonist is in combat while drafted, attacking or
// fleeing (shaken: a live calm entry), and not otherwise (PLAY-010).
func TestInCombat(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 20, Height: 12})
	p, _ := g.SpawnNamedPawn(5, 5, "Ada")
	if g.inCombat(p) {
		t.Fatal("an idle colonist is not in combat")
	}
	g.setDraft(p, true)
	if !g.inCombat(p) {
		t.Fatal("a drafted colonist is in combat")
	}
	g.setDraft(p, false)
	g.jobs.Set(p, job.Record{Kind: job.Attack, Target: 99})
	if !g.inCombat(p) {
		t.Fatal("an attacking colonist is in combat")
	}
	g.jobs.Set(p, job.Record{})
	now := uint64(g.clock.Now())
	g.calm[p] = now + 100
	if !g.inCombat(p) {
		t.Fatal("a fleeing (shaken) colonist is in combat")
	}
	g.calm[p] = now // expired
	if g.inCombat(p) {
		t.Fatal("an expired calm window is not combat")
	}
}
