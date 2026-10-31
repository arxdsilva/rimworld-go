package game

import (
	"rimworld-go/internal/application/game/farm"

	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/chunk"
	"rimworld-go/internal/domain/world/plant"
)

// Plant light rules in the growth pass (MAPGEN-3). A plant with MinLight
// grows only on a cell lit at least that much (ordinary moss: daylight
// or lamplight, never a dark cave). A plant with DiesInLight (duskmoss)
// withers in that much light, losing a share of its full growth each
// window it stands in it, and is removed at nothing; once withering it
// recovers only below plant.WitherStopLight (hysteresis). Every other
// plant grows by its growing light (FOOD-2, farm.GrowLight): full under
// the open sky, a sun lamp's, a plain lamp's at most 50 %, too little.

// witherNoticeRange: one wither notice per patch a day — a notice within
// this range (Chebyshev) covers a new death.
const witherNoticeRange = 12

// lightGrows applies plant id's light rules for the window that ends now
// (p is updated in place) and reports whether it may grow this window.
// A plant withered to nothing is deleted and added to fp.Gone, like a
// frost kill.
func (g *Game) lightGrows(id entity.EntityID, p *plant.Plant, fp *farm.Pass) bool {
	d := plant.DefOf(p.Species)
	if d.MinLight <= 0 && d.DiesInLight <= 0 {
		return true
	}
	light := g.cellLightPermil(p.X, p.Y)
	if plant.Harmed(p.Species, p.Withering, light) {
		p.Withering = true
		p.Growth -= plant.WitherLoss(p.Species, climateInterval)
		if p.Growth > 0 {
			g.setPlant(id, *p)
			return false
		}
		g.plantDied(id, *p, farm.Withered, fp) // plant_growth.go
		now := uint64(g.clock.Now())
		noticeOnce(g, p.X, p.Y, farm.WitherText(p.Species), now-min(now, clock.TicksPerDay-1), witherNoticeRange)
		return false
	}
	if p.Withering {
		p.Withering = false
		g.setPlant(id, *p)
	}
	return plant.LitEnough(p.Species, light)
}

// fillPlantLight sets the view's light needs for the inspector ("Needs
// darkness" / "Needs light", "Withering in the light") and the FOOD-2
// readouts: fertility (the caller's), growing light, lifespan used, blight.
func fillPlantLight(pv *PlantView, p plant.Plant, fertility int) {
	d := plant.DefOf(p.Species)
	pv.NeedsDark, pv.NeedsLight, pv.Withering = d.DiesInLight > 0, d.MinLight > 0, p.Withering
	pv.Fertility, pv.Light, pv.Lifespan, pv.Blighted = fertility, int(p.Light), int(p.Life), p.Blight > 0
}

// Plant view growth states (PlantView.GrowState).
const (
	growStateGrowing = "growing"
	growStateResting = "resting" // the rest hours, 19:00-06:00, in any light
	growStateDark    = "dark"    // too little growing light (roofed, a plain lamp)
	growStateCold    = "cold"
	growStateHot     = "hot"
	growStateRipe    = "ripe"
)

// growNow is what a view build reads once for every plant's growth state:
// the scenario's growth percent, whether plants grow this hour and the sun
// lamps shining.
type growNow struct {
	gp    int
	sun   bool
	lamps []farm.Lamp
}

// growNowOf reads the growth conditions of this tick.
func (g *Game) growNowOf() growNow {
	now := uint64(g.clock.Now())
	return growNow{gp: g.growthPct(), sun: plant.GrowingHour(int(clock.HourOfDay(g.clock.Now()))), lamps: g.sunLamps(now)}
}

// PlantGrowthView is why a plant is or isn't growing now (PLAY-017), for
// the plant inspector. State is "growing", "resting" (the rest hours),
// "dark" (too little growing light: a sun lamp or the open sky), "cold",
// "hot" or "ripe"; RipeIn the estimated ticks until ripe at the current
// conditions (-1 unknown or not growing). It follows the hour, the
// weather and the lamps rather than the plant, so it is asked for one
// plant on demand instead of riding on every PlantView.
type PlantGrowthView struct {
	State  string
	RipeIn int
}

// PlantGrowth is plant id's growth state now (ok false: no such plant).
func (g *Game) PlantGrowth(id entity.EntityID) (PlantGrowthView, bool) {
	p, ok := g.plants.Get(id)
	if !ok {
		return PlantGrowthView{}, false
	}
	st, in := g.growState(p, g.growNowOf())
	return PlantGrowthView{State: st, RipeIn: in}, true
}

