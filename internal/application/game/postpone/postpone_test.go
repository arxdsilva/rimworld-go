package postpone

import (
	"math/rand/v2"
	"testing"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/sim/entity"
)

// checkPostKinds fails unless p.kinds lists exactly m's keys by pawn and
// kind.
func checkPostKinds(t *testing.T, p *Store) {
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
	t.Parallel()
	p := New()
	rng := rand.New(rand.NewPCG(7, 1))
	kinds := []job.Kind{job.Haul, job.Cut, job.Repair}
	for i := 0; i < 5000; i++ {
		k := Key{Pawn: entity.EntityID(1 + rng.IntN(3)), Kind: kinds[rng.IntN(len(kinds))],
			Target: entity.EntityID(rng.IntN(6))}
		if _, ok := p.m[k]; ok && rng.IntN(2) == 0 {
			p.Drop(k)
		} else {
			p.Put(k, Val{Until: uint64(i)})
		}
		checkPostKinds(t, &p)
	}
	p.Reindex()
	checkPostKinds(t, &p)
}
