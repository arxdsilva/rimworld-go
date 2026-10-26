package game

import (
	"testing"

	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/weather"
)

// PLAY-021 CONSUMERS: a lamp lights only on power, a torch lamp while it
// burns its logs, and a room is lit by the lights shining in it now.

// onPower gives the device at (x, y) power: a conduit at (x, y) (under
// it) and a fuelled wood-fired generator with its top-left at (gx, gy),
// within reach of the conduit. The grid is balanced at once, so the
// device runs now. A full tank lasts about 22 game hours.
func onPower(t *testing.T, g *Game, x, y, gx, gy int) (gen entity.EntityID) {
	t.Helper()
	buildNow(t, g, construction.PowerConduit, item.Steel, construction.South, x, y)
	gen = buildNow(t, g, construction.WoodGenerator, item.Steel, construction.South, gx, gy)
	g.setFuel(gen, g.fuelCap(gen))
	g.balancePower(uint64(g.clock.Now()))
	return gen
}

// litLamp builds a lamp at (x, y) on power (onPower: the generator's
// top-left at (gx, gy)); it is lit now.
func litLamp(t *testing.T, g *Game, x, y, gx, gy int) (lamp, gen entity.EntityID) {
	t.Helper()
	lamp = buildNow(t, g, construction.Lamp, item.Wood, construction.South, x, y)
	gen = onPower(t, g, x, y, gx, gy)
	if !g.powered(lamp) {
		t.Fatalf("lamp at %d,%d is not on power", x, y)
	}
	return lamp, gen
}

// glowView is building id's Glow and GlowRadius in the read model.
func glowView(t *testing.T, g *Game, id entity.EntityID) (bool, int) {
	t.Helper()
	for _, bv := range g.World().Buildings {
		if bv.ID == id {
			return bv.Glow, bv.GlowRadius
		}
	}
	t.Fatalf("no view of %d", id)
	return false, 0
}

// roomLit reports the room at (x, y) lit in the read model.
func roomLit(t *testing.T, g *Game, x, y int) bool {
	t.Helper()
	v, ok := g.RoomInfo(x, y)
	if !ok {
		t.Fatalf("no room at %d,%d", x, y)
	}
	return v.Lit
}

// TestLampNeedsPower: a lamp with no grid gives no light (grid, Glow); on
// power it lights to its radius; when the generator runs dry it goes dark
// at the next balance.
func TestLampNeedsPower(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 30, Height: 12})
	dark := buildNow(t, g, construction.Lamp, item.Wood, construction.South, 4, 5)
	g.Step(powerInterval)
	if l := g.artificialLight(4, 5); l != 0 {
		t.Fatalf("a lamp without power lights its cell %.2f", l)
	}
	if on, _ := glowView(t, g, dark); on {
		t.Fatal("a lamp without power glows")
	}
	lamp, gen := litLamp(t, g, 20, 5, 16, 4)
	if l := g.artificialLight(20, 5); !near(l, lampLight) {
		t.Fatalf("a powered lamp lights its cell %.3f, want %.1f", l, lampLight)
	}
	if l := g.artificialLight(20, 5+lampRadius); !near(l, lampLight/float64(lampRadius+1)) {
		t.Fatalf("a powered lamp at its radius: %.3f", l)
	}
	if on, rad := glowView(t, g, lamp); !on || rad != lampRadius {
		t.Fatalf("powered lamp: glow %v radius %d", on, rad)
	}
	if l := g.artificialLight(4, 5); l != 0 {
		t.Fatalf("the unconnected lamp lights %.2f", l)
	}
	g.setFuel(gen, 0) // the generator runs dry
	g.Step(powerInterval)
	if g.powered(lamp) || g.artificialLight(20, 5) != 0 {
		t.Fatalf("lamp powered %v, light %.2f after the generator ran dry", g.powered(lamp), g.artificialLight(20, 5))
	}
	if on, _ := glowView(t, g, lamp); on {
		t.Fatal("an unpowered lamp glows")
	}
}

