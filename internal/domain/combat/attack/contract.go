// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package attack

// Step is one factor of a ranged hit chance, in the order Report.Chain
// applies them (SHT-201): the weapon band, then the shooter, weather,
// target size, light and a downed target, the clamp to [0, 1000] and
// cover last.
type Step uint8

// The steps of a ranged hit chance, in order.
const (
	StepBand Step = iota
	StepShooter
	StepWeather
	StepSize
	StepLight
	StepDowned
	StepClamp
	StepCover
	NumSteps
)

// DownedIntercept is the share (permille) of InterceptChance a downed
// creature lying in a shot's path keeps (SHT-006). Original tuning, not a
// published value (.agents/specs/CLEAN_ROOM.md).
const DownedIntercept Permille = 250

// Chain is the cumulative hit chance (permille) after each step: entry
// StepClamp is Aim and entry StepCover is Total. A melee report holds its
// Total at every step. wiki: Weapons (Accuracy)
func (r Report) Chain() [NumSteps]Permille {
	if r.Melee {
		var out [NumSteps]Permille
		total := r.Total()
		for i := range out {
			out[i] = total
		}
		return out
	}
	return r.ranged()
}

// ranged is the ranged chain, whatever Melee says, so Aim keeps its meaning.
func (r Report) ranged() [NumSteps]Permille {
	var out [NumSteps]Permille
	c := int64(r.Band)
	out[StepBand] = Permille(c)
	c = c * int64(r.Shooter) / one
	out[StepShooter] = Permille(c)
	for i, f := range [...]int64{int64(r.Weather), int64(r.Size), int64(r.Light)} {
		c = c * max(f, 0) / one
		out[StepWeather+Step(i)] = Permille(c)
	}
	if r.Downed != 0 { // zero is neutral
		c = c * max(int64(r.Downed), 0) / one
	}
	out[StepDowned] = Permille(c)
	aim := clamp(Permille(c))
	out[StepClamp] = aim
	out[StepCover] = aim * (one - clamp(r.Cover)) / one
	return out
}

// Manipulation and sight weights: shooting score points per 100 % of the
// capacity beyond (or below) healthy. wiki: Shooting accuracy
const (
	ManipWeight = 8  // capped at 100 %
	SightWeight = 12 // capped at 200 %
)

// ManipPoints are the shooting score points manipulation (permille, 1000
// healthy) adds: 8 per 100 %, capped at 100 % (SHT-001), truncating toward
// zero.
func ManipPoints(manip int) int { return (min(manip, 1000) - 1000) * ManipWeight / 1000 }

// SightPoints are the shooting score points sight (permille, 1000
// healthy) adds: 12 per 100 %, capped at 200 % (SHT-003), truncating
// toward zero.
func SightPoints(sight int) int { return (min(sight, 2000) - 1000) * SightWeight / 1000 }

// DownedFactor is the hit-chance factor (permille) for a ranged shot at a
// downed target dist tiles away (SHT-008): 8.5 up to 3 tiles, 1 at 4 and
// 0.5 from 5 on; a negative dist counts as 0. wiki: Combat (downed targets)
func DownedFactor(dist int) Permille {
	switch d := max(dist, 0); {
	case d <= 3:
		return 8500
	case d == 4:
		return one
	}
	return 500
}
