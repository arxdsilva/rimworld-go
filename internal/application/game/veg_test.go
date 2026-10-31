package game

import (
	"bytes"
	"reflect"
	"testing"

	"rimworld-go/internal/domain/ai/pathing"
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/world/biome"
	"rimworld-go/internal/domain/world/plant"
	"rimworld-go/internal/domain/world/temperature"
	"rimworld-go/internal/domain/world/terrain"
	"rimworld-go/internal/domain/world/weather"
)

// PLAY-010 (VEG-SIM): wild ground plants, bramble walk cost, grass growth.

func mustPlant(t *testing.T, g *Game, s plant.Species, x, y int) {
	t.Helper()
	if _, err := g.spawnPlant(s, x, y, plant.DefOf(s).GrowTicks); err != nil {
		t.Fatalf("spawn %s at %d,%d: %v", s, x, y, err)
	}
}

func TestPlantWalkCost(t *testing.T) {
	g := New(Config{Seed: 1, Width: 16, Height: 8})
	g.HoldWeather()
	g.SetWeather(weather.Clear, 0)
	mustPlant(t, g, plant.Bramble, 2, 2)
	mustPlant(t, g, plant.ElephantEar, 3, 2)
	mustPlant(t, g, plant.Moss, 4, 2)
	mustPlant(t, g, plant.BerryBush, 5, 2)
	g.setTerrain(6, 2, terrain.Marsh)
	mustPlant(t, g, plant.Bramble, 6, 2)
	for _, c := range []struct{ x, step, extra int }{
		{1, 100, 0},  // plain soil
		{2, 150, 5},  // bramble: half again as slow
		{3, 110, 1},  // elephant ear
		{4, 100, 0},  // moss: no cost
		{5, 100, 0},  // a berry bush costs nothing (as before)
		{6, 300, 15}, // bramble on marsh: both costs
	} {
		if got := g.weatherSlow(c.x, 2, 100); got != c.step {
			t.Errorf("x=%d: step %d, want %d", c.x, got, c.step)
		}
		if got := g.groundExtra(c.x, 2); got != c.extra {
			t.Errorf("x=%d: path extra %d, want %d", c.x, got, c.extra)
		}
	}
	if g.groundExtra(-1, 2) != 0 || g.weatherSlow(15, 7, 100) != 100 {
		t.Fatal("edges cost nothing extra")
	}
}

// TestBrambleSlowsWalking: a drafted pawn crossing ten bramble tiles
// takes about 1.5x as long as on bare soil.
func TestBrambleSlowsWalking(t *testing.T) {
	walk := func(s plant.Species, on bool) uint64 {
		g := corridor(12)
		g.HoldWeather()
		g.SetWeather(weather.Clear, 0)
		if on {
			for x := 1; x < 11; x++ {
				mustPlant(t, g, s, x, 1)
			}
		}
		id := orderMove(t, g, 0, 1, 11, 1)
		start := uint64(g.Tick())
		for i := 0; i < 3000; i++ {
			g.Step(1)
			if p, _ := g.positions.Get(id); p.X == 11 {
				return uint64(g.Tick()) - start
			}
		}
		t.Fatalf("pawn never crossed the %s", s)
		return 0
	}
	soil, bramble, moss := walk(plant.Bramble, false), walk(plant.Bramble, true), walk(plant.Moss, true)
	if bramble*10 < soil*13 || bramble*10 > soil*16 {
		t.Fatalf("bramble %d ticks vs soil %d: want about 1.5x", bramble, soil)
	}
	if moss != soil {
		t.Fatalf("moss %d ticks vs soil %d: moss does not slow walkers", moss, soil)
	}
}

