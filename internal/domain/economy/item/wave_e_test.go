package item

import "testing"

// TestWaveECatalogue (PRD-4 contract): the new kinds parse by name, carry
// a wiki value, and land in their categories: meals and kibble are cooked
// food, the hard drugs are drugs, the rest materials or manufactured.
func TestWaveECatalogue(t *testing.T) {
	t.Parallel()
	cats := map[Kind]string{
		Chemfuel: "Materials", Wort: "Materials", Reagent: "Materials", Uranium: "Materials", Plasteel: "Materials",
		FineMeal: "Food", LavishMeal: "Food", Pemmican: "Food", SurvivalMeal: "Food", Kibble: "Food",
		AshFlake: "Medicine", Silverdust: "Medicine", BattleTonic: "Medicine",
		AdvancedComponents: "Manufactured", ReinforcedBarrel: "Manufactured",
	}
	for k := Chemfuel; k <= ReinforcedBarrel; k++ {
		want, ok := cats[k]
		if !ok {
			t.Errorf("%s has no expected category here", k)
			continue
		}
		if got, ok := ParseKind(k.String()); !ok || got != k || k.Value() <= 0 || k.Category() != want {
			t.Errorf("%s: parse %v %v, value %d, category %q want %q", k, got, ok, k.Value(), k.Category(), want)
		}
		if k.IsFood() && k.Raw() {
			t.Errorf("%s is cooked, not raw", k)
		}
	}
	if !AshFlake.IsDrug() || Reagent.IsDrug() || !Wort.Perishable() || Wort.IsFood() || SurvivalMeal.Perishable() || Kibble.Perishable() {
		t.Error("drugs, wort and the meals that keep")
	}
	if LavishMeal.Preference() <= FineMeal.Preference() || FineMeal.Preference() <= Meal.Preference() {
		t.Error("the better the meal, the sooner it is eaten")
	}
}
