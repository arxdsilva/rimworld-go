package caravan

import "testing"

func TestSpeed(t *testing.T) {
	t.Parallel()
	if SpeedFor(1000) != 3000 || SpeedFor(500) != 1500 || SpeedFor(-3) != 0 {
		t.Fatal("speed")
	}
	c := &Caravan{}
	if c.Speed() != 3000 {
		t.Fatal("default")
	}
	c.Slowest = 500
	if c.Speed() != 1500 {
		t.Fatal("slow")
	}
}

func TestAdvanceAlongPath(t *testing.T) {
	t.Parallel()
	c := &Caravan{Tile: 1, Path: []int{2, 3, 4}, Dest: 4, State: Travelling, FoodDays: 5000}
	// 3 tiles/day: 8 hours is exactly one tile.
	if !Advance(c, 8) || c.Tile != 2 || c.Progress != 0 || len(c.Path) != 2 {
		t.Fatalf("%+v", c)
	}
	if Advance(c, 4) || c.Progress != 500 || c.Tile != 2 {
		t.Fatalf("half %+v", c)
	}
	if !Advance(c, 100) || c.Tile != 4 || c.State != Arrived || !IsArrived(c) || c.Progress != 0 {
		t.Fatalf("end %+v", c)
	}
	if c.FoodDays >= 5000 {
		t.Fatal("food unspent")
	}
	if Advance(c, 5) {
		t.Fatal("arrived caravans stay")
	}
}

func TestAdvanceEdges(t *testing.T) {
	t.Parallel()
	c := &Caravan{State: Travelling}
	if Advance(c, 3) || c.State != Arrived {
		t.Fatal("empty path arrives")
	}
	v := &Caravan{State: Visiting, Path: []int{1}}
	if Advance(v, 24) || len(v.Path) != 1 {
		t.Fatal("visiting does not move")
	}
	z := &Caravan{State: Travelling, Path: []int{1}}
	if Advance(z, 0) {
		t.Fatal("zero hours")
	}
	r := &Caravan{State: Returning, Path: []int{9}, FoodDays: 10}
	if !r.Step(2500*24) || r.Tile != 9 || r.FoodDays != 0 {
		t.Fatalf("%+v", r)
	}
	if (&Caravan{State: Travelling, Path: []int{1}}).Step(100) {
		t.Fatal("sub-hour")
	}
}
