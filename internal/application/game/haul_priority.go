package game

import (
	"rimworld-go/internal/domain/building/zone"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
)

// Storage priority (PLAY-023), as RimWorld: haulers fill the
// highest-priority storage zone that takes a thing and has room
// (storageCell), and a stack lying in a lower-priority zone is carried up
// when a higher one takes it and has room. Two rules keep stacks from
// flapping between zones: a stack only ever moves to a strictly higher
// priority (never sideways or down), and a stack set down below the best
// zone for it (say the better one was full) stays at least rehaulMinStay
// before it may move up. Nothing here runs while every storage zone has
// the same priority, so such colonies haul exactly as before.

// rehaulMinStay is the least time a stack set down in a lower-priority
// zone stays there before a hauler may carry it up (an original value:
// one in-game hour).
const rehaulMinStay = clock.TicksPerHour

// haulPrio caches, for one haul search, the best storage priority taking
// each item kind and whether a zone above a given priority has room for
// it (zones and stacks don't change during the search).
type haulPrio struct {
	top  [256]int8 // best priority + 3 taking the kind (0 unknown, 1 none)
	room map[haulPrioKey]bool
}

type haulPrioKey struct {
	k item.Kind
	p zone.Priority
}

// topStorage is the best priority among the storage zones taking kind k;
// ok is false when none does.
func (g *Game) topStorage(k item.Kind) (best zone.Priority, ok bool) {
	g.zones.Each(func(z zone.Zone) bool {
		if z.Kind.Storage() && allows(z, k) && (!ok || z.Priority > best) {
			best, ok = z.Priority, true
		}
		return true
	})
	return best, ok
}

// topFor is topStorage through the search's cache.
func (h *haulPrio) topFor(g *Game, k item.Kind) (zone.Priority, bool) {
	if h.top[k] == 0 {
		h.top[k] = 1
		if p, ok := g.topStorage(k); ok {
			h.top[k] = int8(p) + 3
		}
	}
	return zone.Priority(h.top[k] - 3), h.top[k] > 1
}

// roomAbove reports a storage zone above priority p that takes kind k and
// has a free cell, through the search's cache (the search stops at the
// first such cell).
func (h *haulPrio) roomAbove(g *Game, k item.Kind, p zone.Priority) bool {
	key := haulPrioKey{k, p}
	if v, seen := h.room[key]; seen {
		return v
	}
	s := storeSearch{g: g, h: haulHost{g}, st: item.Stack{Kind: k}, any: true}
	_, _, ok := s.run(p)
	if h.room == nil {
		h.room = map[haulPrioKey]bool{}
	}
	h.room[key] = ok
	return ok
}

// staysStored reports a stack that lies in a storage zone taking it and
// stays there: no zone of higher priority takes its kind, it was set down
// there less than rehaulMinStay ago, or no higher zone has room. Any
// other stack in storage is a re-haul candidate; a stack outside storage
// never stays.
func (g *Game) staysStored(id entity.EntityID, st *item.Stack, h *haulPrio) bool {
	z, ok := g.storageAt(st.X, st.Y)
	if !ok || !z.AllowsStack(*st) { // PRD-4: special filters too
		return false
	}
	if top, ok := h.topFor(g, st.Kind); !ok || z.Priority >= top {
		return true // the common case: already in the best zone for it
	}
	if at, seen := g.haul.storedAt[id]; seen && uint64(g.clock.Now()) < at+rehaulMinStay {
		return true
	}
	return !h.roomAbove(g, st.Kind, z.Priority)
}

// haulDest is where hauler id carries stack st: for a stack already in
// storage (a re-haul) a cell in a strictly higher-priority zone, else the
// best storage cell inside its area; either way one whose zone's special
// filters take st (PRD-4).
func (g *Game) haulDest(id entity.EntityID, st item.Stack) (int, int, bool) {
	floor := zone.Priority(noPriority)
	if z, ok := g.storageAt(st.X, st.Y); ok && z.AllowsStack(st) {
		floor = z.Priority
	}
	var keep func(x, y int) bool
	if r := g.restrictedRef(id); r != 0 {
		keep = func(x, y int) bool { return g.refContains(r, x, y) }
	}
	return g.storageCell(st, st.X, st.Y, keep, floor)
}

// noteStored records when the stack now lying on (x, y) was set down
// there: in a storage zone below the best one taking its kind it starts
// the minimum stay, anywhere else any old record is dropped.
func (g *Game) noteStored(x, y int) {
	id := g.itemAt(x, y)
	if id == 0 {
		return
	}
	st := g.stacks.Ref(id)
	if st == nil || st.Carrier != 0 {
		return
	}
	if z, ok := g.storageAt(x, y); ok && allows(z, st.Kind) {
		if top, _ := g.topStorage(st.Kind); z.Priority < top {
			if g.haul.storedAt == nil {
				g.haul.storedAt = map[entity.EntityID]uint64{}
			}
			g.haul.storedAt[id] = uint64(g.clock.Now())
			return
		}
	}
	delete(g.haul.storedAt, id)
}

// pruneStored forgets the stays that are over or whose stack is gone
// (deletion only, so the map's order never matters).
func (g *Game) pruneStored() {
	if len(g.haul.storedAt) == 0 {
		return
	}
	now := uint64(g.clock.Now())
	for id, at := range g.haul.storedAt {
		if now >= at+rehaulMinStay || g.stacks.Ref(id) == nil {
			delete(g.haul.storedAt, id)
		}
	}
	if len(g.haul.storedAt) == 0 {
		g.haul.storedAt = nil // an empty map saves (and hashes) as none
	}
}

// cmdSetZonePriority sets storage zone id's priority to the one at index
// i of zone.Priorities (highest first); other zones and other indexes are
// ignored.
func (g *Game) cmdSetZonePriority(id uint16, i int) {
	z, ok := g.zones.Zone(zone.ID(id))
	if !ok || !z.Kind.Storage() || i < 0 || i >= len(zone.Priorities) {
		return
	}
	g.zones.Update(z.ID, func(z *zone.Zone) { z.Priority = zone.Priorities[i] })
}
