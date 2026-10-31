package game

import (
	"rimworld-go/internal/domain/incident"
	"rimworld-go/internal/domain/scenario"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/world/temperature"
)

// Climate events (TEMP-2, incident/climate.go): storyteller nature events
// (incidents.go). While one runs its ramped offset sits in the climate's
// YearMean (g.ng.climateOff says how much), so rooms, crops, snow and the
// weather follow one outdoor curve; saves write the curve without it.

// extremeEvent is a pre-TEMP-2 save's heat wave (Kind 1) or cold snap (2),
// its Delta in the saved curve (SetupState.Extreme; migrateExtreme).
type extremeEvent struct {
	Kind        uint8
	Delta       int
	Until, Cool uint64
}

// climateTick (every tick) ends the climate event that is over and moves
// the outdoor curve to the event's offset now.
func (g *Game) climateTick(now uint64) {
	if id, ended := g.incidents.EndClimate(now); ended {
		hx, hy := g.home()
		g.message(hx, hy, "%s", incident.ClimateEndText(id))
	}
	g.applyClimate(now)
}

// applyClimate puts the climate event's offset at tick now in the curve.
func (g *Game) applyClimate(now uint64) {
	g.climate.YearMean += g.incidents.ClimateShift(now, &g.ng.climateOff)
}

// baseClimate is the outdoor curve without the climate event (saves).
func baseClimate(g *Game) (c temperature.Climate) {
	c = g.climate
	c.YearMean -= g.ng.climateOff
	return c
}

// climateFits: climate event id suits the season's temperature now.
func climateFits(g *Game, id incident.ID, now uint64) bool {
	return incident.ClimateFits(id, int(baseClimate(g).DailyMean(clock.Tick(now))))
}

// natureOdds is the nature pool's odds: the disease frequency
// (disease.go); the wizard's Extreme weather triples the climate events.
func natureOdds(g *Game) incident.Odds {
	o := incident.Odds{Disease: g.diseasePct(), Climate: incident.FullFreq}
	if g.hasSetup() && g.ng.setup.Difficulty.ExtremeWeather {
		o.Climate = incident.ExtremeClimatePct
	}
	return o
}

// migrateExtreme (save v8) makes an older save's heat wave or cold snap
// the climate event at full strength until its old end.
func migrateExtreme(g *Game) {
	e, now := g.ng.legacyExtreme, uint64(g.clock.Now())
	if g.ng.legacyExtreme = (extremeEvent{}); e.Kind == 0 {
		return
	}
	id := incident.HeatWave + incident.ID(min(e.Kind, 2)-1) // ColdSnap follows HeatWave
	g.climate.YearMean -= e.Delta
	if g.incidents.StartClimate(id, 0, e.Until) {
		g.incidents.Fired(id, g.rel(now))
	}
	g.applyClimate(max(now, 1) - 1)
}

// weatherWet is the rain and snow odds (percent of temperate) for the
// next weather block: still air allows none.
func (g *Game) weatherWet(wet int) int {
	if g.hasCondition(scenario.StillAir) {
		return 0
	}
	return wet
}
