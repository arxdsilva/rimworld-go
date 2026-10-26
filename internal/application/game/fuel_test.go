package game

import (
	"testing"

	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/weather"
)

// PLAY-021 FUELGEN: the shared fuel table (fuel.go). Campfires keep today's
// numbers; torch lamps and wood-fired generators burn the same way.

// fuelGame is a small clear-weather map for the fuel tests.
func fuelGame(t *testing.T) *Game {
	t.Helper()
	g := New(Config{Seed: 1, Width: 40, Height: 24})
	jumpTo(g, 10, 12) // a spring noon
	g.SetWeather(weather.Clear, 0)
	g.HoldWeather() // Step must not roll rain over the torches
	return g
}

// placeFuel puts a finished building of kind k at (x, y), outside the
// build rules (the tests are about fuel, not placement).
func placeFuel(t *testing.T, g *Game, k construction.Kind, x, y int) entity.EntityID {
	t.Helper()
	stuff := item.Wood
	if construction.DefOf(k).MetalOnly {
		stuff = item.Steel
	}
	id := g.placeGenerated(construction.Building{Kind: k, Stuff: stuff, X: x, Y: y})
	if id == 0 {
		t.Fatalf("could not place %v at %d,%d", k, x, y)
	}
	return id
}

func TestFuelSpecTable(t *testing.T) {
	t.Parallel()
	camp, ok := fuelSpec(construction.Campfire)
	if !ok || camp.CapTicks != CampfireCapacity*TicksPerWood || camp.Below != CampfireCapacity*TicksPerWood/2 || camp.BurnPermil != 1000 {
		t.Fatalf("campfire spec changed: %+v %v", camp, ok)
	}
	torch, ok := fuelSpec(construction.TorchLamp)
	if !ok || torch.CapTicks != 8*TicksPerWood || torch.BurnPermil != 600 || torch.Below <= 0 || torch.Below >= torch.CapTicks {
		t.Fatalf("torch spec: %+v %v", torch, ok)
	}
	gen, ok := fuelSpec(construction.WoodGenerator)
	if !ok || gen.CapTicks != 15*TicksPerWood || gen.BurnPermil != 1600 || gen.Below <= 0 || gen.Below >= gen.CapTicks {
		t.Fatalf("wood generator spec: %+v %v", gen, ok)
	}
	for _, k := range []construction.Kind{construction.Table, construction.Lamp, construction.SolarPanel, construction.Bed} {
		if _, ok := fuelSpec(k); ok {
			t.Errorf("%v must not take fuel", k)
		}
	}
}

func TestCampfireBurnsOneIntervalPerClimateStep(t *testing.T) {
	t.Parallel()
	g := fuelGame(t)
	if !g.BuildNow("campfire", "wood", "south", 5, 5) {
		t.Fatal("build campfire")
	}
	g.FuelCampfire(5, 5)
	id := g.layerBldgAt(5, 5, construction.LayerObject)
	full := g.fuel[id]
	jumpTo(g, 45, 1) // a cold winter morning: the fire burns, never banks
	g.heatRooms()
	if got := full - g.fuel[id]; got != climateInterval {
		t.Fatalf("campfire burnt %d ticks of fuel in one interval, want %d", got, climateInterval)
	}
}

func TestTorchLampBurnsLogsInPhaseClimate(t *testing.T) {
	t.Parallel()
	g := fuelGame(t)
	id := placeFuel(t, g, construction.TorchLamp, 6, 6)
	spec, _ := fuelSpec(construction.TorchLamp)
	g.setFuel(id, spec.CapTicks)
	g.heatRooms()
	if want := spec.CapTicks - climateInterval*600/1000; g.fuel[id] != want {
		t.Fatalf("one interval: fuel %d, want %d (600 permille)", g.fuel[id], want)
	}
	before := g.fuel[id]
	g.Step(4 * climateInterval) // phaseClimate burns it on its own
	if burnt := before - g.fuel[id]; burnt < 3*150 || burnt%150 != 0 {
		t.Fatalf("torch burnt %d in four intervals, want 3-4 steps of 150", burnt)
	}
	b, _ := g.bldgs.Get(id)
	if !g.burning(id, b) {
		t.Fatal("a fuelled torch in dry weather must burn")
	}
	g.setFuel(id, 100)
	g.heatRooms()
	if g.fuel[id] != 0 {
		t.Fatalf("the last of the fuel burns out to 0, got %d", g.fuel[id])
	}
	if g.burning(id, b) {
		t.Fatal("an empty torch must not burn")
	}
}

