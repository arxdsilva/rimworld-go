package game

import (
	"testing"

	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/temperature"
)

// TEMP-1 wiki proofs (ENG-003): the room model against the wiki's own
// measurements, in the real game.

// wikiRoom walls in an n x n interior with its top-left at (x0, y0), one
// wooden wall thick, leaving out the wall cell gap (nil: none), roofs it
// and returns the room id.
func wikiRoom(t *testing.T, g *Game, x0, y0, n int, gap *[2]int) int {
	t.Helper()
	for x := x0 - 1; x <= x0+n; x++ {
		for y := y0 - 1; y <= y0+n; y++ {
			edge := x == x0-1 || x == x0+n || y == y0-1 || y == y0+n
			if edge && (gap == nil || gap[0] != x || gap[1] != y) {
				buildNow(t, g, construction.Wall, item.Wood, construction.South, x, y)
			}
		}
	}
	if gap != nil {
		return -1 // the caller fills the gap, then roofs
	}
	g.RoofRooms()
	return g.roomAt(x0, y0)
}

// settle steps g for days, keeping generator gen fuelled, and returns the
// air of room r over the last few hours (lo, hi).
func settle(g *Game, gen entity.EntityID, r, days int) (lo, hi temperature.Temp) {
	stepFuelled(g, gen, days*clock.TicksPerDay/climateInterval, nil)
	lo, hi = 1<<20, -1<<20
	stepFuelled(g, gen, 4*clock.TicksPerHour/climateInterval, func(int) {
		lo, hi = min(lo, g.thermo.Temps[r]), max(hi, g.thermo.Temps[r])
	})
	return lo, hi
}

// TestHeaterHoldsWikiRooms: wiki: Heater, at -17 C outside one heater
// holds a 9x9 room at about 20 C and an 11x11 at about 10 C.
func TestHeaterHoldsWikiRooms(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		n        int
		min, max temperature.Temp
	}{{9, 185, 215}, {11, 85, 115}} {
		g := New(Config{Seed: 1, Width: 30, Height: 30})
		steadyAir(g, -170)
		r := wikiRoom(t, g, 8, 8, c.n, nil)
		heater := buildNow(t, g, construction.Heater, item.Steel, construction.South, 8, 12)
		gen := onPower(t, g, 8, 12, 4, 11) // the generator outside: its own heat stays out
		if r < 0 || g.roomAt(8, 12) != r || g.roomAt(4, 11) >= 0 || !g.powered(heater) {
			t.Fatalf("%dx%d: room %d, generator room %d, powered %v", c.n, c.n, r, g.roomAt(4, 11), g.powered(heater))
		}
		if v := g.World().Rooms[r]; v.ExposedWalls != 4*c.n || v.RoofKind != "thin" {
			t.Fatalf("%dx%d: %d exposed walls, roof %q", c.n, c.n, v.ExposedWalls, v.RoofKind)
		}
		lo, hi := settle(g, gen, r, 3)
		t.Logf("%dx%d at -17 C: %v..%v", c.n, c.n, lo, hi)
		if lo < c.min || hi > c.max {
			t.Errorf("%dx%d holds %v..%v, wiki about %v", c.n, c.n, lo, hi, (c.min+c.max)/2)
		}
	}
}

