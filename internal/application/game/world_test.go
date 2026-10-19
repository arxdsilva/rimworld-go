package game

import (
	"strings"
	"testing"

	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/world/plant"
	"rimworld-go/internal/domain/world/terrain"
)

func TestNewColonyIsGeneratedAroundAnOpenStart(t *testing.T) {
	g := NewColony(Config{Seed: 3, Width: 48, Height: 40})
	w := g.World()
	if w.TerrainW != 48 || len(w.Terrain) != 48*40 {
		t.Fatalf("terrain view %d x %d", w.TerrainW, len(w.Terrain))
	}
	counts := map[string]int{}
	for _, c := range w.Terrain {
		counts[TerrainNames()[c]]++
	}
	if counts["rock"] == 0 || counts["soil"] == 0 {
		t.Fatalf("terrain = %v", counts)
	}
	trees := 0
	for _, p := range w.Plants {
		if p.Species == "tree" {
			trees++
		}
	}
	if trees < 10 {
		t.Fatalf("trees = %d", trees)
	}
	for _, p := range g.Snapshot() {
		if !g.grid.Walkable(p.X, p.Y) {
			t.Fatalf("%s spawned on a blocked tile", p.Name)
		}
	}
	// same seed, same world
	h := NewColony(Config{Seed: 3, Width: 48, Height: 40})
	if len(h.World().Plants) != len(w.Plants) {
		t.Fatal("generation must be deterministic")
	}
}

func TestChopTreeDropsWood(t *testing.T) {
	g := New(Config{Seed: 1, Width: 10, Height: 5})
	g.SpawnNamedPawn(1, 2, "Ada")
	g.SpawnPlantSpecies("tree", 6, 2, plant.DefOf(plant.Tree).GrowTicks)
	g.Enqueue(command.Command{Kind: command.DesignateCut, X: 0, Y: 0, X2: 9, Y2: 4})
	g.Step(TicksPerCell*20 + plant.DefOf(plant.Tree).CutWork + 10)
	if len(g.World().Plants) != 0 {
		t.Fatal("tree should be cut down")
	}
	if g.World().Wood != plant.DefOf(plant.Tree).Wood {
		t.Fatalf("wood = %d", g.World().Wood)
	}
}

func TestMineRockDropsStoneAndOpensTile(t *testing.T) {
	g := New(Config{Seed: 1, Width: 10, Height: 5})
	g.SpawnNamedPawn(1, 2, "Bo")
	for y := 0; y < 5; y++ {
		g.setTerrain(6, y, terrain.Rock)
	}
	if g.grid.Walkable(6, 2) {
		t.Fatal("rock must block")
	}
	if ok, r := g.CanPlace("wall", 6, 2); ok || r != ReasonRock {
		t.Fatalf("build on rock: %v %q", ok, r)
	}
	g.Enqueue(command.Command{Kind: command.DesignateMine, X: 6, Y: 2, X2: 6, Y2: 2})
	g.Step(TicksPerCell*20 + MineWork + 10)
	if g.TerrainAt(6, 2) != "stone" || !g.grid.Walkable(6, 2) {
		t.Fatalf("mined tile = %s walkable=%v", g.TerrainAt(6, 2), g.grid.Walkable(6, 2))
	}
	if g.World().Stone != MineYield || len(g.World().Mines) != 0 {
		t.Fatalf("stone=%d mines=%v", g.World().Stone, g.World().Mines)
	}
	if g.TerrainAt(6, 1) != "rock" {
		t.Fatal("only the designated tile is mined")
	}
}

func TestStoneWallUsesStoneBlocks(t *testing.T) {
	g := New(Config{Seed: 1, Width: 10, Height: 5})
	g.SpawnNamedPawn(1, 2, "Cora")
	g.SpawnStack(item.Stone, 5, 2, 3)
	g.SpawnStack(item.Wood, 5, 2, 1)
	if ok, r := g.CanPlaceStuff("table", "stone", 5, 2); ok || r != ReasonStuff {
		t.Fatalf("stone table: %v %q", ok, r)
	}
	g.Enqueue(command.Command{Kind: command.DesignateBuild, Build: uint8(construction.Wall), Stuff: uint8(item.Stone), X: 6, Y: 2})
	g.Step(TicksPerCell*60 + construction.WorkFor(construction.Wall, item.Stone) + 20)
	b, ok := building(g, 6, 2, "wall")
	if !ok || b.Stuff != "stone" {
		t.Fatalf("stone wall = %+v %v", b, ok)
	}
	if g.World().Wood != 5 || g.World().Stone != 3 {
		t.Fatalf("wood=%d stone=%d", g.World().Wood, g.World().Stone)
	}
}