func TestRainPutsOutAnOutdoorTorchButNotARoofedOne(t *testing.T) {
	t.Parallel()
	g := fuelGame(t)
	hut(t, g, 10, 2, 16, 8)
	out := placeFuel(t, g, construction.TorchLamp, 5, 5)
	in := placeFuel(t, g, construction.TorchLamp, 13, 4)
	spec, _ := fuelSpec(construction.TorchLamp)
	g.setFuel(out, spec.CapTicks)
	g.setFuel(in, spec.CapTicks)
	ob, _ := g.bldgs.Get(out)
	ib, _ := g.bldgs.Get(in)

	g.SetWeather(weather.Rain, 0.2)
	if !g.burning(out, ob) {
		t.Fatal("light rain (below the dousing intensity) must not put a torch out")
	}
	g.SetWeather(weather.Rain, 0.6)
	if g.burning(out, ob) {
		t.Fatal("hard rain must put the outdoor torch out")
	}
	if !g.burning(in, ib) {
		t.Fatal("a torch under a roof keeps burning in the rain")
	}
	g.heatRooms()
	if g.fuel[out] != spec.CapTicks {
		t.Fatalf("a doused torch burnt fuel: %d", g.fuel[out])
	}
	if g.fuel[in] != spec.CapTicks-150 {
		t.Fatalf("the sheltered torch should burn 150, has %d", g.fuel[in])
	}
	g.SetWeather(weather.Clear, 0)
	g.heatRooms()
	if g.fuel[out] != spec.CapTicks-150 {
		t.Fatalf("the torch should burn again after the rain: %d", g.fuel[out])
	}
}

func TestHaulerRefuelsTorchLamp(t *testing.T) {
	g := New(Config{Seed: 1, Width: 14, Height: 8})
	jumpTo(g, 10, 12)
	g.SetWeather(weather.Clear, 0)
	g.HoldWeather()
	id, _ := g.SpawnNamedPawn(2, 2, "Ada")
	g.SetNeeds(id, needs.Full, needs.Full)
	tid := placeFuel(t, g, construction.TorchLamp, 7, 3)
	if _, err := g.SpawnStack(item.Wood, 30, 2, 5); err != nil {
		t.Fatal(err)
	}
	g.Step(1500)
	spec, _ := fuelSpec(construction.TorchLamp)
	if g.fuel[tid] < spec.CapTicks-TicksPerWood {
		t.Fatalf("torch fuel = %d after refuelling, cap %d", g.fuel[tid], spec.CapTicks)
	}
	if st, _ := stackOf(g, item.Wood); st.Qty != 22 {
		t.Fatalf("wood left = %d, want 22 (a torch takes 8 logs)", st.Qty)
	}
	if g.fuel[tid] > spec.CapTicks {
		t.Fatalf("torch overfilled: %d", g.fuel[tid])
	}
}

func TestWoodGeneratorBurnsAndHaulerRefuelsIt(t *testing.T) {
	g := New(Config{Seed: 1, Width: 16, Height: 10})
	jumpTo(g, 10, 12)
	g.SetWeather(weather.Clear, 0)
	g.HoldWeather()
	pid, _ := g.SpawnNamedPawn(2, 2, "Ada")
	g.SetNeeds(pid, needs.Full, needs.Full)
	gid := placeFuel(t, g, construction.WoodGenerator, 8, 3)
	spec, _ := fuelSpec(construction.WoodGenerator)

	// burning: 1600 permille of an interval, only while fuelled
	if g.generatorRunning(gid) {
		t.Fatal("an empty generator is not running")
	}
	g.setFuel(gid, spec.CapTicks)
	if !g.generatorRunning(gid) {
		t.Fatal("a fuelled generator runs by default")
	}
	g.heatRooms()
	if want := spec.CapTicks - climateInterval*1600/1000; g.fuel[gid] != want {
		t.Fatalf("one interval: fuel %d, want %d (1600 permille)", g.fuel[gid], want)
	}
	g.setFuel(gid, 300)
	g.heatRooms()
	if g.fuel[gid] != 0 {
		t.Fatalf("burns out to exactly 0, got %d", g.fuel[gid])
	}
	g.heatRooms()
	if g.fuel[gid] != 0 || g.generatorRunning(gid) {
		t.Fatal("an empty generator burns nothing and stops running")
	}

	// refuelling: a hauler brings logs until it holds a full tank
	if _, err := g.SpawnStack(item.Wood, 40, 2, 5); err != nil {
		t.Fatal(err)
	}
	g.Step(2500)
	if g.fuel[gid] < spec.CapTicks-TicksPerWood {
		t.Fatalf("generator fuel = %d after refuelling, cap %d", g.fuel[gid], spec.CapTicks)
	}
	if st, _ := stackOf(g, item.Wood); st.Qty != 25 {
		t.Fatalf("wood left = %d, want 25 (a generator takes 15 logs)", st.Qty)
	}
}

