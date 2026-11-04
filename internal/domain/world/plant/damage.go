package plant

import "rimworld-go/internal/domain/sim/clock"

// Cold and heat damage (PLAY-016, our numbers). Out of its climate a
// wild plant (or a cold-resistant crop) is hurt over hours instead of
// dying at once: past its kill temperature (ColdLimit: a wild plant's
// ColdKill, a cold-resistant crop's FrostKill; HeatKill on the hot side) Damage grows by DamageRate an in-game hour plus
// DamagePerTenth for every tenth of a degree further, at most
// DamageRateMax, and the plant dies at DamageDead. Rates are tenths of a
// percent of damage an hour.
//
// Hysteresis: damage starts past the kill temperature and stops only
// once the air is DamageStopMargin back on the safe side, so air
// hovering at the threshold doesn't start and stop it by turns. Out of
// danger a plant below DamageDying heals DamageHealRate an hour; from
// DamageDying on it keeps dying at DamageRate, warm or not.
const (
	DamageRate       = 250 // 25%/h just past the threshold: dead in about 4 h
	DamagePerTenth   = 5   // +5%/h for each degree further
	DamageRateMax    = 600 // 60%/h: a hard freeze or blaze kills in under 2 h
	DamageHealRate   = 100 // 10%/h once out of danger
	DamageStopMargin = 20  // tenths of a degree back on the safe side
	DamageDying      = 60  // percent: from here the plant keeps dying
	DamageDead       = 100 // percent: the plant dies
)

// damageRate is the damage rate (tenths of a percent an hour) excess
// tenths of a degree past the kill temperature.
func damageRate(excess int) int {
	return min(DamageRate+DamagePerTenth*max(excess, 0), DamageRateMax)
}

// Dead reports p has died of cold or heat damage.
func (p Plant) Dead() bool { return p.Damage >= DamageDead }

// Cold (TEMP-2, wiki: Plants). Every plant has its own cold threshold, a
// fixed pseudo-random one between ColdWarmest (-10 °C) and ColdColdest
// (-18 °C) derived from its id (ColdThreshold, never the simulation's
// RNG), so below -18 °C no plant is safe. Air below it kills a crop at
// once; any other plant, and a cold-resistant crop (ColdHardy: healroot),
// loses its leaves instead: it stops growing until LeafRegrow (24 h)
// after the last air below its threshold, and then only above 0 °C
// (TempFactor). The leafless timer is a minimum duration, re-armed while
// the cold lasts, so air hovering at the threshold cannot flap it. A
// cold-resistant crop still dies of the damage below its FrostKill
// (healroot -29 °C), a wild biome plant below its ColdKill (original
// tuning, PLAY-016).
const (
	ColdWarmest = -100 // tenths of a degree, wiki: Plants
	ColdColdest = -180
	LeafRegrow  = clock.TicksPerDay // wiki: Plants, leaves regrow in 24 h
	coldSalt    = 0x7e3a
)

// ColdThreshold is the cold threshold (tenths of a degree C) of the plant
// with id seed: ColdWarmest down to ColdColdest, fixed per plant.
func ColdThreshold(seed uint64) int {
	return ColdWarmest - int(Roll(seed, coldSalt, 0)%uint64(ColdWarmest-ColdColdest+1))
}

// Leafless reports p bare of leaves from the cold at tick now: it does
// not grow.
func (p Plant) Leafless(now uint64) bool { return now < p.Bare }

// Expose advances p's cold or heat damage over the ticks t0+1 .. t1 at
// air temperature tenthsC (one temperature for the whole window, as
// GrowSpan grows at one rate); seed is the plant's id (ColdThreshold).
// Below its cold threshold a crop dies at once and any other plant loses
// its leaves until LeafRegrow after the end of t1's hour (whole hours, so
// a long frost rewrites the plant once an hour, not every window). The damage of a window is the
// difference of running totals at t1 and t0, so windows at a steady
// temperature add up exactly to the whole span, whatever the stagger.
// started reports that damage began this window (the once-a-day
// warning).
func (p Plant) Expose(seed uint64, tenthsC int, t0, t1 uint64) (q Plant, started bool) {
	d := DefOf(p.Species)
	if tenthsC < ColdThreshold(seed) {
		if d.Crop && !d.ColdHardy {
			p.Damage, p.DamageHeat, p.Damaging = DamageDead, false, true
			return p, false
		}
		p.Bare = max(p.Bare, (t1/clock.TicksPerHour+1)*clock.TicksPerHour+LeafRegrow)
	}
	cold, hasCold := p.ColdLimit() // farm.go: a crop's own limit
	excess, hot := 0, false        // tenths of a degree past the kill temperature
	switch {
	case hasCold && tenthsC < cold:
		excess = cold - tenthsC
	case d.HeatKill != 0 && tenthsC > d.HeatKill:
		excess, hot = tenthsC-d.HeatKill, true
	}
	switch {
	case excess > 0:
		started = !p.Damaging
		p.Damaging, p.DamageHeat = true, hot
	case p.Damaging && p.DamageHeat:
		p.Damaging = tenthsC > d.HeatKill-DamageStopMargin
	case p.Damaging:
		p.Damaging = hasCold && tenthsC < cold+DamageStopMargin
	}
	if t1 <= t0 {
		return p, started
	}
	switch {
	case p.Damaging:
		p.Damage = uint8(min(int(p.Damage)+damageSpan(damageRate(excess), t0, t1), DamageDead))
	case p.Damage >= DamageDying:
		p.Damage = uint8(min(int(p.Damage)+damageSpan(DamageRate, t0, t1), DamageDead))
	case p.Damage > 0:
		p.Damage = uint8(max(int(p.Damage)-damageSpan(DamageHealRate, t0, t1), 0))
		if p.Damage == 0 {
			p.DamageHeat = false
		}
	}
	return p, started
}

// damageSpan is the damage (percent) rate (tenths of a percent an hour)
// deals over the ticks t0+1 .. t1: the difference of the running totals,
// which telescopes over consecutive windows.
func damageSpan(rate int, t0, t1 uint64) int {
	r, per := uint64(rate), uint64(10*clock.TicksPerHour)
	return int(t1*r/per - t0*r/per)
}

// DamagedYield is a harvest of qty from p reduced by its damage: up to half
// (wiki: Plants; 50 % health loses 25 %), rounded down (HRV-004).
func (p Plant) DamagedYield(qty int) int {
	if qty <= 0 {
		return 0
	}
	return qty * (200 - min(int(p.Damage), 100)) / 200
}
