package game

import (
	"math"
	"testing"

	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/world/plant"
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
