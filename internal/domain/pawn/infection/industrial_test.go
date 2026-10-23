package infection

import "testing"

// TestIndustrialBeatsHerbal: industrial medicine festers least, beats an
// infection fastest and heals most per tend.
func TestIndustrialBeatsHerbal(t *testing.T) {
	t.Parallel()
	if Chance(IndustrialMedicine) != 1 || Chance(IndustrialMedicine) >= Chance(Medicine) {
		t.Fatalf("industrial chance %d", Chance(IndustrialMedicine))
	}
	if Step(50, true, IndustrialMedicine) != 38 || Step(50, true, IndustrialMedicine) >= Step(50, true, Medicine) {
		t.Fatalf("industrial step %d", Step(50, true, IndustrialMedicine))
	}
	if Step(50, false, IndustrialMedicine) != 54 {
		t.Fatal("untreated grows whatever the last care")
	}
	tests := []struct {
		c    Care
		want int
	}{{Untreated, 100}, {SelfTended, 50}, {Tended, 100}, {Medicine, 200}, {IndustrialMedicine, 250}}
	for _, tc := range tests {
		if got := HealPct(tc.c); got != tc.want {
			t.Errorf("HealPct(%d) = %d, want %d", tc.c, got, tc.want)
		}
	}
}
