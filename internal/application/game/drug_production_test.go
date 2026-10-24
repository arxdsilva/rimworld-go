package game

import (
	"testing"

	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/recipe"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/research"
	"rimworld-go/internal/domain/sim/command"
)

// brewBench runs one bill of r at a freshly built bench k with ingredient
// in, qty, and returns the game after enough time to finish it.
func brewBench(t *testing.T, researched bool, k construction.Kind, r recipe.ID, in item.Kind, qty int) *Game {
	t.Helper()
	g := New(Config{Seed: 1, Width: 12, Height: 6})
	id, _ := g.SpawnNamedPawn(1, 1, "Ada")
	placeBuilt(t, g, k, construction.South, 5, 2) // New is a sandbox that knows everything
	if !researched {
		g.research = research.Progress{}
	}
	g.SpawnStack(in, qty, 1, 4)
	g.Enqueue(command.Command{Kind: command.AddBill, X: 5, Y: 2, Recipe: uint8(r), Mode: uint8(recipe.DoCount), Count: 1})
	g.Step(1)
	for i := 0; i < 30; i++ {
		g.SetNeeds(id, needs.Full, needs.Full)
		g.Step(TicksPerCell * 10)
	}
	return g
}

func TestBrewBeerAtBrewingBench(t *testing.T) {
	g := brewBench(t, true, construction.BrewingBench, recipe.BrewBeer, item.Brewgrain, 6)
	if n := g.stockOf(item.Beer); n != 3 {
		t.Fatalf("beer = %d, want 3", n)
	}
	if n := g.stockOf(item.Brewgrain); n != 0 {
		t.Fatalf("brewgrain left = %d, want 0", n)
	}
}

func TestBrewTeaAtDrugTable(t *testing.T) {
	g := brewBench(t, true, construction.DrugTable, recipe.BrewTea, item.AshleafLeaf, 4)
	if n := g.stockOf(item.AshleafTea); n != 2 {
		t.Fatalf("tea = %d, want 2", n)
	}
}

func TestPressTabletAtDrugTable(t *testing.T) {
	g := brewBench(t, true, construction.DrugTable, recipe.PressTablet, item.AshleafLeaf, 6)
	if n := g.stockOf(item.SparkTablet); n != 1 {
		t.Fatalf("tablets = %d, want 1", n)
	}
}

// Without Herbal remedies the tablet bill is refused (the table is built
// before the research is forgotten: it is gated too).
func TestPressTabletNeedsResearch(t *testing.T) {
	g := brewBench(t, false, construction.DrugTable, recipe.PressTablet, item.AshleafLeaf, 6)
	if n := g.stockOf(item.SparkTablet); n != 0 {
		t.Fatalf("tablets = %d without research, want 0", n)
	}
	if n := len(g.World().Buildings[0].Bills); n != 0 {
		t.Fatalf("bills = %d, want the locked recipe refused", n)
	}
}
