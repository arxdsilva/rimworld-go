// Package stain is blood and ichor on the ground (SIM-020): how strong a
// stain is, how it fades over a week and how rain and snow wash it off
// open ground. Pure and deterministic; the game owns the stains.
package stain

import "rimworld-go/internal/domain/sim/clock"

// Kind is what was spilled.
type Kind uint8

const (
	Blood Kind = iota // creatures and people
	Ichor             // hive insects
)

func (k Kind) String() string {
	if k == Ichor {
		return "ichor"
	}
	return "blood"
}

// Stain is one stained tile.
type Stain struct {
	X, Y    int
	Kind    Kind
	Amount  uint16 // 0..Full: how strong it is now
	Born    uint64 // tick of the last fresh blood (it dries from here)
	Variant uint8  // which splat shape, a hash of the tile
	Rem     int    // fade/wash remainder (Fade, Wash frac); saved so a load continues identically
}

const (
	// Full is the strongest stain.
	Full = 1000
	// FadeTicks: a full stain fades to nothing in 7 days.
	FadeTicks = 7 * clock.TicksPerDay
	// RainWash: a full stain washes off in 2 h of intensity-1 rain;
	// SnowWash in 3 h of snow.
	RainWash = 2 * clock.TicksPerHour
	SnowWash = 3 * clock.TicksPerHour
	// Cap is the most stains kept on a map (the faintest go first).
	Cap = 2000
)

// Wash and Fade share one remainder per stain, counted in units of
// 1/FadeTicks of an amount point, so every step is integer math and
// stepped calls sum exactly to one big call.
const (
	rainScale = FadeTicks / RainWash
	snowScale = FadeTicks / SnowWash
)

// drain takes scaled (1/FadeTicks amount points) off a, carrying the
// remainder in frac. A stain that reaches nothing drops its remainder.
func drain(a uint16, scaled int, frac *int) uint16 {
	acc := *frac + scaled
	dec := acc / FadeTicks
	*frac = acc % FadeTicks
	if dec >= int(a) {
		*frac = 0
		return 0
	}
	return a - uint16(dec)
}

// Fade is a after ticks of fading (linear: a full stain is gone after
// FadeTicks). frac carries the remainder between calls so steps sum
// exactly.
func Fade(a uint16, ticks int, frac *int) uint16 {
	if ticks <= 0 || a == 0 {
		return a
	}
	return drain(a, Full*ticks, frac)
}

// Wash is a after ticks of rain (snow: snowfall) at intensity 0..1 on
// open ground: a full stain washes off in RainWash (SnowWash) at
// intensity 1. frac is the same remainder Fade uses.
func Wash(a uint16, ticks int, intensity float64, snow bool, frac *int) uint16 {
	if ticks <= 0 || a == 0 || intensity <= 0 {
		return a
	}
	pm := int(min(intensity, 1)*1000 + 0.5) // permille, deterministic rounding
	scale := rainScale
	if snow {
		scale = snowScale
	}
	return drain(a, Full*ticks*pm*scale/1000, frac)
}

// Add is a plus add, capped at Full.
func Add(a uint16, add int) uint16 {
	return uint16(max(0, min(Full, int(a)+add)))
}
