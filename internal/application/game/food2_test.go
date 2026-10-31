package game

import (
	"testing"

	"rimworld-go/internal/application/game/farm"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/incident"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/plant"
)

// FOOD-2 growing at the wiki's numbers: the growth model, partial harvests,
// lifespans, hydroponics, the sun lamp, blight and wild regrowth.

// openSky lifts the roof of mildLight's map over the rectangle.
func openSky(g *Game, x0, y0, x1, y1 int) {
	for y := y0; y <= y1; y++ {
		for x := x0; x <= x1; x++ {
			g.setRoof(y*g.grid.Width()+x, 0)
		}
	}
}

// growthAt is plant id's growth units (0 if gone).
func growthAt(g *Game, id entity.EntityID) int {
	p, _ := g.plants.Get(id)
	return p.Growth
}

// sunPower puts the device at (x, y) on a net strong enough for a sun lamp:
// a conduit run east from it with n fuelled wood-fired generators along it.
func sunPower(t *testing.T, g *Game, x, y, n int) {
	t.Helper()
	for i := 0; i <= 3*n; i++ {
		buildNow(t, g, construction.PowerConduit, item.Steel, construction.South, x+i, y)
	}
	for i := 0; i < n; i++ {
		gen := buildNow(t, g, construction.WoodGenerator, item.Steel, construction.South, x+1+3*i, y-2)
		g.setFuel(gen, g.fuelCap(gen))
	}
	g.balancePower(uint64(g.clock.Now()))
}

// TestRiceRipensInFiveAndAHalfDays (wiki: Rice plant, 3 grow days, 5.54
// real days on soil): plants grow 13 hours a day.
func TestRiceRipensInFiveAndAHalfDays(t *testing.T) {
	t.Parallel()
	g := mildLight(t)
	openSky(g, 20, 20, 20, 20)
	id, _ := g.spawnPlant(plant.Rice, 20, 20, 0)
	start := uint64(g.clock.Now())
	for !plantRipe(g, id) && uint64(g.clock.Now())-start < 10*clock.TicksPerDay {
		g.Step(climateInterval)
	}
	days := float64(uint64(g.clock.Now())-start) / clock.TicksPerDay
	if days < 5.0 || days > 6.0 {
		t.Fatalf("rice ripened in %.2f days, wiki 5.54", days)
	}
	if pv, _ := plantView(g, id); pv.Fertility != 100 || pv.Light != 100 || pv.Lifespan <= 30 || pv.Lifespan > 45 {
		t.Fatalf("view: fertility %d light %d lifespan %d%%", pv.Fertility, pv.Light, pv.Lifespan)
	}
}

// plantRipe reports plant id ripe.
func plantRipe(g *Game, id entity.EntityID) bool {
	p, ok := g.plants.Get(id)
	return ok && p.Ripe()
}

// TestCropRestsAtNightUnderALamp (wiki: Plants, rest 19:00-06:00 in any
// light): an open-sky crop beside a lit lamp stands still at night and
// grows by day.
func TestCropRestsAtNightUnderALamp(t *testing.T) {
	t.Parallel()
	g := mildLight(t)
	openSky(g, 9, 9, 12, 12)
	litLamp(t, g, 10, 10, 12, 11)
	id, _ := g.spawnPlant(plant.Potato, 11, 10, 0)
	jumpTo(g, 10, 19)
	g.Step(climateInterval)
	before := growthAt(g, id)
	if st, _ := g.PlantGrowth(id); st.State != growStateResting {
		t.Fatalf("at night: %q, want resting", st.State)
	}
	g.Step(10 * clock.TicksPerHour) // to 05:00
	if got := growthAt(g, id); got != before {
		t.Fatalf("grew %d at night beside a lamp", got-before)
	}
	g.Step(4 * clock.TicksPerHour) // to 09:00
	if growthAt(g, id) <= before {
		t.Fatal("no growth in the morning")
	}
}

