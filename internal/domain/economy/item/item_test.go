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
	if !(Meal.Nutrition() > Potatoes.Nutrition() && Potatoes.Nutrition() > Berries.Nutrition() && Wheat.Nutrition() > Lettuce.Nutrition()) {
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

func TestRot(t *testing.T) {
	if Wheat.Perishable() || Beans.Perishable() || Food.Perishable() || Wood.Perishable() {
		t.Error("dried grain, beans, rations and materials should keep")
	}
	if !Meat.Perishable() || Meat.ShelfTicks() >= Potatoes.ShelfTicks() {
		t.Error("meat should spoil before potatoes")
	}
	if RotRate(-50) != 0 || RotRate(0) != 0 || RotRate(50) != 500 || RotRate(250) != 1000 {
		t.Error("rot rate curve wrong")
	}
	s := Stack{Kind: Meat, Qty: 5, Rot: Meat.ShelfTicks()}
	if !s.Rotten() || s.FreshTicks() != 0 {
		t.Error("meat at shelf life should be rotten")
	}
	if got := MergeRot(10, 0, 30, 100); got != 75 {
		t.Errorf("merge rot = %d, want 75", got)
	}
	fresh, rotten := Stack{Kind: CorpseDustbuck, Qty: 1}, Stack{Kind: CorpseDustbuck, Qty: 1, Rot: CorpseDustbuck.ShelfTicks()}
	if CanMerge(fresh, rotten) {
		t.Error("rotten and fresh corpses must not merge")
	}
}
