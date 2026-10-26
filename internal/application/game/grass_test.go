package game

import (
	"bytes"
	"reflect"
	"testing"

	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/clock"
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

// ---- grass and light (PLAY-017) ----

// TestOutdoorGrassGrowsInTheSunWindowOnly: open grass regrows between
// sunrise and sunset, not through the night.
func TestOutdoorGrassGrowsInTheSunWindowOnly(t *testing.T) {
	g := mildGame()
	x, y := grassTile(t, g, 2, 2, 20, 12)
	jumpTo(g, 10, 21) // an hour after sunset (a window looks back a tenth of an hour)
	regrowNow(t, g, x, y)
	g.Step(8 * clock.TicksPerHour) // 21:00 to 05:00
	if got := g.grassGrowthAt(x, y); got != plant.GrassShoots {
		t.Fatalf("grass grew to %d through the night", got)
	}
	g.Step(5 * clock.TicksPerHour) // dawn and the morning
	if got := g.grassGrowthAt(x, y); got <= plant.GrassShoots {
		t.Fatalf("grass stood still at dawn: %d", got)
	}
}

// TestOutdoorGrassDailyTotalUnchanged: whatever hour a day starts at, open
// grass gains the day's regrowth it always did (a third of the way from
// shoots to full), now all in the sun's hours.
func TestOutdoorGrassDailyTotalUnchanged(t *testing.T) {
	want := plant.GrassGrowSpan(plant.TempFactor(150), 0, clock.TicksPerDay) // round the clock, as before
	if want < 27 || want > 29 {
		t.Fatalf("legacy day total %d", want)
	}
	for _, h := range []int{0, 5, 6, 12, 19, 20, 23} {
		g := mildGame()
		x, y := grassTile(t, g, 2, 2, 20, 12)
		jumpTo(g, 10, h)
		regrowNow(t, g, x, y)
		g.Step(clock.TicksPerDay)
		if got := g.grassGrowthAt(x, y) - plant.GrassShoots; got < want-1 || got > want+1 {
			t.Errorf("day from %02d:00: grass gained %d, want %d (+-1)", h, got, want)
		}
	}
}

// TestRoofedGrassNeedsAStrongLamp: grass under a roof does not regrow in a
// day; a lamp one tile away (0.6) grows it round the clock, one four tiles
// away (0.3) is not enough.
func TestRoofedGrassNeedsAStrongLamp(t *testing.T) {
	g := mildGame()
	a, b := grassTile(t, g, 2, 2, 20, 12)
	lx, ly := a+1, b
	if g.hasGrass(lx, ly) {
		g.clearGrass(lx, ly)
	}
	ax, ay := grassTile(t, g, 12, 2, 22, 12)
	cx, cy := grassTile(t, g, 2, 12, 20, 15)
	for _, c := range [][2]int{{a, b}, {ax, ay}, {cx, cy}} {
		regrowNow(t, g, c[0], c[1])
	}
	roofOver(g, 0, 0, 23, 15)
	_, gen := litLamp(t, g, lx, ly, lx+1, ly+1) // PLAY-021: a lamp on power
	if l := g.artificialLight(a, b); !plant.StrongLight(l) {
		t.Fatalf("lamp one tile away gives %.2f", l)
	}
	if l := g.artificialLight(ax, ay); plant.StrongLight(l) {
		t.Fatalf("test needs a tile the lamp leaves dim, got %.2f", l)
	}
	jumpTo(g, 10, 12)
	g.Step(clock.TicksPerDay / 2)
	g.setFuel(gen, g.fuelCap(gen)) // a tank lasts under a day
	g.Step(clock.TicksPerDay / 2)
	if got := g.grassGrowthAt(ax, ay); got != plant.GrassShoots {
		t.Fatalf("roofed unlit grass grew to %d", got)
	}
	if got := g.grassGrowthAt(cx, cy); got != plant.GrassShoots {
		t.Fatalf("roofed unlit grass grew to %d", got)
	}
	want := plant.GrassGrowSpan(plant.TempFactor(150), 0, clock.TicksPerDay)
	if got := g.grassGrowthAt(a, b) - plant.GrassShoots; got < want-1 || got > want+1 {
		t.Fatalf("lamp-lit roofed grass gained %d in a day, want %d", got, want)
	}
}

// TestLampGrowsRoofedGrassAtNight: lamplight grows roofed grass through
// the night too.
func TestLampGrowsRoofedGrassAtNight(t *testing.T) {
	g := mildGame()
	x, y := grassTile(t, g, 2, 2, 20, 12)
	if g.hasGrass(x+1, y) {
		g.clearGrass(x+1, y)
	}
	regrowNow(t, g, x, y)
	roofOver(g, 0, 0, 23, 15)
	litLamp(t, g, x+1, y, x+2, y+1) // PLAY-021: a lamp on power
	jumpTo(g, 10, 22)
	g.Step(4 * clock.TicksPerHour)
	if got := g.grassGrowthAt(x, y); got <= plant.GrassShoots {
		t.Fatalf("lamp-lit grass stood still at night: %d", got)
	}
}