func TestWoodGeneratorIsNotDousedByRain(t *testing.T) {
	t.Parallel()
	g := fuelGame(t)
	gid := placeFuel(t, g, construction.WoodGenerator, 8, 8)
	spec, _ := fuelSpec(construction.WoodGenerator)
	g.setFuel(gid, spec.CapTicks)
	g.SetWeather(weather.Rain, 0.9)
	b, _ := g.bldgs.Get(gid)
	if !g.burning(gid, b) {
		t.Fatal("a generator is a machine: rain does not put it out")
	}
}

func TestRefuelPicksTheEmptiestByFillAcrossKinds(t *testing.T) {
	g := New(Config{Seed: 1, Width: 24, Height: 10})
	jumpTo(g, 10, 12)
	g.SetWeather(weather.Clear, 0)
	g.HoldWeather()
	pid, _ := g.SpawnNamedPawn(2, 2, "Ada")
	g.SetNeeds(pid, needs.Full, needs.Full)
	if !g.BuildNow("campfire", "wood", "south", 10, 3) {
		t.Fatal("build campfire")
	}
	camp := g.layerBldgAt(10, 3, construction.LayerObject)
	torch := placeFuel(t, g, construction.TorchLamp, 16, 3)
	cs, _ := fuelSpec(construction.Campfire)
	ts, _ := fuelSpec(construction.TorchLamp)
	// the campfire holds 40% of its tank, the torch 20%: the torch first,
	// although it holds fewer ticks than the campfire on both counts
	g.setFuel(camp, cs.CapTicks*40/100)
	g.setFuel(torch, ts.CapTicks*20/100)
	if _, err := g.SpawnStack(item.Wood, 4, 2, 5); err != nil {
		t.Fatal(err)
	}
	if !g.assignRefuel(pid) {
		t.Fatal("nobody was sent to refuel")
	}
	j, _ := g.jobs.Get(pid)
	if j.Dest != torch {
		t.Fatalf("hauler heads for %d, want the emptier torch %d", j.Dest, torch)
	}
}

func TestFuelUsersListIsCachedPerStructVerInBuildingOrder(t *testing.T) {
	t.Parallel()
	g := fuelGame(t)
	camp1 := placeFuel(t, g, construction.Campfire, 3, 3)
	_ = placeFuel(t, g, construction.Table, 6, 3)
	torch := placeFuel(t, g, construction.TorchLamp, 9, 3)
	gen := placeFuel(t, g, construction.WoodGenerator, 12, 3)
	camp2 := placeFuel(t, g, construction.Campfire, 3, 8)

	want := []entity.EntityID{camp1, torch, gen, camp2}
	got := g.fuelUsers()
	if len(got) != len(want) {
		t.Fatalf("fuel users %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("fuel users %v, want %v (building order)", got, want)
		}
	}
	builds := g.vents.fuel.builds
	_ = g.fuelUsers()
	g.heatRooms()
	if g.vents.fuel.builds != builds {
		t.Fatal("the fuel-user list was rebuilt without a structure change")
	}
	late := placeFuel(t, g, construction.TorchLamp, 20, 3)
	got = g.fuelUsers()
	if len(got) != 5 || got[4] != late {
		t.Fatalf("a new torch must join the list: %v", got)
	}
	g.removeBuilding(camp1)
	for _, id := range g.fuelUsers() {
		if id == camp1 {
			t.Fatal("a removed campfire is still on the list")
		}
	}
}

func TestFuelCampfireFillsAnyFuelUser(t *testing.T) {
	t.Parallel()
	g := fuelGame(t)
	torch := placeFuel(t, g, construction.TorchLamp, 6, 6)
	g.FuelCampfire(6, 6)
	if ts, _ := fuelSpec(construction.TorchLamp); g.fuel[torch] != ts.CapTicks {
		t.Fatalf("torch filled to %d, want %d", g.fuel[torch], ts.CapTicks)
	}
}

func TestFuelPctIsTheShareOfAFullTank(t *testing.T) {
	t.Parallel()
	g := fuelGame(t)
	torch := placeFuel(t, g, construction.TorchLamp, 6, 6)
	table := placeFuel(t, g, construction.Table, 9, 6)
	ts, _ := fuelSpec(construction.TorchLamp)
	g.setFuel(torch, ts.CapTicks/4)
	b, _ := g.bldgs.Get(torch)
	if pct, ok := g.fuelPct(torch, b); !ok || pct != 25 {
		t.Fatalf("a quarter-full torch: %d%% %v", pct, ok)
	}
	tb, _ := g.bldgs.Get(table)
	if _, ok := g.fuelPct(table, tb); ok {
		t.Fatal("a table has no fuel gauge")
	}
}
