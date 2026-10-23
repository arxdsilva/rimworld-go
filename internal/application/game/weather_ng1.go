package game

import (
	"rimworld-go/internal/domain/scenario"
	"rimworld-go/internal/domain/sim/clock"
)

// Extreme weather (NG-1 §5.3, difficulty ExtremeWeather). On a summer day
// a heat wave (+12 °C) and on a winter day a cold snap (-14 °C) may start,
// extremePct percent per in-season day, rolled at midnight. It lasts 2-4
// days, shifting the whole climate curve (so rooms, crops and the weather
// follow), and no new one starts for extremeCool afterwards: the minimum
// duration and the cooldown keep it from flapping (AGENTS.md hysteresis).
// Setup colonies only; owned by SETUP.

const (
	extremePct     = 6
	extremeMinDays = 2
	extremeSpread  = 3 // days added at random: 2..4
	extremeCool    = 3 * clock.TicksPerDay
	heatWaveDelta  = 120
	coldSnapDelta  = -140
)

// Extreme event kinds.
const (
	extremeNone uint8 = iota
	extremeHeat
	extremeCold
)

// extremeEvent is the heat wave or cold snap in progress (saved).
type extremeEvent struct {
	Kind  uint8
	Delta int    // added to the climate's YearMean while it lasts
	Until uint64 // tick it ends (checked at midnight)
	Cool  uint64 // no new event before this tick
}

// extremeDay runs at midnight: end the event that is due, or maybe start
// one that fits the season.
func (g *Game) extremeDay(now uint64) {
	e := &g.ng.extreme
	if e.Kind != extremeNone {
		if now < e.Until {
			return
		}
		g.climate.YearMean -= e.Delta
		msg := "The heat wave has broken."
		if e.Kind == extremeCold {
			msg = "The cold snap is over."
		}
		g.message(g.grid.Width()/2, g.grid.Height()/2, "%s", msg)
		*e = extremeEvent{Cool: now + extremeCool}
		return
	}
	if !g.ng.setup.Difficulty.ExtremeWeather || now < e.Cool {
		return
	}
	kind, delta, title, text := extremeNone, 0, "", ""
	switch g.season(now) {
	case "Summer":
		kind, delta, title = extremeHeat, heatWaveDelta, "Heat wave"
		text = "A heat wave has settled over the land. Keep everyone out of the midday sun and find somewhere cool to sleep."
	case "Winter":
		kind, delta, title = extremeCold, coldSnapDelta, "Cold snap"
		text = "A bitter cold snap has come down from the north. Keep the fires fed and everyone indoors where you can."
	default:
		return
	}
	if g.rng.IntN(100) >= extremePct {
		return
	}
	days := extremeMinDays + g.rng.IntN(extremeSpread)
	*e = extremeEvent{Kind: kind, Delta: delta, Until: now + uint64(days)*clock.TicksPerDay}
	g.climate.YearMean += delta
	g.post(MessageView{Tick: now, Text: text, X: g.grid.Width() / 2, Y: g.grid.Height() / 2, Danger: true,
		Letter: true, Title: title})
}

// extremeKey names the event in progress for the view ("" when none).
func (g *Game) extremeKey() string {
	switch g.ng.extreme.Kind {
	case extremeHeat:
		return "heat_wave"
	case extremeCold:
		return "cold_snap"
	}
	return ""
}

// weatherWet is the rain and snow odds (percent of temperate) for the
// next weather block: still air allows none.
func (g *Game) weatherWet(wet int) int {
	if g.hasCondition(scenario.StillAir) {
		return 0
	}
	return wet
}
