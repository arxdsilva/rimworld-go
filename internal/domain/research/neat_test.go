package research

import (
	"slices"
	"testing"
)

// TestNeatStorageResearchGates (NST-A): four pieces need Machine tools, two
// need Metalworking, and the other 21 are gated by nothing.
func TestNeatStorageResearchGates(t *testing.T) {
	t.Parallel()
	gates := map[Project][]string{
		MachineTools: {"safe", "drum", "horizontal weapon rack", "vertical weapon rack"},
		Metalworking: {"textile rack", "meat hook"},
	}
	for p, names := range gates {
		for _, n := range names {
			if !slices.Contains(DefOf(p).Unlocks, "build:"+n) {
				t.Errorf("%v does not unlock %s", p, n)
			}
		}
	}
	free := []string{"display case", "round pedestal", "small display shelf", "display shelf", "weapon rack", "narrow wardrobe",
		"wardrobe", "narrow display cabinet", "display cabinet", "open shelf", "two tier display shelf", "narrow tall shelf",
		"tall shelf", "mini pallet", "half pallet", "pallet", "small crate", "crate", "large crate", "tall crate", "corpse dump spot"}
	if len(free) != 21 {
		t.Fatalf("%d free pieces", len(free))
	}
	var none Progress
	for _, n := range free {
		if p, locked := none.Locks("build:" + n); locked {
			t.Errorf("%s is locked by %v", n, p)
		}
	}
}
