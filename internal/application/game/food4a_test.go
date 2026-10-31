package game

import (
	"slices"
	"testing"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/ai/work"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/recipe"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// FOOD-4a: cooking and preservation in the whole game.

// TestHaulerFillsHopper: a pawn doing only hauling fills a hopper with a
// FillHopper job (wiki: Hopper: hauling and cooking work both).
func TestHaulerFillsHopper(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 30, Height: 16})
	buildNow(t, g, construction.Hopper, item.Steel, construction.South, 4, 4)
	spawn(t, g, item.Rice, 40, 12, 4)
	ada, _ := g.SpawnNamedPawn(10, 10, "Ada")
	onlyWork(g, ada, work.Haul)
	filling := false
	for i := 0; i < 200 && g.itemAt(4, 4) == 0; i++ {
		runFed(g, 1, ada)
		if j, _ := g.jobs.Get(ada); j.Kind == job.FillHopper {
			filling = true
			if a := pawnView(g, ada).Action; j.Carry != 0 && a != ActionHaul {
				t.Fatalf("a hopper fill shows %q", a)
			}
		}
	}
	if st, _ := g.stacks.Get(g.itemAt(4, 4)); !filling || st.Kind != item.Rice || st.Qty != 40 {
		t.Fatalf("FillHopper seen %v, hopper %+v", filling, st)
	}
}

// TestLegacyRationLoadsAsSurvivalMeal: an older save's rations, on the
// ground and carried, load as packaged survival meals, and a second load
// changes nothing.
func TestLegacyRationLoadsAsSurvivalMeal(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 12, Height: 6})
	id, _ := g.SpawnNamedPawn(1, 1, "Ada")
	ground, _ := g.SpawnStack(item.Food, 7, 4, 4)
	held, _ := g.SpawnStack(item.Food, 2, 1, 1)
	st, _ := g.stacks.Get(held)
	st.Carrier = id
	g.putStack(held, st)
	l := loadAsVersion(t, g, SaveVersion-1)
	for _, sid := range []entity.EntityID{ground, held} {
		if st, ok := l.stacks.Get(sid); !ok || st.Kind != item.SurvivalMeal {
			t.Fatalf("stack %d loaded as %+v", sid, st)
		}
	}
	if st, _ := l.stacks.Get(held); st.Carrier != id || st.Qty != 2 {
		t.Fatalf("the carried ration %+v", st)
	}
	again := saveLoad(t, l)
	if again.StateHash() != l.StateHash() {
		t.Fatal("a second load changed the game")
	}
}

// TestCookedMealShowsItsIngredients: a fueled stove cooks a meal from two
// raw foods; ItemView.Ingredients names both, the meat by its species.
func TestCookedMealShowsItsIngredients(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 12, Height: 6})
	id, _ := g.SpawnNamedPawn(1, 1, "Ada")
	placeBuilt(t, g, construction.Stove, construction.South, 5, 2)
	g.FuelCampfire(5, 2) // FOOD-4: the stove burns wood
	g.SpawnStack(item.Rice, 4, 1, 4)
	meat, _ := g.SpawnStack(item.Meat, 6, 2, 4) // neither stack is a meal alone
	st, _ := g.stacks.Get(meat)
	st.Animal = 1 // tagged: species 0
	g.putStack(meat, st)
	g.Enqueue(command.Command{Kind: command.AddBill, X: 5, Y: 2, Recipe: uint8(recipe.SimpleMeal), Mode: uint8(recipe.DoCount), Count: 1})
	for i := 0; i < 30 && g.stockOf(item.Meal) == 0; i++ {
		g.SetNeeds(id, needs.Full, needs.Full)
		g.Step(TicksPerCell * 10)
	}
	for _, iv := range g.World().Items {
		if iv.Kind == item.Meal.String() {
			if !slices.Contains(iv.Ingredients, "rice") || len(iv.Ingredients) != 2 || iv.MaxHP != 50 {
				t.Fatalf("meal view %+v", iv)
			}
			return
		}
	}
	t.Fatal("no meal cooked")
}

// TestUnfueledStoveWaits: a stove with no wood cooks nothing (wiki:
// Fueled stove); fuelled, it cooks.
func TestUnfueledStoveWaits(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 12, Height: 6})
	id, _ := g.SpawnNamedPawn(1, 1, "Ada")
	onlyWork(g, id, work.Cook)
	placeBuilt(t, g, construction.Stove, construction.South, 5, 2)
	g.SpawnStack(item.Rice, 20, 1, 4)
	g.Enqueue(command.Command{Kind: command.AddBill, X: 5, Y: 2, Recipe: uint8(recipe.SimpleMeal), Mode: uint8(recipe.DoCount), Count: 1})
	cook := func() {
		for i := 0; i < 20 && g.stockOf(item.Meal) == 0; i++ {
			g.SetNeeds(id, needs.Full, needs.Full)
			g.Step(TicksPerCell * 10)
		}
	}
	cook()
	if g.stockOf(item.Meal) != 0 || g.World().Buildings[0].Bills[0].Status != BillNoFuel {
		t.Fatalf("a cold stove cooked: meals %d status %q", g.stockOf(item.Meal), g.World().Buildings[0].Bills[0].Status)
	}
	g.FuelCampfire(5, 2)
	cook()
	if g.stockOf(item.Meal) != 1 {
		t.Fatal("the fuelled stove cooked nothing")
	}
}