// TestReplanDetoursBrambles: a replan (the costed query) goes round a
// bramble patch when the way round is cheap, and through it when not.
func TestReplanDetoursBrambles(t *testing.T) {
	g := New(Config{Seed: 1, Width: 30, Height: 12})
	for y := 0; y < 12; y++ {
		for x := 0; x < 30; x++ {
			if id := g.plantAt(x, y); id != 0 {
				g.removePlant(id)
			}
			g.setTerrain(x, y, terrain.Soil)
		}
	}
	id, _ := g.SpawnNamedPawn(2, 5, "Ada")
	for y := 0; y <= 5; y++ {
		for x := 10; x <= 13; x++ {
			mustPlant(t, g, plant.Bramble, x, y)
		}
	}
	path, ok := pathing.Find(g.costedQuery(id), 2, 5, 20, 5)
	if !ok {
		t.Fatal("no path")
	}
	for _, p := range path {
		if pid := g.plantAt(p.X, p.Y); pid != 0 {
			t.Fatalf("the path pushes through the brambles at %v though a step round is cheaper", p)
		}
	}
	for y := 6; y < 12; y++ {
		for x := 10; x <= 13; x++ {
			mustPlant(t, g, plant.Bramble, x, y)
		}
	}
	if _, ok := pathing.Find(g.costedQuery(id), 2, 5, 20, 5); !ok {
		t.Fatal("brambles are slow, not a wall")
	}
}

func TestGroundPlantsNeverBlockBuilding(t *testing.T) {
	g := New(Config{Seed: 1, Width: 16, Height: 10})
	g.SpawnStack(item.Wood, 75, 0, 0)
	for i, s := range []plant.Species{plant.Bramble, plant.Moss, plant.Agave, plant.BallCactus, plant.ElephantEar, plant.FireLily, plant.Dandelion} {
		x, y := 2+i, 5
		if id := g.plantAt(x, y); id != 0 {
			g.removePlant(id)
		}
		mustPlant(t, g, s, x, y)
		if ok, why := g.canPlace(construction.Wall, x, y); !ok {
			t.Fatalf("%s blocks a wall: %s", s, why)
		}
		placeBuilt(t, g, construction.Wall, construction.South, x, y)
		if g.plantAt(x, y) != 0 {
			t.Fatalf("building on %s should clear it", s)
		}
	}
}

func TestGroundPlantLabels(t *testing.T) {
	for s, want := range map[plant.Species]string{
		plant.Bramble: "bramble thicket", plant.Moss: "ground moss", plant.Agave: "desert rosette",
		plant.BallCactus: "ball cactus", plant.ElephantEar: "elephant ear", plant.FireLily: "fire lily",
		plant.Dandelion: "dandelions", plant.BerryBush: "berry bush", plant.JungleTree: "jungle tree",
	} {
		if got := s.DisplayName(); got != want {
			t.Errorf("%s.DisplayName() = %q, want %q", s, got, want)
		}
	}
	has := func(l []string, s string) bool {
		for _, v := range l {
			if v == s {
				return true
			}
		}
		return false
	}
	if p := BiomeInfoOf(biome.Desert).Plants; !has(p, "desert rosette") || !has(p, "ball cactus") || !has(p, "shrub") {
		t.Fatalf("desert picker plants %v", p)
	}
	if p := BiomeInfoOf(biome.ColdBog).Plants; !has(p, "ground moss") {
		t.Fatalf("cold bog picker plants %v", p)
	}
}

// TestBrowseGroundPlants: grazers browse moss, brambles and the
// wildflowers when no grass is left, never the cactus types or agave.
func TestBrowseGroundPlants(t *testing.T) {
	setup := func(s plant.Species) (*Game, animal.State, Position) {
		g := New(Config{Seed: 1, Width: 20, Height: 12})
		bareGround(g)
		for len(g.plantIDs) > 0 {
			g.removePlant(g.plantIDs[0])
		}
		mustPlant(t, g, s, 8, 5)
		deer, _ := g.SpawnAnimal("deer", 5, 5)
		pos, _ := g.positions.Get(deer)
		return g, animalOf(g, deer), pos
	}
	for _, s := range []plant.Species{plant.Moss, plant.Bramble, plant.FireLily, plant.Dandelion, plant.ElephantEar} {
		g, a, pos := setup(s)
		ft, ok := g.findFeed(&a, pos, animal.EcologyOf(a.Species).Diet, nil)
		if !ok || ft.Kind != animal.FeedPlant || ft.X != 8 {
			t.Fatalf("a deer should browse %s: %+v %v", s, ft, ok)
		}
		if got := g.browse(0, &a, ft, 0); got != animal.BrowseFood || g.plantAt(8, 5) != 0 {
			t.Fatalf("%s browsed for %d, left %d", s, got, g.plantAt(8, 5))
		}
	}
	for _, s := range []plant.Species{plant.Agave, plant.BallCactus} {
		g, a, pos := setup(s)
		if ft, ok := g.findFeed(&a, pos, animal.EcologyOf(a.Species).Diet, nil); ok {
			t.Fatalf("a deer should never browse %s: %+v", s, ft)
		}
	}
}

