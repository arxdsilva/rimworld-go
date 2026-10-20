package item

import (
	"testing"

	"rimworld-go/internal/domain/sim/clock"
)

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

func TestDried(t *testing.T) {
	s := Stack{Kind: Meat, Qty: 5, Dried: true, Rot: 10 * Meat.ShelfTicks()}
	if s.Shelf() != 0 || s.Rotten() || s.Dryable() {
		t.Error("dried meat keeps and can't be dried again")
	}
	for _, k := range Foods() {
		if k.Perishable() && !(Stack{Kind: k, Qty: 1}).Dryable() {
			t.Errorf("%s should be dryable", k)
		}
		if k.Perishable() && k.DryTicks() <= 0 {
			t.Errorf("%s has no drying time", k)
		}
	}
	if (Stack{Kind: Wheat, Qty: 1}).Dryable() || (Stack{Kind: CorpseDustbuck, Qty: 1}).Dryable() {
		t.Error("grain is already dry; corpses are butchered")
	}
	if CanMerge(Stack{Kind: Meat, Qty: 1}, Stack{Kind: Meat, Qty: 1, Dried: true}) {
		t.Error("dried and fresh meat must not merge")
	}
	if DryRate(250) != 1000 || DryRate(-50) != 250 || DryRate(100) != 625 {
		t.Error("dry rate curve wrong")
	}
}

func TestKnifeSpearAreValuedWeapons(t *testing.T) {
	t.Parallel()
	for _, k := range []Kind{Knife, Spear} {
		if !k.IsWeapon() || k.Value() <= 0 {
			t.Fatalf("%v: weapon=%v value=%d", k, k.IsWeapon(), k.Value())
		}
	}
	if Knife.Value() >= Spear.Value() || Spear.Value() >= Club.Value()+20 {
		t.Fatalf("values knife %d spear %d club %d", Knife.Value(), Spear.Value(), Club.Value())
	}
}

func TestHumanCorpseKeepsThreeDaysAndBonesNeverRot(t *testing.T) {
	if got, want := CorpseHuman.ShelfTicks(), 3*clock.TicksPerDay; got != want {
		t.Fatalf("human corpse shelf %d, want %d (3 days)", got, want)
	}
	if Bones.Perishable() || Bones.IsFood() || Bones.IsCorpse() {
		t.Fatal("bones never rot, are not food and are not butchered")
	}
	if !CorpseHuman.IsCorpse() || CorpseHuman.IsFood() {
		t.Fatal("a human corpse is a corpse, never food")
	}
}

func TestBodiesOfDifferentPawnsNeverMerge(t *testing.T) {
	a := Stack{Kind: CorpseHuman, Qty: 1, X: 2, Y: 2, Of: 7}
	b := Stack{Kind: CorpseHuman, Qty: 1, X: 2, Y: 2, Of: 9}
	if CanMerge(a, b) {
		t.Fatal("two people's bodies must stay two stacks")
	}
	if CanMerge(Stack{Kind: Bones, Qty: 1, Of: 7}, Stack{Kind: Bones, Qty: 1, Of: 9}) {
		t.Fatal("two people's bones must stay two stacks")
	}
	if !CanMerge(Stack{Kind: Wood, Qty: 1}, Stack{Kind: Wood, Qty: 2}) {
		t.Fatal("ordinary stacks still merge")
	}
}
