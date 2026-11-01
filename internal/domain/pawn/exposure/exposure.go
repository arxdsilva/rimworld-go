// Package exposure is how air temperature affects a pawn or an animal: a
// comfortable range (set by what it wears, or by its species), mood
// thoughts outside it, and hypothermia or heatstroke that build up more
// than 10 °C past it and fade once it is back inside (TEMP-3, wiki:
// Temperature, Hypothermia, Heatstroke; ENG-003). Temperatures are tenths
// of a degree C. Severities are Full-based fixed point (Full is 1.0) so
// the wiki's per-60-tick rates stay exact; Permille converts for views
// and body conditions.
package exposure

import "rimworld-go/internal/domain/pawn/mood"

// Bands is a comfortable range, tenths of a degree C.
type Bands struct{ Min, Max int }

// Everyday is a human's base comfortable range. wiki: Temperature
// (16–26 °C).
var Everyday = Bands{Min: 160, Max: 260}

// Wearing widens the band by apparel insulation: cold lowers Min, heat
// raises Max (negative heat insulation lowers it).
func (b Bands) Wearing(cold, heat int) Bands {
	return Bands{Min: b.Min - cold, Max: b.Max + heat}
}

// DangerMargin is how far past the comfortable range (tenths) hypothermia
// or heatstroke starts to build. wiki: Hypothermia (more than 10 °C).
const DangerMargin = 100

// RungStep is the width of one rung of the temperature mood ladders
// (tenths): every 10 °C further past the range is one rung worse.
// Original tuning: the wiki publishes no trigger temperatures.
const RungStep = 100

// ColdDanger/HeatDanger are the lines past which exposure builds.
func (b Bands) ColdDanger() int { return b.Min - DangerMargin }
func (b Bands) HeatDanger() int { return b.Max + DangerMargin }

// Freezing/Sweltering are the second rungs of the ladders (Shivering,
// Overheated): an urgent change of clothes (apparel.go).
func (b Bands) Freezing() int   { return b.Min - RungStep }
func (b Bands) Sweltering() int { return b.Max + RungStep }

// Comfortable reports t inside the band.
func (b Bands) Comfortable(t int) bool { return t >= b.Min && t <= b.Max }

// coldLadder and hotLadder are the mood rungs, mildest first. wiki:
// Thoughts (−4, −8, −12, −16; they never stack).
var (
	coldLadder = [...]mood.Kind{mood.Chilly, mood.Shivering, mood.NumbingCold, mood.AbsolutelyFreezing}
	hotLadder  = [...]mood.Kind{mood.Sweaty, mood.Overheated, mood.Baking, mood.BlisteringHot}
)

// Thought is the temperature thought of a pawn with band b in air t: a
// rung per RungStep past the range, capped at the worst (false: none).
func (b Bands) Thought(t int) (mood.Kind, bool) {
	switch {
	case t < b.Min:
		return coldLadder[min((b.Min-t)/RungStep, len(coldLadder)-1)], true
	case t > b.Max:
		return hotLadder[min((t-b.Max)/RungStep, len(hotLadder)-1)], true
	}
	return 0, false
}

// Full is severity 1.0 (death, or the insects' worst slowdown).
const Full int32 = 100_000_000

// Stage lines, Full-based. wiki: Hypothermia (4 %, 20 %, 35 %, 62 %).
const (
	Trivial = Full / 25        // 0.04
	Minor   = Full / 5         // 0.20
	Serious = Full / 100 * 35  // 0.35
	Extreme = Full / 100 * 62  // 0.62: consciousness capped at 10 %, downed
	Mild    = Minor            // the pawn goes looking for warmth or shade
	Danger  = Serious          // bad enough to get a sleeper out of bed
	perMil  = Full / 1000      // one permille
	every   = 60               // the wiki's rate interval, ticks
	tenthK  = int32(645)       // 0.0000645 per °C is 645 per tenth, Full-based
	hypoMin = int32(75_000)    // 0.00075
	heatMin = int32(37_500)    // 0.000375
	recMin  = int32(150_000)   // 0.0015
	recMax  = int32(1_500_000) // 0.015
	recPer  = int32(27)        // 0.027 × severity, per mille
)

// Permille is sev in permille (views and body condition severities).
func Permille(sev int32) int32 { return sev / perMil }

// FromPermille is a permille severity in Full units (save migration).
func FromPermille(p int) int32 { return int32(min(max(p, 0), 1000)) * perMil }