// ---- grass growth ----

// mildGame is a temperate game held at 15 C, clear skies.
func mildGame() *Game {
	g := grassGame()
	g.climate = temperature.Climate{YearMean: 150}
	g.HoldWeather()
	g.SetWeather(weather.Clear, 0)
	return g
}

// regrowNow grazes (x, y) and makes its regrowth due now.
func regrowNow(t *testing.T, g *Game, x, y int) {
	t.Helper()
	if !g.grazeGrass(x, y) {
		t.Fatal("nothing to graze")
	}
	for i := range g.grass.regrow {
		g.grass.regrow[i].At = uint64(g.clock.Now())
	}
	g.regrowGrass()
	if !g.hasGrass(x, y) {
		t.Fatal("grass did not regrow")
	}
}

func TestGrassRegrowsAsShootsAndFillsOut(t *testing.T) {
	g := mildGame()
	x, y := grassTile(t, g, 2, 2, 20, 12)
	ox, oy := grassTile(t, g, x+1, y, 20, 12)
	ver := g.World().GrassVer
	if g.World().GrassGrowth != nil || g.grassGrowthAt(ox, oy) != 100 {
		t.Fatal("an untouched map is all full grass, no growth layer")
	}
	regrowNow(t, g, x, y)
	if got := g.grassGrowthAt(x, y); got != plant.GrassShoots {
		t.Fatalf("regrown at %d, want shoots %d", got, plant.GrassShoots)
	}
	w := g.World()
	if w.GrassVer == ver || len(w.GrassGrowth) != len(w.Grass) {
		t.Fatalf("view: ver %d -> %d, growth layer %d", ver, w.GrassVer, len(w.GrassGrowth))
	}
	i, oi := y*w.TerrainW+x, oy*w.TerrainW+ox
	if w.GrassGrowth[i] != plant.GrassShoots || w.GrassGrowth[oi] != 100 {
		t.Fatalf("view growth %d (full tile %d)", w.GrassGrowth[i], w.GrassGrowth[oi])
	}
	start := uint64(g.clock.Now())
	full := uint64(0)
	prev := plant.GrassShoots
	for d := 0; d < 5*clock.TicksPerDay && full == 0; d += climateInterval {
		g.Step(climateInterval)
		gr := g.grassGrowthAt(x, y)
		if gr < prev {
			t.Fatalf("growth fell %d -> %d", prev, gr)
		}
		prev = gr
		if gr == 100 {
			full = uint64(g.clock.Now()) - start
		}
	}
	want := uint64(plant.GrassFillTicks)
	if full < want*9/10 || full > want*11/10 {
		t.Fatalf("shoots filled out in %d ticks, want %d +-10%%", full, want)
	}
	if g.grass.nGrowing != 0 || g.World().GrassGrowth != nil {
		t.Fatal("a full tile leaves the growing set")
	}
	// the view never rewrites a slice it handed out
	regrowNow(t, g, ox, oy)
	a := g.World().GrassGrowth
	keep := append([]uint8(nil), a...)
	g.Step(2 * clock.TicksPerHour)
	if !reflect.DeepEqual(a, keep) {
		t.Fatal("a handed-out growth layer changed")
	}
}

