package game

import (
	"bytes"
	"reflect"
	"testing"

	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/world/biome"
	"rimworld-go/internal/domain/world/plant"
	"rimworld-go/internal/domain/world/temperature"
	"rimworld-go/internal/domain/world/terrain"
	"rimworld-go/internal/domain/world/weather"
)

func TestBiomeClimate(t *testing.T) {
	tundra := New(Config{Seed: 1, Width: 16, Height: 16, Biome: biome.Tundra})
	desert := New(Config{Seed: 1, Width: 16, Height: 16, Biome: biome.Desert})
	temperate := New(Config{Seed: 1, Width: 16, Height: 16})
	if tundra.climate != biome.DefOf(biome.Tundra).Climate || desert.climate != biome.DefOf(biome.Desert).Climate ||
		temperate.climate != temperature.Temperate {
		t.Fatal("game climate must be its biome's")
	}
	for _, g := range []*Game{tundra, desert, temperate} {
		jumpTo(g, 40, 12)
		g.SetWeather(weather.Clear, 0)
	}
	tu, te, de := tundra.World().OutdoorTemp, temperate.World().OutdoorTemp, desert.World().OutdoorTemp
	if !(tu < te && te < de) {
		t.Fatalf("outdoor tundra %d, temperate %d, desert %d", tu, te, de)
	}
	if got := tundra.TemperatureAt(3, 3); got != int(biome.DefOf(biome.Tundra).Climate.Outdoor(tundra.clock.Now())) {
		t.Fatalf("tundra tile %d", got)
	}
}

func TestBiomeGrassSeeding(t *testing.T) {
	g := NewColony(Config{Seed: 4, Width: 64, Height: 48, Biome: biome.BorealForest})
	w := g.World()
	ground, grass := 0, 0
	for i, c := range g.terr.Cells {
		if c.Fertility() > 0 && c != terrain.Sand {
			ground++
			if w.Grass[i] == 1 {
				grass++
			}
		}
	}
	// 700 permille seeded; plants, the start kit and Home take some back
	if got := grass * 1000 / ground; got < 550 || got > 730 {
		t.Fatalf("boreal grass %d permille of grass ground", got)
	}
	ice := NewColony(Config{Seed: 4, Width: 48, Height: 40, Biome: biome.IceSheet})
	iw := ice.World()
	for i, c := range ice.terr.Cells {
		if c == terrain.Ice && iw.Grass[i] == 1 {
			t.Fatal("grass on ice")
		}
	}
}

func TestGrassRegrows(t *testing.T) {
	g := grassGame() // temperate: regrows in 6 days +-20%
	x, y := grassTile(t, g, 2, 2, 20, 12)
	if !g.grazeGrass(x, y) || g.hasGrass(x, y) {
		t.Fatal("grazing should leave the tile bare")
	}
	if g.grazeGrass(x, y) {
		t.Fatal("a bare tile has nothing to graze")
	}
	// grazed on the landing morning (hour 6): due 4.8 to 7.2 days later
	jumpTo(g, 5, 0) // too soon
	g.Step(1)
	if g.hasGrass(x, y) {
		t.Fatal("grass back before 80% of its regrow time")
	}
	jumpTo(g, 8, 0) // due
	g.Step(1)
	if !g.hasGrass(x, y) {
		t.Fatal("grass should have grown back")
	}
	if len(g.grass.regrow) != 0 {
		t.Fatalf("regrow queue %v", g.grass.regrow)
	}
}

func TestCutGrassRegrowsButBuiltDoesNot(t *testing.T) {
	g := grassGame()
	g.SpawnStack(item.Wood, 20, 0, 0)
	cx, cy := grassTile(t, g, 2, 2, 10, 12)
	g.clearGrass(cx, cy)
	g.scheduleRegrow(cx, cy) // as a finished cut job does
	bx, by := grassTile(t, g, 12, 2, 20, 12)
	g.grazeGrass(bx, by)
	placeBuilt(t, g, construction.Floor, construction.South, bx, by)
	jumpTo(g, 8, 0)
	g.Step(1)
	if !g.hasGrass(cx, cy) {
		t.Fatal("cut grass should grow back")
	}
	if g.hasGrass(bx, by) {
		t.Fatal("grass must not regrow under a floor")
	}
}

func TestGrassNeverRegrowsWithoutRegrowDays(t *testing.T) {
	// the ice sheet has no regrow days (deserts got some for their grazers)
	g := New(Config{Seed: 1, Width: 24, Height: 16, Biome: biome.IceSheet})
	g.setTerrain(5, 5, terrain.Soil)
	g.seedGrass().cells[5*g.grid.Width()+5] = 1
	if !g.grazeGrass(5, 5) {
		t.Fatal("no grass to graze")
	}
	if len(g.grass.regrow) != 0 {
		t.Fatal("a biome with Regrow 0 queues no regrowth")
	}
}

