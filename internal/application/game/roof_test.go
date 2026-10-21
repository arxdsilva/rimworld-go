package game

import (
	"bytes"
	"reflect"
	"strings"
	"testing"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/pawn/mood"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/roof"
	"rimworld-go/internal/domain/world/room"
	"rimworld-go/internal/domain/world/temperature"
	"rimworld-go/internal/domain/world/terrain"
	"rimworld-go/internal/domain/world/weather"
)

// countRoof counts tiles with every flag of f set.
func countRoof(g *Game, f roof.Cell) int {
	n := 0
	for _, c := range g.roofs {
		if c.Has(f) {
			n++
		}
	}
	return n
}

// countRoofIn counts tiles with every flag of f set in the rectangle.
func countRoofIn(g *Game, f roof.Cell, x0, y0, x1, y1 int) int {
	n := 0
	for y := y0; y <= y1; y++ {
		for x := x0; x <= x1; x++ {
			if g.roofAt(x, y).Has(f) {
				n++
			}
		}
	}
	return n
}

// knockDown removes every object-layer building in the rectangle except
// where keep says so.
func knockDown(g *Game, x0, y0, x1, y1 int, keep func(x, y int) bool) {
	for y := y0; y <= y1; y++ {
		for x := x0; x <= x1; x++ {
			if keep != nil && keep(x, y) {
				continue
			}
			if bid := g.layerBldgAt(x, y, construction.LayerObject); bid != 0 {
				g.removeBuilding(bid)
			}
		}
	}
}

// worker spawns a well-rested, fed colonist.
func worker(t *testing.T, g *Game, x, y int) entity.EntityID {
	t.Helper()
	id, err := g.SpawnNamedPawn(x, y, "Ada")
	if err != nil {
		t.Fatal(err)
	}
	g.SetNeeds(id, needs.Full, needs.Full)
	return id
}

func TestAutoRoofPlansClosedRoom(t *testing.T) {
	g := New(Config{Seed: 1, Width: 20, Height: 12})
	hutWalls(t, g, 1, 1, 7, 7) // 5x5 interior
	g.ensureRooms()
	if n := countRoof(g, roof.PlanBuild); n != 25 {
		t.Fatalf("planned %d tiles, want 25", n)
	}
	if n := countRoofIn(g, roof.PlanBuild, 2, 2, 6, 6); n != 25 {
		t.Fatalf("planned %d interior tiles, want 25", n)
	}
	if g.roofAt(1, 1) != roof.Supported || g.roofAt(4, 7).Has(roof.PlanBuild) {
		t.Fatalf("walls and doors are never planned: %b %b", g.roofAt(1, 1), g.roofAt(4, 7))
	}
	if !g.roofAt(4, 4).Has(roof.Supported) || g.roofAt(15, 6).Has(roof.Supported) {
		t.Fatal("support: within 6 tiles of a wall only")
	}
	if g.roomRoofed(g.roomAt(4, 4)) != 0 || !g.underSky(4, 4) {
		t.Fatal("a planned roof is no roof yet")
	}
	// a rebuild does not plan twice
	g.markRoomsDirty()
	g.ensureRooms()
	if n := countRoof(g, roof.PlanBuild); n != 25 || len(g.roofx.plans) != 25 {
		t.Fatalf("after rebuild: %d plans, index %d", n, len(g.roofx.plans))
	}
}

func TestAutoRoofSkipsLargeRoom(t *testing.T) {
	g := New(Config{Seed: 1, Width: 20, Height: 20})
	hutWalls(t, g, 1, 1, 15, 15) // 13x13 = 169 cells > AutoMaxCells
	g.ensureRooms()
	if r := g.roomAt(8, 8); r < 0 || g.rooms[r].contents.Cells != 169 {
		t.Fatalf("room %d", r)
	}
	if n := countRoof(g, roof.PlanBuild); n != 0 {
		t.Fatalf("planned %d tiles over a 169-cell room, want 0", n)
	}
	// the player can still plan it by hand, supported tiles only: the
	// centre (8, 8) is 7 tiles from every wall
	g.designateRoof(2, 2, 14, 14, true)
	if n := countRoof(g, roof.PlanBuild); n != 168 || g.roofAt(8, 8).Has(roof.PlanBuild) {
		t.Fatalf("designated %d, want 168 (all but the centre)", n)
	}
}