// TestPlainLampGrowsNoCropSunLampDoes (wiki: Sun lamp, a standing lamp's 50 %
// is under the 51 % crops need): under a roof a lamp-lit crop stands
// still, a crop within 5.5 tiles of a sun lamp grows as under the sky.
func TestPlainLampGrowsNoCropSunLampDoes(t *testing.T) {
	t.Parallel()
	g := mildLight(t)
	litLamp(t, g, 5, 5, 5, 7)
	lamp, _ := g.spawnPlant(plant.Rice, 6, 5, 0)
	sun := buildNow(t, g, construction.SunLamp, item.Steel, construction.South, 20, 20)
	sunPower(t, g, 20, 20, 4)
	if !g.powered(sun) {
		t.Fatal("the sun lamp has no power")
	}
	near, _ := g.spawnPlant(plant.Rice, 23, 22, 0) // 3.6 tiles
	far, _ := g.spawnPlant(plant.Rice, 20, 28, 0)  // 8 tiles: 50 %
	openSky(g, 35, 35, 35, 35)
	sky, _ := g.spawnPlant(plant.Rice, 35, 35, 0)
	jumpTo(g, 11, 6)
	g.Step(6 * clock.TicksPerHour)
	if on, _ := glowView(t, g, sun); !on {
		t.Fatal("the shining sun lamp does not glow")
	}
	g.Step(18 * clock.TicksPerHour)
	if n := growthAt(g, lamp); n != 0 {
		t.Fatalf("a crop beside a plain lamp grew %d", n)
	}
	if st, _ := g.PlantGrowth(lamp); st.State != growStateDark {
		t.Fatalf("lamp-lit crop state %q, want dark", st.State)
	}
	if n := growthAt(g, far); n != 0 {
		t.Fatalf("a crop 8 tiles from the sun lamp grew %d", n)
	}
	a, b := growthAt(g, near), growthAt(g, sky)
	if a == 0 || a < b*9/10 || a > b*11/10 {
		t.Fatalf("under the sun lamp %d, under the sky %d", a, b)
	}
	if pv, _ := plantView(g, near); pv.Light != 100 {
		t.Fatalf("sun lamp light %d%%", pv.Light)
	}
	if pv, _ := plantView(g, lamp); pv.Light != 50 {
		t.Fatalf("lamp light %d%%", pv.Light)
	}
	jumpTo(g, 12, 20)
	g.Step(climateInterval)
	if on, _ := glowView(t, g, sun); on {
		t.Fatal("the sun lamp glows at night")
	}
}

// TestHydroponicsBasin (wiki: Hydroponics basin, 280 %, plants die without
// power): rice in a powered basin grows 2.8 times as fast as on soil;
// cut the power and it withers within a day.
func TestHydroponicsBasin(t *testing.T) {
	t.Parallel()
	g := mildLight(t)
	openSky(g, 0, 0, 39, 39)
	basin := buildNow(t, g, construction.HydroponicsBasin, item.Steel, construction.South, 10, 10)
	gen := onPower(t, g, 10, 10, 12, 10)
	if !g.powered(basin) {
		t.Fatal("the basin has no power")
	}
	hydro, _ := g.spawnPlant(plant.Rice, 10, 11, 0)
	soil, _ := g.spawnPlant(plant.Rice, 30, 30, 0)
	jumpTo(g, 11, 6)
	g.Step(clock.TicksPerDay)
	a, b := growthAt(g, hydro), growthAt(g, soil)
	if b == 0 || a < b*27/10 || a > b*29/10 {
		t.Fatalf("a day's growth: basin %d, soil %d (want 2.8x)", a, b)
	}
	if pv, _ := plantView(g, hydro); pv.Fertility != plant.HydroFertile {
		t.Fatalf("basin fertility %d", pv.Fertility)
	}
	g.setFuel(gen, 0)
	g.balancePower(uint64(g.clock.Now()))
	if g.powered(basin) {
		t.Fatal("still powered")
	}
	start := uint64(g.clock.Now())
	for plantAlive(g, hydro) && uint64(g.clock.Now())-start < 2*clock.TicksPerDay {
		g.Step(climateInterval)
	}
	if plantAlive(g, hydro) || uint64(g.clock.Now())-start > clock.TicksPerDay {
		t.Fatalf("unpowered basin rice alive %v after %d ticks", plantAlive(g, hydro), uint64(g.clock.Now())-start)
	}
	if countText(g, farm.KillText(plant.Rice, 1, farm.Starved)) != 1 {
		t.Fatal("no withered notice")
	}
	// growers sow a basin only with a hydroponic crop: rice yes, corn no
	buildNow(t, g, construction.HydroponicsBasin, item.Steel, construction.South, 16, 10)
	g.addGrowZone(10, 10, 10, 13, uint8(plant.Rice))
	g.addGrowZone(16, 10, 16, 13, uint8(plant.Corn))
	if !g.sowable(10, 12) {
		t.Fatal("a zoned basin cell is not sowable with rice")
	}
	if g.sowable(16, 12) {
		t.Fatal("a basin takes corn")
	}
}

