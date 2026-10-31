package thermo

import (
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/world/temperature"
)

// Heat sources, in hundredths of heat/s (wiki: Temperature, Heat sources:
// one heat/s pushed into a room of n cells warms it by 1/n degree-cells'
// worth; see heatPerStep for the step's conversion).
const (
	// heaterRate: wiki: Heater (+21 heat/s up to its thermostat target).
	heaterRate = 2100
	// coolerRate: wiki: Cooler (-21 heat/s on the cold side, down to its
	// target). coolerHotPct: the hot side gets the pumped heat and the
	// motor's on top (the wiki gives no figure: original tuning).
	coolerRate   = 2100
	coolerHotPct = 125
	// coolerLift: a cooler loses a 1/coolerLift share of its output per
	// degree its hot side is warmer than its cold side, and moves nothing
	// past coolerLift degrees (original tuning, thousandths of a degree;
	// with the hot side outdoors it holds a 10x10 room about 19 C below
	// the outside air, the wiki's 10-16 C at 27-38 C outside).
	coolerLift = 50_000
	// passiveRate and passiveFloor: wiki: Passive cooler (-11 heat/s, not
	// below 17 C).
	passiveRate  = 1100
	passiveFloor = temperature.Milli(17_000)
	// campfireRate and campfireCap: wiki: Campfire (+21 heat/s up to
	// 28 C). torchRate and torchCap: wiki: Torch lamp (+3.5 up to 23 C).
	campfireRate = 2100
	campfireCap  = temperature.Milli(28_000)
	torchRate    = 350
	torchCap     = temperature.Milli(23_000)
	// While working (benches) or burning (the generator): wiki: Electric
	// smelter (9), Fueled stove (4), Electric stove (3), Wood-fired
	// generator (6).
	smelterRate      = 900
	fueledStoveRate  = 400
	electricStovRate = 300
	woodGenRate      = 600
	// steamVentRate: a geothermal steam vent standing in a room (original
	// tuning; the outdoor warming around one stays in the game's vents.go).
	steamVentRate = 100
	// fireRate: a fire of fire size 100 (the wiki publishes no figure:
	// original tuning; a half-size fire heats like a campfire).
	fireRate = 4200
	// bodyRate: wiki: Temperature, a humanlike pushes 0.3 heat/s per body
	// size (0.3 x 4.1666665 = 1.25 per 250 ticks) while the air is at
	// most bodyMax; any other creature bodyAnimalPct of that.
	bodyRate      = 30
	bodyAnimalPct = 60
	bodyMax       = temperature.Milli(40_000)
)

// heatPerStep converts heat to air: one interval of a source of r
// hundredths of heat/s adds r * heatPerStep / heatDiv degree-milli-cells
// (1.15 degree-cells per heat/s per 120 ticks). Calibrated together with
// the wiki's wall table and thin-roof loss (domain/world/temperature) so
// the wiki's own heater proofs hold: at -17 C outside one heater keeps a
// single-walled, roofed 9x9 at about 20 C and an 11x11 at about 10 C
// (TestHeaterHoldsWikiRooms). The same conversion serves every source.
const (
	heatPerStep = 115
	heatDiv     = 10
)

// Heats reports a building kind with a heat output (BuildingRate): the
// ones heats lists and the cooler.
func Heats(k construction.Kind) bool { return heats(k) || k == construction.Cooler }

// Rise is how far above the outside air a steady source of rate
// hundredths of heat/s holds a closed room of exposure e, alone.
func Rise(e temperature.Exposure, rate int) temperature.Milli {
	return e.Rise(int64(rate) * heatPerStep / heatDiv)
}

// SteamVentRate is a steam vent's heat in its room (steamVentRate).
const SteamVentRate = steamVentRate

// heats reports a building kind that pushes heat into the room it stands
// in (the cooler and vent are wall-mounted links: cache.go).
func heats(k construction.Kind) bool {
	switch k {
	case construction.Heater, construction.PassiveCooler, construction.Campfire, construction.TorchLamp,
		construction.ElectricSmelter, construction.Stove, construction.ElectricStove, construction.WoodGenerator,
		construction.SteamVent:
		return true
	}
	return false
}

// push is one source's effect on a room this step: rate hundredths of
// heat/s, and the air it stops at (heating: not above limit; cooling: not
// below; capped false: no limit).
type push struct {
	rate   int
	limit  temperature.Milli
	capped bool
}

// pushOf is site s's push into its room now (rate 0: idle). The cooler's
// hot side is pushed separately (Step).
func (m *Model) pushOf(s site, air temperature.Milli) push {
	h := m.h
	switch s.kind {
	case construction.Heater:
		target, active := h.Thermostat(s.id)
		if active && h.Powered(s.id) {
			return push{rate: heaterRate, limit: temperature.Temp(target).Milli(), capped: true}
		}
	case construction.Cooler:
		target, active := h.Thermostat(s.id)
		if active && h.Powered(s.id) {
			return push{rate: -coolerRate * m.coolerEff(s, air) / 1000, limit: temperature.Temp(target).Milli(), capped: true}
		}
	case construction.PassiveCooler:
		if h.Burning(s.id) {
			return push{rate: -passiveRate, limit: passiveFloor, capped: true}
		}
	case construction.Campfire:
		if h.Burning(s.id) {
			return push{rate: campfireRate, limit: campfireCap, capped: true}
		}
	case construction.TorchLamp:
		if h.Burning(s.id) {
			return push{rate: torchRate, limit: torchCap, capped: true}
		}
	case construction.WoodGenerator:
		if h.Burning(s.id) {
			return push{rate: woodGenRate}
		}
	case construction.ElectricSmelter, construction.ElectricStove:
		if !h.Worked(s.id) || !h.Powered(s.id) {
			break
		}
		if s.kind == construction.ElectricSmelter {
			return push{rate: smelterRate}
		}
		return push{rate: electricStovRate}
	case construction.Stove:
		if h.Worked(s.id) && h.Burning(s.id) {
			return push{rate: fueledStoveRate}
		}
	case construction.SteamVent:
		return push{rate: steamVentRate}
	}
	return push{}
}

// coolerEff is cooler s's output share now, permille: it falls by
// 1/coolerLift per degree its hot side (a room, or the outdoor air) is
// warmer than the cold side's air.
func (m *Model) coolerEff(s site, cold temperature.Milli) int {
	hot := m.h.Outdoor().Milli()
	if s.other >= 0 && s.other < len(m.fine) {
		hot = m.fine[s.other]
	}
	lift := max(0, int(hot-cold))
	return max(0, 1000-lift*1000/coolerLift)
}

// bodyHeat is a creature's heat, hundredths of heat/s.
func bodyHeat(b Body) int {
	r := bodyRate * b.Size / 1000
	if !b.Human {
		r = r * bodyAnimalPct / 100
	}
	return r
}

// apply moves room r's air by p for one interval (its cells n).
func apply(t temperature.Milli, p push, n int) temperature.Milli {
	d := temperature.Milli(p.rate * heatPerStep / (heatDiv * max(1, n)))
	switch {
	case !p.capped:
		return t + d
	case d > 0 && t < p.limit:
		return min(t+d, p.limit)
	case d < 0 && t > p.limit:
		return max(t+d, p.limit)
	}
	return t
}
