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
