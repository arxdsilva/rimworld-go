package game

import (
	"bytes"
	"reflect"
	"testing"

	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/world/plant"
	"rimworld-go/internal/domain/world/terrain"
)

func grassGame() *Game {
	return New(Config{Seed: 1, Width: 24, Height: 16})
}

// grassTile finds a grass tile in [x0,x1)x[y0,y1) that is not on a plant.
func grassTile(t *testing.T, g *Game, x0, y0, x1, y1 int) (int, int) {
	t.Helper()
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			if g.hasGrass(x, y) && g.plantAt(x, y) == 0 {
				return x, y
			}
		}
	}
	t.Fatal("no grass tile in range")
	return 0, 0
}

func gridGrass(w WorldView, x, y int) bool { return w.Grass[y*w.TerrainW+x] == 1 }

func TestGrassSeededFromTerrain(t *testing.T) {
	g := grassGame()
	g.setTerrain(3, 3, terrain.Stone)
	w := g.World()
	if len(w.Grass) != w.TerrainW*g.Height() {
		t.Fatalf("grass len %d", len(w.Grass))
	}
	n := 0
	for y := 0; y < g.Height(); y++ {
		for x := 0; x < g.Width(); x++ {
			tt := g.terr.At(x, y)
			want := plant.GrassAt(tt, x, y) && g.plantAt(x, y) == 0
			if gridGrass(w, x, y) != want {
				t.Fatalf("(%d,%d) grass %v want %v", x, y, gridGrass(w, x, y), want)
			}
			if want {
				n++
			}
		}
	}
	if n == 0 {
		t.Fatal("expected some grass")
	}
}

func TestBuildingClearsGrass(t *testing.T) {
	g := grassGame()
	g.SpawnStack(item.Wood, 20, 0, 0)
	x, y := grassTile(t, g, 2, 2, 20, 12)
	placeBuilt(t, g, construction.Wall, construction.South, x, y)
	if g.hasGrass(x, y) {
		t.Fatal("grass must be gone under a completed wall")
	}
}

func TestFloorClearsGrass(t *testing.T) {
	g := grassGame()
	g.SpawnStack(item.Wood, 20, 0, 0)
	x, y := grassTile(t, g, 2, 2, 20, 12)
	placeBuilt(t, g, construction.Floor, construction.South, x, y)
	if g.hasGrass(x, y) {
		t.Fatal("grass must be gone under a floor")
	}
}

func TestSowingClearsGrass(t *testing.T) {
	g := grassGame()
	id, _ := g.SpawnNamedPawn(1, 1, "Ada")
	_ = id
	x, y := grassTile(t, g, 4, 3, 12, 10)
	g.Enqueue(command.Command{Kind: command.DesignateGrowZone, X: x, Y: y, X2: x, Y2: y})
	g.Step(TicksPerCell*40 + 4*plant.DefOf(plant.Potato).SowWork)
	if g.plantAt(x, y) == 0 {
		t.Fatal("tile should be sown")
	}
	if g.hasGrass(x, y) {
		t.Fatal("sowing must clear grass")
	}
}

func TestCutPlantsMarksGrassAndGrowerCutsIt(t *testing.T) {
	g := grassGame()
	g.SpawnNamedPawn(2, 2, "Ada")
	x, y := grassTile(t, g, 4, 3, 12, 10)
	g.Enqueue(command.Command{Kind: command.DesignateCut, X: x, Y: y, X2: x, Y2: y})
	g.Step(1)
	if cut := g.World().GrassCut; !reflect.DeepEqual(cut, [][2]int{{x, y}}) {
		t.Fatalf("GrassCut = %v", cut)
	}
	g.Step(TicksPerCell*30 + 6*GrassCutWork)
	w := g.World()
	if gridGrass(w, x, y) || len(w.GrassCut) != 0 {
		t.Fatalf("grass should be cut: grass=%v marks=%v", gridGrass(w, x, y), w.GrassCut)
	}
}

