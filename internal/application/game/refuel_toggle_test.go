package game

import (
	"testing"

	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// REF-1/13: the Auto refuel switch and the hostile skip.

func setAuto(g *Game, id entity.EntityID, on bool) {
	applyNow(g, command.Command{Kind: command.SetAutoRefuel, Target: uint64(id), Enabled: on})
}

func refuelView(g *Game, id entity.EntityID) BuildingView {
	for _, v := range g.buildingViews() {
		if v.ID == id {
			return v
		}
	}
	return BuildingView{}
}

func refuelFixture(t *testing.T) (*Game, entity.EntityID) {
	t.Helper()
	g := fuelGame(t)
	pid, _ := g.SpawnNamedPawn(2, 2, "Ada")
	g.SetNeeds(pid, needs.Full, needs.Full)
	gid := placeFuel(t, g, construction.WoodGenerator, 8, 3)
	if _, err := g.SpawnStack(item.Wood, 90, 2, 5); err != nil {
		t.Fatal(err)
	}
	return g, gid
}

func TestAutoRefuelOffSkipsHaulers(t *testing.T) {
	t.Parallel()
	g, gid := refuelFixture(t)
	setAuto(g, gid, false)
	g.Step(5000)
	if g.fuel[gid] != 0 {
		t.Fatalf("fuel = %d with Auto refuel off, want 0", g.fuel[gid])
	}
	setAuto(g, gid, true)
	g.Step(5000)
	if g.fuel[gid] == 0 {
		t.Fatal("switching Auto refuel back on refuels the generator")
	}
}

func TestAutoRefuelToggleRejectsNonBurners(t *testing.T) {
	t.Parallel()
	g := fuelGame(t)
	bed := placeFuel(t, g, construction.Bed, 3, 3)
	tank := placeFuel(t, g, construction.FishTank, 10, 4)
	hot := g.placeGenerated(construction.Building{Kind: construction.WoodGenerator, Stuff: item.Wood, X: 14, Y: 8, Hostile: true})
	for _, id := range []entity.EntityID{bed, tank, hot, 9999} {
		setAuto(g, id, false)
		if b, ok := g.bldgs.Get(id); ok && b.NoAutoRefuel {
			t.Fatalf("building %d took the switch", id)
		}
	}
}

func TestAutoRefuelSurvivesSave(t *testing.T) {
	t.Parallel()
	g, gid := refuelFixture(t)
	setAuto(g, gid, false)
	h := loadAsVersion(t, g, SaveVersion)
	if b, _ := h.bldgs.Get(gid); !b.NoAutoRefuel {
		t.Fatal("NoAutoRefuel lost over a save")
	}
}

func TestHaulersSkipHostileBurners(t *testing.T) {
	t.Parallel()
	g := fuelGame(t)
	pid, _ := g.SpawnNamedPawn(2, 2, "Ada")
	g.SetNeeds(pid, needs.Full, needs.Full)
	hot := g.placeGenerated(construction.Building{Kind: construction.WoodGenerator, Stuff: item.Wood, X: 8, Y: 3, Hostile: true})
	if hot == 0 {
		t.Fatal("hostile generator not placed")
	}
	if _, err := g.SpawnStack(item.Wood, 90, 2, 5); err != nil {
		t.Fatal(err)
	}
	g.Step(5000)
	if g.fuel[hot] != 0 {
		t.Fatalf("hostile burner fuel = %d, want 0", g.fuel[hot])
	}
}

func TestBuildingViewRefuelFields(t *testing.T) {
	t.Parallel()
	g, gid := refuelFixture(t)
	if v := refuelView(g, gid); !v.Refuelable || !v.AutoRefuel {
		t.Fatalf("fresh generator view: %+v", v)
	}
	setAuto(g, gid, false)
	if v := refuelView(g, gid); !v.Refuelable || v.AutoRefuel {
		t.Fatalf("after switching off: Refuelable=%v AutoRefuel=%v", v.Refuelable, v.AutoRefuel)
	}
	tank := placeFuel(t, g, construction.FishTank, 10, 4)
	if v := refuelView(g, tank); v.Refuelable {
		t.Fatal("a fish tank is not refuelable by order")
	}
}
