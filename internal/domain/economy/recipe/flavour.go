package recipe

import "rimworld-go/internal/domain/economy/item"

// Flavour ingredients (CSE-014): what a meal that no recipe made (a trader's,
// a settlement's) is shown as made of. Display only: every choice here
// carries no mood thought, so nothing the simulation reads changes.

var (
	flavourVeg  = []item.Kind{item.Rice, item.Potatoes, item.Corn, item.Lettuce, item.Pumpkins, item.Berries, item.Strawberries, item.Radishes}
	flavourMeat = []item.Kind{item.Meat, item.Fish, item.Egg, item.Milk}
)

// FlavourSlots are the neutral choices for each input of the recipe that
// cooks meal kind k: nil for anything that is not a cooked meal. Never human
// or insect meat, never a food with a bad thought, never a composite meal.
// The slices are shared: read only.
func FlavourSlots(k item.Kind) [][]item.Kind {
	switch k {
	case item.Meal:
		return [][]item.Kind{flavourVeg}
	case item.FineMeal, item.LavishMeal:
		return [][]item.Kind{flavourVeg, flavourMeat}
	}
	return nil
}

// FlavourKind is the neutral kind for slot of meal kind k, picked by the
// random word h (a pure function of the stack: no state, no spawn hook).
func FlavourKind(k item.Kind, slot int, h uint64) item.Kind {
	s := FlavourSlots(k)
	if slot < 0 || slot >= len(s) {
		return 0
	}
	return s[slot][h%uint64(len(s[slot]))]
}
