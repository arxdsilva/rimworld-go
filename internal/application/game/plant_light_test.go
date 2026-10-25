package game

import (
	"math"
	"testing"

	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/world/chunk"
	"rimworld-go/internal/domain/world/plant"
	"rimworld-go/internal/domain/world/temperature"
	"rimworld-go/internal/domain/world/terrain"
	"rimworld-go/internal/domain/world/weather"
)

// lightFixture is a 40×40 bare map on a clear summer day, every cell
// roofed (a dark cave) unless a test lifts it.
func lightFixture(t *testing.T) *Game {
	t.Helper()
	g := New(Config{Seed: 1, Width: 40, Height: 40})
	bareGround(g)
	g.SetWeather(weather.Clear, 0)
	jumpTo(g, 10, 10)
	roofOver(g, 0, 0, 39, 39)
	return g
}

func near(a, b float64) bool { return math.Abs(a-b) < 1e-4 }

// TestLightGridLampAndCampfire: a lamp gives 0.7 at its cell falling off
// linearly to nothing past radius 6; a campfire 0.6 to radius 5; rock
// shadows; overlapping lights take the brighter.
func TestLightGridLampAndCampfire(t *testing.T) {
	g := lightFixture(t)
	placeBuilt(t, g, construction.Lamp, construction.South, 10, 10)
	for _, c := range []struct {
		x, y int
		want float64
	}{
		{10, 10, 0.7}, {13, 10, 0.7 * 4 / 7}, {16, 10, 0.7 / 7}, {17, 10, 0}, {10, 4, 0.7 / 7}, {15, 15, 0}, {14, 14, 0.7 * (7 - math.Sqrt(32)) / 7},
	} {
		if got := float64(g.cellLight(c.x, c.y)); !near(got, c.want) {
			t.Fatalf("lamp light at %d,%d = %.4f, want %.4f", c.x, c.y, got, c.want)
		}
	}
	placeBuilt(t, g, construction.Campfire, construction.South, 30, 30)
	g.FuelCampfire(30, 30) // PLAY-017: a fire lights only while it burns
	for _, c := range []struct {
		x, y int
		want float64
	}{
		{30, 30, 0.6}, {32, 30, 0.6 * 4 / 6}, {35, 30, 0.1}, {36, 30, 0}, {30, 25, 0.1},
	} {
		if got := float64(g.cellLight(c.x, c.y)); !near(got, c.want) {
			t.Fatalf("campfire light at %d,%d = %.4f, want %.4f", c.x, c.y, got, c.want)
		}
	}
	// a rock wall between lamp and cell casts a shadow
	g.setTerrain(12, 10, terrain.Rock)
	if l := g.cellLight(13, 10); l != 0 {
		t.Fatalf("behind rock: %.3f", l)
	}
	if l := g.cellLight(12, 10); !near(float64(l), 0.7*5/7) {
		t.Fatalf("the rock face itself is lit: %.3f", l)
	}
	// overlapping: a second lamp beside the first, the brighter wins (no sum)
	placeBuilt(t, g, construction.Lamp, construction.South, 9, 12)
	if l := g.cellLight(10, 10); !near(float64(l), 0.7) {
		t.Fatalf("overlap at the first lamp: %.3f", l)
	}
}

// TestCellLightDaylight: an unroofed cell gets the outdoor daylight, a
// roofed one only artificial light.
func TestCellLightDaylight(t *testing.T) {
	g := lightFixture(t)
	g.setRoof(5*40+5, 0)
	jumpTo(g, 11, 12)
	if l := g.cellLight(5, 5); !near(float64(l), 1) {
		t.Fatalf("noon open sky: %.3f", l)
	}
	if l := g.cellLight(6, 5); l != 0 {
		t.Fatalf("noon under the roof: %.3f", l)
	}
	jumpTo(g, 12, 2)
	if l := g.cellLight(5, 5); !near(float64(l), 0.12) {
		t.Fatalf("night open sky: %.3f", l)
	}
}

