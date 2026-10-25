package game

import (
	"strings"

	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/plant"
)

// Plant light rules in the growth pass (MAPGEN-3). A plant with MinLight
// grows only on a cell lit at least that much (ordinary moss: daylight
// or lamplight, never a dark cave). A plant with DiesInLight (duskmoss)
// withers in that much light, losing a share of its full growth each
// window it stands in it, and is removed at nothing; once withering it
// recovers only below plant.WitherStopLight (hysteresis). Species with
// neither rule never read the light: their growth is as before.

// witherNoticeRange and witherNoticeTicks: one wither notice per patch —
// a notice within this range (Chebyshev) and time covers a new death.
const (
	witherNoticeRange = 12
	witherNoticeTicks = clock.TicksPerDay
)

// lightGrows applies plant id's light rules for the window that ends now
// (p is updated in place) and reports whether it may grow this window.
// A plant withered to nothing is deleted and added to fp.gone, like a
// frost kill.
func (g *Game) lightGrows(id entity.EntityID, p *plant.Plant, fp *frostPass) bool {
	d := plant.DefOf(p.Species)
	if d.MinLight <= 0 && d.DiesInLight <= 0 {
		return true
	}
	light := g.cellLightPermil(p.X, p.Y)
	if plant.Harmed(p.Species, p.Withering, light) {
		p.Withering = true
		p.Growth -= plant.WitherLoss(p.Species, climateInterval)
		if p.Growth > 0 {
			g.plants.Set(id, *p)
			return false
		}
		g.witherAway(id, *p, fp)
		return false
	}
	if p.Withering {
		p.Withering = false
		g.plants.Set(id, *p)
	}
	return plant.LitEnough(p.Species, light)
}

// witherAway removes a plant withered to nothing (the list is compacted
// by endFrostPass) and posts one notice per patch.
func (g *Game) witherAway(id entity.EntityID, p plant.Plant, fp *frostPass) {
	g.plants.Delete(id)
	g.plantsChanged()
	g.plantLightChanged(p.Species)
	if fp.gone == nil {
		fp.gone = map[entity.EntityID]bool{}
	}
	fp.gone[id] = true
	text := witherText(p.Species)
	now := uint64(g.clock.Now())
	for i := len(g.messages) - 1; i >= 0 && g.messages[i].Tick+witherNoticeTicks > now; i-- {
		m := g.messages[i]
		if m.Text == text && max(abs(m.X-p.X), abs(m.Y-p.Y)) <= witherNoticeRange {
			return
		}
	}
	g.message(p.X, p.Y, "%s", text)
}

// witherText is the notice for a plant of species s withered by light.
func witherText(s plant.Species) string {
	name := strings.ReplaceAll(plantLabel(s), "_", " ")
	if name == "" {
		return "A plant withered in the light"
	}
	return strings.ToUpper(name[:1]) + name[1:] + " withered in the light"
}

// fillPlantLight sets the view's light needs for the inspector ("Needs
// darkness" / "Needs light", "Withering in the light").
func fillPlantLight(pv *PlantView, p plant.Plant) {
	d := plant.DefOf(p.Species)
	pv.NeedsDark, pv.NeedsLight, pv.Withering = d.DiesInLight > 0, d.MinLight > 0, p.Withering
}

// The sun-or-lamp growth rule (PLAY-017). In each growth window a plant
// grows for the longer of two spans: the sun's hours (plant.GrowSpan,
// GrowFromHour..GrowToHour, which matches the outdoor light curve), only on
// a cell with no roof over it, and every hour of the window when a lamp or
// burning campfire lights its cell at plant.StrongGrowLight or more. Under
// the open sky with no lamp that is the plain GrowSpan it always was; under
// a roof it is nothing without a strong light. Moss and duskmoss (and the
// other cave species) keep their own rules (lightGrows) and the plain span.

// growSpan is the growth plant p gains at rate (percent) over the ticks
// t0+1 .. t1.
func (g *Game) growSpan(p plant.Plant, rate int, t0, t1 uint64) int {
	if plant.OwnLightRules(p.Species) {
		return plant.GrowSpan(rate, t0, t1)
	}
	n := 0
	if !g.roofAt(p.X, p.Y).Roofed() {
		n = plant.GrowSpan(rate, t0, t1) // the sun's hours
	}
	if plant.StrongLight(g.artificialLight(p.X, p.Y)) {
		n = max(n, plant.RoundClockSpan(rate, t0, t1)) // a lamp's: every hour
	}
	return n
}

// Plant view growth states (PlantView.GrowState).
const (
	growStateGrowing = "growing"
	growStateResting = "resting" // night under the open sky, no strong lamp
	growStateDark    = "dark"    // roofed (or too dim for moss) with no strong lamp
	growStateCold    = "cold"
	growStateHot     = "hot"
	growStateRipe    = "ripe"
)

// growNow is what a view build reads once for every plant's growth state:
// the scenario's growth percent and whether the sun is up.
type growNow struct {
	gp  int
	sun bool
}

// growNowOf reads the growth conditions of this tick.
func (g *Game) growNowOf() growNow {
	return growNow{gp: g.growthPct(), sun: plant.GrowingHour(int(clock.HourOfDay(g.clock.Now())))}
}

// fillPlantGrowth sets the view's GrowState and RipeIn from the weather,
// the hour and the lamps now. They change with time rather than with the
// plant, so the view memo never carries them over.
func (g *Game) fillPlantGrowth(pv *PlantView, p plant.Plant, now growNow) {
	pv.GrowState, pv.RipeIn = g.growState(p, now)
}

// growState says why plant p is or isn't growing now, and how many ticks
// it needs to ripen at the current conditions (-1: it is ripe, or not
// growing, or withering). The estimate counts the sun's hours as 14 of
// every 24 for a plant that grows by the sun, and every hour for one under
// a strong lamp.
func (g *Game) growState(p plant.Plant, now growNow) (string, int) {
	if p.Ripe() {
		return growStateRipe, -1
	}
	d := plant.DefOf(p.Species)
	own := plant.OwnLightRules(p.Species)
	roofed := g.roofAt(p.X, p.Y).Roofed()
	lamp := !own && plant.StrongLight(g.artificialLight(p.X, p.Y))
	switch {
	case !own && roofed && !lamp:
		return growStateDark, -1
	case d.MinLight > 0 && !plant.LitEnough(p.Species, g.cellLightPermil(p.X, p.Y)):
		return growStateDark, -1
	}
	t := int(g.temperatureAt(p.X, p.Y))
	if plant.TempFactor(t) == 0 {
		if t > 0 {
			return growStateHot, -1
		}
		return growStateCold, -1
	}
	rate := g.growRate(p, t, now.gp)
	if rate <= 0 || p.Withering {
		return growStateGrowing, -1
	}
	active := ((d.GrowTicks-p.Growth)*100 + rate - 1) / rate // growing ticks still needed
	if lamp {
		return growStateGrowing, active
	}
	ripeIn := (active*24 + sunHours - 1) / sunHours // only sunHours of every 24 count
	if !own && !now.sun {
		return growStateResting, ripeIn
	}
	return growStateGrowing, ripeIn
}

// sunHours is how many hours of a day plants grow by the sun.
const sunHours = plant.GrowToHour - plant.GrowFromHour
