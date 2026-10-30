package haul

import (
	"slices"
	"testing"

	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/entity"
)

// fakeItems is an item list and store for the ground index.
type fakeItems struct {
	ids    []entity.EntityID
	stacks map[entity.EntityID]*item.Stack
	w, h   int
	check  bool
}

func newFakeItems(w, h int) *fakeItems {
	return &fakeItems{stacks: map[entity.EntityID]*item.Stack{}, w: w, h: h, check: true}
}

func (f *fakeItems) add(id entity.EntityID, st item.Stack) {
	f.ids = append(f.ids, id)
	f.stacks[id] = &st
}

func (f *fakeItems) ItemIDs() []entity.EntityID              { return f.ids }
func (f *fakeItems) StackRef(id entity.EntityID) *item.Stack { return f.stacks[id] }
func (f *fakeItems) Store() any                              { return f }
func (f *fakeItems) MapSize() (int, int)                     { return f.w, f.h }
func (f *fakeItems) SelfCheck() bool                         { return f.check }
func (f *fakeItems) move(id entity.EntityID, x, y int, gr *Ground) {
	f.stacks[id].X, f.stacks[id].Y = x, y
	gr.Moved()
}

// TestGroundListsInItemOrder: a cell lists its loose stacks in item-list
// order, carried and gone stacks drop out without a rebuild, and the
// derived answers (First, Others, Room, Shelf) read that list.
func TestGroundListsInItemOrder(t *testing.T) {
	f := newFakeItems(8, 8)
	f.add(5, item.Stack{Kind: item.Wood, Qty: 10, X: 2, Y: 3})
	f.add(3, item.Stack{Kind: item.Steel, Qty: 70, X: 2, Y: 3})
	f.add(9, item.Stack{Kind: item.Wood, Qty: 4, X: 2, Y: 3, Carrier: 77})
	f.add(7, item.Stack{Kind: item.Wood, Qty: 900, X: 2, Y: 3})
	var gr Ground
	if got := gr.Loose(f, 2, 3); !slices.Equal(got, []entity.EntityID{5, 3, 7}) {
		t.Fatalf("Loose = %v, want item-list order without the carried stack", got)
	}
	if gr.First(f, 2, 3) != 5 || gr.First(f, 0, 0) != 0 || gr.Others(f, 3, 2, 3) != 2 {
		t.Fatal("First and Others read the list")
	}
	if n, top := gr.Shelf(f, item.Wood, 2, 3); n != 3 || !top {
		t.Fatalf("Shelf = %d, %v: three stacks, the 10 wood takes a load", n, top)
	}
	if r := gr.Room(f, item.Wood, 2, 3, 3); r != item.Wood.StackLimit()-10 {
		t.Fatalf("Room = %d: a full cell tops up its smallest wood stack", r)
	}
	f.stacks[5].Carrier = 1 // picked up and deleted with no Moved: verified, not trusted
	delete(f.stacks, 3)
	if got := gr.Loose(f, 2, 3); !slices.Equal(got, []entity.EntityID{7}) {
		t.Fatalf("Loose = %v after a pickup and a delete", got)
	}
}

// TestGroundRebuildsOnMoved: a stack set down elsewhere is found once the
// generation moves; without the bump the self-check panics.
func TestGroundRebuildsOnMoved(t *testing.T) {
	f := newFakeItems(8, 8)
	f.add(1, item.Stack{Kind: item.Wood, Qty: 1, X: 1, Y: 1})
	f.add(2, item.Stack{Kind: item.Wood, Qty: 1, X: 4, Y: 4})
	var gr Ground
	gr.Warm(f)
	f.move(2, 1, 1, &gr)
	if got := gr.Loose(f, 1, 1); !slices.Equal(got, []entity.EntityID{1, 2}) {
		t.Fatalf("Loose = %v after a move", got)
	}
	f.stacks[1].X = 6 // moved with no Moved
	defer func() {
		if recover() == nil {
			t.Fatal("a move the index missed must panic under the self-check")
		}
	}()
	gr.Loose(f, 6, 1)
}

// TestGroundOffMapAndNewStore: an off-map stack is answered by the scan,
// and a replaced store (a load) rebuilds.
func TestGroundOffMapAndNewStore(t *testing.T) {
	f := newFakeItems(4, 4)
	f.add(1, item.Stack{Kind: item.Wood, Qty: 1, X: -1, Y: 2})
	var gr Ground
	if gr.First(f, -1, 2) != 1 || gr.First(f, 9, 9) != 0 {
		t.Fatal("off-map lookups scan once a stack lies off the map")
	}
	g := newFakeItems(4, 4)
	g.add(8, item.Stack{Kind: item.Wood, Qty: 1, X: 3, Y: 3})
	if gr.First(g, 3, 3) != 8 || gr.First(g, -1, 2) != 0 {
		t.Fatal("a new store rebuilds the lists")
	}
}