// TestCoolerHoldsWikiRoom: wiki: Cooler, at 27-38 C outside one cooler
// (its hot side outdoors) holds a 10x10 room at 10-16 C. Read at 30 and
// 35 C outside; the band follows the outside air.
func TestCoolerHoldsWikiRoom(t *testing.T) {
	t.Parallel()
	for _, out := range []int{300, 350} {
		g := New(Config{Seed: 1, Width: 30, Height: 30})
		steadyAir(g, out)
		gap := [2]int{18, 12} // the east wall
		wikiRoom(t, g, 8, 8, 10, &gap)
		cooler := buildNow(t, g, construction.Cooler, item.Steel, construction.West, 18, 12)
		g.RoofRooms()
		r := g.roomAt(12, 12)
		gen := onPower(t, g, 18, 12, 20, 11)
		g.Enqueue(command.Command{Kind: command.SetThermostat, Target: uint64(cooler), Count: -200})
		g.Step(1)
		if r < 0 || !g.powered(cooler) || g.rooms[r].contents.Cells != 100 {
			t.Fatalf("room %d powered %v", r, g.powered(cooler))
		}
		lo, hi := settle(g, gen, r, 3)
		t.Logf("10x10 at %v outside: %v..%v", temperature.Temp(out), lo, hi)
		if lo < 100 || hi > 160 {
			t.Errorf("at %v outside the cooler holds %v..%v, wiki 10-16 C", temperature.Temp(out), lo, hi)
		}
		if v := bldgViewOf(g, cooler); v.HeatRate >= 0 || v.HeatRate < -210 {
			t.Errorf("a working cooler shows %d tenths of heat/s, want its (negative) cold-side output", v.HeatRate)
		}
	}
}

// TestDoubleWallCounts: a room ringed by two walls has double faces only
// (the wiki: half the loss), and it cools half as fast as a single one.
func TestDoubleWallCounts(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 30, Height: 20})
	steadyAir(g, -200)
	one := wikiRoom(t, g, 3, 3, 5, nil)
	for x := 13; x <= 21; x++ { // two rings around a 5x5 at (15, 5)
		for y := 3; y <= 11; y++ {
			if x == 13 || x == 21 || y == 3 || y == 11 || x == 14 || x == 20 || y == 4 || y == 10 {
				buildNow(t, g, construction.Wall, item.Wood, construction.South, x, y)
			}
		}
	}
	g.RoofRooms()
	two := g.roomAt(17, 7)
	v := g.World().Rooms
	if v[one].ExposedWalls != 20 || v[one].DoubleWalls != 0 || v[two].ExposedWalls != 0 || v[two].DoubleWalls != 20 {
		t.Fatalf("single %d/%d, double %d/%d", v[one].ExposedWalls, v[one].DoubleWalls, v[two].ExposedWalls, v[two].DoubleWalls)
	}
	ea, eb := g.thermo.RoomOf(one).Exposure, g.thermo.RoomOf(two).Exposure
	ea.Thin, eb.Thin = 0, 0 // the walls alone
	la, lb := ea.Loss(-9555000), eb.Loss(-9555000)
	if d := 2*lb - la; d > 50 || d < -50 {
		t.Fatalf("double wall loses %d, single %d: want half", lb, la)
	}
}

// TestOpenDoorExchangesClosedDoesNot: twin huts in the cold, one door
// held open: its hut cools toward the outside, the shut one barely moves
// in the same time.
func TestOpenDoorExchangesClosedDoesNot(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 20, Height: 10})
	steadyAir(g, -200)
	hut(t, g, 1, 1, 7, 7)
	hut(t, g, 10, 1, 16, 7)
	open, shut := g.roomAt(4, 4), g.roomAt(13, 4)
	g.combat.Doors.SetHold(g.layerBldgAt(4, 7, construction.LayerObject), true)
	g.Step(clock.TicksPerHour / 4) // the door swings open
	g.thermo.Temps[open], g.thermo.Temps[shut] = 200, 200
	g.Step(clock.TicksPerHour)
	a, b := g.thermo.Temps[open], g.thermo.Temps[shut]
	t.Logf("after an hour at -20 C: open door %v, shut %v", a, b)
	if !(a < b-30) {
		t.Fatalf("open-door hut %v, shut %v: the open door should let the cold in", a, b)
	}
}

