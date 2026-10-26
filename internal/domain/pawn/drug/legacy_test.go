package drug

import (
	"slices"
	"testing"

	"rimworld-go/internal/domain/economy/item"
)

// TestLegacyListsConvert: an older save's fixed-size per-drug arrays
// become the same values in the per-drug slices.
func TestLegacyListsConvert(t *testing.T) {
	tol, last := LegacyLists([LegacyDrugs]int32{}, [LegacyDrugs]uint64{})
	if tol != nil || last != nil {
		t.Fatal("empty arrays: nil lists")
	}
	tol, last = LegacyLists([LegacyDrugs]int32{0, 300, 0}, [LegacyDrugs]uint64{5, 0, 0})
	s := State{Tolerance: tol, LastDose: last}
	if s.ToleranceOf(item.AshleafTea) != 300 || s.ToleranceOf(item.Beer) != 0 || s.ToleranceOf(item.SparkTablet) != 0 {
		t.Fatalf("tolerance %v", tol)
	}
	if s.LastDoseOf(item.Beer) != 5 || s.LastDoseOf(item.AshleafTea) != 0 || !slices.Equal(last, []uint64{5}) {
		t.Fatalf("last dose %v", last)
	}
	if s.ToleranceOf(item.Wood) != 0 || s.LastDoseOf(item.Wood) != 0 {
		t.Fatal("not a drug")
	}
}

// TestPerDrugListsGrowLazily: a dose grows the lists only as far as the
// drug's slot, and tolerance gone to zero everywhere drops the list.
func TestPerDrugListsGrowLazily(t *testing.T) {
	var s State
	s.Ingest(item.AshleafTea, never, 7)
	if len(s.Tolerance) != 2 || len(s.LastDose) != 2 || s.LastDoseOf(item.AshleafTea) != 7 || s.ToleranceOf(item.AshleafTea) != 150 {
		t.Fatalf("%v %v", s.Tolerance, s.LastDose)
	}
	s.Tick(200, 7+200*hr)
	if s.Tolerance != nil {
		t.Fatalf("decayed tolerance: %v", s.Tolerance)
	}
	if s.LastDoseOf(item.AshleafTea) != 7 {
		t.Fatal("the last dose is remembered")
	}
}
