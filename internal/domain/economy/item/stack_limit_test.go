package item

import "testing"

// TestStackLimit (PRD-4, wiki item pages): the wave E kinds carry their
// wiki stack limits; every older kind keeps StackLimit.
func TestStackLimit(t *testing.T) {
	t.Parallel()
	want := map[Kind]int{Chemfuel: 150, Silverdust: 150, Reagent: 150, Pemmican: 75, Plasteel: 75,
		AdvancedComponents: 50, Wort: 25, FineMeal: 10, SurvivalMeal: 10, Meal: StackLimit, Wood: StackLimit}
	for k, n := range want {
		if k.StackLimit() != n {
			t.Errorf("%s stacks to %d, want %d", k, k.StackLimit(), n)
		}
	}
	if Kind(255).StackLimit() != StackLimit {
		t.Fatal("an unknown kind keeps the plain limit")
	}
}
