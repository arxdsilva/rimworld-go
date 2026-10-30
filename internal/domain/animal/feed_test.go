package animal

import (
	"testing"

	"rimworld-go/internal/domain/economy/item"
)

// FEED-001: the diet table follows the wiki (kibble, pemmican and meals
// for every diet but strict carnivores; no meals for tree eaters), feed
// comes before meals, and the starving search has a hysteresis band.
func TestFeedValueDietTable(t *testing.T) {
	t.Parallel()
	eats := func(d Diet, k item.Kind) bool { return FeedValue(d, k) > 0 }
	for _, k := range []item.Kind{item.Kibble, item.Pemmican, item.Meal, item.FineMeal, item.LavishMeal, item.SurvivalMeal, item.Food} {
		for _, d := range []Diet{Herbivore, Carnivore, Omnivore} {
			if FeedValue(d, k) != k.Nutrition() {
				t.Errorf("%v eats %v at its nutrition: %d", d, k, FeedValue(d, k))
			}
		}
		if eats(StrictCarnivore, k) || eats(NoDiet, k) || eats(Diet(99), k) {
			t.Errorf("strict carnivores and the needless never eat %v", k)
		}
		if eats(Dendrovore, k) != (FoodTier(k) == TierFeed) {
			t.Errorf("tree eaters take kibble and pemmican, no meals: %v", k)
		}
	}
	// a herbivore: kibble, meals, vegetables, straw, never meat
	if !eats(Herbivore, item.Corn) || !eats(Herbivore, item.Straw) || eats(Herbivore, item.Meat) {
		t.Error("herbivore table")
	}
	// a strict carnivore: meat only (carcasses through CarcassFood)
	for _, k := range item.Foods() {
		if eats(StrictCarnivore, k) != (k == item.Meat) {
			t.Errorf("a strict carnivore and %v", k)
		}
	}
	// a tree eater: raw vegetables and berries, no straw, meat or meals
	if !eats(Dendrovore, item.Potatoes) || !eats(Dendrovore, item.Berries) || eats(Dendrovore, item.Straw) || eats(Dendrovore, item.Meat) {
		t.Error("tree eater table")
	}
	for _, k := range []item.Kind{item.Milk, item.Egg, item.HerbalMedicine, item.Brewgrain} {
		if eats(Omnivore, k) {
			t.Errorf("no animal eats %v", k)
		}
	}
}

func TestFoodTierAndPrepared(t *testing.T) {
	t.Parallel()
	for _, k := range item.Foods() {
		meal := k == item.Meal || k == item.FineMeal || k == item.LavishMeal || k == item.SurvivalMeal || k == item.Food
		if (FoodTier(k) == TierMeal) != meal {
			t.Errorf("tier of %v", k)
		}
		if Prepared(k) != (meal || k == item.Kibble || k == item.Pemmican) {
			t.Errorf("prepared %v", k)
		}
	}
	if FoodTier(item.Straw) != TierFeed || FoodTier(item.CorpseAnimal) != TierFeed {
		t.Error("straw and carcasses are feed")
	}
}

func TestStrictCarnivoreDiet(t *testing.T) {
	t.Parallel()
	if !StrictCarnivore.Meat() || StrictCarnivore.Plants() || !StrictCarnivore.MeatOnly() || !Carnivore.MeatOnly() || Omnivore.MeatOnly() {
		t.Error("a strict carnivore eats meat only")
	}
	if StrictCarnivore.String() != "meat only" || EcologyOf(DireWolf).Diet != StrictCarnivore {
		t.Error("the dire wolf is our strict carnivore")
	}
	for d, want := range map[Diet]string{Herbivore: "kibble, pemmican and meals", Carnivore: "kibble, pemmican and meals",
		Omnivore: "kibble, pemmican and meals", Dendrovore: "kibble and pemmican", StrictCarnivore: "", NoDiet: ""} {
		if d.Made() != want {
			t.Errorf("%v made %q, want %q", d, d.Made(), want)
		}
	}
}

// No flap: the far search starts below StarveSearch and holds until food
// is back at StarveSearchEnd.
func TestFarSearchBand(t *testing.T) {
	t.Parallel()
	if StarveSearch >= StarveSearchEnd || StarveSearchEnd >= FeedStart {
		t.Fatal("band order")
	}
	was := false
	var flips int
	for _, food := range []int{3000, 1600, 1499, 1000, 1600, 2000, 2499, 1600, 2500, 2000, 1600, 1499} {
		if f := FarSearch(food, was); f != was {
			flips++
			was = f
		}
	}
	if flips != 3 {
		t.Errorf("starts below 15 %%, ends at 25 %%, starts again below 15 %%: %d flips", flips)
	}
}