// TestTorchLampBurnsLogs: a torch lamp lights nothing until it has logs;
// fuelled, it gives a smaller, dimmer light than a lamp, lights its room
// and burns its logs at 600 per mille; out of fuel it goes dark.
func TestTorchLampBurnsLogs(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 24, Height: 12})
	g.HoldWeather()
	g.SetWeather(weather.Clear, 0)
	hut(t, g, 2, 2, 9, 9) // interior 3..8
	torch := buildNow(t, g, construction.TorchLamp, item.Wood, construction.South, 5, 4)
	if l := g.artificialLight(5, 4); l != 0 || roomLit(t, g, 5, 5) {
		t.Fatalf("an unfuelled torch lamp: light %.2f, room lit %v", l, roomLit(t, g, 5, 5))
	}
	g.FuelCampfire(5, 4) // a full tank of logs
	if got, want := g.fuel[torch], torchLogs*TicksPerWood; got != want {
		t.Fatalf("torch tank %d, want %d", got, want)
	}
	if l := g.artificialLight(5, 4); !near(l, torchLight) {
		t.Fatalf("a burning torch lights its cell %.3f, want %.1f", l, torchLight)
	}
	if l := g.artificialLight(5, 4+torchRadius); !near(l, torchLight/float64(torchRadius+1)) {
		t.Fatalf("a burning torch at its radius: %.3f", l)
	}
	if l := g.artificialLight(5, 4+torchRadius+1); l != 0 {
		t.Fatalf("a burning torch past its radius: %.3f", l)
	}
	if on, rad := glowView(t, g, torch); !on || rad != torchRadius {
		t.Fatalf("burning torch: glow %v radius %d", on, rad)
	}
	if !roomLit(t, g, 7, 7) {
		t.Fatal("a burning torch does not light its room")
	}
	f0 := g.fuel[torch]
	g.Step(climateInterval)
	if burnt, want := f0-g.fuel[torch], climateInterval*torchBurn/1000; burnt != want {
		t.Fatalf("a torch burnt %d ticks of fuel in a climate interval, want %d", burnt, want)
	}
	g.setFuel(torch, 0) // burnt out
	if l := g.artificialLight(5, 4); l != 0 || roomLit(t, g, 5, 5) {
		t.Fatalf("a burnt-out torch: light %.2f, room lit %v", l, roomLit(t, g, 5, 5))
	}
	if on, _ := glowView(t, g, torch); on {
		t.Fatal("a burnt-out torch glows")
	}
}

// TestTorchLampGoesOutInTheRain: under the open sky heavy rain puts a
// torch out (no light, no glow, no fuel burnt) with no building change;
// it lights again when the rain stops, and under a roof it burns on.
func TestTorchLampGoesOutInTheRain(t *testing.T) {
	t.Parallel()
	g := weatherGame(t)
	g.HoldWeather()
	g.SetWeather(weather.Clear, 0)
	torch := buildNow(t, g, construction.TorchLamp, item.Wood, construction.South, 4, 6)
	g.FuelCampfire(4, 6)
	if l := g.artificialLight(4, 6); !near(l, torchLight) {
		t.Fatalf("a burning torch in clear weather lights %.3f", l)
	}
	g.SetWeather(weather.Rain, 0.8)
	if l := g.artificialLight(4, 6); l != 0 {
		t.Fatalf("a torch in the rain still lights %.3f", l)
	}
	if on, _ := glowView(t, g, torch); on {
		t.Fatal("a doused torch glows")
	}
	f := g.fuel[torch]
	g.Step(climateInterval)
	if g.fuel[torch] != f {
		t.Fatalf("a doused torch burnt fuel: %d -> %d", f, g.fuel[torch])
	}
	g.SetWeather(weather.Clear, 0)
	if l := g.artificialLight(4, 6); !near(l, torchLight) {
		t.Fatalf("the torch after the rain lights %.3f", l)
	}
	// under a roof it burns through the rain
	hut(t, g, 12, 2, 20, 10)
	roofed := buildNow(t, g, construction.TorchLamp, item.Wood, construction.South, 16, 5)
	g.FuelCampfire(16, 5)
	g.SetWeather(weather.Rain, 0.8)
	if l := g.artificialLight(16, 5); !near(l, torchLight) || !roomLit(t, g, 16, 7) {
		t.Fatalf("a roofed torch in the rain: light %.3f, room lit %v", l, roomLit(t, g, 16, 7))
	}
	if on, _ := glowView(t, g, roofed); !on {
		t.Fatal("a roofed torch in the rain does not glow")
	}
	if l := g.artificialLight(4, 6); l != 0 {
		t.Fatalf("the open-sky torch lights %.3f in the rain", l)
	}
}

