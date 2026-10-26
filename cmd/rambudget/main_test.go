package main

import "testing"

func TestBudget(t *testing.T) {
	b := budget(16*1024, 4*1024)
	if b.BudgetMiB != 12*1024 {
		t.Fatalf("budget %d MiB, want available minus the reserve", b.BudgetMiB)
	}
	for _, k := range b.Kinds {
		if want := b.BudgetMiB / k.PeakMiB; k.Max != min(want, maxAgents) {
			t.Errorf("%s: max %d, want %d", k.Name, k.Max, min(want, maxAgents))
		}
	}
	// less free memory than the reserve: nothing fits
	if b := budget(3*1024, 4*1024); b.BudgetMiB != 0 || b.Kinds[0].Max != 0 {
		t.Fatalf("tight machine budget %+v", b)
	}
	// PLAY-020: a small-map game capture is its own kind, between a light
	// agent and an art one (the PLAY-019 art diet made it cheaper)
	peak := map[string]int{}
	for _, k := range b.Kinds {
		peak[k.Name] = k.PeakMiB
	}
	if c := peak["capture"]; c == 0 || c >= peak["art"] || c <= peak["light"] {
		t.Fatalf("capture peak %d MiB, want between light %d and art %d", c, peak["light"], peak["art"])
	}
	// a huge machine is still capped at the parallel-work ceiling
	if b := budget(1<<20, 4*1024); b.Kinds[len(b.Kinds)-1].Max != maxAgents {
		t.Fatalf("cap: %+v", b.Kinds)
	}
}
