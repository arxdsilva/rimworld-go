package game

import (
	"testing"

	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/recipe"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/sim/command"
)

// placeBuilt drops a completed building (fixture shortcut).
func placeBuilt(t *testing.T, g *Game, k construction.Kind, f construction.Facing, x, y int) {
	t.Helper()
	g.Enqueue(command.Command{Kind: command.DesignateBuild, Build: uint8(k), Facing: uint8(f), X: x, Y: y})
	g.Step(1)
	if k.Instant() {
		return // placed on designation
	}
	fid := g.layerFrameAt(x, y, construction.DefOf(k).Layer)
	fr, ok := g.frames.Get(fid)
	if !ok {
		t.Fatalf("no %s frame at %d,%d", k, x, y)
	}
	g.completeFrame(fid, fr)
}

func TestMultiTilePlacementAndBlocking(t *testing.T) {
	g := New(Config{Seed: 1, Width: 10, Height: 6})
	if ok, r := g.CanPlaceAt("workbench", "wood", "south", 8, 2); !ok {
		t.Fatalf("bench at 8,2: %q", r)
	}
	if ok, r := g.CanPlaceAt("workbench", "wood", "south", 9, 2); ok || r != ReasonOutside {
		t.Fatalf("bench hanging off the map: %v %q", ok, r)
	}
	if ok, r := g.CanPlaceAt("workbench", "wood", "south", 4, 5); ok || r != ReasonSpot {
		t.Fatalf("work spot off the map: %v %q", ok, r)
	}
	placeBuilt(t, g, construction.Workbench, construction.South, 3, 2)
	if g.grid.Walkable(3, 2) || g.grid.Walkable(4, 2) || !g.grid.Walkable(3, 3) {
		t.Fatal("bench blocks its two cells, work spot stays open")
	}
	if ok, r := g.CanPlaceAt("wall", "wood", "south", 3, 3); ok || r != ReasonSpot {
		t.Fatalf("wall on the work spot: %v %q", ok, r)
	}
	b := g.World().Buildings[0]
	if b.W != 2 || b.H != 1 || !b.HasSpot || b.SpotX != 3 || b.SpotY != 3 || !b.Bench {
		t.Fatalf("bench view = %+v", b)
	}
	placeBuilt(t, g, construction.Bed, construction.East, 6, 4)
	if g.layerBldgAt(7, 4, construction.LayerObject) == 0 {
		t.Fatal("east bed covers two cells horizontally")
	}
}

func TestStoveCooksMealsFromRawFood(t *testing.T) {
	g := New(Config{Seed: 1, Width: 12, Height: 6})
	id, _ := g.SpawnNamedPawn(1, 1, "Ada")
	placeBuilt(t, g, construction.Stove, construction.South, 5, 2)
	g.FuelCampfire(5, 2)                 // FOOD-4: the stove burns wood
	g.SpawnStack(item.Berries, 30, 1, 4) // FOOD-1: 10 raw units a meal
	g.Enqueue(command.Command{Kind: command.AddBill, X: 5, Y: 2, Recipe: uint8(recipe.SimpleMeal), Mode: uint8(recipe.DoCount), Count: 2})
	g.Step(1)
	if bl := g.World().Buildings[0].Bills; len(bl) != 1 || bl[0].Label != "Cook simple meal" {
		t.Fatalf("bills = %+v", bl)
	}
	for i := 0; i < 20; i++ {
		g.SetNeeds(id, needs.Full, needs.Full)
		g.Step(TicksPerCell * 10)
	}
	w := g.World()
	if w.FoodByKind["meal"] != 2 || w.FoodByKind["berries"] != 10 {
		t.Fatalf("food after cooking = %v", w.FoodByKind)
	}
	if bl := w.Buildings[0].Bills[0]; bl.Done != 2 || bl.Status != BillDone {
		t.Fatalf("bill = %+v", bl)
	}
}