func TestAutoRoofRespectsNoAuto(t *testing.T) {
	g := New(Config{Seed: 1, Width: 20, Height: 12})
	hutWalls(t, g, 1, 1, 7, 7)
	g.ensureRooms()
	g.designateRoof(3, 3, 3, 3, false)
	if c := g.roofAt(3, 3); !c.Has(roof.NoAuto) || c.Has(roof.PlanBuild) {
		t.Fatalf("remove designation: %b", c)
	}
	g.markRoomsDirty()
	g.ensureRooms()
	if g.roofAt(3, 3).Has(roof.PlanBuild) || countRoof(g, roof.PlanBuild) != 24 {
		t.Fatal("auto-roofing must leave a NoAuto tile alone")
	}
	g.designateRoof(3, 3, 3, 3, true)
	if c := g.roofAt(3, 3); c.Has(roof.NoAuto) || !c.Has(roof.PlanBuild) {
		t.Fatalf("designating a roof clears NoAuto: %b", c)
	}
}

func TestBuildersRoofTileByTile(t *testing.T) {
	g := New(Config{Seed: 1, Width: 14, Height: 10})
	hutWalls(t, g, 1, 1, 5, 5) // 3x3 interior
	id := worker(t, g, 3, 3)
	g.ensureRooms()
	if countRoof(g, roof.PlanBuild) != 9 {
		t.Fatal("want 9 plans")
	}
	if !g.assignRoof(id) {
		t.Fatal("assignRoof")
	}
	if j, _ := g.jobs.Get(id); j.Kind != job.Roof || j.WorkLeft != roof.BuildWork || j.DestX != 3 || j.DestY != 3 {
		t.Fatalf("first job %+v: the tile underfoot, work %d", j, roof.BuildWork)
	}
	g.interrupt(id, job.Interrupted)
	built, jobs := 0, 0
	var last [2]int
	for tick := 0; tick < 3000 && built < 9; tick++ {
		g.Step(1)
		g.SetNeeds(id, needs.Full, needs.Full)
		j, _ := g.jobs.Get(id)
		if j.Kind == job.Roof {
			if [2]int{j.DestX, j.DestY} != last {
				last = [2]int{j.DestX, j.DestY}
				jobs++
				if j.WorkLeft < roof.BuildWork-1 { // may have worked a tick already
					t.Fatalf("new roof job with work %d, want %d", j.WorkLeft, roof.BuildWork)
				}
			}
			if o, ok := g.reserve.TileOwner(j.DestX, j.DestY); j.Active() && (!ok || o != id) {
				t.Fatalf("roof tile %v not reserved by the builder", last)
			}
			if g.workTotal(id, j) != roof.BuildWork {
				t.Fatal("workTotal")
			}
			if j.State == job.Do && g.actionOf(id, j) != ActionBuild {
				t.Fatalf("action %q, want %q", g.actionOf(id, j), ActionBuild)
			}
		}
		n := countRoof(g, roof.Built)
		if n < built || n > built+1 {
			t.Fatalf("built went %d -> %d in one tick", built, n)
		}
		built = n
	}
	if built != 9 || countRoof(g, roof.PlanBuild) != 0 || len(g.roofx.plans) != 0 {
		t.Fatalf("built %d, plans left %d", built, countRoof(g, roof.PlanBuild))
	}
	if jobs != 9 {
		t.Fatalf("%d roof jobs, want one per tile", jobs)
	}
	if r := g.roomAt(3, 3); g.roomRoofed(r) != 1000 || g.underSky(3, 3) {
		t.Fatal("finished room should be fully roofed")
	}
}

func TestRoofWorkSpotBesideBlockedTile(t *testing.T) {
	g := New(Config{Seed: 1, Width: 14, Height: 10})
	hutWalls(t, g, 1, 1, 5, 5)
	g.ensureRooms()
	g.designateRoof(2, 2, 4, 4, false) // nothing auto
	if !g.BuildNow("wall", "wood", "south", 10, 4) {
		t.Fatal("wall")
	}
	g.designateRoof(10, 3, 10, 3, true) // beside a free-standing wall
	g.grid.SetWalkable(10, 3, false)    // as under blocking furniture
	id := worker(t, g, 7, 3)
	if !g.assignRoof(id) {
		t.Fatal("assignRoof")
	}
	j, _ := g.jobs.Get(id)
	if j.DestX != 10 || j.DestY != 3 || (j.TX == 10 && j.TY == 3) || max(abs(j.TX-10), abs(j.TY-3)) != 1 {
		t.Fatalf("work spot %d,%d for tile 10,3", j.TX, j.TY)
	}
}

