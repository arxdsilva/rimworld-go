package game

import (
	"testing"

	"rimworld-go/internal/application/game/thermo"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/temperature"
	"rimworld-go/internal/domain/world/weather"
)

// PLAY-021 CONSUMERS: heaters and coolers on their thermostats.

// steadyAir gives g a climate with no seasons and no day swing: the
// outside air is always tenths (tenths of a degree C), and clear.
func steadyAir(g *Game, tenths int) {
	g.climate = temperature.Climate{YearMean: tenths, PeakDay: 15}
	g.HoldWeather()
	g.SetWeather(weather.Clear, 0)
}

// stepFuelled steps g n climate intervals, topping the generator up each
// one, calling each after every interval.
func stepFuelled(g *Game, gen entity.EntityID, n int, each func(i int)) {
	for i := range n {
		g.setFuel(gen, g.fuelCap(gen))
		g.Step(climateInterval)
		if each != nil {
			each(i)
		}
	}
}

// TestHeaterWarmsColdRoomToTarget: in steady -10 °C air a powered heater
// brings a cold 25-cell room up to its 21 °C target and holds it there.
// TEMP-1: a working heater pushes its wiki 21 heat/s only up to its
// target, so it no longer overshoots into the band and cycles: it stays
// on (heating, 175 W) and the air sits at the target, inside the band.
// The band's hysteresis is TestThermostatBandDoesNotFlap's. Without power
// it adds nothing, though it still asks.
func TestHeaterWarmsColdRoomToTarget(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 20, Height: 10})
	steadyAir(g, -100)
	hut(t, g, 1, 1, 7, 7) // interior 2..6: 25 cells
	heater := buildNow(t, g, construction.Heater, item.Steel, construction.South, 6, 3)
	gen := onPower(t, g, 6, 3, 9, 2) // TEMP-1: the generator outside (it heats its room now)
	r := g.roomAt(4, 4)
	if r < 0 || g.rooms[r].contents.Cells != 25 {
		t.Fatalf("hut room %d", r)
	}
	g.thermo.Temps[r] = -70 // cold
	target := g.thermoTarget(heater)
	if target != 210 {
		t.Fatalf("default target %d", target)
	}
	warmed := -1
	stepFuelled(g, gen, 12*clock.TicksPerHour/climateInterval, func(i int) {
		if warmed < 0 && int(g.thermo.Temps[r]) >= target-thermo.BandLow {
			warmed = i
		}
	})
	if warmed < 0 {
		t.Fatalf("the room reached %v in 12 h, want the target", g.thermo.Temps[r])
	}
	t.Logf("warmed to the band in %d climate intervals", warmed+1)

	flips := 0
	lo, hi := int(g.thermo.Temps[r]), int(g.thermo.Temps[r])
	was := g.thermoActive(heater)
	stepFuelled(g, gen, clock.TicksPerDay/climateInterval, func(int) {
		temp, on := int(g.thermo.Temps[r]), g.thermoActive(heater)
		lo, hi = min(lo, temp), max(hi, temp)
		if on != was {
			flips++
			was = on
		}
	})
	t.Logf("a day at the target: %d switches, air %v..%v", flips, temperature.Temp(lo), temperature.Temp(hi))
	if flips > 2 {
		t.Fatalf("%d thermostat switches in a day holding a cold room, want it to stay on", flips)
	}
	if lo < target-thermo.BandLow || hi > target+thermo.BandHigh {
		t.Fatalf("air %v..%v, want inside the %v..%v band", temperature.Temp(lo), temperature.Temp(hi),
			temperature.Temp(target-thermo.BandLow), temperature.Temp(target+thermo.BandHigh))
	}
	if v := bldgViewOf(g, heater); v.HeatRate != 210 || !v.ThermoActive {
		t.Fatalf("a working heater shows %d tenths of heat/s (active %v), want 210", v.HeatRate, v.ThermoActive)
	}

	// the generator runs dry: the room cools, the heater asks for power
	g.setFuel(gen, 0)
	g.Step(4 * clock.TicksPerHour)
	if g.powered(heater) || !g.thermoActive(heater) || int(g.thermo.Temps[r]) >= target-thermo.BandLow-20 {
		t.Fatalf("unpowered heater: powered %v active %v, room %v", g.powered(heater), g.thermoActive(heater), g.thermo.Temps[r])
	}
	if v := bldgViewOf(g, heater); v.HeatRate != 0 {
		t.Fatalf("an unpowered heater shows %d heat", v.HeatRate)
	}
}

