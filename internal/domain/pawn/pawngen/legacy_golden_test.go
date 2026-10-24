package pawngen

import (
	"fmt"
	"hash/fnv"
	"testing"

	"rimworld-go/internal/domain/pawn/trait"
)

// legacyGolden fingerprints 1 200 generated pawns as they were before the
// ENG-003 traits (computed with Rules{} on f957fc8a): Legacy rules must keep
// generating exactly these.
const legacyGolden = 0xb198f6b3c2d04186

func pawnPrint(r Rules) uint64 {
	h := fnv.New64a()
	for seed := uint64(0); seed < 300; seed++ {
		for slot := 0; slot < 4; slot++ {
			rr := r
			if slot == 3 {
				rr.Forced = []ForcedTrait{{Trait: trait.Gentle, Chance: 50}}
			}
			fmt.Fprintf(h, "%+v;", Generate(seed*7919, slot, int(seed%3), rr))
		}
	}
	return h.Sum64()
}

func TestLegacyRulesUnchanged(t *testing.T) {
	if got := pawnPrint(Rules{Legacy: true}); got != legacyGolden {
		t.Fatalf("legacy generation moved: %#x, want %#x", got, uint64(legacyGolden))
	}
	if pawnPrint(Rules{}) == legacyGolden {
		t.Fatal("the full trait set changed nothing")
	}
}

// TestNewTraitsGenerated: outside legacy rules the ENG-003 traits appear,
// orientation traits rarely, and never two of one group.
func TestNewTraitsGenerated(t *testing.T) {
	seen := map[trait.ID]int{}
	for _, p := range many(3000, Rules{}) {
		for _, a := range p.Traits {
			seen[a]++
		}
	}
	if seen[trait.Striking] == 0 || seen[trait.Hedonist] == 0 || seen[trait.Gay] == 0 {
		t.Fatalf("new traits missing: %v", seen)
	}
	if seen[trait.Gay] >= seen[trait.Striking] || seen[trait.Asexual] > seen[trait.Gay] {
		t.Fatalf("orientation not rare: gay %d asexual %d striking %d", seen[trait.Gay], seen[trait.Asexual], seen[trait.Striking])
	}
	for _, p := range many(300, Rules{Legacy: true}) {
		for _, a := range p.Traits {
			if a > trait.Sturdy {
				t.Fatalf("legacy pawn rolled %s", trait.DefOf(a).Label)
			}
		}
	}
}
