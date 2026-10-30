package haul

import (
	"testing"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/ai/pathing"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/entity"
)

// fakeHost is a map of stacks; the hauler stands at (px, py) and reaches
// every cell but (wallX, wallY).
type fakeHost struct {
	stacks       map[entity.EntityID]item.Stack
	px, py       int
	wallX, wallY int
	released     []entity.EntityID
}

func (f *fakeHost) Stack(id entity.EntityID) (item.Stack, bool) {
	st, ok := f.stacks[id]
	return st, ok
}
func (f *fakeHost) SetStack(id entity.EntityID, st item.Stack) { f.stacks[id] = st }
func (f *fakeHost) DeleteItem(id entity.EntityID)              { delete(f.stacks, id) }
func (f *fakeHost) Release(_, target entity.EntityID)          { f.released = append(f.released, target) }
func (f *fakeHost) PathTo(_ entity.EntityID, x, y int) ([]pathing.Point, bool) {
	if x == f.wallX && y == f.wallY {
		return nil, false
	}
	if x == f.px && y == f.py {
		return nil, true
	}
	return []pathing.Point{{X: x, Y: y}}, true
}

const carried = 100

func newFake() *fakeHost {
	return &fakeHost{stacks: map[entity.EntityID]item.Stack{
		carried: {Kind: item.Meal, Qty: 4, Carrier: 1, Rot: 100, Poison: 10},
	}, wallX: -1, wallY: -1}
}

func TestGatherWalksPicksAndMerges(t *testing.T) {
	f := newFake()
	f.stacks[2] = item.Stack{Kind: item.Meal, Qty: 4, X: 3, Y: 0, Rot: 300, Poison: 30}
	f.stacks[3] = item.Stack{Kind: item.Meal, Qty: 6, X: 5, Y: 0}
	j := job.Record{Kind: job.Haul, Carry: carried, Count: 10, Pickups: []entity.EntityID{2, 4, 3, 6}}
	g := NewGather(f)
	if !g.Next(1, &j) || j.TX != 3 || j.State != job.Goto || j.PickupI != 0 {
		t.Fatalf("walks to the first pile: %+v", j)
	}
	f.px, f.py = 3, 0
	if !g.Arrive(1, &j) || j.TX != 5 || j.PickupI != 2 {
		t.Fatalf("takes it and walks on: %+v", j)
	}
	c := f.stacks[carried]
	if c.Qty != 8 || c.Rot != 200 || c.Poison != 20 {
		t.Fatalf("merged load %+v, want 8 with rot and poison averaged", c)
	}
	if _, ok := f.stacks[2]; ok {
		t.Fatal("a pile taken whole is gone")
	}
	f.px = 5
	if g.Arrive(1, &j) || j.PickupI != len(j.Pickups) {
		t.Fatalf("done after the last pile: %+v", j)
	}
	if c := f.stacks[carried]; c.Qty != 10 {
		t.Fatalf("load %d, want the cap 10", c.Qty)
	}
	if st := f.stacks[3]; st.Qty != 4 {
		t.Fatalf("the rest of the last pile stays: %+v", st)
	}
	if len(f.released) != 2 {
		t.Fatalf("released %v, want both piles", f.released)
	}
}

func TestGatherSkipsPilesThatNoLongerJoin(t *testing.T) {
	f := newFake()
	f.stacks[2] = item.Stack{Kind: item.Wood, Qty: 4, X: 3}                  // another kind
	f.stacks[3] = item.Stack{Kind: item.Meal, Qty: 4, X: 4, Forbidden: true} // forbidden since
	f.stacks[4] = item.Stack{Kind: item.Meal, Qty: 4, X: 5, Carrier: 9}      // picked up by another
	f.stacks[6] = item.Stack{Kind: item.Meal, Qty: 4, X: 7, Y: 7}            // walled off
	f.wallX, f.wallY = 7, 7
	f.stacks[7] = item.Stack{Kind: item.Meal, Qty: 2, X: 0, Y: 0} // underfoot: taken at once
	j := job.Record{Kind: job.Haul, Carry: carried, Pickups: []entity.EntityID{2, 4, 3, 4, 4, 4, 5, 4, 6, 4, 7, 2}}
	if NewGather(f).Next(1, &j) {
		t.Fatalf("nothing left to walk to: %+v", j)
	}
	if c := f.stacks[carried]; c.Qty != 6 {
		t.Fatalf("load %d, want 4 plus the 2 underfoot", c.Qty)
	}
	if j.PickupI != len(j.Pickups) || len(f.released) != 6 {
		t.Fatalf("every pile done and released: %+v %v", j, f.released)
	}
}

func TestGatherStopsWhenFull(t *testing.T) {
	f := newFake()
	f.stacks[2] = item.Stack{Kind: item.Meal, Qty: 4, X: 3}
	j := job.Record{Kind: job.Haul, Carry: carried, Count: 4, Pickups: []entity.EntityID{2, 4}}
	if NewGather(f).Arrive(1, &j) || f.stacks[carried].Qty != 4 || f.stacks[2].Qty != 4 {
		t.Fatalf("a full load takes nothing more: %+v %+v", j, f.stacks)
	}
}
