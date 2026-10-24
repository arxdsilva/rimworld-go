package game

import (
	"math/rand/v2"
	"slices"
	"testing"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/sim/entity"
)

// checkPostKinds fails unless p.kinds lists exactly m's keys by pawn and
// kind.
func checkPostKinds(t *testing.T, p *postState) {
	t.Helper()
	n := 0
	for pk, l := range p.kinds {
		if len(l) == 0 {
			t.Fatalf("empty list kept for %v", pk)
		}
		for _, k := range l {
			if _, ok := p.m[k]; !ok || k.Pawn != pk.Pawn || k.Kind != pk.Kind {
				t.Fatalf("listed %v under %v, in m: %v", k, pk, ok)
			}
		}
		n += len(l)
	}
	if n != len(p.m) {
		t.Fatalf("lists hold %d keys, m %d", n, len(p.m))
	}
}

// TestPostKindsTrackEntries: the per-pawn, per-kind key lists follow every
// put, renewal and drop, and a reindex rebuilds them.
func TestPostKindsTrackEntries(t *testing.T) {
	p := newPostState()
	rng := rand.New(rand.NewPCG(7, 1))
	kinds := []job.Kind{job.Haul, job.Cut, job.Repair}
	for i := 0; i < 5000; i++ {
		k := postKey{Pawn: entity.EntityID(1 + rng.IntN(3)), Kind: kinds[rng.IntN(len(kinds))],
			Target: entity.EntityID(rng.IntN(6))}
		if _, ok := p.m[k]; ok && rng.IntN(2) == 0 {
			p.drop(k)
		} else {
			p.put(k, postVal{Until: uint64(i)})
		}
		checkPostKinds(t, &p)
	}
	p.reindex()
	checkPostKinds(t, &p)
}

// TestPostponedTargetsCoverPostponedFor: a colony at work keeps the lists
// in step through renewals, lazy pruning, walkability syncs and sweeps,
// and every target postponedFor reports put off is in postponedTargets.
func TestPostponedTargetsCoverPostponedFor(t *testing.T) {
	g := NewColony(Config{Seed: 4, Width: 48, Height: 40})
	g.Step(10)
	rng := rand.New(rand.NewPCG(2, 8))
	seen := 0
	for step := 0; step < 30; step++ {
		for _, id := range g.pawnIDs {
			for n := 0; n < 3 && len(g.itemIDs) > 0; n++ {
				why := job.Blocked
				if rng.IntN(2) == 0 {
					why = job.Unreachable
				}
				g.postpone(postKey{Pawn: id, Kind: job.Haul, Target: g.itemIDs[rng.IntN(len(g.itemIDs))]}, why)
			}
		}
		g.Step(150)
		checkPostKinds(t, &g.postponed)
		for k := range g.postponed.m {
			if k.Target == 0 {
				continue
			}
			got := g.postponedTargets(k.Pawn, k.Kind, nil)
			if g.postponedFor(k.Pawn, k.Kind, k.Target, k.X, k.Y) {
				seen++
				if !slices.Contains(got, k.Target) {
					t.Fatalf("%v put off but not in postponedTargets %v", k, got)
				}
			}
		}
		checkPostKinds(t, &g.postponed)
	}
	if seen == 0 {
		t.Fatal("nothing stayed put off: the test no longer exercises postponement")
	}
}
