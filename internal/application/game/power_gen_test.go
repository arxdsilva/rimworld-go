package game

import (
	"testing"

	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/plant"
	"rimworld-go/internal/domain/world/roof"
	"rimworld-go/internal/domain/world/terrain"
	"rimworld-go/internal/domain/world/weather"
)

// PLAY-021 FUELGEN: what each producer makes (power_gen.go). Integer W.

// genGame is a 50x50 clear-weather map at the given day and hour with the
// area x0..x1, y0..y1 cleared of plants.
func genGame(t *testing.T, day, hour int, x0, y0, x1, y1 int) *Game {
	t.Helper()
	g := New(Config{Seed: 2, Width: 50, Height: 50})
	jumpTo(g, day, hour)
	g.SetWeather(weather.Clear, 0)
	g.ClearPlants(x0, y0, x1, y1)
	return g
}

func TestGeothermalIsConstant(t *testing.T) {
	t.Parallel()
	for _, h := range []int{2, 8, 12, 20} {
		g := genGame(t, 10, h, 0, 0, 49, 49)
		id := placeFuel(t, g, construction.GeothermalGenerator, 20, 20)
		g.SetWeather(weather.Storm, 1)
		g.weather.WindSpeed = 0
		if got := g.genOutput(id); got != 1800 {
			t.Fatalf("hour %d: geothermal makes %d W, want 1800", h, got)
		}
	}
}

func TestWoodGeneratorMakes800WhileFuelled(t *testing.T) {
	t.Parallel()
	g := genGame(t, 10, 12, 0, 0, 49, 49)
	id := placeFuel(t, g, construction.WoodGenerator, 20, 20)
	if got := g.genOutput(id); got != 0 {
		t.Fatalf("an empty generator makes %d W", got)
	}
	g.setFuel(id, 1)
	if got := g.genOutput(id); got != 800 {
		t.Fatalf("a fuelled generator makes %d W, want 800", got)
	}
	g.setFuel(id, 0)
	if got := g.genOutput(id); got != 0 {
		t.Fatalf("a burnt-out generator makes %d W", got)
	}
}

func TestGenOutputOfNonProducersIsZero(t *testing.T) {
	t.Parallel()
	g := genGame(t, 10, 12, 0, 0, 49, 49)
	tbl := placeFuel(t, g, construction.Table, 10, 10)
	if got := g.genOutput(tbl); got != 0 {
		t.Fatalf("a table makes %d W", got)
	}
	if got := g.genOutput(entity.EntityID(9999)); got != 0 {
		t.Fatalf("an unknown id makes %d W", got)
	}
}

func TestSolarFollowsDaylight(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		hour int
		want int
	}{
		{"noon is full", 12, 900},
		{"night is dark", 2, 0},
		{"dusk floor is zero", 23, 0},
		// 900 x (0.85 - 0.12) / 0.88 at 08:00, truncated
		{"morning", 8, 746},
	}
	for _, c := range cases {
		g := genGame(t, 10, c.hour, 0, 0, 49, 49)
		id := placeFuel(t, g, construction.SolarPanel, 20, 20)
		if got := g.genOutput(id); got != c.want {
			t.Errorf("%s (hour %d): %d W, want %d", c.name, c.hour, got, c.want)
		}
	}
}

func TestSolarIsScaledByTheUnroofedShare(t *testing.T) {
	t.Parallel()
	g := genGame(t, 10, 12, 0, 0, 49, 49)
	id := placeFuel(t, g, construction.SolarPanel, 20, 20) // 3x3: 20..22
	g.ensureRoofGrid()
	w := g.grid.Width()
	roofCell := func(x, y int) { g.setRoof(y*w+x, roof.Natural) }
	if got := g.genOutput(id); got != 900 {
		t.Fatalf("open sky: %d W, want 900", got)
	}
	// one row of three cells roofed: 6 of 9 under the sky = 666 permille
	for x := 20; x <= 22; x++ {
		roofCell(x, 20)
	}
	if got := g.genOutput(id); got != 599 {
		t.Fatalf("a third roofed: %d W, want 599 (900 x 666/1000)", got)
	}
	// roof elsewhere does not matter
	roofCell(30, 30)
	if got := g.genOutput(id); got != 599 {
		t.Fatalf("a roof away from the panel changed its output: %d W", got)
	}
	for y := 21; y <= 22; y++ {
		for x := 20; x <= 22; x++ {
			roofCell(x, y)
		}
	}
	if got := g.genOutput(id); got != 0 {
		t.Fatalf("a fully roofed panel makes %d W, want 0", got)
	}
}

