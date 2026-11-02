package features

import (
	"testing"

	"rimworld-go/internal/domain/world/mapgen"
	"rimworld-go/internal/domain/world/site"
)

// TestShedsHoldScreensAndScopes (AX-6): over many maps some sheds hold a
// big screen television (three free floor cells in a row) or a
// telescope (a free floor cell with a free one south of it, its spot),
// never on loot or an urn and never both in one shed.
func TestShedsHoldScreensAndScopes(t *testing.T) {
	t.Parallel()
	screens, scopes := 0, 0
	for seed := uint64(1); seed <= 120; seed++ {
		c := fixture(seed, 300, 300, site.Site{Settled: true})
		_ = (Ruins{}).Run(c)
		for _, r := range c.R.Ruins {
			if len(r.Screens)+len(r.Scopes) > 1 {
				t.Fatalf("seed %d: %d screens and %d scopes in one shed", seed, len(r.Screens), len(r.Scopes))
			}
			floor := map[[2]int]bool{}
			for _, f := range r.Floors {
				floor[[2]int{f.X, f.Y}] = !hasLoot(&r, f.X, f.Y) && !hasUrn(&r, f.X, f.Y)
			}
			for _, s := range r.Screens {
				if r.Kind != mapgen.Shed || !floor[[2]int{s.X, s.Y}] || !floor[[2]int{s.X + 1, s.Y}] || !floor[[2]int{s.X + 2, s.Y}] {
					t.Fatalf("seed %d: screen at %d,%d off three free floor cells", seed, s.X, s.Y)
				}
				screens++
			}
			for _, s := range r.Scopes {
				if r.Kind != mapgen.Shed || !floor[[2]int{s.X, s.Y}] || !floor[[2]int{s.X, s.Y + 1}] {
					t.Fatalf("seed %d: telescope at %d,%d off two free floor cells", seed, s.X, s.Y)
				}
				scopes++
			}
		}
	}
	if screens == 0 || scopes == 0 {
		t.Fatalf("%d screens and %d telescopes over 120 maps, want some of each", screens, scopes)
	}
}
