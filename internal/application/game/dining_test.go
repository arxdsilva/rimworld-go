package game

import (
	"testing"

	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/recipe"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/sim/clock"
)

func TestColonistCarriesMealToTable(t *testing.T) {
	g := New(Config{Seed: 1, Width: 20, Height: 10})
	id, _ := g.SpawnNamedPawn(2, 5, "Ada")
	g.SpawnStack(item.Meal, 5, 3, 5)
	placeBuilt(t, g, construction.Table, construction.South, 12, 5)
	placeBuilt(t, g, construction.Chair, construction.South, 12, 6)
	g.SetNeeds(id, needs.CriticalHunger-1, needs.Full)
	ate := false
	for i := 0; i < 400 && !ate; i++ {
		g.Step(1)
		for _, th := range pawn(t, g, id).Thoughts {
			if th.Label == "ate at a table" {
				ate = true
			}
		}
	}
	if !ate {
		p := pawn(t, g, id)
		t.Fatalf("at (%d,%d) job %s thoughts %+v: should have eaten at the table", p.X, p.Y, p.Job, p.Thoughts)
	}
	p := pawn(t, g, id)
	if p.X != 12 || p.Y != 6 {
		t.Fatalf("ate at (%d,%d), want the chair at (12,6)", p.X, p.Y)
	}
	want := 5 - portionsFor(needs.CriticalHunger-1, item.Stack{Kind: item.Meal})
	if st, ok := stackOf(g, item.Meal); !ok || st.Qty != want || st.X != 3 {
		t.Fatalf("only the portions eaten should be taken (%d left where they were): %+v", want, st)
	}
}

func TestCampfireCooksWithFuelOnly(t *testing.T) {
	g := New(Config{Seed: 1, Width: 16, Height: 10})
	id, _ := g.SpawnNamedPawn(2, 2, "Ada")
	g.SetNeeds(id, needs.Full, needs.Full)
	if !g.BuildNow("campfire", "wood", "south", 8, 4) {
		t.Fatal("campfire")
	}
	bid := g.layerBldgAt(8, 4, construction.LayerObject)
	g.craft.Bills.Set(bid, []recipe.Bill{{Recipe: recipe.CampfireMeal, Mode: recipe.Forever, Target: 1}})
	g.SpawnStack(item.Potatoes, 10, 3, 6)
	g.Step(clock.TicksPerHour / 2)
	if _, ok := stackOf(g, item.Meal); ok {
		t.Fatal("an unlit campfire cooked a meal")
	}
	if bv := g.craft.Bills.AppendViews(nil, bid); bv[0].Status != BillNoFuel {
		t.Fatalf("bill status %q", bv[0].Status)
	}
	g.FuelCampfire(8, 4)
	for i := 0; i < 20; i++ {
		g.Step(100)
		g.SetNeeds(id, needs.Full, needs.Full)
		if _, ok := stackOf(g, item.Meal); ok {
			return
		}
	}
	t.Fatal("a lit campfire should cook a simple meal")
}
