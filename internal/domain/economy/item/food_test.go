package item

import "testing"

// TestFoodCatalogue (FOOD-0 contract): the food kinds parse by name, are
// valued, stack to the raw-food 75 and land in their categories; the
// raw foods are raw and edible, hay is feed (edible, not raw, eaten
// last), hops are a material; both new meats are meat (the meat side of
// recipes), and strawberries skip the raw-food thought like berries.
func TestFoodCatalogue(t *testing.T) {
	t.Parallel()
	cats := map[Kind]string{Rice: "Food", Strawberries: "Food", Hay: "Food", Hops: "Materials", HumanMeat: "Food",
		InsectMeat: "Food", AgaveFruit: "Food"}
	for k := Rice; k < numKinds; k++ {
		want, ok := cats[k]
		if !ok {
			t.Errorf("%s has no expected category here", k)
			continue
		}
		if got, ok := ParseKind(k.String()); !ok || got != k || k.Value() <= 0 || k.Category() != want || k.StackLimit() != 75 {
			t.Errorf("%s: parse %v %v, value %d, category %q want %q, stack %d", k, got, ok, k.Value(), k.Category(), want,
				k.StackLimit())
		}
	}
	for _, k := range []Kind{Rice, Strawberries, HumanMeat, InsectMeat, AgaveFruit} {
		if !k.Raw() || !k.IsFood() || k.Nutrition() <= 0 {
			t.Errorf("%s: raw %v food %v", k, k.Raw(), k.IsFood())
		}
	}
	if Hay.Raw() || !Hay.IsFood() || Hay.Preference() != Kibble.Preference() || Hay.Perishable() {
		t.Error("hay: animal feed, eaten last, never rots")
	}
	if Hops.IsFood() || HumanMeat.Preference() != 0 {
		t.Error("hops are not food; human meat is eaten only when nothing else is left")
	}
	if !HumanMeat.IsMeat() || !InsectMeat.IsMeat() || !Meat.IsMeat() || Milk.IsMeat() {
		t.Error("IsMeat: the three meats, not animal products")
	}
	if !Strawberries.RawPenaltyFree() || !Berries.RawPenaltyFree() || Rice.RawPenaltyFree() {
		t.Error("strawberries and berries are eaten raw without the thought; rice is not")
	}
	if Strawberries.Nutrition() != Berries.Nutrition() || Rice.Nutrition() != Wheat.Nutrition() || HumanMeat.Nutrition() != Meat.Nutrition() {
		t.Error("FOOD-0 keeps the current nutrition scale")
	}
}
