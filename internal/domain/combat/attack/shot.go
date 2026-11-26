// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package attack

import "math"

// ShooterFactor is the shooter's share of a hit chance (permille): its
// accuracy per tile (ppm) raised to the distance. wiki: Shooting
func ShooterFactor(perTilePPM, dist int) Permille {
	return Permille(pow(perTilePPM, dist) / (ppm / one))
}

// SizeFactor is the target-size factor (permille) for a body size in
// permille (a human is 1000): the body size clamped to 0.5..2.0. A
// one-tile structure counts as 1000 (its cover effectiveness × size ×
// 2.5 for a sentry turret's 40 percent). wiki: Weapons (Accuracy)
func SizeFactor(bodySize Permille) Permille { return min(max(bodySize, 500), 2000) }

// Report is a hit chance with its factors (permille, 1000 neutral), for a
// shooter at Dist tiles from its target: Chance = clamp(Band × Shooter ×
// Weather × Size × Light) × (1000 − Cover), a pure product with no flat
// term (Shooting accuracy points are in Shooter, through ShootingScore;
// SHT-002). InRange and InSight say whether the shot can be taken at all;
// Melee reports a melee weapon, whose Chance is its flat skill-adjusted
// accuracy (Band).
type Report struct {
	Chance                              Permille
	Band, Shooter, Weather, Size, Light Permille
	Cover                               Permille
	// Downed is the factor for a downed target by distance (SHT-008,
	// DownedFactor); zero is neutral.
	Downed                  Permille
	Dist                    int
	InRange, InSight, Melee bool
}

// Aim is the chance that the aim holds, before cover: Band × Shooter ×
// weather × size × light (in that order, each division truncating),
// clamped to [0, 1000].
func (r Report) Aim() Permille { return r.ranged()[StepClamp] }

// Total is the chance to hit the target: the aim, then cover letting the
// shot through.
func (r Report) Total() Permille {
	if r.Melee {
		return clamp(r.Band)
	}
	return r.ranged()[StepCover]
}

// missPoints is the miss radius (thousandths of a tile) by aim chance
// (permille), descending; between points it is linear, past the last it
// stays. wiki: Combat (miss radius by accuracy)
var missPoints = [...]struct{ aim, r int }{
	{1000, 1000}, {220, 2000}, {110, 4000}, {70, 6000}, {40, 8000}, {20, 10000},
}

// MissRadius is how far (tiles) from its target a shot whose aim failed
// can land, from the aim chance it had: a good aim misses close by.
func MissRadius(aim Permille) int {
	p := missPoints[:]
	if aim >= p[0].aim {
		return p[0].r / one
	}
	for i := 1; i < len(p); i++ {
		if aim >= p[i].aim {
			a, c := p[i-1], p[i]
			return (c.r + (a.r-c.r)*(aim-c.aim)/(a.aim-c.aim)) / one
		}
	}
	return p[len(p)-1].r / one
}

// StrayHit: a shot landing off target strikes something in the cell it
// lands on this often (permille); otherwise it hits nothing. wiki: Combat
const StrayHit Permille = 500

// InterceptChance is the chance (permille) that a shot flying through a
// standing creature's cell strikes it: 40 percent of its body size,
// clamped to 4..80 percent. wiki: Combat (the published formula is
// garbled; this is its plain reading). The caller scales it by
// InterceptScale of the distance from the muzzle (SHT-005).
func InterceptChance(bodySize Permille) Permille {
	return min(max(bodySize*400/one, 40), 800)
}

// DistanceInterceptFactor is the interception distance factor (permille)
// for a cell at (dx, dy) tiles from the muzzle: 0 up to InterceptNear
// tiles, 1000 from InterceptFull, linear in Euclidean distance between.
func DistanceInterceptFactor(dx, dy int) Permille {
	const lo, hi = InterceptNear * InterceptNear, InterceptFull * InterceptFull
	d2 := dx*dx + dy*dy
	switch {
	case d2 <= lo:
		return 0
	case d2 >= hi:
		return one
	}
	milli := int(math.Sqrt(float64(d2) * one * one))
	return Permille((milli - InterceptNear*one) / (InterceptFull - InterceptNear))
}

// DownedInterceptChance is one tenth of the standing interception chance.
func DownedInterceptChance(chance Permille) Permille { return chance * DownedIntercept / one }

// FriendlyFireChance applies the player friendly-fire difficulty scale.
func FriendlyFireChance(chance Permille, percent int) Permille {
	return chance * Permille(min(max(percent, 0), 100)) / 100
}