// TestRoomLitFollowsTheLights: a room is lit by a powered lamp, not an
// unpowered one; by a burning torch lamp or campfire, not a cold one. The
// light sources are listed once per structure, not per view.
func TestRoomLitFollowsTheLights(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 30, Height: 12})
	g.HoldWeather()
	g.SetWeather(weather.Clear, 0)
	hut(t, g, 1, 1, 8, 8)   // the lamp's
	hut(t, g, 11, 1, 18, 8) // the torch's
	hut(t, g, 21, 1, 28, 8) // the campfire's
	lamp := buildNow(t, g, construction.Lamp, item.Wood, construction.South, 4, 3)
	buildNow(t, g, construction.TorchLamp, item.Wood, construction.South, 14, 3)
	buildNow(t, g, construction.Campfire, item.Wood, construction.South, 24, 3)
	g.Step(powerInterval)
	for _, c := range [][2]int{{5, 5}, {15, 5}, {25, 5}} {
		if roomLit(t, g, c[0], c[1]) {
			t.Fatalf("room at %d,%d lit by an unpowered lamp or a cold fire", c[0], c[1])
		}
	}
	onPower(t, g, 4, 3, 2, 5)
	if !g.powered(lamp) || !roomLit(t, g, 5, 5) {
		t.Fatalf("a powered lamp (%v) does not light its room", g.powered(lamp))
	}
	g.FuelCampfire(14, 3)
	g.FuelCampfire(24, 3)
	if !roomLit(t, g, 15, 5) || !roomLit(t, g, 25, 5) {
		t.Fatal("a burning torch or campfire does not light its room")
	}
	builds := g.light.srcs.builds
	for range 3 {
		g.roomViews()
		g.World()
	}
	if g.light.srcs.builds != builds {
		t.Fatalf("light sources listed %d more times with no building change", g.light.srcs.builds-builds)
	}
	g.setFuel(g.layerBldgAt(14, 3, construction.LayerObject), 0)
	if roomLit(t, g, 15, 5) || !roomLit(t, g, 25, 5) || !roomLit(t, g, 5, 5) {
		t.Fatal("a burnt-out torch's room stays lit, or another went dark")
	}
}

// TestLampLockedBeforeElectricity: a colony builds torch lamps from the
// start; the lamp waits for Electricity.
func TestLampLockedBeforeElectricity(t *testing.T) {
	t.Parallel()
	g := NewColony(Config{Seed: 3, Width: 48, Height: 40})
	if !g.buildLocked(construction.Lamp) || g.lockedBy("build:lamp") != "Electricity" {
		t.Fatalf("lamp locked %v by %q, want Electricity", g.buildLocked(construction.Lamp), g.lockedBy("build:lamp"))
	}
	if g.buildLocked(construction.TorchLamp) {
		t.Fatal("the torch lamp is locked")
	}
	g.AddResearch("electricity", 1<<20)
	if g.buildLocked(construction.Lamp) {
		t.Fatal("the lamp is still locked after Electricity")
	}
}
