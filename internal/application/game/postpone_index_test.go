package game

import (
	"math/rand/v2"
	"slices"
	"testing"

	"rimworld-go/internal/application/game/postpone"
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/sim/entity"
)

// checkPostKinds fails unless p's per-pawn, per-kind key lists hold exactly
// its entries (through the exported API: every entry is listed once under
// its pawn and kind, and the lists hold no more keys than there are entries).
func checkPostKinds(t *testing.T, p *postpone.Store) {
	t.Helper()
	type pk struct {
		pawn entity.EntityID
		kind job.Kind
	}
	seen := map[pk]bool{}
	n := 0
	p.All(func(k postpone.Key, _ postpone.Val) {
		l := p.Keys(k.Pawn, k.Kind)
		if c := slices.Index(l, k); c < 0 || slices.Index(l[c+1:], k) >= 0 {
			t.Fatalf("%v not listed exactly once under its pawn and kind: %v", k, l)
		}
		if !seen[pk{k.Pawn, k.Kind}] {
			seen[pk{k.Pawn, k.Kind}] = true
			n += len(l)
		}
	})
	if n != p.Len() {
		t.Fatalf("lists hold %d keys, store %d", n, p.Len())
	}
}

// TestPostponedTargetsCoverPostponedFor: a colony at work keeps the lists
// in step through renewals, lazy pruning, walkability syncs and sweeps,
// and every target postponedFor reports put off is in postponedTargets.
func TestPostponedTargetsCoverPostponedFor(t *testing.T) {
	t.Parallel()
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
				g.postpone(postpone.Key{Pawn: id, Kind: job.Haul, Target: g.itemIDs[rng.IntN(len(g.itemIDs))]}, why)
			}
		}
		g.Step(150)
		checkPostKinds(t, &g.postponed.Store)
		var all []postpone.Key
		g.postponed.All(func(k postpone.Key, _ postpone.Val) { all = append(all, k) })
		for _, k := range all {
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
		checkPostKinds(t, &g.postponed.Store)
	}
	if seen == 0 {
		t.Fatal("nothing stayed put off: the test no longer exercises postponement")
	}
}