func TestDesignateRoofOnlySupported(t *testing.T) {
	g := New(Config{Seed: 1, Width: 30, Height: 12})
	if !g.BuildNow("wall", "wood", "south", 2, 5) {
		t.Fatal("wall")
	}
	g.designateRoof(10, 5, 12, 5, false) // NoAuto on unsupported tiles
	g.designateRoof(4, 5, 4, 5, true)
	g.RoofRooms()
	if !g.roofAt(4, 5).Has(roof.Built) {
		t.Fatal("RoofRooms completes a designated roof")
	}
	g.designateRoof(12, 5, -3, 5, true) // reversed and partly off the map
	for x := 0; x <= 12; x++ {
		c := g.roofAt(x, 5)
		want := x <= 8 && x != 2 && x != 4 // within 6, not the wall, not roofed
		if c.Has(roof.PlanBuild) != want {
			t.Errorf("x=%d: planned=%v, want %v (%b)", x, c.Has(roof.PlanBuild), want, c)
		}
		if c.Has(roof.NoAuto) {
			t.Errorf("x=%d: designating clears NoAuto", x)
		}
	}
	if g.roofAt(2, 5).Has(roof.PlanBuild) {
		t.Fatal("no roof plan on a wall")
	}
	g.designateRoof(-5, -5, -1, -1, true) // entirely off the map: no-op
}

func TestRemoveRoofSetsNoAutoAndUnroofs(t *testing.T) {
	g := New(Config{Seed: 1, Width: 14, Height: 10})
	hut(t, g, 1, 1, 5, 5) // roofed 3x3
	g.roofOnMined(4, 4)   // a natural roof inside: never removed
	g.designateRoof(2, 2, 2, 2, true)
	g.designateRoof(0, 0, 6, 6, false)
	if n := countRoofIn(g, roof.NoAuto, 0, 0, 6, 6); n != 49 {
		t.Fatalf("NoAuto on %d tiles, want 49", n)
	}
	if n := countRoof(g, roof.PlanRemove); n != 8 {
		t.Fatalf("%d removal plans, want 8 built roofs", n)
	}
	if c := g.roofAt(4, 4); c.Has(roof.PlanRemove) || !c.Has(roof.Natural) {
		t.Fatalf("natural roof untouched: %b", c)
	}
	id := worker(t, g, 3, 3)
	jobs := 0
	var last [2]int
	for tick := 0; tick < 3000 && countRoof(g, roof.Built) > 0; tick++ {
		g.Step(1)
		g.SetNeeds(id, needs.Full, needs.Full)
		if j, _ := g.jobs.Get(id); j.Kind == job.Unroof && [2]int{j.DestX, j.DestY} != last {
			last = [2]int{j.DestX, j.DestY}
			jobs++
			if j.WorkLeft < roof.RemoveWork-1 || g.workTotal(id, j) != roof.RemoveWork {
				t.Fatalf("unroof work %d", j.WorkLeft)
			}
		}
	}
	if countRoof(g, roof.Built) != 0 || countRoof(g, roof.PlanRemove) != 0 || jobs != 8 {
		t.Fatalf("built %d, removal plans %d, jobs %d", countRoof(g, roof.Built), countRoof(g, roof.PlanRemove), jobs)
	}
	if !g.roofAt(4, 4).Has(roof.Natural) {
		t.Fatal("natural roof must stay")
	}
	g.markRoomsDirty()
	g.ensureRooms()
	if n := countRoof(g, roof.PlanBuild); n != 0 {
		t.Fatalf("auto-roof re-planned %d removed tiles", n)
	}
	if r := g.roomAt(3, 3); g.roomRoofed(r) != 1000/9 {
		t.Fatalf("roofed %d, want one natural tile of 9", g.roomRoofed(r))
	}
}

func TestCancelClearsRoofPlans(t *testing.T) {
	g := New(Config{Seed: 1, Width: 20, Height: 10})
	hutWalls(t, g, 1, 1, 7, 7)
	g.ensureRooms()
	g.designateRoof(2, 2, 2, 2, false) // NoAuto
	g.designateRoof(6, 6, 6, 6, true)
	g.RoofRooms()
	g.designateRoof(6, 6, 6, 6, false) // built roof planned for removal
	g.designateRoof(3, 3, 6, 6, false)
	g.designateRoof(3, 3, 5, 5, true) // plans again, NoAuto cleared
	g.Enqueue(command.Command{Kind: command.DesignateCancel, X: 7, Y: 7, X2: 2, Y2: 2})
	g.Step(1)
	if n := countRoof(g, roof.PlanBuild) + countRoof(g, roof.PlanRemove); n != 0 || len(g.roofx.plans) != 0 {
		t.Fatalf("%d plans left after cancel", n)
	}
	if !g.roofAt(2, 2).Has(roof.NoAuto) || !g.roofAt(6, 6).Has(roof.NoAuto) {
		t.Fatal("cancel keeps NoAuto")
	}
	if !g.roofAt(6, 6).Has(roof.Built) {
		t.Fatal("cancel keeps built roofs")
	}
}

