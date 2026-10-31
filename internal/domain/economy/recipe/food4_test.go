package recipe

import (
	"testing"

	"rimworld-go/internal/domain/economy/item"
)

// TestMealRecipesAreTheWikis pins FOOD-4's recipes (wiki: Meals): inputs
// in 0.05 nutrition units, outputs, minimum skill and work (x0.4).
func TestMealRecipesAreTheWikis(t *testing.T) {
	t.Parallel()
	cases := []struct {
		id        ID
		veg, meat int // per side; one-sided recipes use veg for their only input
		out       item.Kind
		qty, work int
		skill     uint8
	}{
		{SimpleMeal, 10, 0, item.Meal, 1, 120, 0},
		{CampfireMeal, 10, 0, item.Meal, 1, 240, 0},
		{CookFineMeal, 5, 5, item.FineMeal, 1, 180, 6},
		{CookLavishMeal, 10, 10, item.LavishMeal, 1, 320, 8},
		{CookSurvivalMeal, 6, 6, item.SurvivalMeal, 1, 180, 8},
		{CookPemmican, 5, 5, item.Pemmican, 16, 280, 0},
		{CookFineMealVeg, 15, 0, item.FineMeal, 1, 180, 6},
		{CookFineMealMeat, 15, 0, item.FineMeal, 1, 180, 6},
		{CookLavishMealVeg, 25, 0, item.LavishMeal, 1, 320, 8},
		{CookLavishMealMeat, 25, 0, item.LavishMeal, 1, 320, 8},
		{CookSimpleMealBulk, 40, 0, item.Meal, 4, 480, 0},
		{CookFineMealBulk, 20, 20, item.FineMeal, 4, 576, 6},
		{CookLavishMealBulk, 40, 40, item.LavishMeal, 4, 1024, 8},
		{CookSurvivalMealBulk, 24, 24, item.SurvivalMeal, 4, 576, 8},
		{MakeKibble, 20, 20, item.Kibble, 50, 180, 0},
		{CookPemmicanBulk, 20, 20, item.Pemmican, 64, 1120, 0},
	}
	for _, c := range cases {
		d := DefOf(c.id)
		var ins []Input
		ins = d.AppendInputs(ins)
		ok := d.Output == c.out && d.OutQty == c.qty && d.Work == c.work && d.MinSkill == c.skill && ins[0].Qty == c.veg
		if c.meat > 0 {
			ok = ok && len(ins) == 2 && ins[1].Qty == c.meat && ins[1].AnyMeat && ins[0].AnyVeg
		} else {
			ok = ok && len(ins) == 1
		}
		if !ok {
			t.Errorf("%s: %+v", d.Name, d)
		}
	}
	if !DefOf(CookFineMealVeg).Accepts(item.Rice) || DefOf(CookFineMealVeg).Accepts(item.Meat) ||
		!DefOf(CookLavishMealMeat).Accepts(item.Egg) || DefOf(CookLavishMealMeat).Accepts(item.Corn) {
		t.Fatal("the variants take one side")
	}
	if !DefOf(MakeKibble).Accepts(item.Hay) || DefOf(CookFineMeal).Accepts(item.Hay) {
		t.Fatal("kibble takes hay; meals don't")
	}
	if DefOf(RefineChemfuelFood).Input.Qty != 70 {
		t.Fatal("chemfuel takes 3.5 nutrition")
	}
}

// TestFoodInputsCountNutrition: a chicken egg is five units, a potato
// one; TakeFrom and Enough read the stack's nutrition; plain inputs count
// items.
func TestFoodInputsCountNutrition(t *testing.T) {
	t.Parallel()
	meal := DefOf(SimpleMeal).Input
	egg, potato := item.Stack{Kind: item.Egg, Qty: 3}, item.Stack{Kind: item.Potatoes, Qty: 9}
	if meal.Weight(egg) != 5 || meal.Weight(potato) != 1 || meal.TakeFrom(egg) != 2 || meal.TakeFrom(potato) != 10 {
		t.Fatalf("weights %d %d takes %d %d", meal.Weight(egg), meal.Weight(potato), meal.TakeFrom(egg), meal.TakeFrom(potato))
	}
	if !meal.Enough(egg, 2) || meal.Enough(egg, 1) || meal.Enough(potato, 9) || meal.Items(egg, 7) != 2 {
		t.Fatal("enough")
	}
	wood := DefOf(Club).Input
	if wood.ByNutrition() || wood.Weight(item.Stack{Kind: item.Wood}) != 1 || wood.TakeFrom(item.Stack{Kind: item.Wood, Qty: 50}) != wood.Qty {
		t.Fatal("plain inputs count items")
	}
	if l := meal.Label(); l != "0.5 nutrition of raw food" {
		t.Fatalf("label %q", l)
	}
	if l := DefOf(MakeKibble).Inputs[0].Label(); l != "1 nutrition of plant food or hay" {
		t.Fatalf("kibble label %q", l)
	}
}
