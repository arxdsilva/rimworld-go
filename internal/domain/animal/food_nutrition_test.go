package animal

import (
	"testing"

	"rimworld-go/internal/domain/economy/item"
)

// TestAnimalDailyNeed (FOOD-1, wiki: Cow, Alpaca): on the nutrition scale
// a cow eats about 0.86 a day and an alpaca 0.44; babies and juveniles
// eat 40 % and 75 % of that.
func TestAnimalDailyNeed(t *testing.T) {
	t.Parallel()
	perDay := func(s Species) int { return FoodPerDay * 100 / Meal(s, 100) }
	if c := perDay(Cow); c < 8500 || c > 8700 {
		t.Fatalf("cow eats %d a day, want about 8600", c)
	}
	if a := perDay(Alpaca); a < 4350 || a > 4450 {
		t.Fatalf("alpaca eats %d a day, want about 4400", a)
	}
	j, a := LifeOf(Cow).JuvenileDays, LifeOf(Cow).AdultDays
	if AgeHunger(Cow, j-1) != 400 || AgeHunger(Cow, j) != 750 || AgeHunger(Cow, a) != 1000 {
		t.Fatal("life-stage hunger: baby 40 %, juvenile 75 %, adult 100 %")
	}
}

// TestEggAndCarcassNutrition (FOOD-1, wiki: Egg): eggs feed by species, an
// untagged one as a chicken's; a carcass by its meat, never a mechanoid's
// wreck or a rotten one.
func TestEggAndCarcassNutrition(t *testing.T) {
	t.Parallel()
	for s, want := range map[Species]int{Chicken: 2500, Duck: 2500, Goose: 5000, Turkey: 5000, Ostrich: 6000} {
		if got := StackNutrition(item.Stack{Kind: item.Egg, Animal: uint8(s)}); got != want {
			t.Errorf("%v egg %d, want %d", s, got, want)
		}
	}
	if StackNutrition(item.Stack{Kind: item.Egg}) != 2500 || StackNutrition(item.Stack{Kind: item.Meal}) != 9000 {
		t.Fatal("an old egg is a chicken's; a meal its kind's")
	}
	deer := item.Stack{Kind: item.CorpseAnimal, Animal: uint8(Deer)}
	if !EdibleCorpse(deer) || StackNutrition(deer) != CarcassFood(Deer) {
		t.Fatal("a deer carcass feeds its meat")
	}
	for s := Species(0); s < numSpecies; s++ {
		if IsMech(s) && EdibleCorpse(item.Stack{Kind: item.CorpseAnimal, Animal: uint8(s)}) {
			t.Fatalf("%v wreck is edible", s)
		}
	}
	if EdibleCorpse(item.Stack{Kind: item.CorpseHuman}) {
		t.Fatal("human corpses stay uneaten")
	}
}

// TestProductBySaturation (FOOD-1, wiki: Saturation): products fill at
// 100 / 50 / 25 / 0 % for fed, hungry, ravenous and malnourished.
func TestProductBySaturation(t *testing.T) {
	t.Parallel()
	full := ProductPerDay(Cow, Full)
	if full <= 0 || ProductPerDay(Cow, ProductFedFood-1) != full/2 || ProductPerDay(Cow, ProductFedFood/2-1) != full/4 || ProductPerDay(Cow, 0) != 0 {
		t.Fatalf("cow product: %d %d %d %d", full, ProductPerDay(Cow, ProductFedFood-1), ProductPerDay(Cow, ProductFedFood/2-1), ProductPerDay(Cow, 0))
	}
}
