package hud

import "testing"

func TestFoodDaysRow(t *testing.T) {
	t.Parallel()
	if _, ok := FoodDaysRow(0); ok {
		t.Fatal("no reading, no row")
	}
	r, ok := FoodDaysRow(34)
	if !ok || r.Text != "3.4 d" || r.Label != "Days of food: 3.4" || r.Kind != FoodDaysKind {
		t.Fatalf("row %+v", r)
	}
}

func TestSkyChip(t *testing.T) {
	t.Parallel()
	if _, ok := SkyChip("", 0, 0); ok {
		t.Fatal("no event, no chip")
	}
	c, ok := SkyChip("eclipse", 3000+600, 1000) // 2600 ticks left = 1.0 h at 2500/h? uses clock
	if !ok || c.Label == "" || c.Label[:7] != "Eclipse" {
		t.Fatalf("chip %+v", c)
	}
	if f, _ := SkyChip("toxic fallout", 0, 5); f.Label != "Toxic fallout" {
		t.Errorf("no end tick: %q", f.Label)
	}
}