// TestThermostatBandDoesNotFlap: outdoor air swinging 17..25 °C around a
// heater's 21 °C target. The heater switches on only below 20 °C and off
// only above 21.5 °C, so it changes state a few times a day, each state
// lasting several equalisation steps, never every step (no-flap).
func TestThermostatBandDoesNotFlap(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 20, Height: 10})
	steadyAir(g, 210)
	g.climate.DaySwing = 40
	hut(t, g, 1, 1, 7, 7)
	heater := buildNow(t, g, construction.Heater, item.Steel, construction.South, 6, 3)
	gen := onPower(t, g, 6, 3, 9, 2) // TEMP-1: the generator outside (it heats its room now)
	stepFuelled(g, gen, clock.TicksPerDay/climateInterval, nil)
	flips, span, minSpan := 0, 0, 1<<30
	was := g.thermoActive(heater)
	for range 2 * clock.TicksPerDay / temperature.EqualiseInterval {
		g.setFuel(gen, g.fuelCap(gen))
		g.Step(temperature.EqualiseInterval)
		span++
		if on := g.thermoActive(heater); on != was {
			flips++
			minSpan, span, was = min(minSpan, span), 0, on
		}
	}
	t.Logf("2 days: %d switches, shortest state %d steps", flips, minSpan)
	if flips < 2 || flips > 24 {
		t.Fatalf("%d thermostat switches in 2 days, want 2..24", flips)
	}
	if minSpan < 3 {
		t.Fatalf("a thermostat state lasted %d equalisation step(s)", minSpan)
	}
}

// bldgViewOf is building id's view.
func bldgViewOf(g *Game, id entity.EntityID) BuildingView {
	for _, b := range g.World().Buildings {
		if b.ID == id {
			return b
		}
	}
	return BuildingView{}
}

// TestHeaterTargetCommand: SetThermostat moves the target (clamped) and
// the heater holds the new one.
func TestHeaterTargetCommand(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 20, Height: 10})
	steadyAir(g, -100)
	hut(t, g, 1, 1, 7, 7)
	heater := buildNow(t, g, construction.Heater, item.Steel, construction.South, 6, 3)
	gen := onPower(t, g, 6, 3, 9, 2) // TEMP-1: the generator outside (it heats its room now)
	g.Enqueue(command.Command{Kind: command.SetThermostat, Target: uint64(heater), Count: 120})
	g.Step(1)
	if got := g.thermoTarget(heater); got != 120 {
		t.Fatalf("target %d, want 120", got)
	}
	r := g.roomAt(4, 4)
	stepFuelled(g, gen, 8*clock.TicksPerHour/climateInterval, nil)
	lo, hi := 1<<30, -1<<30
	stepFuelled(g, gen, 4*clock.TicksPerHour/climateInterval, func(int) {
		lo, hi = min(lo, int(g.thermo.Temps[r])), max(hi, int(g.thermo.Temps[r]))
	})
	if lo < 120-thermo.BandLow-12 || hi > 120+thermo.BandHigh+12 {
		t.Fatalf("air %v..%v at a 12 °C target", temperature.Temp(lo), temperature.Temp(hi))
	}
	g.Enqueue(command.Command{Kind: command.SetThermostat, Target: uint64(heater), Count: 9999})
	g.Step(1)
	if got := g.thermoTarget(heater); got != thermoMax {
		t.Fatalf("target %d, want clamped to %d", got, thermoMax)
	}
}