func TestBillModesSuspendAndRemove(t *testing.T) {
	g := New(Config{Seed: 1, Width: 12, Height: 6})
	id, _ := g.SpawnNamedPawn(1, 1, "Bo")
	placeBuilt(t, g, construction.Workbench, construction.South, 5, 2)
	g.SpawnStack(item.Wood, 20, 1, 4)
	g.Enqueue(command.Command{Kind: command.AddBill, X: 6, Y: 2, Recipe: uint8(recipe.Club), Mode: uint8(recipe.UntilHave), Count: 1})
	g.Enqueue(command.Command{Kind: command.AddBill, X: 5, Y: 2, Recipe: uint8(recipe.SimpleMeal)}) // wrong bench: ignored
	g.Step(1)
	if n := len(g.World().Buildings[0].Bills); n != 1 {
		t.Fatalf("bills = %d", n)
	}
	for i := 0; i < 30; i++ {
		g.SetNeeds(id, needs.Full, needs.Full)
		g.Step(TicksPerCell * 10)
	}
	if g.stockOf(item.Club) != 1 {
		t.Fatalf("clubs = %d, want exactly 1 (until you have 1)", g.stockOf(item.Club))
	}
	if st := g.World().Buildings[0].Bills[0].Status; st != BillHaveEnough {
		t.Fatalf("status = %q", st)
	}
	g.Enqueue(command.Command{Kind: command.SetBill, X: 5, Y: 2, Index: 0, Mode: uint8(recipe.Forever), Enabled: true})
	g.Step(1)
	if bl := g.World().Buildings[0].Bills[0]; !bl.Suspended || bl.Mode != "forever" || bl.Status != BillSuspended {
		t.Fatalf("set bill = %+v", bl)
	}
	g.Enqueue(command.Command{Kind: command.RemoveBill, X: 5, Y: 2, Index: 0})
	g.Step(1)
	if n := len(g.World().Buildings[0].Bills); n != 0 {
		t.Fatalf("bills after remove = %d", n)
	}
}

func TestBillWaitsForIngredients(t *testing.T) {
	g := New(Config{Seed: 1, Width: 12, Height: 6})
	placeBuilt(t, g, construction.Stove, construction.South, 5, 2)
	g.FuelCampfire(5, 2)                // FOOD-4: the stove burns wood
	g.SpawnStack(item.Berries, 1, 1, 4) // one short
	g.Enqueue(command.Command{Kind: command.AddBill, X: 5, Y: 2, Recipe: uint8(recipe.SimpleMeal), Mode: uint8(recipe.Forever)})
	g.Step(1)
	if st := g.World().Buildings[0].Bills[0].Status; st != BillNoInput {
		t.Fatalf("status = %q", st)
	}
	if len(Recipes()) < 2 {
		t.Fatal("recipes listed")
	}
}

func TestBillsDontOvershootWithTwoCooks(t *testing.T) {
	g := New(Config{Seed: 1, Width: 12, Height: 6})
	a, _ := g.SpawnNamedPawn(1, 1, "Ada")
	b, _ := g.SpawnNamedPawn(2, 1, "Bo")
	placeBuilt(t, g, construction.Stove, construction.South, 5, 2)
	g.FuelCampfire(5, 2) // FOOD-4: the stove burns wood
	placeBuilt(t, g, construction.Stove, construction.South, 8, 2)
	g.FuelCampfire(8, 2) // FOOD-4: the stove burns wood
	g.SpawnStack(item.Meal, 9, 1, 4)
	g.SpawnStack(item.Berries, 10, 1, 3) // RES-1: two cooks may share a stack (count claims); two stacks only shorten the walks
	g.SpawnStack(item.Berries, 10, 2, 3)
	g.Enqueue(command.Command{Kind: command.AddBill, X: 5, Y: 2, Recipe: uint8(recipe.SimpleMeal), Mode: uint8(recipe.UntilHave), Count: 10})
	g.Enqueue(command.Command{Kind: command.AddBill, X: 8, Y: 2, Recipe: uint8(recipe.SimpleMeal), Mode: uint8(recipe.UntilHave), Count: 10})
	g.Step(1)
	for i := 0; i < 400; i++ {
		g.SetNeeds(a, needs.Full, needs.Full)
		g.SetNeeds(b, needs.Full, needs.Full)
		g.Step(TicksPerCell)
		if n := g.stockOf(item.Meal); n > 10 {
			t.Fatalf("meals = %d, want at most 10", n)
		}
	}
	if n := g.stockOf(item.Meal); n < 10 {
		t.Fatalf("meals = %d, want the target reached", n)
	}
}
