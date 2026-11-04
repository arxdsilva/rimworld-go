package item

import "rimworld-go/internal/domain/gameplay"

// MealRule is how meals stack by their ingredients (CSE-012, CSE-013). The
// zero value is the default: tainted meals stay apart, insect meat counts as
// a taint. The caller turns the colony's habit rules into it; item cannot see
// them.
type MealRule struct {
	OddNormal bool // CSE-013: insect meat is no taint
	Mix       bool // SeparateMeals off: ingredients never keep stacks apart
}

// RuleOf is the meal rule the colony's habit rules set.
func RuleOf(s gameplay.Settings) MealRule {
	return MealRule{OddNormal: s.On(gameplay.OddMeatNormal), Mix: !s.On(gameplay.SeparateMeals)}
}

// Taint bits of a meal's ingredients.
const (
	taintHuman uint8 = 1 << iota
	taintInsect
)

// mealTaint is the set of "bad ingredient" flags on st: human meat, and
// insect meat unless r.OddNormal. A stack without ingredients has none.
func mealTaint(st Stack, r MealRule) uint8 {
	var t uint8
	for _, k := range st.Ingredients {
		switch k {
		case HumanMeat:
			t |= taintHuman
		case InsectMeat:
			if !r.OddNormal {
				t |= taintInsect
			}
		}
	}
	return t
}

// SameMeal is SameLoad that also keeps meals whose bad-ingredient flags
// differ apart, so one tainted meal does not spread to a clean stack
// (CSE-012). Every place that joins stacks or loads asks this.
func SameMeal(a, b Stack, r MealRule) bool {
	return SameLoad(a, b) && (r.Mix || mealTaint(a, r) == mealTaint(b, r))
}

// CanMergeMeal is CanMerge under r.
func CanMergeMeal(a, b Stack, r MealRule) bool {
	return SameMeal(a, b, r) && a.Carrier == 0 && b.Carrier == 0 && a.X == b.X && a.Y == b.Y &&
		a.Forbidden == b.Forbidden && a.Unclaimed == b.Unclaimed && a.HaulMarked == b.HaulMarked
}