// plantAlive reports plant id still standing.
func plantAlive(g *Game, id entity.EntityID) bool {
	_, ok := g.plants.Get(id)
	return ok
}

// TestPartialHarvestAtSeventyPercent (wiki: Plants, half the yield at 65 %,
// linear to all of it ripe): a colonist harvests corn marked at 70 %.
func TestPartialHarvestAtSeventyPercent(t *testing.T) {
	t.Parallel()
	g := mildLight(t)
	openSky(g, 0, 0, 39, 39)
	id, _ := g.SpawnNamedPawn(5, 5, "Ada")
	setSkill(g, id, skPlants, 10)
	corn, _ := g.spawnPlant(plant.Corn, 8, 5, plant.DefOf(plant.Corn).GrowTicks*70/100+100)
	g.Enqueue(command.Command{Kind: command.DesignateHarvest, X: 8, Y: 5, X2: 8, Y2: 5})
	g.ApplyPending()
	p, _ := g.plants.Get(corn)
	if !p.Designated {
		t.Fatal("corn at 70 % could not be marked for harvest")
	}
	want := p.HarvestYield(g.skillYield(id, skPlants, g.diffYield(DiffCrop, 22)))
	for i := 0; i < 40 && plantAlive(g, corn); i++ {
		g.SetNeeds(id, needs.Full, needs.Full)
		g.Step(100)
	}
	st, ok := stackOf(g, item.Corn)
	if !ok || st.Qty != want || want >= 22 || want < 11 {
		t.Fatalf("harvest at 70 %%: %d corn (found %v), want %d", st.Qty, ok, want)
	}
}

// TestOldCropDiesOfAge (wiki: Plants, lifespan = grow days x 5): ripe
// rice left standing rots away at 15 days old.
func TestOldCropDiesOfAge(t *testing.T) {
	t.Parallel()
	g := mildLight(t)
	openSky(g, 0, 0, 39, 39)
	id, _ := g.spawnPlant(plant.Rice, 10, 10, plant.DefOf(plant.Rice).GrowTicks)
	g.Step(climateInterval)
	p, _ := g.plants.Get(id)
	if p.Sown == 0 {
		t.Fatal("the lifespan clock did not start")
	}
	p.Sown = uint64(g.clock.Now()) - plant.Lifespan(plant.Rice) + 2*climateInterval
	g.setPlant(id, p)
	if pv, _ := plantView(g, id); pv.Lifespan < 99 {
		g.Step(climateInterval)
	}
	g.Step(3 * climateInterval)
	if plantAlive(g, id) {
		t.Fatal("15-day-old rice still standing")
	}
	if countText(g, "A rice plant died of old age") != 1 {
		t.Fatal("no old-age notice")
	}
}

// blightField sows rice zone plants over the rectangle.
func blightField(g *Game, x0, y0, x1, y1 int) []entity.EntityID {
	var out []entity.EntityID
	for y := y0; y <= y1; y++ {
		for x := x0; x <= x1; x++ {
			id, _ := g.spawnPlant(plant.Rice, x, y, 0)
			p, _ := g.plants.Get(id)
			p.Zone = true
			g.setPlant(id, p)
			out = append(out, id)
		}
	}
	return out
}

