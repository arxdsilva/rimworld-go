package item

import "testing"

// TestStoCbtCatalogue (STO/CBT contract): the new kinds parse by name,
// carry a wiki value and land in their categories; the paste meal is a
// cooked, quickly rotting food; since CBT-3 the weapons equip and the
// belts are worn in the belt slot.
func TestStoCbtCatalogue(t *testing.T) {
	t.Parallel()
	cats := map[Kind]string{NutrientPasteMeal: "Food", SmokepopPack: "Apparel", ShieldBelt: "Apparel",
		RocketLauncher: "Weapons", TripleRocketLauncher: "Weapons", SmokeLauncher: "Weapons", ChargeRifle: "Weapons",
		ChargeLance: "Weapons", Ikwa: "Weapons"}
	for k := NutrientPasteMeal; k < numKinds; k++ {
		want, ok := cats[k]
		if !ok {
			t.Errorf("%s has no expected category here", k)
			continue
		}
		if got, ok := ParseKind(k.String()); !ok || got != k || k.Value() <= 0 || k.Category() != want {
			t.Errorf("%s: parse %v %v, value %d, category %q want %q", k, got, ok, k.Value(), k.Category(), want)
		}
		if want == "Weapons" && !k.IsWeapon() || want == "Apparel" && (!k.IsApparel() || k.Slot() != Belt) {
			t.Errorf("%s is not equippable or wearable (slot %v)", k, k.Slot())
		}
	}
	if NutrientPasteMeal.Raw() || !NutrientPasteMeal.Perishable() || NutrientPasteMeal.StackLimit() != 10 ||
		NutrientPasteMeal.Preference() >= Meal.Preference() || NutrientPasteMeal.Nutrition() != Meal.Nutrition() {
		t.Error("the paste meal: cooked, rots, stacks to 10, less liked than a simple meal, as filling")
	}
}
