package item

import "testing"

// TestFoodRotDaysAreTheWikis pins FOOD-4's days to rot (wiki item pages).
func TestFoodRotDaysAreTheWikis(t *testing.T) {
	t.Parallel()
	for k, days := range map[Kind]float64{Meat: 2, Meal: 4, FineMeal: 4, LavishMeal: 4, NutrientPasteMeal: 0.75,
		Pemmican: 70, Berries: 14, Strawberries: 14, Milk: 14, Egg: 15, Rice: 40, Corn: 60, Potatoes: 30,
		Hops: 30, AgaveFruit: 25, HumanMeat: 2, InsectMeat: 2, SurvivalMeal: 0, Kibble: 0, Hay: 0, Food: 0} {
		if got := float64(k.ShelfTicks()) / float64(24*2500); got != days {
			t.Errorf("%v rots in %.2f days, want %.2f", k, got, days)
		}
	}
}

// TestFoodDeterioratesAtItsWikiRate: food has hit points and loses them
// outdoors at the wiki's rate, x5 in rain and x3 on wet ground.
func TestFoodDeterioratesAtItsWikiRate(t *testing.T) {
	t.Parallel()
	cases := []struct {
		k      Kind
		hp, pd int
	}{
		{Meal, 50, 10000}, {NutrientPasteMeal, 50, 10000}, {SurvivalMeal, 50, 250}, {Pemmican, 60, 2000},
		{Kibble, 60, 6000}, {Meat, 60, 6000}, {Rice, 100, 6000}, {Berries, 100, 6000},
	}
	for _, c := range cases {
		s := Stack{Kind: c.k, Qty: 1}
		if s.MaxHP() != c.hp || DeteriorationRate(s, false, false) != c.pd {
			t.Errorf("%v: %d HP %d/day, want %d %d", c.k, s.MaxHP(), DeteriorationRate(s, false, false), c.hp, c.pd)
		}
	}
	if r := DeteriorationRate(Stack{Kind: Rice}, true, true); r != 6000*RainFactor*WetFactor {
		t.Fatalf("rain and marsh: %d", r)
	}
	if MaxHP(Wood) != 0 || HasQuality(Meal) {
		t.Fatal("wood has no condition; food rolls no quality")
	}
}

// TestMergeFoodUnionsIngredients: merged meals keep every ingredient up
// to MaxIngredients, average rot, poison and wear, and merge whatever
// their wear.
func TestMergeFoodUnionsIngredients(t *testing.T) {
	t.Parallel()
	a := Stack{Kind: Meal, Qty: 1, Rot: 100, Poison: 10, Wear: 10}
	a.AddIngredient(Rice, 0)
	a.AddIngredient(Meat, 3)
	a.AddIngredient(Rice, 0) // already there
	b := Stack{Kind: Meal, Qty: 1, Rot: 300, Poison: 30, Wear: 30}
	b.AddIngredient(Meat, 4) // another species
	b.AddIngredient(Corn, 0)
	b.AddIngredient(Potatoes, 0)
	if !SameLoad(a, b) {
		t.Fatal("meals of different wear merge")
	}
	m := MergeFood(a, b, 1)
	want := [MaxIngredients]Kind{Rice, Meat, Meat, Corn}
	if m.Ingredients != want || m.IngredientSpecies != [MaxIngredients]uint8{0, 3, 4, 0} {
		t.Fatalf("ingredients %v %v", m.Ingredients, m.IngredientSpecies)
	}
	if m.Rot != 200 || m.Poison != 20 || m.Wear != 20 || m.Qty != 1 {
		t.Fatalf("merged %+v", m)
	}
	if !m.HasIngredient(Corn) || m.HasIngredient(Potatoes) {
		t.Fatal("the fifth ingredient is dropped")
	}
	if k, sp := IngredientOf(Stack{Kind: Meat, Animal: 5}); k != Meat || sp != 5 {
		t.Fatal("meat keeps its species tag")
	}
	if _, sp := IngredientOf(Stack{Kind: Egg, Animal: 5}); sp != 0 {
		t.Fatal("only meat keeps a species")
	}
	if SameLoad(Stack{Kind: Rifle, Wear: 1}, Stack{Kind: Rifle}) {
		t.Fatal("made things still stack by wear")
	}
}

// TestLegacyRationFoldsIntoSurvivalMeal (FOOD-4): an older save's ration
// loads as a packaged survival meal, keeping everything else; a second
// pass changes nothing.
func TestLegacyRationFoldsIntoSurvivalMeal(t *testing.T) {
	t.Parallel()
	in := Stack{Kind: Food, Qty: 12, X: 3, Y: 4, Carrier: 9, Forbidden: true}
	got, ok := ConvertLegacy(in)
	want := in
	want.Kind = SurvivalMeal
	if !ok || got != want {
		t.Fatalf("converted %v: %+v", ok, got)
	}
	if again, ok := ConvertLegacy(got); ok || again != got {
		t.Fatal("a survival meal converts again")
	}
}