func TestRoofCollapsesWhenWallRemoved(t *testing.T) {
	g := New(Config{Seed: 1, Width: 24, Height: 10})
	hut(t, g, 1, 1, 12, 5) // interior x 2..11, y 2..4: 30 roofed tiles
	g.roofOnMined(11, 4)   // natural: never collapses
	if countRoof(g, roof.Built) != 29 {
		t.Fatalf("built %d", countRoof(g, roof.Built))
	}
	// only the west wall (x=1) is left: x <= 7 stays within reach
	knockDown(g, 1, 1, 12, 5, func(x, _ int) bool { return x == 1 })
	g.ensureRooms() // support recomputed; collapse waits for the climate tick
	if countRoof(g, roof.Built) != 29 {
		t.Fatal("roofs fall at the climate interval, not on rebuild")
	}
	before := len(g.messages)
	g.Step(climateInterval)
	if n := countRoof(g, roof.Built); n != 18 {
		t.Fatalf("built %d after collapse, want 18 (x 2..7)", n)
	}
	if countRoofIn(g, roof.Built, 8, 0, 12, 9) != 0 || !g.roofAt(11, 4).Has(roof.Natural) {
		t.Fatal("unsupported tiles fall, the natural roof stays")
	}
	var notices []string
	for _, m := range g.messages[before:] {
		if strings.Contains(m.Text, "roof collapsed") {
			notices = append(notices, m.Text)
		}
	}
	want := "A roof collapsed near (8, 2) — no support within 6 tiles."
	if len(notices) != 1 || notices[0] != want {
		t.Fatalf("notices %q, want one %q", notices, want)
	}
	w := g.World()
	if len(w.RoofCollapses) != 1 || w.RoofCollapses[0].X != 8 || w.RoofCollapses[0].Y != 2 || w.RoofCollapses[0].Tick == 0 {
		t.Fatalf("collapse stamps %+v", w.RoofCollapses)
	}
	g.Step(climateInterval)
	n := 0
	for _, m := range g.messages[before:] {
		if strings.Contains(m.Text, "roof collapsed") {
			n++
		}
	}
	if n != 1 {
		t.Fatal("one notice per collapse batch")
	}
	g.Step(clock.TicksPerHour + climateInterval)
	if len(g.World().RoofCollapses) != 0 {
		t.Fatal("collapse stamps last an hour")
	}
}

func TestMinedRockLeavesNaturalRoof(t *testing.T) {
	g := New(Config{Seed: 1, Width: 14, Height: 10})
	for _, p := range [][2]int{{6, 4}, {6, 6}} {
		g.setTerrain(p[0], p[1], terrain.Rock)
	}
	id := worker(t, g, 3, 4)
	g.markMine(6, 4, 6, 4, true)
	for i := 0; i < 2000 && g.terr.At(6, 4) == terrain.Rock; i++ {
		g.Step(1)
		g.SetNeeds(id, needs.Full, needs.Full)
	}
	if g.terr.At(6, 4) == terrain.Rock {
		t.Fatal("rock not mined")
	}
	if c := g.roofAt(6, 4); !c.Has(roof.Natural) || g.underSky(6, 4) {
		t.Fatalf("mined tile: %b", c)
	}
	// a sapper digging through rock leaves the same overhead rock
	g.digRock(6, 6, rockHP)
	if !g.roofAt(6, 6).Has(roof.Natural) {
		t.Fatal("sapper-dug tile should keep a natural roof")
	}
	// natural roofs never collapse and can't be planned away
	g.Step(climateInterval * 2)
	g.designateRoof(6, 4, 6, 6, false)
	if c := g.roofAt(6, 4); !c.Has(roof.Natural) || c.Has(roof.PlanRemove) {
		t.Fatalf("natural roof after remove designation: %b", c)
	}
	if !g.roofAt(6, 6).Has(roof.Natural) {
		t.Fatal("natural roof gone")
	}
}

// roomTempAt is the air of the room at (x, y).
func roomTempAt(g *Game, x, y int) temperature.Temp {
	return g.roomTemps[g.roomAt(x, y)]
}

