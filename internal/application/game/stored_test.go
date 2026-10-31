package game

import (
	"reflect"
	"testing"

	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/command"
)

// PLAY-018 §6: WorldView.Stored counts what lies in stockpiles, one row per
// kind, while Food/Wood/... keep counting every loose stack.

// storedYard is a 24x12 map with a stockpile over x 16..22, y 1..10.
func storedYard(t testing.TB) *Game {
	t.Helper()
	g := New(Config{Seed: 1, Width: 24, Height: 12})
	g.Enqueue(command.Command{Kind: command.DesignateStockpile, X: 16, Y: 1, X2: 22, Y2: 10})
	g.Step(1)
	return g
}

func spawn(t testing.TB, g *Game, k item.Kind, qty, x, y int) {
	t.Helper()
	if _, err := g.SpawnStack(k, qty, x, y); err != nil {
		t.Fatal(err)
	}
}

func TestStoredCountsOnlyStockpiledUncarriedUnfogged(t *testing.T) {
	t.Parallel()
	g := storedYard(t)
	ada, err := g.SpawnNamedPawn(1, 1, "Ada")
	if err != nil {
		t.Fatal(err)
	}
	spawn(t, g, item.Wood, 30, 16, 1)  // stored
	spawn(t, g, item.Wood, 12, 17, 1)  // stored, same kind: one row
	spawn(t, g, item.Wood, 50, 3, 3)   // loose on open ground: not stored
	spawn(t, g, item.Steel, 8, 18, 2)  // stored
	spawn(t, g, item.Meal, 5, 19, 2)   // stored food
	spawn(t, g, item.Parka, 1, 20, 3)  // stored apparel
	spawn(t, g, item.Rifle, 1, 21, 4)  // stored weapon
	spawn(t, g, item.Silver, 40, 5, 5) // loose
	// a carried stack standing on a stockpile cell is in someone's hands
	carried, err := g.SpawnStack(item.Wood, 100, 16, 2)
	if err != nil {
		t.Fatal(err)
	}
	st, _ := g.stacks.Get(carried)
	st.Carrier = ada
	g.putStack(carried, st)
	// a stack in unexplored fog neither shows nor counts
	spawn(t, g, item.Steel, 77, 22, 9)
	g.fog.hidden = make([]uint64, (24*12+63)/64)
	i := 9*24 + 22
	g.fog.hidden[i>>6] |= 1 << (uint(i) & 63)

	w := g.World()
	got := map[string]int{}
	for _, r := range w.Stored {
		got[r.Kind] = r.Qty
	}
	want := map[string]int{"wood": 42, "steel": 8, "meal": 5, "parka": 1, "rifle": 1}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("stored %v, want %v", got, want)
	}
	if len(w.Stored) != len(want) {
		t.Fatalf("%d rows, want one per kind (%d): %+v", len(w.Stored), len(want), w.Stored)
	}
	// the colony totals still count every uncarried stack, in stockpiles or
	// not (the carried 100 wood and the fogged 77 steel are left out)
	if w.Wood != 30+12+50 || w.Steel != 8 || w.Silver != 40 {
		t.Fatalf("totals changed: wood %d, steel %d, silver %d", w.Wood, w.Steel, w.Silver)
	}
}

func TestStoredRowsLabelledAndSorted(t *testing.T) {
	t.Parallel()
	g := storedYard(t)
	spawn(t, g, item.Steel, 3, 16, 1)
	spawn(t, g, item.Meal, 4, 17, 1)
	spawn(t, g, item.Wood, 5, 18, 1)
	spawn(t, g, item.Meat, 6, 19, 1)
	spawn(t, g, item.HerbalMedicine, 7, 20, 1)
	spawn(t, g, item.Rifle, 1, 21, 1)
	spawn(t, g, item.CorpseHuman, 1, 22, 1)
	spawn(t, g, item.Components, 2, 16, 2)
	spawn(t, g, item.FertileEgg, 1, 17, 2)
	spawn(t, g, item.Tuque, 1, 18, 2)
	want := []StoredResourceView{ // FOOD-3: a fertile egg is food
		{"fertile egg", "Fertile egg", "Food", 1}, {"meal", "Meal", "Food", 4}, {"meat", "Meat", "Food", 6},
		{"steel", "Steel", "Materials", 3}, {"wood", "Wood", "Materials", 5},
		{"herbal medicine", "Herbal medicine", "Medicine", 7},
		{"tuque", "Tuque", "Apparel", 1},
		{"rifle", "Rifle", "Weapons", 1},
		{"components", "Components", "Manufactured", 2},
		{"human corpse", "Human corpse", "Corpses", 1},
	}
	if got := g.World().Stored; !reflect.DeepEqual(got, want) {
		t.Fatalf("stored\n got %+v\nwant %+v", got, want)
	}
}

func TestStoredEmptyWithoutStockpileStacks(t *testing.T) {
	t.Parallel()
	g := storedYard(t)
	spawn(t, g, item.Wood, 9, 2, 2)
	if n := len(g.World().Stored); n != 0 {
		t.Fatalf("%d rows for loose stacks only", n)
	}
	// a stockpile painted over a loose stack takes it into the list
	g.Enqueue(command.Command{Kind: command.DesignateStockpile, X: 1, Y: 1, X2: 3, Y2: 3})
	g.Step(1)
	if got := g.World().Stored; len(got) != 1 || got[0].Kind != "wood" || got[0].Qty != 9 {
		t.Fatalf("stored %+v", got)
	}
}
