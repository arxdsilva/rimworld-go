package raid

import "testing"

// TestNamesAppendOnly: every arrival and strategy has a name, unknown
// values fall back to the plain walk-in assault, and the first strategies
// keep game's raid-plan names (the saved byte must read the same).
func TestNamesAppendOnly(t *testing.T) {
	t.Parallel()
	for a := Arrival(0); a < NumArrivals; a++ {
		if a.String() == "" || a.DropPod() != (a != WalkInEdge) {
			t.Errorf("arrival %d: %q drop pod %v", a, a, a.DropPod())
		}
	}
	if NumArrivals.String() != "walk in" || NumArrivals.DropPod() {
		t.Error("an unknown arrival walks in")
	}
	for s, want := range map[Strategy]string{ImmediateAssault: "assault", Siege: "siege", Sappers: "sappers",
		StageThenAssault: "staged assault", Steal: "steal", Burn: "burn", NumStrategies: "assault"} {
		if s.String() != want {
			t.Errorf("strategy %d = %q, want %q", s, s, want)
		}
	}
	if ImmediateAssault != 0 || Siege != 1 || Sappers != 2 {
		t.Error("the raid plans keep their saved values")
	}
}