func TestUnroofedRoomTracksOutdoors(t *testing.T) {
	g := New(Config{Seed: 1, Width: 20, Height: 10})
	g.HoldWeather()
	jumpTo(g, 45, 10)       // a winter morning
	hut(t, g, 10, 1, 14, 5) // roofed control (RoofRooms roofs every plan)
	hutWalls(t, g, 1, 1, 5, 5)
	g.ensureRooms()
	g.Step(2000)
	r := g.roomAt(3, 3)
	if g.roomRoofed(r) != 0 || g.World().Rooms[r].Roofed != 0 {
		t.Fatal("room should be unroofed")
	}
	out := temperature.Outdoor(g.clock.Now())
	if d := roomTempAt(g, 3, 3) - out; d > 10 || d < -10 {
		t.Fatalf("unroofed room %d, outside %d (tenths): want within 1 C", roomTempAt(g, 3, 3), out)
	}
	if d := roomTempAt(g, 12, 3) - out; d < 20 {
		t.Fatalf("roofed control only %d above outside", d)
	}
}

func TestRoofedRoomUnchanged(t *testing.T) {
	g := New(Config{Seed: 1, Width: 20, Height: 10})
	jumpTo(g, 45, 0)
	hut(t, g, 1, 1, 7, 7)
	g.ensureRooms()
	r := g.roomAt(4, 4)
	if g.roomRoofed(r) != 1000 || g.World().Rooms[r].Roofed != 1 {
		t.Fatal("room should be fully roofed")
	}
	g.roomTemps[r] = 150
	for i := 0; i < 12; i++ {
		jumpTo(g, 45, i*2)
		now := g.clock.Now()
		outside := temperature.Outdoor(now) + temperature.IndoorBonus
		cells := g.rooms[r].contents.Cells
		t0 := g.roomTemps[r]
		want := t0 + (outside-t0)*temperature.Temp(roomLeak(cells))/1000 // the pre-roof formula
		g.heatRooms()
		if got := g.roomTemps[r]; got != want {
			t.Fatalf("step %d: %v, want %v (old formula)", i, got, want)
		}
	}
}

func TestHalfRoofedBetween(t *testing.T) {
	g := New(Config{Seed: 1, Width: 30, Height: 10})
	g.HoldWeather()
	jumpTo(g, 45, 0)
	hutWalls(t, g, 1, 1, 7, 7)   // roofed
	hutWalls(t, g, 10, 1, 16, 7) // bare
	hutWalls(t, g, 19, 1, 25, 7) // half: rows 2..3 of 2..6 bare
	g.ensureRooms()
	g.designateRoof(11, 2, 15, 6, false)
	g.designateRoof(20, 2, 24, 3, false)
	g.RoofRooms()
	full, bare, half := g.roomAt(4, 4), g.roomAt(13, 4), g.roomAt(22, 4)
	if g.roomRoofed(full) != 1000 || g.roomRoofed(bare) != 0 || g.roomRoofed(half) != 600 {
		t.Fatalf("roofed %d %d %d", g.roomRoofed(full), g.roomRoofed(bare), g.roomRoofed(half))
	}
	g.Step(clock.TicksPerHour * 2)
	tf, tb, th := g.roomTemps[full], g.roomTemps[bare], g.roomTemps[half]
	if !(tb < th && th < tf) {
		t.Fatalf("bare %v < half %v < full %v expected", tb, th, tf)
	}
}

func TestCampfireInUnroofedRoomBarelyHelps(t *testing.T) {
	g := New(Config{Seed: 1, Width: 20, Height: 10})
	g.HoldWeather()
	g.SetWeather(weather.Clear, 0)
	jumpTo(g, 45, 0)
	hut(t, g, 1, 1, 7, 7)
	hutWalls(t, g, 10, 1, 16, 7)
	g.ensureRooms()
	g.designateRoof(11, 2, 15, 6, false)
	for _, x := range []int{4, 13} {
		if !g.BuildNow("campfire", "wood", "south", x, 3) {
			t.Fatal("build campfire")
		}
		g.FuelCampfire(x, 3)
	}
	g.Step(clock.TicksPerDay / 2)
	out := temperature.Outdoor(g.clock.Now())
	roofed, bare := g.temperatureAt(3, 5)-out, g.temperatureAt(12, 5)-out
	if roofed < 150 {
		t.Fatalf("roofed fire room only %v above outside", roofed)
	}
	if bare*3 > roofed || bare > 100 {
		t.Fatalf("unroofed fire room %v above outside (roofed %v): should barely help", bare, roofed)
	}
}

// pavilion is a free-standing wall at (10, 5) with a roof over x 11..13,
// y 4..6: roofed tiles that belong to no room.
func pavilion(t *testing.T, g *Game) {
	t.Helper()
	if !g.BuildNow("wall", "wood", "south", 10, 5) {
		t.Fatal("wall")
	}
	g.designateRoof(11, 4, 13, 6, true)
	g.RoofRooms()
	if g.roomAt(12, 5) != room.Outdoors || g.underSky(12, 5) {
		t.Fatal("pavilion tile should be outdoors but covered")
	}
}

