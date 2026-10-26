package features

import (
	"testing"

	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/pkg/randx"
)

// TestVaultsHoldNanomed (MED-05): nanomed kits are a rare vault find (one
// to three), never crafted, and the other finds still turn up.
func TestVaultsHoldNanomed(t *testing.T) {
	t.Parallel()
	nano, special := 0, 0
	for seed := uint64(1); seed <= 4000; seed++ {
		for _, e := range vaultLoot(randx.New(seed)) {
			switch {
			case e.kind == item.NanomedKit:
				nano++
				if e.lo != 1 || e.hi != 3 {
					t.Fatalf("nanomed stack %d-%d", e.lo, e.hi)
				}
			case e.lo == 1 && e.hi == 1 && e.kind != item.Rifle:
				special++
			}
		}
	}
	if nano == 0 || special == 0 || nano >= special {
		t.Fatalf("nanomed %d vs other rare finds %d", nano, special)
	}
}
