package diet

import (
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/mood"
)

// Thought is one memory a meal leaves: its kind, its mood effect and
// label (the def's, or the cannibal's flipped line).
type Thought struct {
	Kind  mood.Kind
	Value int
	Label string
}

// Cannibal lines (original wording; wiki: Traits, Cannibal: raw +20,
// cooked +15, a day each).
const (
	cannibalRaw    = "relished raw human flesh"
	cannibalCooked = "relished a meal of human flesh"
)

// AppendAte appends the thoughts eating from st gives a person (wiki:
// Food, Meals, Kibble, Human meat, Insect meat): the meal's own taste
// (lavish +12, fine +5, paste -4, kibble -12, raw food -7 unless milk or
// berries), raw human meat -20 or a meal with it -15 (a cannibal +20 and
// +15), raw insect meat -6 or a meal with it -3. Where the colonist ate
// (a table) is the caller's.
func AppendAte(dst []Thought, st item.Stack, cannibal bool) []Thought {
	k := st.Kind
	switch {
	case k == item.LavishMeal:
		dst = append(dst, def(mood.AteLavishMeal))
	case k == item.FineMeal:
		dst = append(dst, def(mood.AteFineMeal))
	case k == item.NutrientPasteMeal:
		dst = append(dst, def(mood.AtePaste))
	case k == item.Kibble:
		dst = append(dst, def(mood.AteKibble))
	case k.Raw() && !k.RawPenaltyFree():
		dst = append(dst, def(mood.AteRaw))
	}
	switch {
	case k == item.HumanMeat:
		dst = append(dst, flip(mood.AteHumanMeat, cannibal, cannibalRaw))
	case !k.Raw() && st.HasIngredient(item.HumanMeat):
		dst = append(dst, flip(mood.AteHumanMeatCooked, cannibal, cannibalCooked))
	}
	switch {
	case k == item.InsectMeat:
		dst = append(dst, def(mood.AteInsectMeat))
	case !k.Raw() && st.HasIngredient(item.InsectMeat):
		dst = append(dst, def(mood.AteInsectMeatCooked))
	}
	return dst
}

func def(k mood.Kind) Thought { return Thought{Kind: k, Value: mood.DefOf(k).Value} }

// flip is kind k's thought, turned to delight for a cannibal.
func flip(k mood.Kind, cannibal bool, label string) Thought {
	t := def(k)
	if cannibal {
		t.Value, t.Label = -t.Value, label
	}
	return t
}

// AteMood is the summed mood effect of the thoughts eating from st gives.
func AteMood(st item.Stack, cannibal bool) int {
	var buf [3]Thought
	sum := 0
	for _, t := range AppendAte(buf[:0], st, cannibal) {
		sum += t.Value
	}
	return sum
}