func TestRoofBlocksRainEffects(t *testing.T) {
	g := weatherGame(t)
	jumpTo(g, 45, 1) // cold enough that fires burn
	pavilion(t, g)
	hutWalls(t, g, 15, 2, 21, 8) // a closed room left open to the sky
	g.ensureRooms()
	g.designateRoof(16, 3, 20, 7, false)
	under := worker(t, g, 12, 5)
	bare := worker(t, g, 18, 5)
	open := worker(t, g, 3, 3)
	g.BuildNow("campfire", "wood", "south", 12, 4)
	g.FuelCampfire(12, 4)
	g.BuildNow("campfire", "wood", "south", 18, 4)
	g.FuelCampfire(18, 4)
	coveredFire := g.layerBldgAt(12, 4, construction.LayerObject)
	bareFire := g.layerBldgAt(18, 4, construction.LayerObject)
	g.SetWeather(weather.Rain, 0.6)
	g.weatherThoughts()
	if remembers(g, under, mood.Soaked) || !remembers(g, bare, mood.Soaked) || !remembers(g, open, mood.Soaked) {
		t.Fatalf("soaked: under roof %v, unroofed room %v, open %v",
			remembers(g, under, mood.Soaked), remembers(g, bare, mood.Soaked), remembers(g, open, mood.Soaked))
	}
	soaked := map[entity.EntityID]bool{}
	for _, pv := range g.Snapshot() {
		soaked[pv.ID] = pv.Soaked
	}
	if soaked[under] || !soaked[bare] || !soaked[open] {
		t.Fatalf("PawnView.Soaked %v", soaked)
	}
	if g.weatherSlow(12, 5, 10) != 10 || g.weatherSlow(18, 5, 10) != 12 || g.weatherSlow(3, 3, 10) != 12 {
		t.Fatal("rain slows steps under the sky only")
	}
	if g.doused(coveredFire) || !g.doused(bareFire) {
		t.Fatal("a roof keeps the rain off a fire; an unroofed room does not")
	}
}

func TestRoofShieldsSnowChill(t *testing.T) {
	g := weatherGame(t)
	pavilion(t, g)
	g.SetWeather(weather.Clear, 0)
	base := g.temperatureAt(12, 5)
	g.SetWeather(weather.Snow, 0.5)
	if g.temperatureAt(12, 5) != base || g.weatherTempDelta(12, 5) != 0 {
		t.Fatal("snow must not chill a roofed tile")
	}
	if d := g.weatherTempDelta(3, 3); d != -20 {
		t.Fatalf("open ground snow delta %d, want -20", d)
	}
	g.SetWeather(weather.Blizzard, 1)
	if g.weatherTempDelta(12, 5) != 0 || g.weatherTempDelta(10, 5) != 0 || g.weatherTempDelta(3, 3) != -50 {
		t.Fatal("blizzard: roofs and walls are covered, open ground is not")
	}
	// a light snow day is an outdoor thought only
	under, open := worker(t, g, 12, 5), worker(t, g, 3, 3)
	g.SetWeather(weather.Snow, 0.4)
	g.weatherThoughts()
	if remembers(g, under, mood.SnowDay) || !remembers(g, open, mood.SnowDay) {
		t.Fatal("snow day under the sky only")
	}
}

func TestDoorUnderRoofNotSoaked(t *testing.T) {
	g := weatherGame(t)
	hut(t, g, 10, 2, 16, 8) // door at 13,8
	id := worker(t, g, 13, 8)
	if g.roomAt(13, 8) != room.Wall || g.underSky(13, 8) {
		t.Fatal("a door tile is covered")
	}
	g.SetWeather(weather.Rain, 0.8)
	g.weatherThoughts()
	if remembers(g, id, mood.Soaked) || g.Snapshot()[0].Soaked {
		t.Fatal("a colonist in a doorway stays dry")
	}
}

func TestSleptUnderRoofNotOutside(t *testing.T) {
	g := New(Config{Seed: 1, Width: 24, Height: 12})
	pavilion(t, g)
	hutWalls(t, g, 15, 2, 21, 8)
	g.ensureRooms()
	under, open, bare := worker(t, g, 12, 5), worker(t, g, 3, 3), worker(t, g, 18, 5)
	for _, id := range []entity.EntityID{under, open, bare} {
		g.afterSleep(id)
	}
	if remembers(g, under, mood.SleptOutside) || !remembers(g, under, mood.SleptOnGround) {
		t.Fatal("sleeping under a roof is on the ground, not outside")
	}
	if !remembers(g, open, mood.SleptOutside) {
		t.Fatal("sleeping under the sky is outside")
	}
	if remembers(g, bare, mood.SleptOutside) || !remembers(g, bare, mood.SleptOnGround) {
		t.Fatal("a room, roofed or not, is not outside")
	}
	// in a bed under the sky (no room) it is still outside
	g.BuildNow("bed", "wood", "south", 5, 8)
	g.sleptAt[open] = g.layerBldgAt(5, 8, construction.LayerObject)
	g.positions.Set(open, Position{X: 5, Y: 8})
	g.memories[open] = nil
	g.afterSleep(open)
	if g.sleptAt[open] != 0 && !remembers(g, open, mood.SleptOutside) {
		t.Fatal("a bed in the open is outside")
	}
}