// windGame sets a vertical turbine at (20,20) (cells 20,20 and 20,21), so
// its band is x 15..19 and 21..25 on rows 20 and 21: twenty cells.
func windGame(t *testing.T, speed float64) (*Game, entity.EntityID) {
	t.Helper()
	g := genGame(t, 10, 12, 10, 14, 30, 28)
	id := g.placeGenerated(construction.Building{Kind: construction.WindTurbine, Stuff: item.Steel, Facing: construction.South, X: 20, Y: 20})
	if id == 0 {
		t.Fatal("no turbine")
	}
	cells, _, _ := construction.Footprint(construction.WindTurbine, construction.South, 20, 20)
	if len(cells) != 2 || cells[0][0] != cells[1][0] {
		t.Fatalf("the test assumes a vertical turbine, footprint %v", cells)
	}
	for _, yy := range []int{20, 21} {
		for x := 15; x <= 25; x++ {
			if x != 20 && !g.grid.Walkable(x, yy) {
				t.Fatalf("band cell %d,%d is not open ground", x, yy)
			}
		}
	}
	g.weather.WindSpeed = speed
	return g, id
}

func TestWindFollowsTheWindSpeedAndCaps(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		speed float64
		want  int
	}{
		{0, 0}, {0.4, 100}, {2.0, 500}, {3.2, 800}, {4.4, 1100}, {4.5, 1100}, {9, 1100},
	} {
		g, id := windGame(t, c.speed)
		if got := g.genOutput(id); got != c.want {
			t.Errorf("wind %v: %d W, want %d", c.speed, got, c.want)
		}
	}
}

func TestWindBandIsBlockedByWallsBuildingsRockRoofsAndTrees(t *testing.T) {
	t.Parallel()
	// each case blocks some band cells (of 20) at wind 2.0 (500 W clear)
	cases := []struct {
		name  string
		block func(g *Game)
		want  int
	}{
		{"clear band", func(g *Game) {}, 500},
		{"a wall on each side beside the turbine", func(g *Game) {
			for _, c := range [][2]int{{19, 20}, {19, 21}, {21, 20}, {21, 21}} {
				g.placeGenerated(construction.Building{Kind: construction.Wall, Stuff: item.Wood, X: c[0], Y: c[1]})
			}
		}, 400}, // 16 of 20 clear = 800 permille
		{"a blocking bench (2x1)", func(g *Game) {
			g.placeGenerated(construction.Building{Kind: construction.Workbench, Stuff: item.Wood, X: 17, Y: 20})
		}, 450}, // 18 clear = 900
		{"rock", func(g *Game) {
			g.setTerrain(24, 21, terrain.Rock)
			g.setTerrain(25, 21, terrain.Rock)
		}, 450}, // 18 clear = 900
		{"trees", func(g *Game) {
			for _, c := range [][2]int{{16, 20}, {16, 21}, {22, 20}} {
				if _, err := g.spawnPlant(plant.Tree, c[0], c[1], 1000); err != nil {
					panic(err)
				}
			}
		}, 425}, // 17 clear = 850
		{"a roof over five cells", func(g *Game) {
			g.ensureRoofGrid()
			for x := 15; x <= 19; x++ {
				g.setRoof(20*g.grid.Width()+x, roof.Natural)
			}
		}, 375}, // 15 clear = 750
		{"a lamp and a bush do not block", func(g *Game) {
			g.placeGenerated(construction.Building{Kind: construction.TorchLamp, Stuff: item.Wood, X: 18, Y: 20})
			if _, err := g.spawnPlant(plant.BerryBush, 22, 21, 1000); err != nil {
				panic(err)
			}
		}, 500},
		{"beyond the band", func(g *Game) {
			g.placeGenerated(construction.Building{Kind: construction.Wall, Stuff: item.Wood, X: 26, Y: 20})
			g.placeGenerated(construction.Building{Kind: construction.Wall, Stuff: item.Wood, X: 14, Y: 21})
			g.placeGenerated(construction.Building{Kind: construction.Wall, Stuff: item.Wood, X: 20, Y: 22})
			g.placeGenerated(construction.Building{Kind: construction.Wall, Stuff: item.Wood, X: 18, Y: 19})
		}, 500},
		{"fully walled in", func(g *Game) {
			for y := 20; y <= 21; y++ {
				for x := 15; x <= 25; x++ {
					if x != 20 {
						g.placeGenerated(construction.Building{Kind: construction.Wall, Stuff: item.Wood, X: x, Y: y})
					}
				}
			}
		}, 0},
	}
	for _, c := range cases {
		g, id := windGame(t, 2.0)
		c.block(g)
		if got := g.genOutput(id); got != c.want {
			t.Errorf("%s: %d W, want %d", c.name, got, c.want)
		}
	}
}