// growState says why plant p is or isn't growing now, and how many ticks
// it needs to ripen at the current conditions (-1: it is ripe, or not
// growing, or withering). The estimate counts the 13 growing hours of
// every 24 (farm.RipeIn).
func (g *Game) growState(p plant.Plant, now growNow) (string, int) {
	if p.Ripe() {
		return growStateRipe, -1
	}
	light := plant.FullGrowLight
	if plant.OwnLightRules(p.Species) {
		if !plant.LitEnough(p.Species, g.cellLightPermil(p.X, p.Y)) {
			return growStateDark, -1
		}
	} else if light = farm.GrowLight(g.roofAt(p.X, p.Y).Roofed(), g.artificialLight(p.X, p.Y), now.lamps, p.X, p.Y, g.combat.Sight.Line); plant.LightFactor(light) == 0 {
		return growStateDark, -1
	}
	t := int(g.temperatureAt(p.X, p.Y))
	if plant.TempFactor(t) == 0 {
		if t > 0 {
			return growStateHot, -1
		}
		return growStateCold, -1
	}
	rate := g.growRate(p, t, now.gp, light)
	if rate <= 0 || p.Withering {
		return growStateGrowing, -1
	}
	active := ((plant.DefOf(p.Species).GrowTicks-p.Growth)*100 + rate - 1) / rate // growing ticks still needed
	if !now.sun {
		return growStateResting, farm.RipeIn(active)
	}
	return growStateGrowing, farm.RipeIn(active)
}

// sunLamps lists the sun lamps shining at tick now: powered, in their
// hours (wiki: Sun lamp, 06:00-19:12).
func (g *Game) sunLamps(now uint64) []farm.Lamp {
	if !plant.SunLampOn(now) {
		return nil
	}
	var out []farm.Lamp
	for _, id := range g.lightSources().ids { // light.go
		if b := g.bldgs.Ref(id); b != nil && b.Kind == construction.SunLamp && g.powered(id) {
			out = append(out, farm.Lamp{X: b.X, Y: b.Y})
		}
	}
	return out
}

// shortOdds: a powered sun lamp under the open sky in rain or snow shorts
// out once in shortOdds climate intervals (wiki: Sun lamp; our odds).
const shortOdds = 20

// sunLampTick runs once a climate interval: the light grid follows the
// sun lamps' hours, and rain shorts out an unroofed shining one now and
// then, setting it alight (a roof prevents it).
func (g *Game) sunLampTick(now uint64) {
	if plant.SunLampOn(now) != plant.SunLampOn(now-min(now, climateInterval)) {
		g.light.ver++ // light.go: the lamps came on or went off
	}
	if !g.rainDouses() {
		return
	}
	for _, l := range g.sunLamps(now) {
		if !g.roofAt(l.X, l.Y).Roofed() && plant.Roll(now, uint64(l.X), uint64(l.Y))%shortOdds == 0 {
			g.startFire(l.X, l.Y, 1)
			g.threat("Short circuit", l.X, l.Y, "Rain shorted out a sun lamp under the open sky at %d,%d and set it alight. Roof it over.", l.X, l.Y)
		}
	}
}

// plantField is the game's plants as farm's blight sees them.
type plantField struct{ g *Game }

func (f plantField) PlantIDs() []entity.EntityID                  { return f.g.plantIDs }
func (f plantField) Plant(id entity.EntityID) (plant.Plant, bool) { return f.g.plants.Get(id) }
func (f plantField) SetPlant(id entity.EntityID, p plant.Plant)   { f.g.setPlant(id, p) }
func (f plantField) PlantAt(x, y int) entity.EntityID             { return f.g.plantAt(x, y) }
func (f plantField) RoomAt(x, y int) int                          { return f.g.roomAt(x, y) }

// sproutWild sprouts a wild plant back in chunk c holding wild plants
// (FOOD-2, farm.Sprout): on free fertile ground under the open sky,
// outside zones, buildings and blueprints; trees and bushes not in the
// Home area.
func (g *Game) sproutWild(cg chunk.Grid, c, wild int, now uint64) {
	x0, y0, x1, y1 := cg.Bounds(c)
	s, x, y, ok := farm.Sprout(g.biome, x0, y0, x1, y1, wild, now, c)
	if !ok || g.terr.At(x, y).Fertility() <= 0 || g.roofAt(x, y).Roofed() || !g.grid.Walkable(x, y) || g.occupiedAt(x, y) ||
		g.layerBldgAt(x, y, construction.LayerFloor) != 0 || g.layerFrameAt(x, y, construction.LayerObject) != 0 {
		return
	}
	if _, zoned := g.zones.At(x, y); zoned || !plant.DefOf(s).Ground && g.inHome(y*g.grid.Width()+x) {
		return
	}
	g.spawnPlant(s, x, y, 0) //nolint:errcheck // a plant already there: no sprout
}
