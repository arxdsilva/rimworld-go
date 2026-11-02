package allow

import (
	"slices"
	"testing"

	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/entity"
)

// fakeHost is a map of stacks and buildings with a home rectangle and a
// fogged column.
type fakeHost struct {
	stacks    map[entity.EntityID]item.Stack
	bldgs     map[entity.EntityID]construction.Building
	ids       []entity.EntityID
	homeMaxX  int // home: x <= homeMaxX
	fogX      int // fogged: x == fogX
	doorFlips int
	puts      int
}

func newFake() *fakeHost {
	return &fakeHost{stacks: map[entity.EntityID]item.Stack{}, bldgs: map[entity.EntityID]construction.Building{}, homeMaxX: 4, fogX: 9}
}

func (h *fakeHost) add(id entity.EntityID, st item.Stack) {
	h.stacks[id] = st
	h.ids = append(h.ids, id)
}

func (h *fakeHost) ItemIDs() []entity.EntityID { return h.ids }
func (h *fakeHost) Stack(id entity.EntityID) (item.Stack, bool) {
	st, ok := h.stacks[id]
	return st, ok
}
func (h *fakeHost) PutStack(id entity.EntityID, st item.Stack) { h.stacks[id] = st; h.puts++ }
func (h *fakeHost) Building(id entity.EntityID) (construction.Building, bool) {
	b, ok := h.bldgs[id]
	return b, ok
}
func (h *fakeHost) SetBuilding(id entity.EntityID, b construction.Building) { h.bldgs[id] = b }
func (h *fakeHost) DoorChanged(construction.Building)                       { h.doorFlips++ }
func (h *fakeHost) InHome(x, _ int) bool                                    { return x <= h.homeMaxX }
func (h *fakeHost) Seen(x, _ int) bool                                      { return x != h.fogX }

func TestSetStackAndBuilding(t *testing.T) {
	h := newFake()
	h.add(1, item.Stack{Kind: item.Wood, Qty: 5, X: 1, Y: 1})
	h.add(2, item.Stack{Kind: item.Wood, Qty: 5, X: 1, Y: 1, Carrier: 7})
	h.bldgs[10] = construction.Building{Kind: construction.Door, X: 3, Y: 3}
	h.bldgs[11] = construction.Building{Kind: construction.Bed, X: 5, Y: 3}
	f := New(h)
	f.Set(1, true)
	f.Set(2, true)
	f.Set(10, true)
	f.Set(11, true)
	if !h.stacks[1].Forbidden || h.stacks[2].Forbidden {
		t.Fatalf("a ground stack forbids, a carried one keeps its flag: %+v %+v", h.stacks[1], h.stacks[2])
	}
	if !h.bldgs[10].Forbidden || !h.bldgs[11].Forbidden || h.doorFlips != 1 {
		t.Fatalf("door and bed forbidden, the door re-walked once: %+v %+v flips %d", h.bldgs[10], h.bldgs[11], h.doorFlips)
	}
	f.Set(10, true) // no change: no re-walk
	if h.doorFlips != 1 {
		t.Fatalf("an unchanged door re-walked: %d", h.doorFlips)
	}
	f.Set(10, false)
	if h.bldgs[10].Forbidden || h.doorFlips != 2 {
		t.Fatalf("door allowed again: %+v %d", h.bldgs[10], h.doorFlips)
	}
}

func TestRectSkipsFogAndOutside(t *testing.T) {
	h := newFake()
	h.add(1, item.Stack{Kind: item.Wood, Qty: 1, X: 2, Y: 2})
	h.add(2, item.Stack{Kind: item.Wood, Qty: 1, X: 9, Y: 2}) // fogged
	h.add(3, item.Stack{Kind: item.Wood, Qty: 1, X: 12, Y: 2})
	New(h).Rect(10, 3, 0, 0, true)
	if !h.stacks[1].Forbidden || h.stacks[2].Forbidden || h.stacks[3].Forbidden {
		t.Fatalf("only the seen stack inside: %+v", h.stacks)
	}
}