// HypoGain is hypothermia's rise per 60 ticks in air t with band b:
// max(0.00075, (d − 10) × 0.0000645), d the degrees below the range
// (0 above the danger line). wiki: Hypothermia.
func HypoGain(t int, b Bands) int32 {
	if t >= b.ColdDanger() {
		return 0
	}
	return max(hypoMin, int32(b.Min-t-DangerMargin)*tenthK)
}

// heatCurve maps degrees past the heat danger line (tenths) to the
// effective degrees heatstroke builds by. wiki: Heatstroke (1:1 to 25,
// 50→40, 100→60, 400→100).
var heatCurve = [...][2]int32{{0, 0}, {250, 250}, {500, 400}, {1000, 600}, {4000, 1000}}

func curve(x int32) int32 {
	for i := 1; i < len(heatCurve); i++ {
		a, c := heatCurve[i-1], heatCurve[i]
		if x <= c[0] {
			return a[1] + (x-a[0])*(c[1]-a[1])/(c[0]-a[0])
		}
	}
	return heatCurve[len(heatCurve)-1][1]
}

// HeatGain is heatstroke's rise per 60 ticks in air t with band b:
// 0.0000645 × curve(x), floor 0.000375, x the degrees past the danger
// line (0 below it). wiki: Heatstroke.
func HeatGain(t int, b Bands) int32 {
	if t <= b.HeatDanger() {
		return 0
	}
	return max(heatMin, curve(int32(t-b.HeatDanger()))*tenthK)
}

// Recovery is a severity's fall per 60 ticks: max(0.0015, min(0.015,
// 0.027 × sev)). wiki: Hypothermia.
func Recovery(sev int32) int32 {
	return max(recMin, min(recMax, sev/1000*recPer))
}

// Step moves hypothermia and heatstroke over ticks in air t with band b,
// in 60-tick steps (the last one a share): each builds past its danger
// line and recovers once the air is back on the comfortable side of its
// limit (hypothermia at Min or warmer, heatstroke at Max or cooler); in
// between it holds. Both can be present at once.
func Step(hypo, heat int32, t, ticks int, b Bands) (int32, int32) {
	for ticks > 0 {
		dt := int32(min(ticks, every))
		ticks -= every
		hypo = move(hypo, HypoGain(t, b), t >= b.Min, dt)
		heat = move(heat, HeatGain(t, b), t <= b.Max, dt)
	}
	return hypo, heat
}

// move is one (partial) step of a severity: up by gain, else down by its
// recovery when recovering.
func move(sev, gain int32, recovering bool, dt int32) int32 {
	switch {
	case gain > 0:
		sev += gain * dt / every
	case recovering && sev > 0:
		sev -= Recovery(sev) * dt / every
	}
	return min(max(sev, 0), Full)
}

// FrostbiteChance is the chance per 60 ticks, in millionths, that a pawn
// at hypothermia hypo frostbites (it needs air at or below 0 °C too):
// 2000 ppm at 37 % rising linearly to 30000 ppm at 100 %, about one
// strike in 12 hours at the onset and more than one an hour near death,
// so a pawn freezing in a couple of hours still takes some. Original
// tuning: the wiki publishes no chance.
func FrostbiteChance(hypo, minHypo int32) int32 {
	if hypo < minHypo {
		return 0
	}
	return frostLo + int32(int64(hypo-minHypo)*(frostHi-frostLo)/int64(Full-minHypo))
}

// The frostbite chance's ends, ppm per 60 ticks (original tuning).
const (
	frostLo = 2000
	frostHi = 30000
)

// Heat burns (TEMP-6, wiki: Temperature: a creature takes burn damage in
// air more than 150 °C above its comfortable maximum).
const (
	// BurnMargin is how far past the comfortable maximum (tenths) the air
	// burns.
	BurnMargin = 1500
	// BurnEvery is how often (ticks) a creature in such air is burned, and
	// burnStep and burnMax how hard: 1 hit point, one more per 40 °C
	// further, at most 6 (original tuning: the wiki gives no rate).
	BurnEvery = 480
	burnStep  = 400
	burnMax   = 6
)

// BurnDamage is the burn a creature whose comfortable maximum is hi
// takes from air t once per BurnEvery ticks (0: the air doesn't burn).
func BurnDamage(t, hi int) int {
	over := t - hi - BurnMargin
	if over <= 0 {
		return 0
	}
	return min(burnMax, 1+over/burnStep)
}