func TestWindBandOfAHorizontalTurbineRunsAlongItsLongSides(t *testing.T) {
	t.Parallel()
	g := genGame(t, 10, 12, 10, 14, 30, 28)
	id := g.placeGenerated(construction.Building{Kind: construction.WindTurbine, Stuff: item.Steel, Facing: construction.East, X: 20, Y: 20})
	cells, _, _ := construction.Footprint(construction.WindTurbine, construction.East, 20, 20)
	if len(cells) != 2 || cells[0][1] != cells[1][1] {
		t.Skipf("east is not horizontal for this def: %v", cells)
	}
	g.weather.WindSpeed = 2.0
	// walls above and below the turbine (rows 19 and 22 of its columns) are
	// inside a horizontal turbine's band; walls beside its ends are not
	g.placeGenerated(construction.Building{Kind: construction.Wall, Stuff: item.Wood, X: 20, Y: 19})
	g.placeGenerated(construction.Building{Kind: construction.Wall, Stuff: item.Wood, X: 21, Y: 22})
	g.placeGenerated(construction.Building{Kind: construction.Wall, Stuff: item.Wood, X: 19, Y: 20})
	g.placeGenerated(construction.Building{Kind: construction.Wall, Stuff: item.Wood, X: 22, Y: 21})
	if got := g.genOutput(id); got != 450 { // 2 of 20 blocked = 900 permille
		t.Fatalf("horizontal turbine: %d W, want 450", got)
	}
}

func TestWindBandOutsideTheMapCountsAsOpenAir(t *testing.T) {
	t.Parallel()
	g := genGame(t, 10, 12, 0, 0, 10, 10)
	id := g.placeGenerated(construction.Building{Kind: construction.WindTurbine, Stuff: item.Steel, Facing: construction.South, X: 2, Y: 5})
	g.weather.WindSpeed = 2.0
	if got := g.genOutput(id); got != 500 {
		t.Fatalf("a turbine near the map edge: %d W, want 500", got)
	}
}

