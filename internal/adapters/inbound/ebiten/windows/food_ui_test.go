package windows

import (
	"testing"

	"rimworld-go/internal/application/game"
)

func TestNextFoodPolicyWraps(t *testing.T) {
	t.Parallel()
	ps := []game.FoodPolicyView{{ID: 3, Name: "Anything"}, {ID: 5, Name: "No raw"}, {ID: 9, Name: "Meat"}}
	for cur, want := range map[string]int{"Anything": 5, "No raw": 9, "Meat": 3, "": 3, "gone": 3} {
		if got, ok := NextFoodPolicy(ps, cur); !ok || got != want {
			t.Errorf("after %q: %d %v, want %d", cur, got, ok, want)
		}
	}
	if _, ok := NextFoodPolicy(nil, "x"); ok {
		t.Error("no policies, no next")
	}
}

func TestFoodRowsKeepOnlyFoods(t *testing.T) {
	t.Parallel()
	rows := foodRows(game.FoodPolicyView{ID: 1}, map[string]bool{})
	if len(rows) == 0 || rows[0].Path != foodPolicyRoot {
		t.Fatalf("rows %+v", rows)
	}
	for _, r := range rows {
		if r.Path != foodPolicyRoot && len(r.Path) < 6 {
			t.Errorf("row %q outside Foods", r.Path)
		}
	}
	open := foodRows(game.FoodPolicyView{ID: 1}, map[string]bool{foodPolicyRoot: true, foodPolicyRoot + "/Raw food": true})
	if len(open) <= len(rows) {
		t.Errorf("expanding shows more rows: %d vs %d", len(open), len(rows))
	}
}

func TestAutoRowsAndStepCap(t *testing.T) {
	t.Parallel()
	rows := AutoRows([]game.AnimalView{{Species: "cow", Label: "cow"}, {Species: "cow", Label: "cow"}, {Species: "sheep", Label: "sheep"}},
		[]game.AutoSlaughterView{{Species: "sheep", MaxMales: 1, MaxFemales: 4, MaxYoung: -1}})
	if len(rows) != 2 || rows[0].species != "cow" || rows[0].caps != [3]int{-1, -1, -1} || rows[1].caps != [3]int{1, 4, -1} {
		t.Fatalf("rows %+v", rows)
	}
	if v, ok := StepCap(-1, true); !ok || v != 0 {
		t.Error("none steps up to 0")
	}
	if _, ok := StepCap(-1, false); ok {
		t.Error("none is the floor")
	}
	if _, ok := StepCap(autoCapMax, true); ok {
		t.Error("the cap has a ceiling")
	}
}