func TestRoofViewBitsHourDay(t *testing.T) {
	g := New(Config{Seed: 1, Width: 20, Height: 10})
	hut(t, g, 1, 1, 7, 7)
	g.designateRoof(4, 4, 4, 4, false)
	jumpTo(g, 3, 6)
	g.clock.Advance(clock.TicksPerHour / 2)
	w := g.World()
	if len(w.RoofCells) != 20*10 || w.TerrainW != 20 {
		t.Fatalf("RoofCells len %d", len(w.RoofCells))
	}
	if c := roof.Cell(w.RoofCells[3*20+3]); c != roof.Built|roof.Supported {
		t.Fatalf("roofed interior bits %b", c)
	}
	if c := roof.Cell(w.RoofCells[4*20+4]); c != roof.Built|roof.Supported|roof.PlanRemove|roof.NoAuto {
		t.Fatalf("planned removal bits %b", c)
	}
	if roof.Cell(w.RoofCells[5*20+19]) != 0 {
		t.Fatal("open, unsupported ground has no bits")
	}
	if w.Hour != 6.5 || w.DayOfYear != 3 {
		t.Fatalf("Hour %v DayOfYear %d", w.Hour, w.DayOfYear)
	}
	r := g.roomAt(3, 3)
	if w.Rooms[r].Roofed != 1 {
		t.Fatalf("RoomView.Roofed %v", w.Rooms[r].Roofed)
	}
	w.RoofCells[0] = 0xff
	if g.roofs[0] == 0xff {
		t.Fatal("the view must be a copy")
	}
	if empty := New(Config{Seed: 1, Width: 4, Height: 4}); empty.exportRoofs() != nil {
		t.Fatal("no roof grid: nothing to export")
	}
}