// coolerRooms builds two roofed 25-cell rooms side by side, west (x 2..6)
// and east (x 8..12), and a cooler in the wall gap between them at (7, 4)
// facing east: its cold side is the east room. The cooler is on power
// (a generator in the west room).
func coolerRooms(t *testing.T, g *Game) (cooler, gen entity.EntityID) {
	t.Helper()
	for x := 1; x <= 13; x++ {
		for _, y := range []int{1, 7} {
			buildNow(t, g, construction.Wall, item.Wood, construction.South, x, y)
		}
	}
	for y := 2; y <= 6; y++ {
		for _, x := range []int{1, 7, 13} {
			if x == 7 && y == 4 {
				continue // the cooler's gap
			}
			buildNow(t, g, construction.Wall, item.Wood, construction.South, x, y)
		}
	}
	cooler = buildNow(t, g, construction.Cooler, item.Steel, construction.East, 7, 4)
	g.RoofRooms()
	if b, _ := g.bldgs.Get(cooler); b.Facing != construction.East {
		t.Fatalf("cooler faces %s", b.Facing)
	}
	gen = onPower(t, g, 7, 4, 4, 4)
	return cooler, gen
}

// TestCoolerCoolsColdSideWarmsHotSide: in steady 35 °C air a powered
// cooler cools its cold-side room well below the outside and pushes the
// heat into the room behind it, which ends up well above the outside. The
// cooler bounds both rooms like a wall. TEMP-1: the hot side is a closed
// room (with the generator in it), so it heats up and the cooler loses
// output with the lift between its sides (game/thermo coolerLift): it no
// longer reaches its 21 °C target, as a vanilla cooler venting into a
// closed room doesn't.
func TestCoolerCoolsColdSideWarmsHotSide(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 20, Height: 10})
	steadyAir(g, 350)
	cooler, gen := coolerRooms(t, g)
	west, east := g.roomAt(4, 3), g.roomAt(10, 3)
	if west < 0 || east < 0 || west == east || g.roomAt(7, 4) >= 0 {
		t.Fatalf("rooms: west %d east %d, cooler cell %d", west, east, g.roomAt(7, 4))
	}
	if g.rooms[east].contents.Cells != 25 {
		t.Fatalf("east room has %d cells", g.rooms[east].contents.Cells)
	}
	settle := temperature.Temp(350)
	g.thermo.Temps[west], g.thermo.Temps[east] = settle, settle
	stepFuelled(g, gen, 12*clock.TicksPerHour/climateInterval, nil)
	if !g.powered(cooler) {
		t.Fatal("the cooler is not on power")
	}
	_, hi := 1<<30, -1<<30
	stepFuelled(g, gen, 6*clock.TicksPerHour/climateInterval, func(int) {
		hi = max(hi, int(g.thermo.Temps[east]))
	})
	if hi > int(settle)-50 || !g.thermoActive(cooler) {
		t.Fatalf("cold side up to %v (working %v), want well below the %v outside", temperature.Temp(hi), g.thermoActive(cooler), settle)
	}
	if w := g.thermo.Temps[west]; w < settle+50 {
		t.Fatalf("hot side %v, want well above the %v an unheated room keeps", w, settle)
	}
	t.Logf("cold side %v, hot side %v", temperature.Temp(hi), g.thermo.Temps[west])
}

// TestCoolerFacingOutdoorsDoesNothing: a cooler whose cold side is the
// open air cools nothing and so warms nothing behind it.
func TestCoolerFacingOutdoorsDoesNothing(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 20, Height: 10})
	steadyAir(g, 350)
	// a closed hut (x 2..6) with a gap in its east wall for the cooler,
	// its cold side outside
	for x := 1; x <= 7; x++ {
		for _, y := range []int{1, 7} {
			buildNow(t, g, construction.Wall, item.Wood, construction.South, x, y)
		}
	}
	for y := 2; y <= 6; y++ {
		for _, x := range []int{1, 7} {
			if x != 7 || y != 4 {
				buildNow(t, g, construction.Wall, item.Wood, construction.South, x, y)
			}
		}
	}
	cooler := buildNow(t, g, construction.Cooler, item.Steel, construction.East, 7, 4)
	g.RoofRooms()
	gen := onPower(t, g, 7, 4, 9, 3) // TEMP-1: the generator outside (it heats its room now)
	r := g.roomAt(4, 3)
	settle := temperature.Temp(350)
	g.thermo.Temps[r] = settle
	stepFuelled(g, gen, 6*clock.TicksPerHour/climateInterval, nil)
	if g.thermoActive(cooler) {
		t.Fatal("a cooler blowing into the open air works")
	}
	if d := g.thermo.Temps[r] - settle; d > 5 || d < -5 {
		t.Fatalf("the room behind an outdoor-facing cooler moved %v", d)
	}
}
