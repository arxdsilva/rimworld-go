package game

import (
	"testing"

	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/plant"
)

func treeZoneGame(t *testing.T) (*Game, entity.EntityID) {
	t.Helper()
	g := New(Config{Seed: 1, Width: 12, Height: 6})
	id, _ := g.SpawnNamedPawn(1, 1, "Ada")
	g.SetNeeds(id, needs.Full, needs.Full)
	g.Enqueue(command.Command{Kind: command.DesignateGrowZone, X: 4, Y: 2, X2: 5, Y2: 3, Build: uint8(plant.Tree)})
	return g, id
}

func treesAt(g *Game) (n int, zoned int) {
	for _, pid := range g.plantIDs {
		if p, ok := g.plants.Get(pid); ok && p.Species == plant.Tree && p.X >= 4 && p.X <= 5 && p.Y >= 2 && p.Y <= 3 {
			n++
			if p.Zone {
				zoned++
			}
		}
	}
	return
}

func TestTreeZoneAccepted(t *testing.T) {
	g, _ := treeZoneGame(t)
	g.Step(1)
	w := g.World()
	if gz := zonesOf(w, ZoneGrow); len(gz) != 1 || gz[0].Crop != "tree" {
		t.Fatalf("zones = %+v", w.Zones)
	}
	if id, ok := ZoneSpeciesID("tree"); !ok || plant.Species(id) != plant.Tree {
		t.Fatalf("ZoneSpeciesID(tree) = %d %v", id, ok)
	}
	if _, ok := ZoneSpeciesID("berry_bush"); ok {
		t.Fatal("berry bush is not zone sowable")
	}
	if !plant.DefOf(plant.Tree).ZoneSowable() || plant.DefOf(plant.Tree).Crop {
		t.Fatal("tree must be zone sowable but not a crop")
	}
}

func TestGrowersSowSaplings(t *testing.T) {
	g, _ := treeZoneGame(t)
	g.Step(TicksPerCell*40 + 4*plant.DefOf(plant.Tree).SowWork)
	if n, z := treesAt(g); n != 4 || z != 4 {
		t.Fatalf("saplings = %d (zone %d), want 4", n, z)
	}
}

// ripeZoneTree plants a mature zone tree at (4,2) with no other zone cells
// needing work, and returns its id.
func ripeZoneTree(t *testing.T) (*Game, entity.EntityID) {
	t.Helper()
	g, _ := treeZoneGame(t)
	g.Step(1)
	pid, err := g.spawnPlant(plant.Tree, 4, 2, plant.DefOf(plant.Tree).GrowTicks)
	if err != nil {
		// a wild tree occupied the cell: clear it first
		g.removePlant(g.plantAt(4, 2))
		pid, _ = g.spawnPlant(plant.Tree, 4, 2, plant.DefOf(plant.Tree).GrowTicks)
	}
	p, _ := g.plants.Get(pid)
	p.Zone = true
	g.plants.Set(pid, p)
	return g, pid
}

func TestMatureZoneTreeChoppedForWood(t *testing.T) {
	g, pid := ripeZoneTree(t)
	before := g.World().Wood
	g.Step(TicksPerCell*40 + plant.DefOf(plant.Tree).CutWork + 60)
	if _, ok := g.plants.Get(pid); ok {
		t.Fatal("mature zone tree was not chopped")
	}
	if got := g.World().Wood - before; got < plant.DefOf(plant.Tree).Wood {
		t.Fatalf("wood gained = %d, want >= %d", got, plant.DefOf(plant.Tree).Wood)
	}
	_ = item.Wood
}

func TestZoneTreeResownAfterChop(t *testing.T) {
	g, pid := ripeZoneTree(t)
	g.Step(TicksPerCell*60 + plant.DefOf(plant.Tree).CutWork + 4*plant.DefOf(plant.Tree).SowWork + 200)
	if _, ok := g.plants.Get(pid); ok {
		t.Fatal("original tree still standing")
	}
	p, ok := func() (plant.Plant, bool) {
		id := g.plantAt(4, 2)
		if id == 0 {
			return plant.Plant{}, false
		}
		return g.plants.Get(id)
	}()
	if !ok || p.Species != plant.Tree || !p.Zone || p.Growth >= plant.DefOf(plant.Tree).GrowTicks {
		t.Fatalf("cell not resown with a sapling: %+v ok=%v", p, ok)
	}
}

func TestWildTreeNotAutoChopped(t *testing.T) {
	g := New(Config{Seed: 1, Width: 12, Height: 6})
	g.SpawnNamedPawn(1, 1, "Ada")
	if id := g.plantAt(6, 4); id != 0 {
		g.removePlant(id)
	}
	pid, _ := g.spawnPlant(plant.Tree, 6, 4, plant.DefOf(plant.Tree).GrowTicks)
	g.Step(TicksPerCell*40 + 300)
	if _, ok := g.plants.Get(pid); !ok {
		t.Fatal("wild undesignated tree was chopped")
	}
}

func TestTreesNeverFrostKilled(t *testing.T) {
	if plant.Frostbitten(plant.Tree, -500) {
		t.Fatal("trees must not be frost-killed")
	}
	g, pid := ripeZoneTree(t)
	g.frostCrops()
	if _, ok := g.plants.Get(pid); !ok {
		t.Fatal("frostCrops removed a tree")
	}
	for _, pv := range g.World().Plants {
		if pv.ID == pid && pv.Wilted {
			t.Fatal("tree reported wilted")
		}
	}
}
