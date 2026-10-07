package game

import (
	"rimworld-go/internal/domain/building/construction"
	"testing"

	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/exposure"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/world/temperature"
)

// hut walls in a room with interior x0+1..x1-1, y0+1..y1-1 and a door at
// the middle of the south wall.
func hut(t *testing.T, g *Game, x0, y0, x1, y1 int) {
	t.Helper()
	for x := x0; x <= x1; x++ {
		for _, y := range []int{y0, y1} {
			kind := "wall"
			if y == y1 && x == (x0+x1)/2 {
				kind = "door"
			}
			if !g.BuildNow(kind, "wood", "south", x, y) {
				t.Fatalf("build %s at %d,%d", kind, x, y)
			}
		}
	}
	for y := y0 + 1; y < y1; y++ {
		for _, x := range []int{x0, x1} {
			if !g.BuildNow("wall", "wood", "south", x, y) {
				t.Fatalf("build wall at %d,%d", x, y)
			}
		}
	}
}

func TestCampfireHeatsItsRoomInWinter(t *testing.T) {
	g := New(Config{Seed: 1, Width: 20, Height: 10})
	jumpTo(g, 45, 0)
	hut(t, g, 1, 1, 7, 7)   // heated
	hut(t, g, 10, 1, 16, 7) // unheated
	if !g.BuildNow("campfire", "wood", "south", 4, 3) {
		t.Fatal("build campfire")
	}
	g.FuelCampfire(4, 3)
	g.Step(clock.TicksPerDay / 2)
	warm, cold := int(g.temperatureAt(3, 5)), int(g.temperatureAt(13, 5))
	out := int(temperature.Outdoor(g.clock.Now()))
	if warm < exposure.ComfortMin+50 {
		t.Errorf("heated room %v, want comfortable", temperature.Temp(warm))
	}
	if cold > out+80 || cold < out-80 {
		t.Errorf("unheated room %v should track the outside %v", temperature.Temp(cold), temperature.Temp(out))
	}
	fuel := g.fuel[g.layerBldgAt(4, 3, construction.LayerObject)]
	if fuel >= CampfireCapacity*TicksPerWood || fuel <= 0 {
		t.Errorf("campfire should have burned some fuel, has %d", fuel)
	}
}

func TestHaulerRefuelsCampfire(t *testing.T) {
	g := New(Config{Seed: 1, Width: 14, Height: 8})
	id, _ := g.SpawnNamedPawn(2, 2, "Ada")
	g.SetNeeds(id, needs.Full, needs.Full)
	if !g.BuildNow("campfire", "wood", "south", 7, 3) {
		t.Fatal("build campfire")
	}
	if _, err := g.SpawnStack(item.Wood, 30, 2, 5); err != nil {
		t.Fatal(err)
	}
	g.Step(1500)
	bid := g.layerBldgAt(7, 3, construction.LayerObject)
	if g.fuel[bid] < (CampfireCapacity-1)*TicksPerWood {
		t.Fatalf("campfire fuel = %d after refuelling", g.fuel[bid])
	}
	if st, _ := stackOf(g, item.Wood); st.Qty != 10 {
		t.Fatalf("wood left = %d, want 10", st.Qty)
	}
}

func TestColdNightOutsideKillsWithoutShelter(t *testing.T) {
	g := New(Config{Seed: 1, Width: 14, Height: 8})
	jumpTo(g, 45, 0)
	id, _ := g.SpawnNamedPawn(2, 2, "Ada")
	downed := false
	for i := 0; i < 24; i++ {
		g.Step(clock.TicksPerHour / 2)
		g.SetNeeds(id, needs.Full, needs.Full)
		v, _ := g.vital.Get(id)
		if v.Status == health.Downed && v.Cause == health.CauseHypothermia {
			downed = true
		}
	}
	v, _ := g.vital.Get(id)
	if !downed || v.Status != health.Dead || v.Cause != health.CauseHypothermia {
		t.Fatalf("status %v cause %v downed %v: a winter night outdoors should be fatal", v.Status, v.Cause, downed)
	}
}

