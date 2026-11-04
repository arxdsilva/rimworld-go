package craft

import (
	"slices"
	"testing"

	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/gameplay"
)

// TestFlavourIngredientsStable (CSE-014): off, a bare meal shows nothing, as
// before; on, it shows one neutral ingredient per recipe input, the same
// every call and for the same id, and real ingredients are never replaced.
func TestFlavourIngredientsStable(t *testing.T) {
	t.Parallel()
	on := gameplay.Settings{}.With(gameplay.FlavourIngredients, true)
	bare := item.Stack{Kind: item.FineMeal, Qty: 1}
	if IngredientNamesFor(bare, 7, gameplay.Settings{}) != nil {
		t.Fatal("off leaves a bare meal bare")
	}
	a := IngredientNamesFor(bare, 7, on)
	if len(a) != 2 || !slices.Equal(a, IngredientNamesFor(bare, 7, on)) {
		t.Fatalf("flavour %v, want 2 and stable", a)
	}
	differ := false
	for id := uint64(1); id < 40; id++ {
		differ = differ || !slices.Equal(a, IngredientNamesFor(bare, id, on))
	}
	if !differ {
		t.Fatal("every stack got the same flavour")
	}
	real := item.Stack{Kind: item.Meal, Qty: 1}
	real.AddIngredient(item.Rice, 0)
	if n := IngredientNamesFor(real, 7, on); len(n) != 1 || n[0] != "rice" {
		t.Fatalf("real ingredients replaced: %v", n)
	}
	if IngredientNamesFor(item.Stack{Kind: item.NutrientPasteMeal}, 7, on) != nil {
		t.Fatal("paste is not flavoured")
	}
}

// TestFlavourIngredientsNeutralOnly: whatever the id, the guess is never
// human or insect meat.
func TestFlavourIngredientsNeutralOnly(t *testing.T) {
	t.Parallel()
	on := gameplay.Settings{}.With(gameplay.FlavourIngredients, true)
	for id := uint64(0); id < 500; id++ {
		for _, k := range []item.Kind{item.Meal, item.FineMeal, item.LavishMeal} {
			for _, n := range IngredientNamesFor(item.Stack{Kind: k}, id, on) {
				if n == item.HumanMeat.String() || n == item.InsectMeat.String() || n == "" {
					t.Fatalf("id %d kind %v: %q", id, k, n)
				}
			}
		}
	}
}
