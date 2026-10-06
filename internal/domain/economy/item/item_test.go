package item

import "testing"

func TestTakeRejectsOverdraw(t *testing.T) {
	if _, ok := Take(2, 3); ok {
		t.Fatal("must not take more than qty")
	}
}

func TestTakeReducesQty(t *testing.T) {
	got, ok := Take(5, 2)
	if !ok || got != 3 {
		t.Fatalf("got qty=%d ok=%v", got, ok)
	}
}

func TestAddRespectsLimit(t *testing.T) {
	got := Add(StackLimit-1, 5)
	if got != StackLimit {
		t.Fatalf("qty=%d", got)
	}
}

func TestFoodKinds(t *testing.T) {
	if !Meal.IsFood() || Wood.IsFood() || Stone.IsFood() {
		t.Fatal("food classification")
	}
	if !(Meal.Nutrition() > Amberroot.Nutrition() && Amberroot.Nutrition() > Berries.Nutrition()) {
		t.Fatal("meals are the most nourishing, berries the least")
	}
	if !(Meal.Preference() > Food.Preference() && Food.Preference() > Meat.Preference()) {
		t.Fatal("preference: meal > ration > raw meat")
	}
	if !Meat.Raw() || Meal.Raw() || Food.Raw() {
		t.Fatal("raw classification")
	}
	for _, k := range Foods() {
		if got, ok := ParseKind(k.String()); !ok || got != k {
			t.Fatalf("parse %s", k)
		}
	}
}
