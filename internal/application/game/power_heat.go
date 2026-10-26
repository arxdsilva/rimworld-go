package game

import "rimworld-go/internal/domain/sim/entity"

// PLAY-021 heaters, coolers and thermostats (CONSUMERS owns this file).
// A thermostat targets 21 °C by default; a device works below target −1 °C
// (a heater) or above target +0.5 °C (a cooler) and idles past the other
// edge (hysteresis, saved as its active flag). A heater adds heat to its
// room; a cooler cools its cold-side room and warms the hot side
// (outdoors: nothing). Both only while powered.

// thermoRoomHeat adds the heaters' and coolers' heat to heat (per room
// id, per climate interval, in tenths of a degree times room cells like
// campfireHeat; a cooler's is negative), beside the campfires' and fires'
// (heat.go).
func (g *Game) thermoRoomHeat(heat []int) {
	// TODO(CONSUMERS)
}

// cmdSetThermostat sets heater or cooler target's target temperature
// (tenths of a degree C).
func (g *Game) cmdSetThermostat(target entity.EntityID, tenths int) {
	// TODO(CONSUMERS)
}
