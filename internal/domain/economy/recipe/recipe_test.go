package recipe

import (
	"testing"

	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
)

func TestRecipesAndBills(t *testing.T) {
	t.Parallel()
	if len(For(construction.Stove)) != 1 || For(construction.Stove)[0].Output != item.Meal {
		t.Fatal("stove cooks meals")
	}
	meal := DefOf(SimpleMeal)
	if !meal.Input.Accepts(item.Berries) || !meal.Input.Accepts(item.Meat) || meal.Input.Accepts(item.Meal) {
		t.Fatal("meals take any raw food")
	}
	// cooking multiplies nutrition
	if meal.OutQty*item.Meal.Nutrition() <= meal.Input.Qty*item.Potatoes.Nutrition() {
		t.Fatal("a meal must be worth more than its raw inputs")
	}
	if !(Bill{Mode: DoCount, Target: 2, Done: 1}).Wants(0) || (Bill{Mode: DoCount, Target: 2, Done: 2}).Wants(0) {
		t.Fatal("do-count")
	}
	if !(Bill{Mode: UntilHave, Target: 5}).Wants(4) || (Bill{Mode: UntilHave, Target: 5}).Wants(5) {
		t.Fatal("until-have")
	}
	if (Bill{Mode: Forever, Suspended: true}).Wants(0) {
		t.Fatal("suspended bills never run")
	}
}
