package game

import (
	"testing"

	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/gameplay"
)

// dropMeal puts a meal of qty with ingredients ks on tile (5, 5) the way a
// dropped load lands.
func dropMeal(t *testing.T, g *Game, qty int, ks ...item.Kind) {
	t.Helper()
	sid, err := g.SpawnStack(item.Meal, qty, 9, 9)
	if err != nil {
		t.Fatal(err)
	}
	st, _ := g.stacks.Get(sid)
	for _, k := range ks {
		st.AddIngredient(k, 0)
	}
	st.X, st.Y = 5, 5
	g.mergeOrSet(sid, st)
}

// TestTaintedMealStaysApart (CSE-012, CSE-013): a meal with human meat does
// not join a clean one on the same tile, two tainted ones do, and the
// toggles change that.
func TestTaintedMealStaysApart(t *testing.T) {
	t.Parallel()
	count := func(rules gameplay.Settings, second ...item.Kind) int {
		g := New(Config{Seed: 1, Width: 24, Height: 12})
		senseOf(g).Rules = rules
		dropMeal(t, g, 3, item.Rice)
		dropMeal(t, g, 2, second...)
		n := 0
		for _, id := range g.itemIDs {
			if st := g.stacks.Ref(id); st != nil && st.Kind == item.Meal {
				n++
			}
		}
		return n
	}
	def := gameplay.Settings{}
	if n := count(def, item.Rice, item.HumanMeat); n != 2 {
		t.Fatalf("tainted beside clean: %d stacks, want 2", n)
	}
	if n := count(def, item.Potatoes); n != 1 {
		t.Fatalf("clean with clean: %d stacks, want 1", n)
	}
	if n := count(def.With(gameplay.SeparateMeals, false), item.HumanMeat); n != 1 {
		t.Fatalf("separate meals off: %d stacks, want 1", n)
	}
	if n := count(def, item.InsectMeat); n != 2 {
		t.Fatalf("insect meal beside clean: %d stacks, want 2", n)
	}
	if n := count(def.With(gameplay.OddMeatNormal, true), item.InsectMeat); n != 1 {
		t.Fatalf("odd meat normal: %d stacks, want 1", n)
	}
}
