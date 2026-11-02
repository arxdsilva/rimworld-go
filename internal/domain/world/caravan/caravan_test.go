package caravan

import (
	"testing"

	"rimworld-go/internal/domain/animal"
)

// flat is every tile at difficulty 1.
func flat(int) int { return 1000 }

// TestSpeed: slowest walker × load (×2 empty to ×0.5 full, wiki) × mount.
func TestSpeed(t *testing.T) {
	t.Parallel()
	if LoadFactor(0) != 2000 || LoadFactor(1000) != 500 || LoadFactor(500) != 1250 || LoadFactor(-5) != 2000 || LoadFactor(4000) != 500 {
		t.Fatal("load factor")
	}
	c := &Caravan{}
	if c.Speed() != 2000 {
		t.Fatalf("an empty caravan at full Moving: %d", c.Speed())
	}
	c.Slowest, c.Load, c.Mount = 500, 1000, 2000
	if c.Speed() != 500 {
		t.Fatalf("half Moving, full, on horses: %d", c.Speed())
	}
	if MountFactor(animal.Horse) != 2000 || MountFactor(animal.Donkey) != 1300 || MountFactor(animal.Cow) != 0 {
		t.Fatal("mount factors outside the wiki's 1.3-2.0")
	}
}

// TestTileTicksWikiFormula: 0.06 days per difficulty point at speed 1.0
// (wiki: Caravan).
func TestTileTicksWikiFormula(t *testing.T) {
	t.Parallel()
	day := 60000
	if TileTicks(1000, 1000) != day*6/100 || TileTicks(4000, 1000) != day*24/100 || TileTicks(1000, 2000) != day*3/100 || TileTicks(0, 1000) != day*6/100 {
		t.Fatalf("tile ticks %d %d %d", TileTicks(1000, 1000), TileTicks(4000, 1000), TileTicks(1000, 2000))
	}
	if Moving(5) || !Moving(6) || !Moving(21) || Moving(22) {
		t.Fatal("travel hours are 06:00-22:00")
	}
}

// TestAdvanceAlongPath: an hour at a time, only by day, food every hour.
func TestAdvanceAlongPath(t *testing.T) {
	t.Parallel()
	// speed 1000 on difficulty 2.5: 9000 ticks a tile, 277 permille an hour
	c := &Caravan{Tile: 1, Path: []int{2, 3}, Dest: 3, State: Travelling, FoodDays: 5000, Load: 667}
	dm := func(int) int { return 2500 }
	if Advance(c, 3, dm) || c.Progress != 0 || c.FoodDays != 5000-42 {
		t.Fatalf("night: %+v", c)
	}
	for h := 6; h < 9; h++ {
		if Advance(c, h, dm) {
			t.Fatalf("a tile in under 3.6 hours: %+v", c)
		}
	}
	if !Advance(c, 9, dm) || c.Tile != 2 || len(c.Path) != 1 {
		t.Fatalf("one tile in 4 hours: %+v", c)
	}
	for h := 10; c.State != Arrived; h++ {
		Advance(c, h%24, dm)
	}
	if c.Tile != 3 || !IsArrived(c) || c.Progress != 0 || Advance(c, 12, dm) {
		t.Fatalf("end %+v", c)
	}
}

// TestAdvanceEdges: an empty path arrives; visiting does not move.
func TestAdvanceEdges(t *testing.T) {
	t.Parallel()
	c := &Caravan{State: Travelling}
	if Advance(c, 12, flat) || c.State != Arrived {
		t.Fatal("empty path arrives")
	}
	v := &Caravan{State: Visiting, Path: []int{1}}
	if Advance(v, 12, flat) || len(v.Path) != 1 {
		t.Fatal("visiting does not move")
	}
	r := &Caravan{State: Returning, Path: []int{9}, FoodDays: 10}
	if !Advance(r, 12, flat) || r.Tile != 9 || r.FoodDays != 0 {
		t.Fatalf("%+v", r)
	}
}

// TestEstimateRestsAtNight: the estimate is Advance's hours, nights
// included.
func TestEstimateRestsAtNight(t *testing.T) {
	t.Parallel()
	path := []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	// 1.44 h a tile at speed 1000: 10 tiles take 15 moving hours, so from
	// 06:00 they arrive at 21:00 the same day.
	if got := Estimate(path, 1000, 6, flat); got != 15*2500 {
		t.Fatalf("estimate %d ticks", got)
	}
	// from 20:00: 2 moving hours, the night, then 13 more
	if got := Estimate(path, 1000, 20, flat); got != (2+8+13)*2500 {
		t.Fatalf("estimate over a night %d ticks", got)
	}
	if Estimate(nil, 1000, 6, flat) != 0 || Estimate(path, 0, 6, flat) != -1 {
		t.Fatal("edges")
	}
}