func TestRoofsSaveRoundTrip(t *testing.T) {
	g := New(Config{Seed: 1, Width: 20, Height: 10})
	hut(t, g, 1, 1, 7, 7)
	g.designateRoof(3, 3, 3, 3, false)
	g.roofOnMined(5, 5)
	var buf bytes.Buffer
	if err := g.Save(&buf, "roofs"); err != nil {
		t.Fatal(err)
	}
	h, _, err := Load(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	gw, hw := g.World(), h.World()
	if !reflect.DeepEqual(gw.RoofCells, hw.RoofCells) {
		t.Fatal("roof cells differ after load")
	}
	if !reflect.DeepEqual(g.roofx.plans, h.roofx.plans) || len(h.roofx.plans) != 1 {
		t.Fatalf("plan index %v vs %v", g.roofx.plans, h.roofx.plans)
	}
	r := h.roomAt(4, 4)
	if h.roomRoofed(r) != g.roomRoofed(g.roomAt(4, 4)) || h.roomRoofed(r) != 1000 {
		t.Fatal("roofed share differs after load")
	}
}

func TestOldSaveAutoRoofsOnLoad(t *testing.T) {
	g := New(Config{Seed: 1, Width: 20, Height: 10})
	hutWalls(t, g, 1, 1, 7, 7)
	g.ensureRooms()
	g.roofs, g.roofx = nil, roofIndex{} // as saved before roofs existed
	var buf bytes.Buffer
	if err := g.Save(&buf, "old"); err != nil {
		t.Fatal(err)
	}
	h, _, err := Load(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	w := h.World()
	n := 0
	for _, c := range w.RoofCells {
		if roof.Cell(c).Has(roof.PlanBuild) {
			n++
		}
	}
	if n != 25 {
		t.Fatalf("old save: %d auto-roof plans after load, want 25", n)
	}
}

func TestImportRoofsRejectsWrongSize(t *testing.T) {
	g := New(Config{Seed: 1, Width: 6, Height: 6})
	g.importRoofs([]uint8{1, 2, 3})
	if len(g.roofs) != 0 {
		t.Fatal("a mis-sized roof grid is dropped")
	}
	cells := make([]uint8, 36)
	cells[7] = uint8(roof.Built | roof.PlanRemove)
	g.importRoofs(cells)
	if !g.roofAt(1, 1).Has(roof.Built) || len(g.roofx.plans) != 1 || g.roofAt(-1, 0) != 0 {
		t.Fatal("import")
	}
}

func TestRoofDeterministic(t *testing.T) {
	run := func(saveAt int) ([]uint8, []PawnView) {
		g := New(Config{Seed: 7, Width: 24, Height: 12})
		hutWalls(t, g, 1, 1, 7, 7)
		hutWalls(t, g, 10, 1, 16, 7)
		g.ensureRooms()
		g.designateRoof(11, 2, 13, 6, false)
		g.designateRoof(20, 2, 22, 2, true) // unsupported: never planned
		worker(t, g, 4, 4)
		worker(t, g, 13, 4)
		for i := 0; i < 900; i++ {
			if i == saveAt {
				var buf bytes.Buffer
				if err := g.Save(&buf, "mid"); err != nil {
					t.Fatal(err)
				}
				h, _, err := Load(bytes.NewReader(buf.Bytes()))
				if err != nil {
					t.Fatal(err)
				}
				g = h
			}
			g.Step(1)
		}
		return g.World().RoofCells, g.Snapshot()
	}
	a, pa := run(-1)
	b, pb := run(-1)
	if !reflect.DeepEqual(a, b) || !reflect.DeepEqual(pa, pb) {
		t.Fatal("same seed, same roofs and pawns")
	}
	if n := countCells(a, roof.Built); n == 0 {
		t.Fatal("builders should have roofed something in 900 ticks")
	}
	c, _ := run(400)
	if !reflect.DeepEqual(a, c) {
		t.Fatal("save/load mid-run changed the roofs")
	}
}

func countCells(cells []uint8, f roof.Cell) int {
	n := 0
	for _, c := range cells {
		if roof.Cell(c).Has(f) {
			n++
		}
	}
	return n
}

func TestRoofJobFailsWhenPlanCancelled(t *testing.T) {
	g := New(Config{Seed: 1, Width: 14, Height: 10})
	hutWalls(t, g, 1, 1, 5, 5)
	g.ensureRooms()
	a := worker(t, g, 3, 3)
	b := worker(t, g, 3, 3)
	if !g.assignRoof(a) || !g.assignRoof(b) {
		t.Fatal("assignRoof")
	}
	ja, _ := g.jobs.Get(a)
	jb, _ := g.jobs.Get(b)
	if ja.DestX == jb.DestX && ja.DestY == jb.DestY {
		t.Fatal("two builders on one reserved tile")
	}
	if jb.DestX != 2 || jb.DestY != 2 {
		t.Fatalf("second builder took %d,%d; nearest ties go by y, then x", jb.DestX, jb.DestY)
	}
	g.Enqueue(command.Command{Kind: command.DesignateCancel, X: 3, Y: 3, X2: 3, Y2: 3})
	g.Step(1)
	if j, _ := g.jobs.Get(a); j.Kind == job.Roof && j.DestX == 3 && j.DestY == 3 && j.Active() {
		t.Fatalf("job on a cancelled plan still active: %+v", j)
	}
	if _, held := g.reserve.TileOwner(3, 3); held {
		t.Fatal("cancelled roof tile still reserved")
	}
	if g.roofAt(3, 3).Has(roof.Built) {
		t.Fatal("cancelled tile must not be roofed")
	}
	// an unknown or out-of-map tile fails safely too
	g.doRoof(a, job.Record{Kind: job.Roof, DestX: -1, DestY: 0, State: job.Do})
	if j, _ := g.jobs.Get(a); j.State != job.Fail {
		t.Fatal("off-map roof job should fail")
	}
}

// TestDousedFireDoesNotLightRoom: a campfire put out by snow in an
// unroofed room leaves the room dark; under a roof it burns and lights it.
func TestDousedFireDoesNotLightRoom(t *testing.T) {
	g := weatherGame(t)
	g.SkipTo(44, 6) // a winter morning: the fire is not banked for warmth
	hutWalls(t, g, 10, 2, 16, 8)
	if !g.BuildNow("campfire", "wood", "south", 13, 5) {
		t.Fatal("build campfire")
	}
	g.FuelCampfire(13, 5)
	g.SetWeather(weather.Snow, 0.8)
	lit := func() bool { return g.roomViews()[g.roomAt(13, 5)].Lit }
	if lit() {
		t.Fatal("a doused fire must not light its unroofed room")
	}
	g.RoofRooms()
	if !lit() {
		t.Fatal("under a roof the fire burns and lights the room")
	}
}