// TestLightGridRebuildsOnlyOnChange: queries, time and ordinary plants
// leave the grid alone; a lamp, a terrain change or a glowing plant
// rebuilds it once.
func TestLightGridRebuildsOnlyOnChange(t *testing.T) {
	g := lightFixture(t)
	g.cellLight(1, 1)
	b := g.light.builds
	for x := range 40 {
		g.cellLight(x, 7)
	}
	g.clock.Advance(clock.TicksPerHour)
	_, _ = g.spawnPlant(plant.Tree, 3, 3, 0)
	g.cellLight(1, 1)
	if g.light.builds != b {
		t.Fatalf("rebuilt without a light change: %d -> %d", b, g.light.builds)
	}
	placeBuilt(t, g, construction.Lamp, construction.South, 20, 20)
	g.cellLight(1, 1)
	g.cellLight(2, 1)
	if g.light.builds != b+1 {
		t.Fatalf("a lamp: %d builds, want %d", g.light.builds, b+1)
	}
	id, _ := g.spawnPlant(plant.Duskmoss, 5, 5, 0)
	if l := g.cellLight(5, 5); !near(float64(l), 0.1) || g.light.builds != b+2 {
		t.Fatalf("duskmoss glow %.3f after %d builds", l, g.light.builds)
	}
	if l := g.cellLight(6, 5); !near(float64(l), 0.05) {
		t.Fatalf("glow beside duskmoss: %.3f", l)
	}
	g.removePlant(id)
	if l := g.cellLight(5, 5); l != 0 || g.light.builds != b+3 {
		t.Fatalf("removed duskmoss: %.3f after %d builds", l, g.light.builds)
	}
}

// growFor steps the game n climate intervals.
func growFor(g *Game, n int) { g.Step(n * climateInterval) }

func growthOf(t *testing.T, g *Game, id uint64) (plant.Plant, bool) {
	t.Helper()
	for _, pid := range g.plantIDs {
		if uint64(pid) == id {
			return g.plants.Get(pid)
		}
	}
	return plant.Plant{}, false
}

// TestMossNeedsLight: ordinary moss grows under the open sky by day and
// next to a lamp, never in a dark roofed cave.
func TestMossNeedsLight(t *testing.T) {
	g := lightFixture(t)
	g.setRoof(5*40+5, 0)
	lit, _ := g.spawnPlant(plant.Moss, 5, 5, 0)
	dark, _ := g.spawnPlant(plant.Moss, 25, 25, 0)
	growFor(g, 12)
	pl, _ := growthOf(t, g, uint64(lit))
	pd, _ := growthOf(t, g, uint64(dark))
	if pl.Growth <= 0 || pd.Growth != 0 {
		t.Fatalf("moss growth: open sky %d, dark cave %d", pl.Growth, pd.Growth)
	}
	placeBuilt(t, g, construction.Lamp, construction.South, 26, 25)
	growFor(g, 12)
	if pd, _ = growthOf(t, g, uint64(dark)); pd.Growth <= 0 {
		t.Fatal("moss beside a lamp does not grow")
	}
}

// TestDuskmossGrowsInTheDark: duskmoss on rough or smoothed stone in a
// dark cave grows (stone counts as CaveFertility); a legacy plant
// ignores light entirely.
func TestDuskmossGrowsInTheDark(t *testing.T) {
	g := lightFixture(t)
	g.setTerrain(10, 10, terrain.Stone)
	g.setTerrain(12, 10, terrain.StoneSmooth)
	a, _ := g.spawnPlant(plant.Duskmoss, 10, 10, 1000)
	b, _ := g.spawnPlant(plant.Duskmoss, 12, 10, 1000)
	g.setRoof(20*40+20, 0) // PLAY-017: a roofed tree no longer grows without a lamp
	tr, _ := g.spawnPlant(plant.Tree, 20, 20, 1000)
	growFor(g, 12)
	for _, id := range []uint64{uint64(a), uint64(b), uint64(tr)} {
		p, ok := growthOf(t, g, id)
		if !ok || p.Growth <= 1000 || p.Withering {
			t.Fatalf("%s in the dark: %+v (alive %v)", p.Species, p, ok)
		}
	}
	pa, _ := growthOf(t, g, uint64(a))
	pt, _ := growthOf(t, g, uint64(tr))
	want := (pt.Growth - 1000) * plant.Rate(plant.Duskmoss, plant.CaveFertility) / 100
	if d := pa.Growth - 1000; d*10 < want*9 || d*10 > want*11 {
		t.Fatalf("duskmoss on stone gained %d, want about %d (fertility %d)", d, want, plant.CaveFertility)
	}
}

