// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package difficulty

import (
	"math"
	"rimworld-go/internal/domain/sim/clock"
)

// ThreatBase contains the authoritative local assets, in native value units.
// PawnWeight is the number of effective adult pawns; AnimalPower is the
// health-weighted combat power of eligible animals, not their raid points.
type ThreatBase struct {
	Wealth, PawnWeight, AnimalPower float64
}

// PawnWeight uses published age, stasis and temporary injury factors.
// The caller must exclude dead pawns, prisoners and foreign pawns.
func PawnWeight(ageYears, healthFraction float64, asleep bool) float64 {
	w := math.Min(1, math.Max(0, (ageYears-3)/20))
	if ageYears >= 13 {
		w = math.Min(1, 0.5+(ageYears-13)/10)
	}
	if asleep {
		w *= 0.30
	}
	return w * InjuryWeight(healthFraction)
}

// InjuryWeight reduces pawn points for temporary injuries (wiki: Raid points).
// Original tuning: 65% injury sensitivity; a fully injured pawn retains 35%
// of its health-based weight.
func InjuryWeight(healthFraction float64) float64 {
	return .35 + .65*min(1, max(0, healthFraction))
}

// ThreatPointsWeighted is the setup storyteller's published wealth and pawn
// curves (https://rimworldwiki.com/wiki/Raid_points). Existing custom
// wealth-independent and escalation settings retain their native semantics.
func ThreatPointsWeighted(s Settings, b ThreatBase, ticks uint64, yearDays int, adaptFactor float64) int {
	w := math.Max(0, b.Wealth)
	years := float64(ticks) / (float64(max(1, yearDays)) * clock.TicksPerDay)
	base := weightedCurve(w, 14000, 400000, 700000, 1000000, 0, 2400, 3600, 4200) +
		weightedCurve(w, 10000, 400000, 1000000, 1000000, 15, 140, 200, 200)*math.Max(0, b.PawnWeight)
	animal := math.Max(0, b.AnimalPower) * 0.08
	if s.IgnoreWealth {
		// This existing custom mode is intentionally not the vanilla wealth curve.
		base = colonistPts*math.Max(0, b.PawnWeight) + timePtsPerYr*years + animal
	} else {
		base = (base + animal) * (0.7 + 0.3*math.Min(1, math.Max(0, (float64(ticks)/clock.TicksPerDay-10)/30)))
	}
	if !s.Adaptation {
		adaptFactor = 1
	}
	pts := base * float64(max(0, s.ThreatScale)) / 100 * max(0, adaptFactor)
	if s.TimeEscalation {
		pts *= math.Max(0, 1+float64(s.EscPerYear)*years/100)
	}
	return int(math.Round(math.Max(pointsFloor, math.Min(PointsCap, pts))))
}

func weightedCurve(x, a, b, c, d, u, v, w, z float64) float64 {
	switch {
	case x <= a:
		return u
	case x < b:
		return u + (v-u)*(x-a)/(b-a)
	case x < c:
		return v + (w-v)*(x-b)/(c-b)
	case x < d:
		return w + (z-w)*(x-c)/(d-c)
	default:
		return z
	}
}

// ScaleRaidPoints applies a native raid multiplier without overflowing.
// Saturate the product, not the input: an explicit large budget may still
// exceed the cap after a reducing faction multiplier.
func ScaleRaidPoints(points, pct int) int {
	if points <= 0 || pct <= 0 {
		return 0
	}
	if points > PointsCap*100/pct {
		return PointsCap
	}
	return points * pct / 100
}