func TestFreezingColonistSeeksWarmRoom(t *testing.T) {
	g := New(Config{Seed: 1, Width: 16, Height: 12})
	jumpTo(g, 45, 0)
	hut(t, g, 6, 1, 12, 7)
	if !g.BuildNow("campfire", "wood", "south", 9, 3) {
		t.Fatal("build campfire")
	}
	g.FuelCampfire(9, 3)
	g.Step(clock.TicksPerHour * 2) // warm the hut up
	id, _ := g.SpawnNamedPawn(2, 10, "Ada")
	for i := 0; i < 16; i++ {
		g.Step(clock.TicksPerHour / 2)
		g.SetNeeds(id, needs.Full, needs.Full)
	}
	v, _ := g.vital.Get(id)
	pos, _ := g.positions.Get(id)
	if v.Status != health.Alive {
		t.Fatalf("colonist %v (%v) at %d,%d: should have walked into the warm hut", v.Status, v.Cause, pos.X, pos.Y)
	}
	if g.roomAt(pos.X, pos.Y) < 0 && g.exposure[id].hypo > exposure.Mild {
		t.Fatalf("still out in the cold at %d,%d with hypothermia %d", pos.X, pos.Y, g.exposure[id].hypo)
	}
}

func TestShortOfWoodCuttingComesFirst(t *testing.T) {
	g := New(Config{Seed: 1, Width: 14, Height: 8})
	if !g.woodLow() {
		t.Fatal("no wood at all should be low")
	}
	if _, err := g.SpawnStack(item.Wood, 50, 2, 2); err != nil {
		t.Fatal(err)
	}
	if g.woodLow() {
		t.Fatal("50 logs and no fires is plenty")
	}
	g.BuildNow("campfire", "wood", "south", 6, 3)
	g.BuildNow("campfire", "wood", "south", 8, 3)
	g.BuildNow("campfire", "wood", "south", 10, 3)
	if !g.woodLow() {
		t.Fatal("three fires need more than 50 logs on hand")
	}
}

func TestUnreachableTreeIsSkipped(t *testing.T) {
	g := New(Config{Seed: 1, Width: 14, Height: 8})
	id, _ := g.SpawnNamedPawn(1, 4, "Ada")
	g.SetNeeds(id, needs.Full, needs.Full)
	// a tree walled in on all sides, and a free one farther away
	if _, err := g.SpawnPlantSpecies("tree", 4, 4, 1000000); err != nil {
		t.Fatal(err)
	}
	for _, d := range [][2]int{{-1, -1}, {0, -1}, {1, -1}, {-1, 0}, {1, 0}, {-1, 1}, {0, 1}, {1, 1}} {
		if !g.BuildNow("wall", "wood", "south", 4+d[0], 4+d[1]) {
			t.Fatal("wall")
		}
	}
	if _, err := g.SpawnPlantSpecies("tree", 11, 4, 1000000); err != nil {
		t.Fatal(err)
	}
	g.markCut(0, 0, 13, 7, true)
	g.Step(1200)
	trees := 0
	for _, pid := range g.plantIDs {
		if p, _ := g.plants.Get(pid); p.X == 11 {
			trees++
		}
	}
	if trees != 0 {
		t.Fatal("the reachable tree should be cut instead of retrying the walled-in one")
	}
}

func TestBreachedRoomCoolsOverHours(t *testing.T) {
	g := New(Config{Seed: 1, Width: 20, Height: 10})
	jumpTo(g, 45, 0)
	hut(t, g, 1, 1, 7, 7)
	if !g.BuildNow("campfire", "wood", "south", 4, 3) {
		t.Fatal("build campfire")
	}
	g.FuelCampfire(4, 3)
	g.Step(clock.TicksPerDay / 4)
	warm := int(g.temperatureAt(3, 5))
	out := int(temperature.Outdoor(g.clock.Now()))
	if warm-out < 100 {
		t.Fatalf("heated hut %d vs outside %d", warm, out)
	}
	g.DamageAt(1, 4, 1000) // knock a wall down
	if g.roomAt(3, 5) >= 0 {
		t.Fatal("the hut is open now")
	}
	if now := int(g.temperatureAt(3, 5)); warm-now > 20 {
		t.Fatalf("a breached room doesn't freeze at once: %d -> %d", warm, now)
	}
	g.Step(4 * clock.TicksPerHour)
	out = int(temperature.Outdoor(g.clock.Now()))
	if now := int(g.temperatureAt(3, 5)); now-out > (warm-out)/3 {
		t.Fatalf("after a few hours it is close to the outside air: %d (outside %d)", now, out)
	}
}