func TestHaulRectAllowsAndMarks(t *testing.T) {
	h := newFake()
	h.add(1, item.Stack{Kind: item.Wood, Qty: 1, X: 1, Y: 1, Forbidden: true})                   // the colony's, forbidden
	h.add(2, item.Stack{Kind: item.Steel, Qty: 1, X: 2, Y: 1, Forbidden: true, Unclaimed: true}) // found loot
	h.add(3, item.Stack{Kind: item.Steel, Qty: 1, X: 3, Y: 1, Unclaimed: true})
	h.add(4, item.Stack{Kind: item.Steel, Qty: 1, X: 9, Y: 1, Forbidden: true, Unclaimed: true}) // fogged
	h.add(5, item.Stack{Kind: item.Wood, Qty: 1, X: 4, Y: 1})                                    // nothing to do
	f := New(h)
	f.HaulRect(0, 0, 10, 2, true)
	if s := h.stacks[1]; s.Forbidden || s.HaulMarked {
		t.Fatalf("the colony's stack is allowed, takes no mark: %+v", s)
	}
	if s := h.stacks[2]; s.Forbidden || !s.HaulMarked {
		t.Fatalf("found loot is allowed and marked: %+v", s)
	}
	if s := h.stacks[3]; !s.HaulMarked {
		t.Fatalf("a found stack is marked: %+v", s)
	}
	if s := h.stacks[4]; !s.Forbidden || s.HaulMarked {
		t.Fatalf("a fogged stack is left: %+v", s)
	}
	if h.puts != 3 {
		t.Fatalf("only changed stacks are put back: %d", h.puts)
	}
	f.HaulRect(0, 0, 10, 2, false) // cancel: unmarks, never forbids again
	if s := h.stacks[2]; s.HaulMarked || s.Forbidden {
		t.Fatalf("cancel unmarks only: %+v", s)
	}
}

func TestMarkAllows(t *testing.T) {
	h := newFake()
	h.add(1, item.Stack{Kind: item.Steel, Qty: 1, X: 1, Y: 1, Forbidden: true, Unclaimed: true})
	h.add(2, item.Stack{Kind: item.Steel, Qty: 1, X: 1, Y: 1, Carrier: 3, Forbidden: true, Unclaimed: true})
	f := New(h)
	f.Mark(1, true)
	f.Mark(2, true)
	if s := h.stacks[1]; s.Forbidden || !s.HaulMarked {
		t.Fatalf("marked and allowed: %+v", s)
	}
	if s := h.stacks[2]; !s.Forbidden || s.HaulMarked {
		t.Fatalf("a carried stack is left: %+v", s)
	}
	f.Mark(1, false)
	if s := h.stacks[1]; s.HaulMarked || s.Forbidden {
		t.Fatalf("unmarked, still allowed: %+v", s)
	}
}

func TestOrderedAllows(t *testing.T) {
	h := newFake()
	h.add(1, item.Stack{Kind: item.Berries, Qty: 4, X: 6, Y: 1, Forbidden: true})
	f := New(h)
	f.Ordered(1)
	f.Ordered(99) // gone: no-op
	if h.stacks[1].Forbidden {
		t.Fatal("an ordered stack is allowed")
	}
}

func TestDropRuleIsHomeArea(t *testing.T) {
	f := New(newFake())
	for _, c := range []struct {
		x           int
		taken, want bool
		wantCorpse  bool
	}{{x: 2, want: false, wantCorpse: false}, {x: 7, want: true, wantCorpse: true}, {x: 7, taken: true, want: true, wantCorpse: false}} {
		if got := f.DropForbidden(c.x, 0); got != c.want {
			t.Fatalf("drop at x=%d forbidden %v, want %v", c.x, got, c.want)
		}
		if got := f.CorpseForbidden(c.x, 0, c.taken); got != c.wantCorpse {
			t.Fatalf("corpse at x=%d taken=%v forbidden %v, want %v", c.x, c.taken, got, c.wantCorpse)
		}
	}
}

func TestRectSurvivesMerges(t *testing.T) {
	h := newFake()
	h.add(1, item.Stack{Kind: item.Wood, Qty: 1, X: 1, Y: 1})
	h.add(2, item.Stack{Kind: item.Wood, Qty: 1, X: 2, Y: 1})
	ids := slices.Clone(h.ids)
	New(h).Rect(0, 0, 3, 3, true)
	if !slices.Equal(ids, h.ids) || !h.stacks[1].Forbidden || !h.stacks[2].Forbidden {
		t.Fatalf("every stack forbidden: %+v", h.stacks)
	}
}
