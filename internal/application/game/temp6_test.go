package game

import (
	"testing"

	"rimworld-go/internal/application/game/thermo"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/exposure"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/world/biome"
	"rimworld-go/internal/domain/world/temperature"
)

// TEMP-6 close-out: the outdoors judgements where the game uses them,
// spontaneous ignition and heat burns, the freezer proof.

// t6Air holds room r's air at t (tenths) and runs scorch at the next
// equalisation tick, as heatRooms does after its step.
func t6Air(g *Game, r int, t temperature.Temp) {
	g.clock.Advance(temperature.EqualiseInterval - int(uint64(g.clock.Now())%temperature.EqualiseInterval))
	g.thermo.Temps[r] = t
	scorch(g)
}

// TestHotRoomIgnitesFlammablesOnCooldown (TEMP-6, wiki: Temperature): a
// storeroom of wood held at 300 C catches fire; at 234 C it never does;
// and fires light no faster than one per thermo.IgniteCooldown however
// long the air stays hot (no-flap: the air never relights every step).
func TestHotRoomIgnitesFlammablesOnCooldown(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 20, Height: 12})
	hut(t, g, 2, 2, 12, 9)
	for x := 3; x <= 11; x++ {
		for y := 3; y <= 8; y++ {
			if _, err := g.SpawnStack(item.Wood, 10, x, y); err != nil {
				t.Fatal(err)
			}
		}
	}
	r := g.roomAt(5, 5)
	for range 200 {
		t6Air(g, r, thermo.IgniteAt-10)
	}
	if len(g.fire.fires) != 0 {
		t.Fatalf("%d fires at 234 C", len(g.fire.fires))
	}
	lit, last, start := 0, uint64(0), uint64(g.clock.Now())
	for range 100 {
		before := len(g.fire.fires)
		t6Air(g, r, 3000)
		if len(g.fire.fires) > before {
			now := uint64(g.clock.Now())
			if lit > 0 && now-last < thermo.IgniteCooldown {
				t.Fatalf("fires lit %d ticks apart", now-last)
			}
			lit, last = lit+1, now
		}
	}
	span := uint64(g.clock.Now()) - start
	if lit == 0 || lit > int(span/thermo.IgniteCooldown)+1 {
		t.Fatalf("%d spontaneous fires in %d ticks", lit, span)
	}
}

// TestHeatBurnsOnCadence (TEMP-6, wiki: Temperature): a colonist and a
// cow in air more than 150 C over their comfortable maximum take burns,
// at most once per exposure.BurnEvery ticks even with the air wobbling
// across the line every step (no-flap); just under it, none.
func TestHeatBurnsOnCadence(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 20, Height: 12})
	hut(t, g, 2, 2, 12, 9)
	id, _ := g.SpawnNamedPawn(5, 5, "Ada")
	cow, err := g.SpawnAnimal("cow", 9, 6)
	if err != nil {
		t.Fatal(err)
	}
	r := g.roomAt(5, 5)
	line := temperature.Temp(g.bandsOf(id).Max + exposure.BurnMargin)
	for range 40 {
		t6Air(g, r, line)
	}
	if v, _ := g.vital.Get(id); v.Injury != 0 {
		t.Fatalf("burned at the line: injury %d", v.Injury)
	}
	hits, prev, start := 0, 0, uint64(g.clock.Now())
	for i := range 80 {
		t6Air(g, r, line+temperature.Temp(i%2*2-1)) // one tenth either side
		if v, _ := g.vital.Get(id); v.Injury > prev {
			hits, prev = hits+1, v.Injury
		}
	}
	span := int(uint64(g.clock.Now()) - start)
	if hits == 0 || hits > span/exposure.BurnEvery+1 {
		t.Fatalf("%d burns in %d ticks wobbling across the line", hits, span)
	}
	a, _ := g.animals.Get(cow)
	before := a.Injury
	for range 8 {
		t6Air(g, r, 3000)
	}
	if a, _ = g.animals.Get(cow); a.Injury <= before {
		t.Fatalf("the cow took no burns at 300 C: %d -> %d", before, a.Injury)
	}
}