func TestRegrowQueueSurvivesSave(t *testing.T) {
	g := New(Config{Seed: 1, Width: 24, Height: 16, Biome: biome.TemperateSwamp})
	for x := 2; x < 20; x++ {
		for y := 2; y < 12; y++ {
			g.grazeGrass(x, y)
		}
	}
	if len(g.grass.regrow) == 0 {
		t.Fatal("nothing grazed")
	}
	var buf bytes.Buffer
	if err := g.Save(&buf, "t"); err != nil {
		t.Fatal(err)
	}
	h, _, err := Load(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if h.biome != biome.TemperateSwamp || !reflect.DeepEqual(h.grass.regrow, g.grass.regrow) {
		t.Fatal("biome or regrow queue lost in the save")
	}
	jumpTo(g, 7, 0)
	jumpTo(h, 7, 0)
	g.Step(1)
	h.Step(1)
	if !reflect.DeepEqual(g.World().Grass, h.World().Grass) {
		t.Fatal("loaded game regrew differently")
	}
}

func TestIceNotSowable(t *testing.T) {
	g := New(Config{Seed: 1, Width: 16, Height: 16, Biome: biome.IceSheet})
	g.setTerrain(4, 4, terrain.Ice)
	g.setTerrain(6, 4, terrain.Sand)
	if g.CanZoneAt(ZoneGrow, 4, 4) {
		t.Fatal("ice cannot be farmed")
	}
	if !g.CanZoneAt(ZoneGrow, 6, 4) {
		t.Fatal("sand can be farmed, slowly")
	}
	if g.CanZoneAt(ZoneStockpile, 4, 4) != true {
		t.Fatal("ice still takes a stockpile")
	}
	g.markMine(4, 4, 4, 4, true)
	if len(g.mines) != 0 {
		t.Fatal("ice cannot be mined")
	}
	for _, s := range Soils() {
		if s.Name == "marsh" && s.WalkCost != 200 {
			t.Fatalf("marsh soil view %+v", s)
		}
		if s.Name == "ice" && (s.Farmable || !s.Walkable || s.Label == "") {
			t.Fatalf("ice soil view %+v", s)
		}
	}
	if plant.Rate(plant.Potato, terrain.Sand.Fertility()) >= plant.Rate(plant.Potato, terrain.Soil.Fertility()) {
		t.Fatal("sand should grow slower than soil")
	}
}

// walkTicks is how long a drafted pawn takes along a 12-long corridor of
// ground t (the end cells stay soil).
func walkTicks(t *testing.T, ground terrain.Type) uint64 {
	t.Helper()
	g := corridor(12)
	g.HoldWeather()
	g.SetWeather(weather.Clear, 0)
	for x := 1; x < 11; x++ {
		g.setTerrain(x, 1, ground)
	}
	id := orderMove(t, g, 0, 1, 11, 1)
	start := uint64(g.Tick())
	for i := 0; i < 2000; i++ {
		g.Step(1)
		if p, _ := g.positions.Get(id); p.X == 11 {
			return uint64(g.Tick()) - start
		}
	}
	t.Fatalf("pawn never crossed the %s", ground)
	return 0
}

func TestMarshSlowsWalking(t *testing.T) {
	soil, marsh, sand := walkTicks(t, terrain.Soil), walkTicks(t, terrain.Marsh), walkTicks(t, terrain.Sand)
	if marsh < soil*17/10 {
		t.Fatalf("marsh %d ticks vs soil %d: should be about twice as slow", marsh, soil)
	}
	if sand <= soil || sand >= marsh {
		t.Fatalf("sand %d ticks: between soil %d and marsh %d", sand, soil, marsh)
	}
}

func TestBiomePlantsDieOutOfClimate(t *testing.T) {
	g := New(Config{Seed: 1, Width: 16, Height: 16}) // temperate
	jungle, _ := g.spawnPlant(plant.JungleTree, 3, 3, 1000)
	oak, _ := g.spawnPlant(plant.Tree, 5, 3, 1000)
	jumpTo(g, 15, 14) // midsummer
	g.frostCrops()
	if _, ok := g.plants.Get(jungle); !ok {
		t.Fatal("a jungle tree lives through a temperate summer")
	}
	jumpTo(g, 45, 5) // midwinter night
	g.SetWeather(weather.Clear, 0)
	frostFor(g, 2*clock.TicksPerHour) // PLAY-016: out of its climate it dies over hours
	if _, ok := g.plants.Get(jungle); ok {
		t.Fatal("a jungle tree should die in a temperate winter")
	}
	if _, ok := g.plants.Get(oak); !ok {
		t.Fatal("the original tree only goes dormant")
	}
	if m := g.messages[len(g.messages)-1].Text; m != "The cold killed a jungle tree" {
		t.Fatalf("message %q", m)
	}
	d := New(Config{Seed: 1, Width: 16, Height: 16, Biome: biome.Desert})
	pine, _ := d.spawnPlant(plant.Pine, 3, 3, 1000)
	cactus, _ := d.spawnPlant(plant.Cactus, 5, 3, 1000)
	jumpTo(d, 15, 15) // the hottest afternoon
	d.SetWeather(weather.Clear, 0)
	frostFor(d, 2*clock.TicksPerHour)
	if _, ok := d.plants.Get(pine); ok {
		t.Fatal("a pine should die in a desert summer")
	}
	if _, ok := d.plants.Get(cactus); !ok {
		t.Fatal("the cactus is at home in the desert")
	}
	tu := New(Config{Seed: 1, Width: 16, Height: 16, Biome: biome.Tundra})
	shrub, _ := tu.spawnPlant(plant.Shrub, 3, 3, 1000)
	jumpTo(tu, 45, 5)
	tu.SetWeather(weather.Blizzard, 1)
	frostFor(tu, 2*clock.TicksPerHour)
	if _, ok := tu.plants.Get(shrub); !ok {
		t.Fatal("a tundra shrub survives the tundra winter")
	}
}

func TestBiomePlantsGrowAndYield(t *testing.T) {
	g := New(Config{Seed: 1, Width: 16, Height: 16, Biome: biome.TropicalSwamp})
	reeds, _ := g.spawnPlant(plant.Reeds, 3, 3, 0)
	jumpTo(g, 10, 8)
	g.phasePlants()
	g.phasePlants()
	if p, _ := g.plants.Get(reeds); p.Growth == 0 {
		t.Fatal("reeds should grow in daylight")
	}
	p, _ := g.plants.Get(reeds)
	p.Growth = plant.DefOf(plant.Reeds).GrowTicks
	g.plants.Set(reeds, p)
	if !p.Harvestable() || plant.DefOf(plant.Reeds).Yields != item.Straw || !plant.DefOf(plant.Reeds).Regrows {
		t.Fatal("ripe reeds are harvested for straw and regrow")
	}
	if p := (plant.Plant{Species: plant.JungleTree, Growth: plant.DefOf(plant.JungleTree).GrowTicks}); p.WoodYield() == 0 {
		t.Fatal("a grown jungle tree gives wood")
	}
}

func TestSameSeedSameBiomeSameWorld(t *testing.T) {
	for _, b := range []biome.ID{biome.BorealForest, biome.Desert, biome.TropicalSwamp, biome.SeaIce} {
		a := NewColony(Config{Seed: 13, Width: 48, Height: 40, Biome: b})
		c := NewColony(Config{Seed: 13, Width: 48, Height: 40, Biome: b})
		wa, wc := a.World(), c.World()
		if !bytes.Equal(wa.Terrain, wc.Terrain) || !bytes.Equal(wa.Grass, wc.Grass) || !reflect.DeepEqual(wa.Plants, wc.Plants) {
			t.Fatalf("%s: same seed and biome gave different worlds", b)
		}
	}
	tf := NewColony(Config{Seed: 13, Width: 48, Height: 40}).World()
	de := NewColony(Config{Seed: 13, Width: 48, Height: 40, Biome: biome.Desert}).World()
	if bytes.Equal(tf.Terrain, de.Terrain) {
		t.Fatal("a desert should not look like a temperate forest")
	}
}

func TestOldSaveLoadsTemperate(t *testing.T) {
	g := NewColony(Config{Seed: 3, Width: 48, Height: 40}) // Biome zero, as every pre-biome save
	g.Step(500)
	var buf bytes.Buffer
	if err := g.Save(&buf, "old"); err != nil {
		t.Fatal(err)
	}
	h, _, err := Load(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if h.biome != biome.TemperateForest || h.climate != temperature.Temperate {
		t.Fatalf("old save loaded as %v, climate %+v", h.biome, h.climate)
	}
	if w := h.World(); w.BiomeKey != "temperate_forest" {
		t.Fatalf("world biome %q", w.BiomeKey)
	}
	tu := NewColony(Config{Seed: 3, Width: 48, Height: 40, Biome: biome.Tundra})
	buf.Reset()
	if err := tu.Save(&buf, "tundra"); err != nil {
		t.Fatal(err)
	}
	if h, _, err = Load(&buf); err != nil || h.climate != biome.DefOf(biome.Tundra).Climate {
		t.Fatalf("tundra save: %v, climate %+v", err, h.climate)
	}
}