func TestGrassGrowthPausesUnderSnowAndFrost(t *testing.T) {
	for _, c := range []struct {
		name string
		set  func(g *Game, x, y int)
	}{
		{"snow", func(g *Game, x, y int) { g.setSnowCell(y*g.grid.Width()+x, 128) }}, // PLAY-015: the tile's own snow
		{"frost", func(g *Game, _, _ int) { g.climate = temperature.Climate{YearMean: -50} }},
	} {
		g := mildGame()
		x, y := grassTile(t, g, 2, 2, 20, 12)
		regrowNow(t, g, x, y)
		for i := 0; i < clock.TicksPerDay/climateInterval; i++ {
			c.set(g, x, y) // snow would melt at 15 C: keep it lying
			g.Step(climateInterval)
		}
		if got := g.grassGrowthAt(x, y); got != plant.GrassShoots {
			t.Fatalf("%s: growth %d, want paused at %d", c.name, got, plant.GrassShoots)
		}
	}
}

func TestClearedGrowingGrassLeavesGrowingSet(t *testing.T) {
	g := mildGame()
	x, y := grassTile(t, g, 2, 2, 20, 12)
	regrowNow(t, g, x, y)
	g.clearGrass(x, y)
	if g.grass.nGrowing != 0 || g.grassGrowthAt(x, y) != 0 {
		t.Fatal("a cleared tile is not growing")
	}
	// a bare tile written over by a test or fixture is dropped in the pass
	sowGrass(g, x, y, x, y)
	regrowNow(t, g, x, y)
	g.grass.cells[y*g.grid.Width()+x] = 0
	g.Step(climateInterval)
	if g.grass.nGrowing != 0 {
		t.Fatal("the pass drops tiles that lost their grass")
	}
}

// TestGrazersPreferFullerGrass: of two tiles at the same distance the
// fuller one is grazed; a full map picks exactly the nearest tile in
// ring order, as before growth existed.
func TestGrazersPreferFullerGrass(t *testing.T) {
	g := mildGame()
	bareGround(g)
	sowGrass(g, 4, 5, 4, 5) // ring order: (4,5) before (6,5)
	sowGrass(g, 6, 5, 6, 5)
	pos := Position{X: 5, Y: 5}
	all := func(int, int) bool { return true }
	if x, y, ok := g.nearestGrass(pos, animal.FeedRadius, all); !ok || x != 4 || y != 5 {
		t.Fatalf("full field: picked %d,%d", x, y)
	}
	regrowNow(t, g, 4, 5)
	g.setGrassGrowth(4, 5, 60)
	if x, y, ok := g.nearestGrass(pos, animal.FeedRadius, all); !ok || x != 6 || y != 5 {
		t.Fatalf("picked %d,%d, want the full tile 6,5", x, y)
	}
	// the next ring counts too: a full tile one step further beats 60%
	g.clearGrass(6, 5)
	sowGrass(g, 7, 5, 7, 5)
	if x, _, _ := g.nearestGrass(pos, animal.FeedRadius, all); x != 7 {
		t.Fatalf("picked x=%d, want the full tile a ring further", x)
	}
	// two rings out is too far to trade
	g.clearGrass(7, 5)
	sowGrass(g, 8, 5, 8, 5)
	if x, _, _ := g.nearestGrass(pos, animal.FeedRadius, all); x != 4 {
		t.Fatalf("picked x=%d, want the near 60%% tile", x)
	}
}

// TestGrazerLeavesShootsNoFlap: a grazer beside a regrowing tile at 20%
// keeps leaving it while fuller grass is in reach, decision after
// decision, and only grazes it when nothing else is within reach.
func TestGrazerLeavesShootsNoFlap(t *testing.T) {
	g := mildGame()
	bareGround(g)
	sowGrass(g, 6, 5, 6, 5)
	sowGrass(g, 15, 5, 15, 5)
	regrowNow(t, g, 6, 5)
	g.setGrassGrowth(6, 5, 20)
	pos := Position{X: 5, Y: 5}
	all := func(int, int) bool { return true }
	for i := 0; i < 20; i++ {
		x, _, ok := g.nearestGrass(pos, animal.FeedRadius, all)
		if !ok || x != 15 {
			t.Fatalf("decision %d: picked x=%d, want the full tile 15", i, x)
		}
	}
	g.clearGrass(15, 5)
	if x, _, ok := g.nearestGrass(pos, animal.FeedRadius, all); !ok || x != 6 {
		t.Fatalf("with nothing else in reach the shoots are grazed: x=%d ok=%v", x, ok)
	}
}

