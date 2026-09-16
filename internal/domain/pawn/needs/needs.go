// Package needs is hunger and rest meters (integer permille, 0–Full).
package needs

import "rimworld-go/internal/domain/pawn/schedule"

const (
	Full            = 10000
	CriticalHunger  = 2500
	CriticalRest    = 2000
	HungerPeriod    = 12 // ticks per 1 hunger lost while not eating
	RestAwakePeriod = 4  // ticks per 1 rest lost while awake
	SleepGain       = 3  // rest restored per tick while sleeping
	EatRestore      = 4000
	EatWorkTicks    = 20
	SleepWorkMin    = 40
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
	p.Rest = clamp(p.Rest + SleepGain)
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

// WorkScheduled is true when autonomous labor is allowed.
func WorkScheduled(a schedule.Activity) bool {
	return a == schedule.Work || a == schedule.Anything
}
