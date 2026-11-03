package item

import "testing"

// TestFishIsRawMeat (FISH-*): fish behave as meat in everything the game asks
// of an item, apart from the nutrition, mass and value of the tuning table.
func TestFishIsRawMeat(t *testing.T) {
	t.Parallel()
	if got, ok := ParseKind("fish"); !ok || got != Fish {
		t.Fatalf("parse fish: %v %v", got, ok)
	}
	if !Fish.IsMeat() || !Fish.Raw() || !Fish.IsFood() || !Fish.Perishable() {
		t.Fatalf("fish: meat %v raw %v food %v rots %v", Fish.IsMeat(), Fish.Raw(), Fish.IsFood(), Fish.Perishable())
	}
	if Fish.Category() != Meat.Category() || Fish.StackLimit() != Meat.StackLimit() || Fish.Preference() != Meat.Preference() {
		t.Fatalf("fish: category %q stack %d preference %d", Fish.Category(), Fish.StackLimit(), Fish.Preference())
	}
	if Fish.Nutrition() != Meat.Nutrition() {
		t.Fatalf("fish nutrition %d, meat %d", Fish.Nutrition(), Meat.Nutrition())
	}
	if Fish.Value() <= 0 {
		t.Fatalf("fish value %d", Fish.Value())
	}
}
