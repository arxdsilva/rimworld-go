package item

import "testing"

func TestFertileEggsStackBySpeciesAndOwner(t *testing.T) {
	a := Stack{Kind: FertileEgg, Qty: 1, X: 2, Y: 2, Animal: 48, Tame: true}
	b := a
	if !CanMerge(a, b) {
		t.Fatal("the same hen's eggs stack")
	}
	b.Tame = false
	if CanMerge(a, b) {
		t.Fatal("wild and tame eggs don't stack")
	}
	b = a
	b.Animal = 49
	if CanMerge(a, b) {
		t.Fatal("eggs stack by species")
	}
	if !FertileEgg.IsFood() || !FertileEgg.Raw() || !Egg.Raw() || !Milk.Raw() {
		t.Fatal("fertile eggs (FOOD-3, wiki: Eggs), eggs and milk are raw food")
	}
}