// TestVentMixesRoomsWhenOpen: two rooms sharing a wall with a vent: open,
// the hot one warms the cold one; flicked shut, they drift apart.
func TestVentMixesRoomsWhenOpen(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 20, Height: 10})
	steadyAir(g, 0)
	for x := 1; x <= 13; x++ {
		for _, y := range []int{1, 7} {
			buildNow(t, g, construction.Wall, item.Wood, construction.South, x, y)
		}
	}
	for y := 2; y <= 6; y++ {
		for _, x := range []int{1, 7, 13} {
			if x != 7 || y != 4 {
				buildNow(t, g, construction.Wall, item.Wood, construction.South, x, y)
			}
		}
	}
	vent := buildNow(t, g, construction.Vent, item.Steel, construction.East, 7, 4)
	g.RoofRooms()
	west, east := g.roomAt(4, 4), g.roomAt(10, 4)
	if west < 0 || east < 0 || west == east {
		t.Fatalf("rooms %d %d", west, east)
	}
	mix := func() temperature.Temp {
		g.thermo.Temps[west], g.thermo.Temps[east] = 300, 0
		g.Step(clock.TicksPerHour / 2)
		return g.thermo.Temps[west] - g.thermo.Temps[east]
	}
	if v := bldgViewOf(g, vent); !v.Open {
		t.Fatal("a new vent is open")
	}
	opened := mix()
	g.Enqueue(command.Command{Kind: command.ToggleVent, Target: uint64(vent), Enabled: false})
	g.Step(1)
	if !g.flickPending(vent) {
		t.Fatal("closing a vent waits for a colonist's flick")
	}
	g.flick(vent)
	if v := bldgViewOf(g, vent); v.Open {
		t.Fatal("the flicked vent still reads open")
	}
	closed := mix()
	t.Logf("difference after half an hour: open %v, shut %v", opened, closed)
	if !(opened < 100 && closed > 250) {
		t.Fatalf("open vent leaves %v between the rooms, shut %v", opened, closed)
	}
}

// TestCoolerFreezer: a 5x5 freezer with one cooler (hot side outdoors)
// set to -10 C holds there at 15 C outside, so meat keeps (rot stops at
// 0 C and below).
func TestCoolerFreezer(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 24, Height: 20})
	steadyAir(g, 150)
	gap := [2]int{11, 8}
	wikiRoom(t, g, 6, 6, 5, &gap)
	cooler := buildNow(t, g, construction.Cooler, item.Steel, construction.West, 11, 8)
	g.RoofRooms()
	r := g.roomAt(8, 8)
	gen := onPower(t, g, 11, 8, 13, 7)
	g.Enqueue(command.Command{Kind: command.SetThermostat, Target: uint64(cooler), Count: -100})
	g.Step(1)
	lo, hi := settle(g, gen, r, 2)
	if lo < -110 || hi > -90 {
		t.Fatalf("freezer %v..%v at 15 C outside, want held at -10 C", lo, hi)
	}
}

// TestCampfireBankingHoldsInWinter: the campfire's bank band (28 C bank,
// 18 C relight, banking only on 4-hour marks) holds in a deep winter too:
// at most one change of state every two hours (no-flap).
func TestCampfireBankingHoldsInWinter(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 16, Height: 12})
	jumpTo(g, 45, 0)
	hut(t, g, 6, 1, 12, 7)
	if !g.BuildNow("campfire", "wood", "south", 9, 3) {
		t.Fatal("build campfire")
	}
	bid := g.layerBldgAt(9, 3, construction.LayerObject)
	b, _ := g.bldgs.Get(bid)
	flips, banks, was := 0, 0, false
	for range 2 * clock.TicksPerDay / climateInterval {
		g.fuel[bid] = CampfireCapacity * TicksPerWood
		g.Step(climateInterval)
		now := g.burning(bid, b)
		if now != was {
			flips++
		}
		if !now {
			banks++
		}
		was = now
	}
	t.Logf("winter: %d changes in 2 days, %d banked intervals", flips, banks)
	if banks == 0 || flips > 24 {
		t.Fatalf("campfire changed state %d times in 2 winter days (banked %d intervals), want 1..24", flips, banks)
	}
}
