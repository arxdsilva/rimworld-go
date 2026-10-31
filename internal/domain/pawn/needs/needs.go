// Package needs is hunger and rest meters (integers, 0–Full).
package needs

import "rimworld-go/internal/domain/pawn/schedule"

const (
	// Full is a full adult food bar, 1.0 nutrition (wiki: Nutrition): item
	// nutrition is on the same scale (a meal 9000, raw food 500 a unit).
	Full = 10000
	// The saturation bands (wiki: Saturation): Fed above HungryAt, Hungry
	// down to RavenousAt, Ravenously hungry above 0, Malnourished at 0.
	HungryAt   = 2500
	RavenousAt = 1250
	// CriticalHunger is the top of the Hungry band (care feeding, prison
	// food bands and social fights read it).
	CriticalHunger = HungryAt
	// EatAt: a colonist goes to eat at 30 % (wiki: Saturation) and keeps
	// wanting food until SatedAt (hysteresis, Pair.Seeking).
	EatAt   = 3000
	SatedAt = 8000
	// BandGap: a pawn leaves a worse band only once its food is this far
	// past the band's edge (the hunger thoughts never flap).
	BandGap = 100
	// HungerPeriod and the band losses: an adult loses fedLoss of Full per
	// HungerPeriod while Fed, 1.6 nutrition a day (wiki: Nutrition), half
	// that while Hungry and a quarter while Ravenously hungry (wiki:
	// Saturation): a full bar empties in 0.625 day at the Fed rate.
	HungerPeriod = 15
	fedLoss      = 4
	// HungerPerHour is the Fed loss over an hour (caravans on the road).
	HungerPerHour   = 2500 * fedLoss / HungerPeriod
	CriticalRest    = 2000
	RestAwakePeriod = 4 // ticks per 1 rest lost while awake
	// SleepGain rest is restored every SleepPeriod ticks asleep: 0.4 a
	// tick, so 20% -> 100% (or 0 -> 80%) takes 8 in-game hours.
	SleepGain   = 2
	SleepPeriod = 5
	// WakeRest is a reasonable sleep: tired sleepers outside a Sleep
	// block, and collapsed ones, rest to here before getting up.
	WakeRest     = 8000
	EatWorkTicks = 20
	SleepWorkMin = 40
)

// Band is a saturation band (wiki: Saturation).
type Band uint8

const (
	Fed Band = iota
	Hungry
	Ravenous
	Malnourished
)

// Pair is a pawn's food and rest. Band is the hunger band with hysteresis
// (the hunger thoughts) and Seeking the eat latch (on at EatAt, off at
// SatedAt); Settle keeps both, and saves from before FOOD-1 load them as
// Fed and not seeking, which the next tick corrects.
type Pair struct {
	Hunger  int
	Rest    int
	Band    Band
	Seeking bool
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

// bandOf is the band of food level h, without hysteresis.
func bandOf(h int) Band {
	switch {
	case h > HungryAt:
		return Fed
	case h > RavenousAt:
		return Hungry
	case h > 0:
		return Ravenous
	}
	return Malnourished
}

// Settle brings Band and Seeking up to date with Hunger: a worse band at
// once, a better one only BandGap past its edge; seeking from EatAt until
// SatedAt.
func (p Pair) Settle() Pair {
	if b := bandOf(p.Hunger); b >= p.Band {
		p.Band = b
	} else {
		p.Band = bandOf(p.Hunger - BandGap)
	}
	switch {
	case p.Hunger <= EatAt:
		p.Seeking = true
	case p.Hunger >= SatedAt:
		p.Seeking = false
	}
	return p
}

// HungerLoss is what food level h loses per HungerPeriod at the neutral
// rate: the Fed, Hungry and Ravenous falls (wiki: Saturation).
func HungerLoss(h int) int {
	switch bandOf(h) {
	case Fed:
		return fedLoss
	case Hungry:
		return fedLoss / 2
	case Ravenous:
		return fedLoss / 4
	}
	return 0
}

// hungerTick is one tick of hunger at the neutral rate.
func hungerTick(p Pair, now uint64) Pair {
	if now%uint64(HungerPeriod) == 0 {
		p.Hunger = clamp(p.Hunger - HungerLoss(p.Hunger))
	}
	return p.Settle()
}

// Decay applies one awake tick of hunger/rest loss.
func Decay(p Pair, now uint64) Pair {
	if now%uint64(RestAwakePeriod) == 0 && p.Rest > 0 {
		p.Rest--
	}
	p.Rest = clamp(p.Rest)
	return hungerTick(p, now)
}

// DecayRated is Decay at hunger rate hr and rest rate rr (permille, 1000
// neutral), the fractions of a point carried in ch and cr.
func DecayRated(p Pair, now uint64, hr, rr int32, ch, cr *int32) Pair {
	if now%uint64(HungerPeriod) == 0 {
		*ch += int32(HungerLoss(p.Hunger)) * hr
		p.Hunger = max(p.Hunger-int(*ch/1000), 0)
		*ch %= 1000
	}
	if now%uint64(RestAwakePeriod) == 0 {
		*cr += rr
		p.Rest = max(p.Rest-int(*cr/1000), 0)
		*cr %= 1000
	}
	return p.Settle()
}

// SleepTick restores rest. Hunger still decays while asleep.
func SleepTick(p Pair, now uint64) Pair {
	if now%uint64(SleepPeriod) == 0 {
		p.Rest = clamp(p.Rest + SleepGain)
	}
	return hungerTick(p, now)
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
	return hungerTick(p, now)
}

// Portions is how many units of nutrition nut a pawn at food level hunger
// eats in one sitting: the units nearest to filling it (at least one, at
// most most); what overflows the bar is wasted (wiki: Saturation).
func Portions(hunger, nut, most int) int {
	if nut <= 0 {
		return 1
	}
	n := (Full - hunger + nut/2) / nut
	return max(1, min(most, n))
}

// Hungry reports a pawn that wants food: at or below EatAt, or still
// seeking (Settle) short of SatedAt.
func (p Pair) Hungry() bool { return p.Hunger <= EatAt || p.Seeking && p.Hunger < SatedAt }

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

// FightBand maps food level h to the social-fight hunger bands
// (social.MalnutritionMult): 0 fed (above CriticalHunger), 1 hungry, 2
// very hungry (half of it), 3 ravenous (a quarter), 4 starving (empty).
func FightBand(h int) int {
	switch {
	case h > CriticalHunger:
		return 0
	case h > CriticalHunger/2:
		return 1
	case h > CriticalHunger/4:
		return 2
	case h > 0:
		return 3
	}
	return 4
}