func TestWindShareIsCachedPerStructRoofAndHour(t *testing.T) {
	t.Parallel()
	g, id := windGame(t, 2.0)
	if got := g.genOutput(id); got != 500 {
		t.Fatalf("baseline %d", got)
	}
	builds := g.vents.wind.builds
	for range 5 {
		_ = g.genOutput(id)
	}
	if g.vents.wind.builds != builds {
		t.Fatal("the band was re-scanned with nothing changed")
	}

	// a building: structVer moves
	g.placeGenerated(construction.Building{Kind: construction.Wall, Stuff: item.Wood, X: 21, Y: 20})
	if got := g.genOutput(id); got != 475 {
		t.Fatalf("after a wall: %d W, want 475", got)
	}

	// a roof: roofVer moves, structVer does not
	sv := g.structVer
	g.ensureRoofGrid()
	for x := 15; x <= 17; x++ {
		g.setRoof(21*g.grid.Width()+x, roof.Natural)
	}
	if g.structVer != sv {
		t.Skip("roofing bumped structVer; the roofVer path is not isolated")
	}
	if got := g.genOutput(id); got != 400 { // 16 clear = 800 permille
		t.Fatalf("after a roof: %d W, want 400", got)
	}

	// an hour later: a tree has grown into the band
	if _, err := g.spawnPlant(plant.Tree, 23, 20, 1000); err != nil {
		t.Fatal(err)
	}
	g.clock.Advance(clock.TicksPerHour)
	if got := g.genOutput(id); got != 375 { // 15 clear = 750 permille
		t.Fatalf("an hour later with a tree: %d W, want 375", got)
	}
}

func TestRoofVerMovesOnlyWhenRoofednessChanges(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 2, Width: 20, Height: 20})
	g.ensureRoofGrid()
	v := g.roofVer()
	g.setRoof(5, roof.PlanBuild)
	g.setRoof(5, roof.PlanBuild|roof.NoAuto)
	if g.roofVer() != v {
		t.Fatalf("a plan flag moved roofVer: %d -> %d", v, g.roofVer())
	}
	g.setRoof(5, roof.Built)
	if g.roofVer() == v {
		t.Fatal("a new roof did not move roofVer")
	}
	v = g.roofVer()
	g.setRoof(5, roof.Built|roof.Supported)
	if g.roofVer() != v {
		t.Fatal("a flag on an already roofed cell moved roofVer")
	}
	g.setRoof(5, 0)
	if g.roofVer() == v {
		t.Fatal("removing a roof did not move roofVer")
	}
	// the counter never goes back: a reload of the roof grid keeps climbing
	v = g.roofVer()
	g.importRoofs(nil)
	if g.roofVer() <= v {
		t.Fatalf("roofVer %d after the grid was reset, was %d", g.roofVer(), v)
	}
}

// TestSolarFollowsWeather: the panel's output is scaled by the sky's share
// of the sun (weather.SunFactor), the same factor the renderer dims by.
func TestSolarFollowsWeather(t *testing.T) {
	t.Parallel()
	cases := []struct {
		kind      weather.Kind
		intensity float64
		want      int
	}{
		{weather.Clear, 0, 900},
		{weather.Cloudy, 0.2, 900 * 820 / 1000}, // 700 at full, 20% hard: 1000 - 300 x 0.6
		{weather.Rain, 1, 900 * 450 / 1000},
		{weather.Storm, 1, 900 * 350 / 1000},
		{weather.Snow, 1, 900 * 400 / 1000},
		{weather.Blizzard, 1, 900 * 250 / 1000},
	}
	prev := 901
	for _, c := range cases {
		g := genGame(t, 10, 12, 0, 0, 49, 49)
		id := placeFuel(t, g, construction.SolarPanel, 20, 20)
		g.SetWeather(c.kind, c.intensity)
		got := g.genOutput(id)
		if got != c.want {
			t.Errorf("%s at %.2f: %d W, want %d", c.kind, c.intensity, got, c.want)
		}
		if c.intensity == 1 && c.kind != weather.Snow { // rain > storm > blizzard
			if got >= prev {
				t.Errorf("%s gave %d W, not less than the lighter sky's %d", c.kind, got, prev)
			}
			prev = got
		}
	}
	// night stays zero whatever the sky
	g := genGame(t, 10, 2, 0, 0, 49, 49)
	id := placeFuel(t, g, construction.SolarPanel, 20, 20)
	g.SetWeather(weather.Rain, 1)
	if got := g.genOutput(id); got != 0 {
		t.Fatalf("rainy night: %d W", got)
	}
}