func TestCancelUnmarksGrass(t *testing.T) {
	g := grassGame()
	x, y := grassTile(t, g, 4, 3, 12, 10)
	g.Enqueue(command.Command{Kind: command.DesignateCut, X: x, Y: y, X2: x, Y2: y})
	g.Step(1)
	g.Enqueue(command.Command{Kind: command.DesignateCancel, X: x, Y: y, X2: x, Y2: y})
	g.Step(1)
	w := g.World()
	if len(w.GrassCut) != 0 || !gridGrass(w, x, y) {
		t.Fatalf("cancel keeps grass, drops mark: marks=%v grass=%v", w.GrassCut, gridGrass(w, x, y))
	}
}

func TestGrassDeterministic(t *testing.T) {
	run := func() WorldView {
		g := grassGame()
		g.SpawnNamedPawn(2, 2, "Ada")
		g.Enqueue(command.Command{Kind: command.DesignateCut, X: 3, Y: 3, X2: 12, Y2: 9})
		g.Step(1500)
		return g.World()
	}
	a, b := run(), run()
	if !reflect.DeepEqual(a.Grass, b.Grass) || !reflect.DeepEqual(a.GrassCut, b.GrassCut) {
		t.Fatal("grass diverged between identical runs")
	}
}

func TestGrassHiddenUnderBuildingsInOldSave(t *testing.T) {
	g := grassGame()
	g.SpawnStack(item.Wood, 20, 0, 0)
	x, y := grassTile(t, g, 2, 2, 20, 12)
	placeBuilt(t, g, construction.Wall, construction.South, x, y)
	px, py := grassTile(t, g, 2, 2, 20, 12)
	g.SpawnCrop("potato", px, py, 10)
	g.importGrass(nil, nil) // an old save: no grass data
	w := g.World()
	if gridGrass(w, x, y) || gridGrass(w, px, py) {
		t.Fatal("reseed must leave building and plant tiles bare")
	}
}

func TestGrassExportImportRoundTrip(t *testing.T) {
	g := grassGame()
	x, y := grassTile(t, g, 4, 3, 12, 10)
	g.Enqueue(command.Command{Kind: command.DesignateCut, X: x, Y: y, X2: x, Y2: y})
	g.Step(1)
	cells, cut := g.exportGrass()
	h := grassGame()
	h.importGrass(cells, cut)
	a, b := g.World(), h.World()
	if !reflect.DeepEqual(a.Grass, b.Grass) || !reflect.DeepEqual(a.GrassCut, b.GrassCut) {
		t.Fatal("round trip changed grass")
	}
}

// TestGrassSaveLoadRoundTrip: cut tiles and cut marks survive Save/Load.
func TestGrassSaveLoadRoundTrip(t *testing.T) {
	g := NewColony(Config{Seed: 5, Width: 40, Height: 30})
	w := g.World()
	var on [][2]int
	for i, v := range w.Grass {
		if v == 1 {
			on = append(on, [2]int{i % w.TerrainW, i / w.TerrainW})
		}
	}
	if len(on) < 2 {
		t.Fatal("expected grass on a generated map")
	}
	g.clearGrass(on[0][0], on[0][1])
	g.markGrass(on[1][0], on[1][1], on[1][0], on[1][1], true)
	var buf bytes.Buffer
	if err := g.Save(&buf, "grass"); err != nil {
		t.Fatal(err)
	}
	l, _, err := Load(&buf)
	if err != nil {
		t.Fatal(err)
	}
	a, b := g.World(), l.World()
	if !reflect.DeepEqual(a.Grass, b.Grass) || !reflect.DeepEqual(a.GrassCut, b.GrassCut) {
		t.Fatal("grass and cut marks must survive a save")
	}
	if b.Grass[on[0][1]*b.TerrainW+on[0][0]] != 0 {
		t.Fatal("a cut tile stays bare after loading")
	}
}