func TestGrazingYieldScalesWithGrowth(t *testing.T) {
	meal := func(growth int) int {
		g := mildGame()
		bareGround(g)
		sowGrass(g, 6, 5, 6, 5)
		if growth < 100 {
			regrowNow(t, g, 6, 5)
			g.setGrassGrowth(6, 5, growth)
		}
		deer, _ := g.SpawnAnimal("deer", 6, 5)
		setAnimal(g, deer, func(a *animal.State) { a.Food = 0 })
		a := animalOf(g, deer)
		g.eat(deer, &a, animal.FeedTarget{Kind: animal.FeedGrass, X: 6, Y: 5}, uint64(g.clock.Now()))
		if g.hasGrass(6, 5) {
			t.Fatal("grazed grass is eaten down")
		}
		return a.Food
	}
	full, half, shoots := meal(100), meal(50), meal(20)
	if full <= 0 || half*2 != full && half*2 != full-1 && half*2 != full+1 {
		t.Fatalf("a half-grown tile gives half: full %d, half %d", full, half)
	}
	if want := full * plant.GrassMinMeal / 100; shoots < want-1 || shoots > want+1 {
		t.Fatalf("shoots give %d, want the minimum %d", shoots, want)
	}
}

func TestGrassGrowthSaveRoundTrip(t *testing.T) {
	g := mildGame()
	x, y := grassTile(t, g, 2, 2, 20, 12)
	regrowNow(t, g, x, y)
	g.Step(clock.TicksPerDay / 2)
	mid := g.grassGrowthAt(x, y)
	if mid <= plant.GrassShoots || mid >= 100 {
		t.Fatalf("growth after half a day %d", mid)
	}
	var buf bytes.Buffer
	if err := g.Save(&buf, "t"); err != nil {
		t.Fatal(err)
	}
	h, _, err := Load(&buf)
	if err != nil {
		t.Fatal(err)
	}
	h.climate = g.climate // the test's mild, held weather is not saved state
	h.HoldWeather()
	if h.World().GrassVer != g.World().GrassVer {
		t.Fatal("the same grass reads the same version after a load")
	}
	if h.grassGrowthAt(x, y) != mid || h.grass.nGrowing != 1 {
		t.Fatalf("loaded growth %d (growing %d), want %d", h.grassGrowthAt(x, y), h.grass.nGrowing, mid)
	}
	g.Step(clock.TicksPerDay)
	h.Step(clock.TicksPerDay)
	if g.grassGrowthAt(x, y) != h.grassGrowthAt(x, y) || !reflect.DeepEqual(g.World().GrassGrowth, h.World().GrassGrowth) {
		t.Fatal("a loaded game grows its grass differently")
	}
	if g.StateHash() != h.StateHash() {
		t.Fatal("state hash differs after load")
	}
}

// TestOldSaveGrassLoadsFull: a save without growth (all older saves)
// loads with every grass tile full; a full map saves no growth layer.
func TestOldSaveGrassLoadsFull(t *testing.T) {
	g := grassGame()
	cells, _ := g.exportGrass()
	if g.exportGrassGrowth() != nil {
		t.Fatal("nothing growing saves no growth")
	}
	h := grassGame()
	h.importGrass(cells, nil)
	h.importGrassGrowth(nil)
	x, y := grassTile(t, h, 2, 2, 20, 12)
	if h.grassGrowthAt(x, y) != 100 || h.World().GrassGrowth != nil {
		t.Fatal("old save grass is full")
	}
	h.importGrassGrowth([]uint8{1, 2, 3}) // wrong size: ignored
	if h.grass.nGrowing != 0 {
		t.Fatal("a malformed growth layer is ignored")
	}
}
