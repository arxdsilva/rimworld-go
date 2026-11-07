package setupview

import (
	"slices"

	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/scenario"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/world/temperature"
)

// Map size options (NG-1 §2.6, §6.9).
var mapSides = [...]int{200, 225, 250, 275, 300, 350, 400, 450, 500}

// PreReleaseSide is the size that leads the list while no listed size is
// enabled; largeSide and up "needs a capable computer".
const (
	PreReleaseSide = 120
	largeSide      = 350
	reasonMapGate  = "Not available yet" // NG-1 §2.7: shown until the engine gate passes
)

// MapSizes lists the map size options, gated by maxEnabledSide (the
// game's MaxEnabledSide); while no listed size is enabled a 120
// "pre-release" size leads the list.
func MapSizes(maxEnabledSide int) []MapSize {
	var out []MapSize
	if maxEnabledSide < mapSides[0] {
		out = append(out, MapSize{Side: PreReleaseSide, Enabled: true, Reason: "pre-release"})
	}
	for _, s := range mapSides {
		v := MapSize{Side: s, Enabled: s <= maxEnabledSide, Large: s >= largeSide}
		if !v.Enabled {
			v.Reason = reasonMapGate
		}
		out = append(out, v)
	}
	return out
}

// CloneFactions deep-copies factions.
func CloneFactions(fs []faction.Faction) []faction.Faction {
	out := make([]faction.Faction, len(fs))
	for i, f := range fs {
		f.Settlements = slices.Clone(f.Settlements)
		f.SettlementNames = slices.Clone(f.SettlementNames)
		f.History = slices.Clone(f.History)
		f.Events = slices.Clone(f.Events)
		out[i] = f
	}
	return out
}

// WithConditions applies the permanent conditions to a climate (NG-1
// §5.1): Endless chill -8 °C; Long summer +6 °C with a 30% smaller swing.
func WithConditions(c temperature.Climate, conds []scenario.Condition) temperature.Climate {
	for _, k := range conds {
		switch k {
		case scenario.EndlessChill:
			c.YearMean -= 80
		case scenario.LongSummer:
			c.YearMean += 60
			c.Amplitude = c.Amplitude * 7 / 10
		}
	}
	return c
}

// Starting seasons (game.Season's values).
const (
	SeasonSpring = iota
	SeasonSummer
	SeasonFall
	SeasonWinter
)

// SeasonStartDay is the calendar day a season starts on for a climate
// peaking on peak (15 north, 45 south): seasons are 15 days centred on
// the temperature curve (temperature.Climate.SeasonAt), summer first.
func SeasonStartDay(peak int, s uint8) int {
	var idx int // 0 summer, 1 fall, 2 winter, 3 spring
	switch s {
	case SeasonSummer:
		idx = 0
	case SeasonFall:
		idx = 1
	case SeasonWinter:
		idx = 2
	default:
		idx = 3
	}
	q := clock.DaysPerQuadrum
	d := peak - q/2 + idx*q
	return (d%clock.DaysPerYear + clock.DaysPerYear) % clock.DaysPerYear
}
