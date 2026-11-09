package raid

import (
	"testing"

	"rimworld-go/internal/domain/sim/entity"
)

// EAI-010 and EAI-020 unit tests.

// tallyHost answers the roster questions the tally and the memory ask;
// any other Host call panics (the nil embedded Host).
type tallyHost struct {
	Host
	ids    []entity.EntityID
	rs     map[entity.EntityID]Raider
	groups []Group
	stand  map[entity.EntityID]bool
	asks   int
}

func (h *tallyHost) RaiderIDs() []entity.EntityID           { return h.ids }
func (h *tallyHost) Raider(id entity.EntityID) Raider       { return h.rs[id] }
func (h *tallyHost) Groups() []Group                        { return h.groups }
func (h *tallyHost) StandingRaider(id entity.EntityID) bool { h.asks++; return h.stand[id] }

// rivalOf: factions 1 and 2 are rivals.
type rivalOf struct{ Mischief }

func (rivalOf) Rival(a, b int) bool { return a != b && a+b == 3 }

func tallyRig() (*tallyHost, *Raiders) {
	h := &tallyHost{rs: map[entity.EntityID]Raider{}, stand: map[entity.EntityID]bool{},
		groups: []Group{{Size: 3, Faction: 1}, {Size: 2, Faction: 2}}}
	for i, g := range []int{0, 0, 0, 1, 1} {
		id := entity.EntityID(10 + i)
		h.ids = append(h.ids, id)
		h.rs[id] = Raider{Group: g}
		h.stand[id] = true
	}
	return h, &Raiders{h: h, m: rivalOf{}}
}

// TestBrokenCountsOncePerTick (EAI-020): every raider asking its group's
// standing count, and whether rivals share the map, costs one pass over
// the raiders a tick; one downed later in the tick counts from the next;
// a raider arriving mid-tick is counted at once.
func TestBrokenCountsOncePerTick(t *testing.T) {
	t.Parallel()
	h, rs := tallyRig()
	for range h.ids {
		if tl := rs.count(100); tl.stand[0] != 3 || tl.stand[1] != 2 || !tl.rivals || len(tl.fac) != 5 {
			t.Fatalf("tally %+v", *tl)
		}
	}
	if h.asks != len(h.ids) {
		t.Fatalf("%d standing asks for %d raiders in one tick", h.asks, len(h.ids))
	}
	h.stand[10] = false
	if rs.count(100).stand[0] != 3 || h.asks != len(h.ids) {
		t.Fatal("a raider downed mid-tick counts from the next")
	}
	if rs.count(101).stand[0] != 2 || h.asks != 2*len(h.ids) {
		t.Fatal("the next tick counts again, once")
	}
	h.ids = append(h.ids, 20)
	h.rs[20], h.stand[20] = Raider{Group: 1}, true
	if rs.count(101).stand[1] != 3 {
		t.Fatal("an arrival mid-tick is counted")
	}
	h.stand[13], h.stand[14], h.stand[20] = false, false, false
	if tl := rs.count(102); tl.rivals || tl.stand[1] != 0 {
		t.Fatalf("the rivals all down: none left, %+v", *tl)
	}
}

// TestRaiderMemoryWindows (EAI-010): a raider remembers the foe it saw for
// raiderMemory ticks, its group the foe any of them saw for groupMemory.
func TestRaiderMemoryWindows(t *testing.T) {
	t.Parallel()
	h, _ := tallyRig()
	e := &eyes{h: h}
	a, b := h.rs[10], h.rs[11]
	e.saw(&a, 7, 1000)
	if m := e.recall(&a, 1000+raiderMemory-1); m[0] != 7 {
		t.Fatalf("remembered %v", m)
	}
	if m := e.recall(&a, 1000+raiderMemory); m[0] != 0 || m[1] != 7 {
		t.Fatalf("its own memory gone, the group's kept: %v", m)
	}
	if m := e.recall(&b, 1000+groupMemory-1); m[1] != 7 {
		t.Fatalf("a group mate knows: %v", m)
	}
	if m := e.recall(&b, 1000+groupMemory); m != [2]entity.EntityID{} {
		t.Fatalf("forgotten: %v", m)
	}
	if m := e.recall(&Raider{Group: 1}, 1000); m != [2]entity.EntityID{} {
		t.Fatalf("another group knows nothing: %v", m)
	}
}
