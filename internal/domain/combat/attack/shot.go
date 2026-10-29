package attack

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
// Weather × Size × Light + Offset) × (1000 − Cover). InRange and InSight
// say whether the shot can be taken at all; Melee reports a melee weapon,
// whose Chance is its flat skill-adjusted accuracy (Band).
type Report struct {
	Chance                              Permille
	Band, Shooter, Weather, Size, Light Permille
	Cover                               Permille
	Offset                              Permille // stat offset (Shooting accuracy points)
	Dist                                int
	InRange, InSight, Melee             bool
}

// Aim is the chance that the aim holds, before cover: Band × Shooter ×
// weather × size × light (in that order, each division truncating) plus
// the offset, clamped to [0, 1000].
func (r Report) Aim() Permille {
	c := int64(r.Band) * int64(r.Shooter) / one
	for _, f := range [...]int64{int64(r.Weather), int64(r.Size), int64(r.Light)} {
		c = c * max(f, 0) / one
	}
	return clamp(Permille(c) + r.Offset)
}

// Total is the chance to hit the target: the aim, then cover letting the
// shot through.
func (r Report) Total() Permille {
	if r.Melee {
		return clamp(r.Band)
	}
	return r.Aim() * (one - clamp(r.Cover)) / one
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

// Stray shots and interception. wiki: Combat
const (
	// StrayHit: a shot landing off target strikes something in the cell
	// it lands on this often (permille); otherwise it hits nothing.
	StrayHit Permille = 500
	// FriendlySafe: a shot never strikes the shooter's allies within this
	// many tiles of the muzzle.
	FriendlySafe = 5
)

// InterceptChance is the chance (permille) that a shot flying through a
// standing creature's cell strikes it: 40 percent of its body size,
// clamped to 4..80 percent. wiki: Combat (the published formula is
// garbled; this is its plain reading)
func InterceptChance(bodySize Permille) Permille {
	return min(max(bodySize*400/one, 40), 800)
}
