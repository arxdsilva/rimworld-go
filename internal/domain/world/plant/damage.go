package plant

import "rimworld-go/internal/domain/sim/clock"

// Cold and heat damage (PLAY-016, our numbers). Out of its climate a
// plant is hurt over hours instead of dying at once: past its kill
// temperature (a crop's ColdLimit, a wild plant's ColdKill, HeatKill on
// the hot side) Damage grows by DamageRate an in-game hour plus
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

// Expose advances p's cold or heat damage over the ticks t0+1 .. t1 at
// air temperature tenthsC (one temperature for the whole window, as
// GrowSpan grows at one rate). The damage of a window is the difference
// of running totals at t1 and t0, so windows at a steady temperature add
// up exactly to the whole span, whatever the stagger. started reports
// that damage began this window (the once-a-day warning).
func (p Plant) Expose(tenthsC int, t0, t1 uint64) (q Plant, started bool) {
	d := DefOf(p.Species)
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
		p.Damaging = tenthsC < cold+DamageStopMargin
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

// DamagedYield is a harvest of qty from p reduced by its damage:
// (100 − Damage)%, rounded down, never below zero.
func (p Plant) DamagedYield(qty int) int {
	if qty <= 0 {
		return 0
	}
	return max(qty*(100-min(int(p.Damage), 100))/100, 0)
}
