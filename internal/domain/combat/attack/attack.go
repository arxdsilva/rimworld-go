package attack

// Permille is a probability or factor in thousandths.
type Permille = int

// Band is one accuracy sample: Acc at Dist tiles.
type Band struct {
	Dist int
	Acc  Permille
}

// Bands are the four vanilla accuracy samples of a weapon, with
// increasing Dist. The zero value means "no bands" (Zero reports it); the
// weapon package then uses the legacy linear falloff.
type Bands struct {
	Touch, Short, Medium, Long Band
}

// Zero reports whether b is unset.
func (b Bands) Zero() bool { return b == Bands{} }

// DefaultBands are the wiki's typical distances (3/12/25/40 tiles) with
// accuracies supplied by the weapon. wiki: Weapons / Accuracy
func DefaultBands(touch, short, medium, long Permille) Bands {
	return Bands{
		Touch:  Band{3, touch},
		Short:  Band{12, short},
		Medium: Band{25, medium},
		Long:   Band{40, long},
	}
}

// Accuracy interpolates linearly between the band samples; before Touch
// it is Touch.Acc and beyond Long it is Long.Acc (range is checked
// elsewhere). Division truncates toward zero.
func (b Bands) Accuracy(dist int) Permille {
	pts := [4]Band{b.Touch, b.Short, b.Medium, b.Long}
	if dist <= pts[0].Dist {
		return pts[0].Acc
	}
	for i := 1; i < 4; i++ {
		if dist <= pts[i].Dist {
			a, c := pts[i-1], pts[i]
			span := c.Dist - a.Dist
			if span <= 0 {
				return c.Acc
			}
			return a.Acc + (c.Acc-a.Acc)*(dist-a.Dist)/span
		}
	}
	return pts[3].Acc
}

// Burst is the fire pattern of one attack: Shots per trigger pull with
// BetweenTicks between shots. Shots 0 is read as 1.
type Burst struct {
	Shots        int
	BetweenTicks int
}

// Count returns the number of shots, at least 1.
func (b Burst) Count() int {
	if b.Shots < 1 {
		return 1
	}
	return b.Shots
}

// Duration is the ticks from the first to the last shot of a burst.
func (b Burst) Duration() int {
	return (b.Count() - 1) * b.BetweenTicks
}

// Timing is the warmup before a shot and the cooldown after it.
type Timing struct {
	Warmup, Cooldown int
}

// Cycle is the ticks one full attack occupies: warmup, burst, cooldown.
func Cycle(t Timing, b Burst) int {
	return t.Warmup + b.Duration() + t.Cooldown
}

const one = 1000

// ppm is the fixed-point scale of per-tile shooter accuracy.
const ppm = 1_000_000

// skillPoints are the wiki's shooter accuracy per tile (ppm) at skill
// levels, linearly interpolated; clamped outside. wiki: Shooting
var skillPoints = [...]struct{ skill, ppm int }{
	{-20, 700_000}, {-10, 800_000}, {0, 890_000}, {5, 945_000},
	{10, 970_000}, {15, 981_667}, {20, 990_000}, {30, 996_500},
	{40, 998_000}, {60, 999_000},
}

// ShooterPerTile is the accuracy per tile of distance (ppm, 1_000_000 =
// 100 percent) for an effective shooting skill; careful and trigger-
// happy shooters pass skill +5 and -5. wiki: Shooting
func ShooterPerTile(skill int) int {
	p := skillPoints[:]
	if skill <= p[0].skill {
		return p[0].ppm
	}
	for i := 1; i < len(p); i++ {
		if skill <= p[i].skill {
			a, c := p[i-1], p[i]
			return a.ppm + (c.ppm-a.ppm)*(skill-a.skill)/(c.skill-a.skill)
		}
	}
	return p[len(p)-1].ppm
}

// PerfectShooter is a per-tile accuracy of 100 percent.
const PerfectShooter = ppm

// Weather and smoke factors (permille). wiki: Weapons (Accuracy)
const (
	ClearFactor = 1000
	FogFactor   = 500
	// SmokeFactor applies when smoke lies between shooter and target.
	// wiki conflict: the page lists "30" without saying whether it is
	// the factor or the loss; the factor reading is used.
	SmokeFactor = 300
)

// NoLightPenalty is the lighting factor: the wiki publishes no base
// penalty. original tuning: not published
const NoLightPenalty = 1000

// pow returns (perTile/1e6)^dist in ppm by repeated fixed-point
// multiplication, deterministic and monotone.
func pow(perTile, dist int) int64 {
	c := int64(ppm)
	for i := 0; i < dist; i++ {
		c = c * int64(perTile) / ppm
	}
	return c
}

// HitChance is the vanilla composition: shooter accuracy per tile raised
// to the distance, times the band accuracy, weather, smoke and what
// cover lets through (1000 - coverPermille), times the target size and
// light factors (permille, 1000 neutral). The result is clamped to
// [0, 1000]. Each product is divided in turn, truncating, so the order is
// fixed. wiki: Weapons (Accuracy), Shooting, Cover
func HitChance(b Bands, dist int, perTilePPM, coverPermille, targetSizeFactor, lightFactor, weatherFactor, smokeFactor Permille) Permille {
	c := int64(b.Accuracy(dist)) * pow(perTilePPM, dist) / ppm
	for _, f := range [...]int64{
		int64(weatherFactor), int64(smokeFactor), int64(one - clamp(coverPermille)),
		int64(targetSizeFactor), int64(lightFactor),
	} {
		if f < 0 {
			f = 0
		}
		c = c * f / one
	}
	if c > one {
		c = one
	}
	if c < 0 {
		c = 0
	}
	return Permille(c)
}

func clamp(p Permille) Permille {
	if p < 0 {
		return 0
	}
	if p > one {
		return one
	}
	return p
}
