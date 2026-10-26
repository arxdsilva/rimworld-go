package game

import (
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/sim/entity"
)

// PLAY-021 heaters, coolers and thermostats (CONSUMERS owns this file).
// A thermostat targets 21 °C by default; a device works below target −1 °C
// (a heater) or above target +0.5 °C (a cooler) and idles past the other
// edge (hysteresis, saved as its active flag). A heater adds heat to its
// room; a cooler cools its cold-side room and warms the hot side
// (outdoors: nothing). Both only while powered.
//
// The thermostat reads the air once a climate interval, before the rooms'
// air moves (heatRooms): the 1.5 °C band between the two edges keeps a
// device from switching on and off every interval.

const (
	// heaterHeat is what a working heater adds to its room per climate
	// interval, in tenths of a degree times room cells (campfireHeat is
	// 160): a 25-cell room holds about 35 °C over the outside air.
	heaterHeat = 220
	// coolerHeat is what a working cooler takes from its cold-side room
	// and adds to its hot-side room per climate interval, same units.
	coolerHeat = 220
	// thermoLow and thermoHigh are the band's edges below and above the
	// target, in tenths of a degree: a heater works below target −1 °C and
	// idles above target +0.5 °C; a cooler works above target +0.5 °C and
	// idles below target −1 °C.
	thermoLow  = 10
	thermoHigh = 5
)

// thermoRoomHeat adds the heaters' and coolers' heat to heat (per room
// id, per climate interval, in tenths of a degree times room cells like
// campfireHeat; a cooler's is negative), beside the campfires' and fires'
// (heat.go). It walks the power buildings (cached per structure), so a
// colony without heaters or coolers writes nothing.
func (g *Game) thermoRoomHeat(heat []int) {
	c := g.powerList()
	for i, id := range c.ids {
		if !isThermostat(c.specs[i]) {
			continue
		}
		b := g.bldgs.Ref(id)
		if b == nil {
			continue
		}
		switch b.Kind {
		case construction.Heater:
			r := g.roomAt(b.X, b.Y)
			if !g.thermostat(id, r, true) {
				continue
			}
			if r < len(heat) {
				heat[r] += heaterHeat
			}
		case construction.Cooler:
			dx, dy := facingStep(b.Facing)
			cold, hot := g.roomAt(b.X+dx, b.Y+dy), g.roomAt(b.X-dx, b.Y-dy)
			if !g.thermostat(id, cold, false) {
				continue
			}
			if cold < len(heat) {
				heat[cold] -= coolerHeat
			}
			if hot >= 0 && hot < len(heat) {
				heat[hot] += coolerHeat
			}
		}
	}
}

// thermostat updates heater or cooler id's active flag from the air of
// room r (heats: a heater, else a cooler) and reports it working on power
// now. With no room (outdoors, or a wall) there is no air to read: the
// flag stays as it is and the device moves no heat.
func (g *Game) thermostat(id entity.EntityID, r int, heats bool) bool {
	if r < 0 || r >= len(g.roomTemps) {
		return false
	}
	t, target := int(g.roomTemps[r]), g.thermoTarget(id)
	active := g.thermoActive(id)
	switch {
	case heats && !active && t < target-thermoLow, !heats && !active && t > target+thermoHigh:
		active = true
	case heats && active && t > target+thermoHigh, !heats && active && t < target-thermoLow:
		active = false
	}
	g.setThermoActive(id, active)
	return active && g.powered(id)
}

// facingStep is the cell step toward facing f (south is +y).
func facingStep(f construction.Facing) (int, int) {
	switch f {
	case construction.North:
		return 0, -1
	case construction.East:
		return 1, 0
	case construction.West:
		return -1, 0
	}
	return 0, 1
}

// cmdSetThermostat sets heater or cooler target's target temperature
// (tenths of a degree C, clamped; anything else is ignored).
func (g *Game) cmdSetThermostat(target entity.EntityID, tenths int) {
	g.setThermoTarget(target, tenths)
}
