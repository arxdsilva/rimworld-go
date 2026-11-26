// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package difficulty

import "math"

// Adapter stores adaptation days in ten-thousandths of a day. The
// historical fields decode pre-correction saves; no old smoothing runs.
type Adapter struct {
	Days        int16
	Value       int8
	GrowthCarry uint8
	Units       int32
	Precise     bool
}

const adaptUnit = 10000

func (a Adapter) units() int32 {
	if a.Precise {
		return a.Units
	}
	return min(100*adaptUnit, max(-60*adaptUnit, int32(a.Days)*adaptUnit+int32(a.GrowthCarry)*100))
}

// AdaptDays is the shared adaptation score, in game days.
func (a Adapter) AdaptDays() float64 {
	return float64(a.units()) / adaptUnit
}

// Migrate retains old earned days and fractions, not the retired smoothed
// multiplier or incomplete daily wound tally. The first 30 days have a
// nonpositive growth ceiling (wiki: Raid points, Adaption factor; NG-2).
func (a *Adapter) Migrate(elapsedDays int) {
	if a.Precise {
		return
	}
	units := a.units()
	if elapsedDays < 30 {
		units = min(units, 0)
	}
	*a = Adapter{Units: units, Precise: true}
}

// HalfDay advances the score once per 12 elapsed game hours.
// Public daily growth rates are 3 below zero, 1 below 60 and .5 thereafter.
// https://rimworldwiki.com/wiki/Raid_points#Adaption_factor
func (a *Adapter) HalfDay(s Settings, elapsedDays int) {
	a.Migrate(elapsedDays)
	rate := int32(5000)
	switch {
	case a.Units < 0:
		rate = 15000
	case a.Units >= 60*adaptUnit:
		rate = 2500
	}
	ceiling := int32(100 * adaptUnit)
	if elapsedDays < 30 {
		ceiling = 0
	}
	a.Units = min(ceiling, a.Units+rate*int32(min(100, max(0, s.AdaptGrowthPercent())))/100)
}

// Lose applies a genuine downing or death once at its pre-event population.
// The public population knots are interpolated, with endpoint clamping.
func (a *Adapter) Lose(population int, death bool, elapsedDays int) {
	a.Migrate(elapsedDays)
	p := float64(population)
	var days float64
	if death {
		days = 30 - 10*min(1, max(0, (p-11)/9))
	} else {
		switch {
		case p <= 1:
			days = 8
		case p <= 3:
			days = 6
		case p <= 11:
			days = 6 - (p-3)/8
		default:
			days = 5 - 1.5*min(1, (p-11)/9)
		}
	}
	a.Units = max(-60*adaptUnit, a.Units-int32(math.Round(days*adaptUnit)))
}

// Factor follows the published score curve. Scaling its deviation from
// one by impact is our continuous implementation of that setting.
func (a Adapter) Factor(s Settings) float64 {
	if !s.Adaptation || s.AdaptStrength <= 0 {
		return 1
	}
	d := a.AdaptDays()
	f := 1.0
	if d < 0 {
		f = .4 + .4*min(1, max(0, (d+30)/30))
	} else {
		f = .8 + d/150
	}
	// Retain the previously supported 0..200 impact range.
	return max(0, 1+(f-1)*float64(min(s.AdaptStrength, 200))/100)
}
