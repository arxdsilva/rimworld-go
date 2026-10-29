// Package needs is hunger and rest meters (integers, 0–Full).
package needs

import "rimworld-go/internal/domain/pawn/schedule"

const (
	Full            = 10000
	CriticalHunger  = 2500
	CriticalRest    = 2000
	HungerPeriod    = 12 // ticks per 1 hunger lost while not eating
	RestAwakePeriod = 4  // ticks per 1 rest lost while awake
	// SleepGain rest is restored every SleepPeriod ticks asleep: 0.4 a
	// tick, so 20% -> 100% (or 0 -> 80%) takes 8 in-game hours.
	SleepGain   = 2
	SleepPeriod = 5
	// WakeRest is a reasonable sleep: tired sleepers outside a Sleep
	// block, and collapsed ones, rest to here before getting up.
	WakeRest     = 8000
	EatRestore   = 4000
	EatWorkTicks = 20
	SleepWorkMin = 40
)

// Pair is a pawn's food and rest.
type Pair struct {
	Hunger int
	Rest   int
}

// NewFull returns fully fed and rested needs.
func NewFull() Pair {
	return Pair{Hunger: Full, Rest: Full}
}

func clamp(v int) int {
	if v < 0 {
		return 0
	}
	if v > Full {
		return Full
	}
	return v
}

// Decay applies one awake tick of hunger/rest loss.
func Decay(p Pair, now uint64) Pair {
	if now%uint64(HungerPeriod) == 0 && p.Hunger > 0 {
		p.Hunger--
	}
	if now%uint64(RestAwakePeriod) == 0 && p.Rest > 0 {
		p.Rest--
	}
	p.Hunger = clamp(p.Hunger)
	p.Rest = clamp(p.Rest)
	return p
}

// SleepTick restores rest. Hunger still decays slowly while asleep.
func SleepTick(p Pair, now uint64) Pair {
	if now%uint64(SleepPeriod) == 0 {
		p.Rest = clamp(p.Rest + SleepGain)
	}
	if now%uint64(HungerPeriod) == 0 && p.Hunger > 0 {
		p.Hunger--
	}
	return p
}

// SleepTickAt is SleepTick in a place of rest effectiveness re (permille,
// BED-1): rest comes back at SleepGain per SleepPeriod scaled by re,
// spread exactly over the ticks (0 to Full in 25000 ticks at 1000, near
// the wiki's 26250). At re 1000 it is SleepTick exactly.
func SleepTickAt(p Pair, now uint64, re int) Pair {
	if re == 1000 {
		return SleepTick(p, now)
	}
	if re > 0 && now > 0 {
		per, q := uint64(SleepGain*re), uint64(SleepPeriod*1000)
		p.Rest = clamp(p.Rest + int(now*per/q-(now-1)*per/q))
	}
	if now%uint64(HungerPeriod) == 0 && p.Hunger > 0 {
		p.Hunger--
	}
	return p
}

// Hungry reports a critical food interrupt.
func (p Pair) Hungry() bool { return p.Hunger <= CriticalHunger }

// Tired reports a critical rest interrupt.
func (p Pair) Tired() bool { return p.Rest <= CriticalRest }

// Exhausted reports rest has hit zero (forced sleep).
func (p Pair) Exhausted() bool { return p.Rest <= 0 }

// Starving reports hunger has hit zero.
func (p Pair) Starving() bool { return p.Hunger <= 0 }

// Percent is 0–100 for HUD bars.
func Percent(v int) int {
	if v <= 0 {
		return 0
	}
	return v * 100 / Full
}

// SleepScheduled is true for Sleep blocks (Anything does not force sleep).
func SleepScheduled(a schedule.Activity) bool {
	return a == schedule.Sleep
}

// SleepAllowed is true when a tired colonist may go to bed: anything but
// Work. In Work hours it keeps going until it collapses at zero.
func SleepAllowed(a schedule.Activity) bool {
	return a != schedule.Work
}

// WorkScheduled is true when autonomous labor is allowed.
func WorkScheduled(a schedule.Activity) bool {
	return a == schedule.Work || a == schedule.Anything
}