// TestBlightStrikesOneRoomAndSpreadsFourTiles (wiki: Blight): the strike
// stays in the origin's room (a walled field beside it is spared), and
// every later case lies within 4 tiles of an earlier one.
func TestBlightStrikesOneRoomAndSpreadsFourTiles(t *testing.T) {
	t.Parallel()
	g := mildLight(t)
	openSky(g, 0, 0, 39, 39)
	out := blightField(g, 4, 4, 12, 12)
	for x := 16; x <= 24; x++ { // a walled shed, 4 tiles east
		for _, y := range []int{3, 13} {
			buildNow(t, g, construction.Wall, item.Wood, construction.South, x, y)
		}
	}
	for y := 3; y <= 13; y++ {
		buildNow(t, g, construction.Wall, item.Wood, construction.South, 16, y)
		buildNow(t, g, construction.Wall, item.Wood, construction.South, 24, y)
	}
	in := blightField(g, 18, 5, 22, 11)
	now := uint64(g.clock.Now())
	for o, ok := farm.Origin(plantField{g}, now); !ok || !contains(out, o); o, ok = farm.Origin(plantField{g}, now) {
		g.clock.Advance(1)
		now = uint64(g.clock.Now())
	}
	if !g.fireIncident(incident.CropBlight, now) {
		t.Fatal("no blight")
	}
	struck := 0
	for _, id := range out {
		if p, _ := g.plants.Get(id); p.Blight == plant.BlightStart {
			struck++
		}
	}
	for _, id := range in {
		if p, _ := g.plants.Get(id); p.Blight != 0 {
			t.Fatal("the blight crossed the wall at the strike")
		}
	}
	if struck == 0 {
		t.Fatal("the strike blighted nothing")
	}
	if pv, _ := plantView(g, out[0]); pv.Blighted != (growthBlight(g, out[0]) > 0) {
		t.Fatal("PlantView.Blighted does not follow the plant")
	}
	sick := map[entity.EntityID]bool{}
	for _, id := range append(out, in...) {
		if growthBlight(g, id) > 0 {
			sick[id] = true
		}
	}
	spread := 0
	for i := 0; i < 8*clock.TicksPerDay/climateInterval; i++ {
		g.Step(climateInterval)
		for _, id := range append(out, in...) {
			if sick[id] || growthBlight(g, id) == 0 {
				continue
			}
			p, _ := g.plants.Get(id)
			nearOne := false
			for s := range sick {
				q, _ := g.plants.Get(s)
				if dx, dy := q.X-p.X, q.Y-p.Y; dx*dx+dy*dy <= plant.BlightRadius*plant.BlightRadius {
					nearOne = true
				}
			}
			if !nearOne {
				t.Fatalf("blight jumped to %d,%d, over 4 tiles from any case", p.X, p.Y)
			}
			sick[id] = true
			spread++
		}
	}
	if spread == 0 {
		t.Fatal("the blight never spread")
	}
	p, _ := g.plants.Get(out[0])
	if p.HarvestYield(6) != 0 && p.Blight > 0 {
		t.Fatal("a blighted plant yields")
	}
}

// growthBlight is plant id's blight severity.
func growthBlight(g *Game, id entity.EntityID) int {
	p, _ := g.plants.Get(id)
	return int(p.Blight)
}

func contains(ids []entity.EntityID, id entity.EntityID) bool {
	for _, x := range ids {
		if x == id {
			return true
		}
	}
	return false
}

// TestWildPlantsRegrow (wiki: Plants): a cleared map greens over again,
// never inside a zone.
func TestWildPlantsRegrow(t *testing.T) {
	t.Parallel()
	g := mildLight(t)
	openSky(g, 0, 0, 39, 39)
	g.ClearPlants(0, 0, 39, 39)
	g.addGrowZone(2, 2, 12, 12, uint8(plant.Rice))
	g.Step(3 * clock.TicksPerDay)
	n := 0
	for _, id := range g.plantIDs {
		p, _ := g.plants.Get(id)
		if p.Zone {
			continue
		}
		n++
		if p.X >= 2 && p.X <= 12 && p.Y >= 2 && p.Y <= 12 {
			t.Fatalf("%s sprouted in the growing zone at %d,%d", p.Species, p.X, p.Y)
		}
	}
	if n < 10 {
		t.Fatalf("%d wild plants sprouted in 3 days", n)
	}
}

// TestCropFrostLimitNoFlap: air hovering at a crop's own frost limit starts
// the damage once and keeps it going; it stops only 2 °C back (the damage
// hysteresis, per plant since FOOD-2).
func TestCropFrostLimitNoFlap(t *testing.T) {
	t.Parallel()
	g := mildLight(t)
	id, _ := g.spawnPlant(plant.Rice, 10, 10, 1000)
	p, _ := g.plants.Get(id)
	lim, _ := p.ColdLimit()
	flips, was := 0, false
	for i := 0; i < 16 && plantAlive(g, id); i++ {
		setAir(g, 10, 10, lim-1+2*(i%2)) // just past it, then just short of the stop line
		frostFor(g, climateInterval)
		p, _ = g.plants.Get(id)
		if p.Damaging != was {
			flips++
			was = p.Damaging
		}
	}
	if flips != 1 {
		t.Fatalf("frost damage flipped %d times at the limit", flips)
	}
	if countText(g, farm.HurtText(plant.Rice, false)) != 1 {
		t.Fatal("the frost warning repeated")
	}
}
