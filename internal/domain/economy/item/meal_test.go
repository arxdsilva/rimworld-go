package item

import (
	"testing"

	"rimworld-go/internal/domain/gameplay"
)

func mealWith(ks ...Kind) Stack {
	s := Stack{Kind: Meal, Qty: 3}
	for _, k := range ks {
		s.AddIngredient(k, 0)
	}
	return s
}

// TestSameMealTaint (CSE-012, CSE-013): meals with differing bad-ingredient
// flags stay apart; the odd-is-normal rule treats insect meat as clean and
// the mix rule ignores ingredients as before.
func TestSameMealTaint(t *testing.T) {
	clean, human, insect := mealWith(Rice), mealWith(Rice, HumanMeat), mealWith(InsectMeat)
	cases := []struct {
		name string
		a, b Stack
		r    MealRule
		want bool
	}{
		{"clean and clean", clean, mealWith(Potatoes), MealRule{}, true},
		{"clean and human", clean, human, MealRule{}, false},
		{"human and human, other food", human, mealWith(HumanMeat), MealRule{}, true},
		{"clean and insect", clean, insect, MealRule{}, false},
		{"human and insect", human, insect, MealRule{}, false},
		{"insect is normal", clean, insect, MealRule{OddNormal: true}, true},
		{"human is never normal", clean, human, MealRule{OddNormal: true}, false},
		{"mixing allowed", clean, human, MealRule{Mix: true}, true},
		{"no ingredients", Stack{Kind: Meal}, human, MealRule{}, false},
		{"different kinds never join", clean, Stack{Kind: FineMeal}, MealRule{Mix: true}, false},
	}
	for _, c := range cases {
		if got := SameMeal(c.a, c.b, c.r); got != c.want || SameMeal(c.b, c.a, c.r) != c.want {
			t.Errorf("%s: SameMeal = %v, want %v", c.name, got, c.want)
		}
	}
}

// TestSameMealAgreesOnEveryPath: the tile merge, the carried load and the
// hauler's pickup all ask SameMeal, so for any pair and rule CanMergeMeal on
// one tile gives SameMeal's answer; a stack never splits on one path and
// merges on another.
func TestSameMealAgreesOnEveryPath(t *testing.T) {
	stacks := []Stack{mealWith(), mealWith(Rice), mealWith(HumanMeat), mealWith(InsectMeat), mealWith(Meat, HumanMeat, InsectMeat)}
	for _, off := range []bool{false, true} {
		for _, odd := range []bool{false, true} {
			s := gameplay.Settings{}.With(gameplay.SeparateMeals, !off).With(gameplay.OddMeatNormal, odd)
			r := RuleOf(s)
			if r.Mix != off || r.OddNormal != odd {
				t.Fatalf("RuleOf %+v = %+v", s, r)
			}
			for _, a := range stacks {
				for _, b := range stacks {
					if CanMergeMeal(a, b, r) != SameMeal(a, b, r) {
						t.Fatalf("paths disagree for %v, %v under %+v", a.Ingredients, b.Ingredients, r)
					}
				}
			}
		}
	}
	if RuleOf(gameplay.Settings{}) != (MealRule{}) {
		t.Fatal("the default settings are the zero rule")
	}
}
