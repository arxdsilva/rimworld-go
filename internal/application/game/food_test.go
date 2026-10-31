package game

import (
	"testing"

	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/world/plant"
)

func TestGrowZoneSowsGrowsAndHarvests(t *testing.T) {
	g := New(Config{Seed: 1, Width: 12, Height: 6})
	id, _ := g.SpawnNamedPawn(1, 1, "Ada")
	g.SetNeeds(id, needs.Full, needs.Full)
	crop, ok := CropID("radish")
	if !ok {
		t.Fatal("radish is a crop")
	}
	if _, ok := CropID("tree"); ok {
		t.Fatal("trees are not crops")
	}
	g.Enqueue(command.Command{Kind: command.DesignateGrowZone, X: 4, Y: 2, X2: 5, Y2: 3, Build: crop})
	g.Step(TicksPerCell*40 + 4*plant.DefOf(plant.Radish).SowWork)
	w := g.World()
	if gz := zonesOf(w, ZoneGrow); len(gz) != 1 || gz[0].Crop != "radish" {
		t.Fatalf("zones = %+v", w.Zones)
	}
	sown := 0
	for _, p := range w.Plants {
		if p.Species == "radish" {
			sown++
		}
	}
	if sown != 4 {
		t.Fatalf("sown = %d, want 4", sown)
	}
	// let them ripen (daylight only, about two days) and be harvested,
	// keeping the pawn fed and rested
	for i := 0; i < 40; i++ {
		g.SetNeeds(id, needs.Full, needs.Full)
		g.Step(plant.DefOf(plant.Radish).GrowTicks / 10)
	}
	if got := g.World().FoodByKind["radishes"]; got < plant.DefOf(plant.Radish).Yield {
		t.Fatalf("radishes harvested = %d", got)
	}
}

func TestBerryHarvestYieldsBerries(t *testing.T) {
	g := New(Config{Seed: 1, Width: 10, Height: 5})
	g.SpawnNamedPawn(1, 2, "Bo")
	g.SpawnPlant(5, 2)
	g.Enqueue(command.Command{Kind: command.DesignateHarvest, X: 5, Y: 2, X2: 5, Y2: 2})
	g.Step(TicksPerCell*20 + 60)
	if g.World().FoodByKind["berries"] != plant.DefOf(plant.BerryBush).Yield {
		t.Fatalf("food = %v", g.World().FoodByKind)
	}
}

func TestPawnsPreferMealsAndNutritionDiffers(t *testing.T) {
	g := New(Config{Seed: 1, Width: 12, Height: 5})
	id, _ := g.SpawnNamedPawn(1, 2, "Cora")
	g.SpawnStack(item.Berries, 3, 2, 2) // closer
	g.SpawnStack(item.Meal, 1, 9, 2)    // further but preferred
	g.SetNeeds(id, needs.CriticalHunger-1, needs.Full)
	g.Step(TicksPerCell*30 + 2*needs.EatWorkTicks + 5) // ravenous: the berries on the way, then the meal
	p := pawn(t, g, id)
	if p.LastMeal != "meal" {
		t.Fatalf("ate %q, want meal", p.LastMeal)
	}
	if p.Hunger < min(100, needs.Percent(needs.CriticalHunger-1+item.Meal.Nutrition()))-2 {
		t.Fatalf("hunger after a meal = %d", p.Hunger)
	}
}

func TestCancelRemovesGrowZone(t *testing.T) {
	g := New(Config{Seed: 1, Width: 10, Height: 5})
	g.Enqueue(command.Command{Kind: command.DesignateGrowZone, X: 2, Y: 1, X2: 4, Y2: 3})
	g.Step(1)
	g.Enqueue(command.Command{Kind: command.DesignateCancel, X: 3, Y: 2, X2: 3, Y2: 2})
	g.Step(1)
	if w := g.World(); len(w.Zones) != 1 || len(w.Zones[0].Cells) != 8 {
		t.Fatalf("cancelling one tile shrinks the zone: %+v", w.Zones)
	}
	g.Enqueue(command.Command{Kind: command.DesignateCancel, X: 2, Y: 1, X2: 4, Y2: 3})
	g.Step(1)
	if len(g.World().Zones) != 0 {
		t.Fatal("cancel should remove the zone")
	}
}

func TestOrdersAtOffersSowOnEmptyZoneCell(t *testing.T) {
	g := New(Config{Seed: 1, Width: 10, Height: 5})
	id, _ := g.SpawnNamedPawn(1, 2, "Ada")
	g.Enqueue(command.Command{Kind: command.DesignateGrowZone, X: 5, Y: 1, X2: 6, Y2: 2})
	g.Step(1)
	opts := g.OrdersAt(id, 5, 1)
	found := false
	for _, o := range opts {
		if o.Label == "Sow potato" && !o.Disabled {
			found = true
		}
	}
	if !found {
		t.Fatalf("opts = %+v", opts)
	}
}