// TestDuskmossWithersUnderOpenSky: duskmoss under a removed roof loses
// growth by day and is removed at nothing, with one notice for the patch.
func TestDuskmossWithersUnderOpenSky(t *testing.T) {
	g := lightFixture(t)
	gt := plant.DefOf(plant.Duskmoss).GrowTicks
	var ids []uint64
	for x := 10; x < 13; x++ {
		g.setTerrain(x, 10, terrain.Stone)
		g.setRoof(10*40+x, 0)
		id, _ := g.spawnPlant(plant.Duskmoss, x, 10, gt/10)
		ids = append(ids, uint64(id))
	}
	g.setTerrain(30, 30, terrain.Stone)
	keep, _ := g.spawnPlant(plant.Duskmoss, 30, 30, gt/10)
	msgs := len(g.messages)
	growFor(g, 2)
	p, ok := growthOf(t, g, ids[0])
	if !ok || !p.Withering || p.Growth >= gt/10 {
		t.Fatalf("after a little sun: %+v alive %v", p, ok)
	}
	if v := g.World().Plants; !plantViewOf(v, ids[0]).Withering || !plantViewOf(v, ids[0]).NeedsDark {
		t.Fatalf("view: %+v", plantViewOf(v, ids[0]))
	}
	g.Step(2 * clock.TicksPerHour) // 8%/h: a tenth-grown plant is gone in 1.25 h
	for _, id := range ids {
		if _, ok := growthOf(t, g, id); ok {
			t.Fatalf("duskmoss %d still standing in the sun", id)
		}
	}
	n := 0
	for _, m := range g.messages[msgs:] {
		if m.Text == "Duskmoss withered in the light" {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("%d wither notices for one patch, want 1", n)
	}
	if p, ok := growthOf(t, g, uint64(keep)); !ok || p.Withering {
		t.Fatalf("duskmoss in the dark: %+v alive %v", p, ok)
	}
	if g.plantAt(10, 10) != 0 {
		t.Fatal("withered plant still indexed")
	}
}

func plantViewOf(v []PlantView, id uint64) PlantView {
	for _, p := range v {
		if uint64(p.ID) == id {
			return p
		}
	}
	return PlantView{}
}

// TestDuskmossWithersNearNewLamp: a lamp lit beside duskmoss in the
// dark makes it wither; taking the lamp away lets it regrow.
func TestDuskmossWithersNearNewLamp(t *testing.T) {
	g := lightFixture(t)
	g.setTerrain(11, 10, terrain.Stone)
	gt := plant.DefOf(plant.Duskmoss).GrowTicks
	id, _ := g.spawnPlant(plant.Duskmoss, 11, 10, gt/2)
	placeBuilt(t, g, construction.Lamp, construction.South, 10, 10)
	growFor(g, 4)
	p, _ := growthOf(t, g, uint64(id))
	if !p.Withering || p.Growth >= gt/2 {
		t.Fatalf("beside a lamp: %+v", p)
	}
	g.removeBuilding(g.layerBldgAt(10, 10, construction.LayerObject))
	low := p.Growth
	growFor(g, 4)
	p, _ = growthOf(t, g, uint64(id))
	if p.Withering || p.Growth <= low {
		t.Fatalf("lamp gone: %+v (was %d)", p, low)
	}
}

// TestWitherNoFlap: a light flickering between 0.40 and 0.46 (around the
// 0.45 harm threshold, above the 0.35 recovery) switches withering on once
// and never off; only a drop below 0.35 stops it, and 0.40 then does not
// restart it.
func TestWitherNoFlap(t *testing.T) {
	g := lightFixture(t)
	g.setTerrain(10, 10, terrain.Stone)
	gt := plant.DefOf(plant.Duskmoss).GrowTicks
	id, _ := g.spawnPlant(plant.Duskmoss, 10, 10, gt)
	art := g.lightCells()
	i := 10*40 + 10
	flips, was := 0, false
	pass := func(v float32) {
		art[i] = v
		var fp frostPass
		p, _ := g.plants.Get(id)
		g.lightGrows(id, &p, &fp)
		p, _ = g.plants.Get(id)
		if p.Withering != was {
			flips++
			was = p.Withering
		}
	}
	for k := range 40 {
		pass([]float32{0.46, 0.40}[k%2])
	}
	if flips != 1 || !was {
		t.Fatalf("flicker at the threshold: %d flips, withering %v", flips, was)
	}
	pass(0.34)
	if was {
		t.Fatal("below 0.35 it still withers")
	}
	for range 10 {
		pass(0.40)
	}
	if was || flips != 2 {
		t.Fatalf("0.40 restarted withering: %d flips", flips)
	}
}

// TestDuskmossWalkCost: duskmoss slows walkers like bramble does.
func TestDuskmossWalkCost(t *testing.T) {
	g := lightFixture(t)
	_, _ = g.spawnPlant(plant.Duskmoss, 4, 4, 1000)
	if c := g.plantWalkCost(4, 4); c != 125 {
		t.Fatalf("duskmoss walk cost %d, want 125", c)
	}
}

// TestPlantViewLightNeeds: the inspector reads moss's and duskmoss's
// light needs from the view.
func TestPlantViewLightNeeds(t *testing.T) {
	g := lightFixture(t)
	m, _ := g.spawnPlant(plant.Moss, 2, 2, 0)
	d, _ := g.spawnPlant(plant.Duskmoss, 4, 2, 0)
	tr, _ := g.spawnPlant(plant.Tree, 6, 2, 0)
	v := g.World().Plants
	if pv := plantViewOf(v, uint64(m)); !pv.NeedsLight || pv.NeedsDark || pv.Withering {
		t.Fatalf("moss view %+v", pv)
	}
	if pv := plantViewOf(v, uint64(d)); pv.NeedsLight || !pv.NeedsDark {
		t.Fatalf("duskmoss view %+v", pv)
	}
	if pv := plantViewOf(v, uint64(tr)); pv.NeedsLight || pv.NeedsDark {
		t.Fatalf("tree view %+v", pv)
	}
}

// ---- sun or strong light (PLAY-017) ----

// mildLight is lightFixture at a steady 15 C (full temperature speed), so
// growth is exactly the rate times the hours it counts.
func mildLight(t *testing.T) *Game {
	t.Helper()
	g := lightFixture(t)
	g.climate = temperature.Climate{YearMean: 150}
	g.HoldWeather()
	g.SetWeather(weather.Clear, 0)
	return g
}

// plainRate is the growth rate (percent) of species s at (x, y) in the
// mild air: what growPlant used before the sun-or-lamp rule.
func plainRate(t *testing.T, g *Game, s plant.Species, x, y int) int {
	t.Helper()
	if tf := plant.TempFactor(int(g.temperatureAt(x, y))); tf != 1000 {
		t.Fatalf("air at %d,%d has temp factor %d, want full speed", x, y, tf)
	}
	return plant.Rate(s, plant.Fertility(s, g.terr.At(x, y)))
}

// legacyGrowth is what the plant at (x, y) gains over the n ticks after
// t0 under the plain rule: GrowSpan per 250-tick window at the plant's
// chunk phase, the sun's hours only.
func legacyGrowth(g *Game, rate, x, y int, t0 uint64, n int) int {
	cg := chunk.New(g.grid.Width(), g.grid.Height())
	ph := uint64(cg.Of(x, y) * climateInterval / cg.Count())
	total := 0
	for now := t0 + 1; now <= t0+uint64(n); now++ {
		if now%climateInterval == ph {
			total += plant.GrowSpan(rate, now-climateInterval, now)
		}
	}
	return total
}

// TestRoofedCropWithoutLampDoesNotGrow: under a roof and away from a
// strong lamp a crop stands still, noon included; the open-sky crop beside
// it grows exactly as before the rule.
func TestRoofedCropWithoutLampDoesNotGrow(t *testing.T) {
	g := mildLight(t)
	g.setRoof(25*40+25, 0)
	roofed, _ := g.spawnPlant(plant.Potato, 5, 5, 0)
	open, _ := g.spawnPlant(plant.Potato, 25, 25, 0)
	rate := plainRate(t, g, plant.Potato, 25, 25)
	t0 := uint64(g.clock.Now())
	g.Step(clock.TicksPerDay)
	if p, _ := growthOf(t, g, uint64(roofed)); p.Growth != 0 {
		t.Fatalf("roofed crop grew %d in a day", p.Growth)
	}
	p, _ := growthOf(t, g, uint64(open))
	if want := legacyGrowth(g, rate, 25, 25, t0, clock.TicksPerDay); p.Growth != want || want == 0 {
		t.Fatalf("open-sky crop grew %d, want exactly %d", p.Growth, want)
	}
}

// TestLampGrowsCropRoundTheClock: a lamp one tile away (0.6) grows a
// roofed crop in every hour of the day; four tiles away (0.3) it does not.
func TestLampGrowsCropRoundTheClock(t *testing.T) {
	g := mildLight(t)
	placeBuilt(t, g, construction.Lamp, construction.South, 10, 10)
	near1, _ := g.spawnPlant(plant.Potato, 11, 10, 0)
	far4, _ := g.spawnPlant(plant.Potato, 14, 10, 0)
	if l := float64(g.cellLight(11, 10)); l < plant.StrongGrowLight {
		t.Fatalf("lamp light one tile away %.2f", l)
	}
	if l := float64(g.cellLight(14, 10)); l >= plant.StrongGrowLight {
		t.Fatalf("lamp light four tiles away %.2f", l)
	}
	rate := plainRate(t, g, plant.Potato, 11, 10)
	jumpTo(g, 10, 20) // dusk: the sun is done for the day
	prev := 0
	for h := 0; h < 24; h += 4 { // night and day alike
		g.Step(4 * clock.TicksPerHour)
		p, _ := growthOf(t, g, uint64(near1))
		if p.Growth <= prev {
			t.Fatalf("lamp-lit crop stood still in the 4 h before hour %d (%d -> %d)", (20+h+4)%24, prev, p.Growth)
		}
		prev = p.Growth
	}
	// a whole day at the rate, give or take the window the plant was found in
	want := rate * clock.TicksPerDay / 100
	if d := prev - want; d < -rate*climateInterval/100-1 || d > rate*climateInterval/100+1 {
		t.Fatalf("lamp-lit crop grew %d in a day, want about %d", prev, want)
	}
	if p, _ := growthOf(t, g, uint64(far4)); p.Growth != 0 {
		t.Fatalf("crop four tiles from the lamp grew %d", p.Growth)
	}
}

// TestLampBeatsNightOutdoors: under the open sky a strong lamp still grows
// the crop through the night (max of the sun's and the lamp's hours).
func TestLampBeatsNightOutdoors(t *testing.T) {
	g := mildLight(t)
	g.setRoof(11*40+10, 0)
	g.setRoof(30*40+30, 0)
	placeBuilt(t, g, construction.Lamp, construction.South, 10, 10)
	lit, _ := g.spawnPlant(plant.Potato, 10, 11, 0)
	dark, _ := g.spawnPlant(plant.Potato, 30, 30, 0)
	jumpTo(g, 10, 21)
	g.Step(6 * clock.TicksPerHour) // 21:00 to 03:00
	if p, _ := growthOf(t, g, uint64(lit)); p.Growth <= 0 {
		t.Fatal("an outdoor crop beside a lamp stood still at night")
	}
	if p, _ := growthOf(t, g, uint64(dark)); p.Growth != 0 {
		t.Fatalf("an outdoor crop grew %d at night with no lamp", p.Growth)
	}
}

// TestOutdoorGrowthUnchanged: with the sky open and no lamp, every
// non-cave species grows exactly as it did (GrowSpan per window), through
// two days and nights.
func TestOutdoorGrowthUnchanged(t *testing.T) {
	g := mildLight(t)
	type pl struct {
		id   uint64
		s    plant.Species
		x, y int
	}
	var ps []pl
	for i, s := range []plant.Species{plant.Potato, plant.Corn, plant.Tree, plant.BerryBush, plant.Reeds} {
		x, y := 3+7*i, 4+5*i
		g.setRoof(y*40+x, 0)
		id, _ := g.spawnPlant(s, x, y, 0)
		ps = append(ps, pl{uint64(id), s, x, y})
	}
	t0 := uint64(g.clock.Now())
	g.Step(2 * clock.TicksPerDay)
	for _, c := range ps {
		p, _ := growthOf(t, g, c.id)
		want := min(legacyGrowth(g, plainRate(t, g, c.s, c.x, c.y), c.x, c.y, t0, 2*clock.TicksPerDay), plant.DefOf(c.s).GrowTicks)
		if p.Growth != want || want == 0 {
			t.Errorf("%s grew %d, want exactly %d", c.s, p.Growth, want)
		}
	}
}

// TestDuskmossGrowthUnchanged: duskmoss keeps the plain rule in a dark
// roofed cave (the sun's hours at its stone fertility), neither stopped by
// the roof nor sped up by a dim lamp.
func TestDuskmossGrowthUnchanged(t *testing.T) {
	g := mildLight(t)
	g.setTerrain(10, 10, terrain.Stone)
	id, _ := g.spawnPlant(plant.Duskmoss, 10, 10, 0)
	rate := plainRate(t, g, plant.Duskmoss, 10, 10)
	t0 := uint64(g.clock.Now())
	g.Step(2 * clock.TicksPerDay)
	p, ok := growthOf(t, g, uint64(id))
	if want := legacyGrowth(g, rate, 10, 10, t0, 2*clock.TicksPerDay); !ok || p.Growth != want || want == 0 {
		t.Fatalf("duskmoss grew %d (alive %v), want exactly %d", p.Growth, ok, want)
	}
}

// TestUnfuelledCampfireGivesNoLight: a campfire lights its surroundings
// only while it burns, and the light grid follows the fuel running out and
// being refilled.
func TestUnfuelledCampfireGivesNoLight(t *testing.T) {
	g := lightFixture(t)
	g.climate = temperature.Climate{YearMean: -100} // cold: the fire never banks down
	g.HoldWeather()
	placeBuilt(t, g, construction.Campfire, construction.South, 30, 30)
	if l := g.cellLight(30, 30); l != 0 {
		t.Fatalf("unfuelled campfire lights its cell at %.2f", l)
	}
	if l := g.cellLight(31, 30); l != 0 {
		t.Fatalf("unfuelled campfire lights the next cell at %.2f", l)
	}
	g.FuelCampfire(30, 30)
	if l := float64(g.cellLight(30, 30)); !near(l, 0.6) {
		t.Fatalf("burning campfire light %.2f, want 0.6", l)
	}
	if l := float64(g.cellLight(31, 30)); !near(l, 0.5) || !plant.StrongLight(l) {
		t.Fatalf("burning campfire, next cell %.2f, want a strong 0.5", l)
	}
	bid := g.layerBldgAt(30, 30, construction.LayerObject)
	g.fuel[bid] = climateInterval // one window of wood left
	builds := g.light.builds
	g.Step(3 * climateInterval)
	if g.fuel[bid] != 0 {
		t.Fatalf("fire still holds %d fuel", g.fuel[bid])
	}
	if l := g.cellLight(30, 30); l != 0 {
		t.Fatalf("burnt-out campfire still lights at %.2f", l)
	}
	if g.light.builds == builds {
		t.Fatal("the light grid was not rebuilt when the fuel ran out")
	}
	g.FuelCampfire(30, 30)
	if l := float64(g.cellLight(30, 30)); !near(l, 0.6) {
		t.Fatalf("refuelled campfire light %.2f", l)
	}
}

// TestBurningCampfireGrowsNearbyCrop: a crop beside a burning campfire
// grows on a dark roofed night; the same crop beside a cold one does not.
func TestBurningCampfireGrowsNearbyCrop(t *testing.T) {
	g := mildLight(t)
	placeBuilt(t, g, construction.Campfire, construction.South, 30, 30)
	placeBuilt(t, g, construction.Campfire, construction.South, 10, 10)
	g.FuelCampfire(30, 30)
	lit, _ := g.spawnPlant(plant.Potato, 31, 30, 0)
	cold, _ := g.spawnPlant(plant.Potato, 11, 10, 0)
	jumpTo(g, 10, 22)
	g.Step(2 * clock.TicksPerHour)
	if p, _ := growthOf(t, g, uint64(lit)); p.Growth <= 0 {
		t.Fatal("crop beside a burning campfire did not grow")
	}
	if p, _ := growthOf(t, g, uint64(cold)); p.Growth != 0 {
		t.Fatalf("crop beside a cold campfire grew %d", p.Growth)
	}
}

// ---- PlantView growth state (PLAY-017) ----

func plantViewNow(g *Game, id uint64) PlantView { return plantViewOf(g.plantViews(nil), id) }

// TestPlantViewGrowState: why a plant is or isn't growing now, and the
// time left to ripe at the current conditions.
func TestPlantViewGrowState(t *testing.T) {
	g := mildLight(t)
	g.setRoof(25*40+25, 0)
	open, _ := g.spawnPlant(plant.Potato, 25, 25, 0)
	roofed, _ := g.spawnPlant(plant.Potato, 5, 5, 0)
	placeBuilt(t, g, construction.Lamp, construction.South, 10, 10)
	lit, _ := g.spawnPlant(plant.Potato, 11, 10, 0)
	ripe, _ := g.spawnPlant(plant.Potato, 20, 20, plant.DefOf(plant.Potato).GrowTicks)
	moss, _ := g.spawnPlant(plant.Moss, 30, 5, 0)
	g.setRoof(33*40+33, 0) // cold and heat show once the light is not the trouble
	g.setRoof(35*40+35, 0)
	setAir(g, 33, 33, -50)
	cold, _ := g.spawnPlant(plant.Potato, 33, 33, 0)
	setAir(g, 35, 35, 480)
	hot, _ := g.spawnPlant(plant.Potato, 35, 35, 0)
	jumpTo(g, 10, 10)
	gt := plant.DefOf(plant.Potato).GrowTicks
	rate := plainRate(t, g, plant.Potato, 25, 25)

	for _, c := range []struct {
		name  string
		id    uint64
		state string
	}{
		{"open sky by day", uint64(open), "growing"},
		{"under a roof, no lamp", uint64(roofed), "dark"},
		{"beside a lamp", uint64(lit), "growing"},
		{"ripe", uint64(ripe), "ripe"},
		{"moss in a dark roofed cave", uint64(moss), "dark"},
		{"freezing air", uint64(cold), "cold"},
		{"scorching air", uint64(hot), "hot"},
	} {
		if got := plantViewNow(g, c.id).GrowState; got != c.state {
			t.Errorf("%s: GrowState %q, want %q", c.name, got, c.state)
		}
	}
	// the sun grows a crop 14 h of every 24: the estimate stretches by 24/14
	if got, want := plantViewNow(g, uint64(open)).RipeIn, gt*100/rate*24/14; got < want-24 || got > want+24 {
		t.Errorf("open sky RipeIn %d, want about %d", got, want)
	}
	// a strong lamp grows it round the clock
	if got, want := plantViewNow(g, uint64(lit)).RipeIn, gt*100/rate; got < want-2 || got > want+2 {
		t.Errorf("lamp-lit RipeIn %d, want about %d", got, want)
	}
	for _, id := range []uint64{uint64(roofed), uint64(ripe), uint64(moss), uint64(cold), uint64(hot)} {
		if got := plantViewNow(g, id).RipeIn; got != -1 {
			t.Errorf("plant %d RipeIn %d, want -1 (not growing)", id, got)
		}
	}
	// half grown: half the time left
	g.plants.Set(open, plant.Plant{Species: plant.Potato, X: 25, Y: 25, Growth: gt / 2})
	if got, want := plantViewNow(g, uint64(open)).RipeIn, gt/2*100/rate*24/14; got < want-24 || got > want+24 {
		t.Errorf("half-grown RipeIn %d, want about %d", got, want)
	}
	// night under the open sky: resting, still counting down; the lamp-lit
	// crop never rests
	jumpTo(g, 10, 23)
	if v := plantViewNow(g, uint64(open)); v.GrowState != "resting" || v.RipeIn <= 0 {
		t.Errorf("open sky by night: %q, RipeIn %d", v.GrowState, v.RipeIn)
	}
	if v := plantViewNow(g, uint64(lit)); v.GrowState != "growing" {
		t.Errorf("lamp by night: %q", v.GrowState)
	}
	if v := plantViewNow(g, uint64(roofed)); v.GrowState != "dark" {
		t.Errorf("roofed by night: %q", v.GrowState)
	}
}

// TestPlantViewGrowStateFollowsTheClock: the memoised view reuses an
// unchanged plant's entry, but the state follows the hour and the lamps.
func TestPlantViewGrowStateFollowsTheClock(t *testing.T) {
	g := mildLight(t)
	g.views.on = true
	g.setRoof(25*40+25, 0)
	id, _ := g.spawnPlant(plant.Potato, 25, 25, 0)
	jumpTo(g, 10, 10)
	if v := plantViewNow(g, uint64(id)); v.GrowState != "growing" {
		t.Fatalf("by day: %q", v.GrowState)
	}
	jumpTo(g, 10, 23) // the plant itself is unchanged
	if v := plantViewNow(g, uint64(id)); v.GrowState != "resting" {
		t.Fatalf("by night, memoised: %q", v.GrowState)
	}
	placeBuilt(t, g, construction.Lamp, construction.South, 24, 25)
	if v := plantViewNow(g, uint64(id)); v.GrowState != "growing" {
		t.Fatalf("lamp lit beside it: %q", v.GrowState)
	}
}

// TestDuskmossViewNeverRests: a cave species has no night to rest through.
func TestDuskmossViewNeverRests(t *testing.T) {
	g := mildLight(t)
	g.setTerrain(10, 10, terrain.Stone)
	id, _ := g.spawnPlant(plant.Duskmoss, 10, 10, 0)
	for _, h := range []int{3, 12} {
		jumpTo(g, 10, h)
		if v := plantViewNow(g, uint64(id)); v.GrowState != "growing" || v.RipeIn <= 0 {
			t.Errorf("duskmoss at %d:00: %q, RipeIn %d", h, v.GrowState, v.RipeIn)
		}
	}
}

// ---- ripening marks (PLAY-017) ----

// TestRipeningMarksZoneCrop: a crop that ripens on a growing-zone cell is
// marked for harvest; one that ripens elsewhere is not.
func TestRipeningMarksZoneCrop(t *testing.T) {
	g := mildLight(t)
	g.setRoof(5*40+5, 0)
	g.setRoof(5*40+6, 0)
	g.addGrowZone(5, 5, 5, 5, uint8(plant.Potato))
	gt := plant.DefOf(plant.Potato).GrowTicks
	inZone, _ := g.spawnPlant(plant.Potato, 5, 5, gt-1)
	outside, _ := g.spawnPlant(plant.Potato, 6, 5, gt-1)
	unripe, _ := g.spawnPlant(plant.Potato, 5, 6, 0)
	g.Step(2 * climateInterval)
	if p, _ := growthOf(t, g, uint64(inZone)); !p.Ripe() || !p.Designated {
		t.Fatalf("zone crop after ripening: %+v", p)
	}
	if p, _ := growthOf(t, g, uint64(outside)); !p.Ripe() || p.Designated {
		t.Fatalf("crop outside every zone after ripening: %+v", p)
	}
	if p, _ := growthOf(t, g, uint64(unripe)); p.Designated {
		t.Fatalf("an unripe crop is marked: %+v", p)
	}
}

// TestMigrateHarvestMarks: an older save's ripe zone crops get the mark
// they now get on ripening; unripe ones, ones outside zones and trees
// don't, and running it twice changes nothing.
func TestMigrateHarvestMarks(t *testing.T) {
	g := mildLight(t)
	g.addGrowZone(5, 5, 6, 5, uint8(plant.Potato))
	gt := plant.DefOf(plant.Potato).GrowTicks
	ripeZone, _ := g.spawnPlant(plant.Potato, 5, 5, gt)
	young, _ := g.spawnPlant(plant.Potato, 6, 5, gt/2)
	ripeWild, _ := g.spawnPlant(plant.Potato, 9, 9, gt)
	tree, _ := g.spawnPlant(plant.Tree, 7, 5, plant.DefOf(plant.Tree).GrowTicks)
	g.migrateHarvestMarks()
	for _, c := range []struct {
		name string
		id   uint64
		want bool
	}{{"ripe zone crop", uint64(ripeZone), true}, {"young zone crop", uint64(young), false},
		{"ripe crop outside zones", uint64(ripeWild), false}, {"ripe tree", uint64(tree), false}} {
		if p, _ := growthOf(t, g, c.id); p.Designated != c.want {
			t.Errorf("%s: Designated %v, want %v", c.name, p.Designated, c.want)
		}
	}
	g.migrateHarvestMarks() // idempotent
	if p, _ := growthOf(t, g, uint64(ripeZone)); !p.Designated {
		t.Fatal("second migration lost the mark")
	}
}