func TestBerryBushRegrowsAfterHarvest(t *testing.T) {
	g := New(Config{Seed: 1, Width: 10, Height: 5})
	g.SpawnNamedPawn(1, 2, "Ada")
	g.SpawnPlant(5, 2)
	g.Enqueue(command.Command{Kind: command.DesignateHarvest, X: 5, Y: 2, X2: 5, Y2: 2})
	g.Step(TicksPerCell*20 + 60)
	w := g.World()
	regrow := plant.DefOf(plant.BerryBush).RegrowTo
	if len(w.Plants) != 1 || w.Plants[0].Harvestable || w.Plants[0].Growth < regrow || w.Plants[0].Growth > regrow+5 {
		t.Fatalf("bush after harvest = %+v", w.Plants)
	}
	if w.Food != plant.DefOf(plant.BerryBush).Yield {
		t.Fatalf("food = %d", w.Food)
	}
	g.Step(plant.DefOf(plant.BerryBush).GrowTicks)
	if !g.World().Plants[0].Harvestable {
		t.Fatal("bush should ripen again")
	}
}

func TestTreeBlocksBuildingButBushIsCleared(t *testing.T) {
	g := New(Config{Seed: 1, Width: 10, Height: 5})
	g.SpawnPlantSpecies("tree", 3, 3, 100)
	if ok, r := g.CanPlace("bed", 3, 3); ok || r != ReasonTree {
		t.Fatalf("bed on tree: %v %q", ok, r)
	}
	g.SpawnPlant(5, 3)
	if ok, _ := g.CanPlace("floor", 5, 3); !ok {
		t.Fatal("bushes do not block planning")
	}
}

func TestOrdersAtOffersChopAndMine(t *testing.T) {
	g := New(Config{Seed: 1, Width: 10, Height: 5})
	id, _ := g.SpawnNamedPawn(1, 2, "Ada")
	g.SpawnPlantSpecies("tree", 4, 2, 500)
	g.setTerrain(7, 2, terrain.Rock)
	labels := func(x, y int) string {
		var out []string
		for _, o := range g.OrdersAt(id, x, y) {
			out = append(out, o.Label+"|"+o.Reason)
		}
		return strings.Join(out, ";")
	}
	if l := labels(4, 2); !strings.Contains(l, "Chop tree|") {
		t.Fatalf("tree menu = %s", l)
	}
	l := labels(7, 2)
	if !strings.Contains(l, "Go here|rock in the way") || !strings.Contains(l, "Mine rock|;") && !strings.HasSuffix(l, "Mine rock|") {
		t.Fatalf("rock menu = %s", l)
	}
	g.Enqueue(command.Command{Kind: command.Prioritize, Pawn: uint64(id), Order: command.OrderMine, X: 7, Y: 2})
	g.Step(2)
	if p := pawn(t, g, id); !strings.HasPrefix(p.Job, "mine") {
		t.Fatalf("prioritized mine, job = %s", p.Job)
	}
}

func TestCancelClearsCutAndMine(t *testing.T) {
	g := New(Config{Seed: 1, Width: 10, Height: 5})
	g.SpawnPlantSpecies("tree", 4, 2, 500)
	g.setTerrain(7, 2, terrain.Rock)
	g.Enqueue(command.Command{Kind: command.DesignateCut, X: 0, Y: 0, X2: 9, Y2: 4})
	g.Enqueue(command.Command{Kind: command.DesignateMine, X: 0, Y: 0, X2: 9, Y2: 4})
	g.Step(1)
	if len(g.World().Mines) != 1 || !g.World().Plants[0].Cut {
		t.Fatal("designations should be set")
	}
	g.Enqueue(command.Command{Kind: command.DesignateCancel, X: 0, Y: 0, X2: 9, Y2: 4})
	g.Step(1)
	if len(g.World().Mines) != 0 || g.World().Plants[0].Cut {
		t.Fatal("cancel should clear cut and mine")
	}
}
