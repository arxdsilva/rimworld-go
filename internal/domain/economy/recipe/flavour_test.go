package recipe

import (
	"testing"

	"rimworld-go/internal/domain/economy/item"
)

// TestFlavourNeutralOnly (CSE-014): no flavour choice is human or insect
// meat, a raw-penalty-free or plain food, or a composite meal.
func TestFlavourNeutralOnly(t *testing.T) {
	for _, meal := range []item.Kind{item.Meal, item.FineMeal, item.LavishMeal} {
		for slot, ks := range FlavourSlots(meal) {
			for _, k := range ks {
				if k == item.HumanMeat || k == item.InsectMeat || !k.IsFood() || !k.Raw() {
					t.Errorf("%v slot %d offers %v", meal, slot, k)
				}
			}
		}
	}
	if FlavourSlots(item.NutrientPasteMeal) != nil || FlavourSlots(item.Kibble) != nil {
		t.Fatal("paste and kibble are never flavoured")
	}
}