// TestFreezerKeepsMeatFrozen (TEMP-6, wiki: Temperature: food doesn't
// spoil at 0 C or below; only an electric cooler makes a freezer): a
// cooler set to -5 C in a desert summer pulls a sealed storeroom below
// freezing and the meat in it stops rotting, while meat outside spoils.
func TestFreezerKeepsMeatFrozen(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 24, Height: 14, Biome: biome.Desert})
	jumpTo(g, 20, 6)
	for x := 4; x <= 10; x++ {
		for y := 4; y <= 10; y++ {
			if (x == 4 || x == 10 || y == 4 || y == 10) && !(x == 7 && y == 4) {
				buildNow(t, g, construction.Wall, item.Wood, construction.South, x, y)
			}
		}
	}
	cooler := buildNow(t, g, construction.Cooler, item.Steel, construction.South, 7, 4) // in the north wall's gap, cold side in
	g.RoofRooms()
	g.setThermoTarget(cooler, -50)
	gen := psFuelled(t, g, 7, 1)
	psConduits(t, g, 2, 6, 8)
	buildNow(t, g, construction.PowerConduit, item.Steel, construction.South, 7, 3)
	in, _ := g.SpawnStack(item.Meat, 20, 6, 7)
	out, _ := g.SpawnStack(item.Meat, 20, 16, 7)
	r := g.roomAt(6, 7)
	frozenAt, rotThen, hottest := -1, 0, -1000
	for h := 1; h <= 72; h++ {
		g.Step(clock.TicksPerHour)
		g.setFuel(gen, g.fuelCap(gen)) // keep the lights on
		hottest = max(hottest, g.TemperatureAt(16, 7))
		st, _ := g.stacks.Get(in)
		if frozenAt < 0 && g.thermo.Temps[r] <= 0 {
			frozenAt, rotThen = h, st.Rot
		}
		if frozenAt > 0 && g.thermo.Temps[r] > 0 {
			t.Fatalf("hour %d: the freezer thawed to %d tenths", h, g.thermo.Temps[r])
		}
	}
	if frozenAt < 0 || frozenAt > 24 {
		t.Fatalf("the freezer room is at %d tenths (frozen at hour %d), %d outside", g.thermo.Temps[r], frozenAt, g.TemperatureAt(16, 7))
	}
	t.Logf("freezer below 0 C from hour %d, %d tenths after 72 h; outside up to %d", frozenAt, g.thermo.Temps[r], hottest)
	if st, _ := g.stacks.Get(in); st.Rot != rotThen || st.Rotten() {
		t.Fatalf("frozen meat rotted on: rot %d at freezing (hour %d), %d now", rotThen, frozenAt, st.Rot)
	}
	if st, ok := g.stacks.Get(out); ok && st.Rot <= rotThen || hottest < 250 {
		t.Fatalf("meat outside (hottest %d tenths) rots no faster: %d", hottest, st.Rot)
	}
}

// TestOutdoorsJudgementsInTheGame (TEMP-6, wiki: Rooms): a small walled
// yard with no roof is no outdoors for benches or sleepers, a big one is;
// the open map is; a room a quarter-plus unroofed is outdoors for surgery.
func TestOutdoorsJudgementsInTheGame(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 48, Height: 30})
	hutWalls(t, g, 1, 1, 8, 8)    // a 6x6 yard
	hutWalls(t, g, 12, 1, 33, 22) // a 20x20 yard
	hut(t, g, 36, 1, 46, 11)      // a roofed 9x9
	shop, rest := craftShop{g}, restRooms{g}
	if _, indoors := shop.Cleanliness(4, 4); !indoors || rest.Outside(4, 4) {
		t.Fatalf("small open yard: bench indoors %v, sleeper outside %v", indoors, rest.Outside(4, 4))
	}
	if _, indoors := shop.Cleanliness(20, 10); indoors || !rest.Outside(20, 10) {
		t.Fatalf("400-cell open yard: bench indoors %v, sleeper outside %v", indoors, rest.Outside(20, 10))
	}
	if _, indoors := shop.Cleanliness(5, 26); indoors || !rest.Outside(5, 26) {
		t.Fatal("the open map is not outdoors")
	}
	if _, out := g.surgeryCleanliness(40, 5); out {
		t.Fatal("a roofed room is outdoors for surgery")
	}
	w := g.grid.Width()
	for i := 0; i < 21; i++ { // 21 of 81 cells: 26 %
		g.setRoof((2+i/9)*w+37+i%9, 0)
	}
	if _, out := g.surgeryCleanliness(40, 5); !out {
		t.Fatal("a room 26 % unroofed is not outdoors for surgery")
	}
	if _, indoors := shop.Cleanliness(40, 5); !indoors {
		t.Fatal("a room 26 % unroofed feels outdoors to a bench")
	}
}
