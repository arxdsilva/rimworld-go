package animal

import "rimworld-go/internal/domain/economy/item"

// EggNutrition is the nutrition of one egg laid by species s (FOOD-1,
// wiki: Egg: chicken, duck, cobra, iguana and tortoise 0.25; goose,
// turkey, cassowary and emu 0.5; ostrich 0.6).
func EggNutrition(s Species) int {
	switch s {
	case Goose, Turkey, Cassowary, Emu:
		return 5000
	case Ostrich:
		return 6000
	}
	return item.Egg.Nutrition()
}

// StackNutrition is what one unit of stack st feeds a pawn: an egg by the
// species that laid it (Stack.Animal; 0, a deer, lays none: an egg from
// before FOOD-1 counts as a chicken's), a carcass by its meat
// (CarcassFood), anything else by its kind.
func StackNutrition(st item.Stack) int {
	switch {
	case (st.Kind == item.Egg || st.Kind == item.FertileEgg) && st.Animal != 0: // FOOD-3: fertilized eggs are eaten too
		return EggNutrition(Species(st.Animal))
	case EdibleCorpse(st):
		return CarcassFood(Species(st.Animal))
	}
	return st.Kind.Nutrition()
}

// EdibleCorpse reports a carcass a malnourished pawn may eat (wiki:
// Malnutrition: anything, corpses too): an animal's, not rotten, never a
// mechanoid's wreck. Human corpses stay uneaten (cannibals are FOOD-4's).
func EdibleCorpse(st item.Stack) bool {
	return st.Kind == item.CorpseAnimal && !IsMech(Species(st.Animal)) && !st.Rotten()
}

// AgeHunger is the hunger rate (permille) of an animal of species s aged
// days: a baby 400, a juvenile 750, an adult 1000. wiki: Saturation
// (life-stage factors); the stages are the species' (breeding.go).
func AgeHunger(s Species, days int) int {
	switch {
	case Adult(s, days):
		return 1000
	case !Juvenile(s, days):
		return 400
	}
	return 750
}
