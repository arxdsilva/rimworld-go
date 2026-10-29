package care

import (
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/sim/entity"
)

// fakeGuestHost is care.Guests over a fakeHost for the guest lifecycle
// (guest_test.go): guests and their factions are a map, and what care asked
// is recorded in order. Leave and JoinOffer give the guest's bed
// slots back, as the game's leave path does (the fake host's ReleaseBed
// only logs).
type fakeGuestHost struct {
	h        *fakeHost
	faction  map[entity.EntityID]uint64
	goodwill []int
	reasons  []string
	left     []entity.EntityID
	offers   []entity.EntityID
}

func newFakeGuestHost(h *fakeHost) *fakeGuestHost {
	return &fakeGuestHost{h: h, faction: map[entity.EntityID]uint64{}}
}

// guest puts guest id of faction f (0: a stranger) on (x, y), off the
// colony's roster.
func (g *fakeGuestHost) guest(id entity.EntityID, f uint64, x, y int) {
	g.h.outsider(id, x, y)
	g.h.vital[id] = health.Record{Status: health.Alive}
	g.faction[id] = f
}

func (g *fakeGuestHost) IsGuest(id entity.EntityID) bool {
	_, ok := g.faction[id]
	return ok && !g.h.prisoners[id]
}
func (g *fakeGuestHost) FactionOf(id entity.EntityID) uint64 { return g.faction[id] }
func (g *fakeGuestHost) Goodwill(faction uint64, delta int, reason string) {
	g.goodwill = append(g.goodwill, delta)
	g.reasons = append(g.reasons, reason)
}
func (g *fakeGuestHost) Leave(id entity.EntityID) {
	g.left = append(g.left, id)
	g.unbed(id)
}
func (g *fakeGuestHost) JoinOffer(id entity.EntityID) {
	g.offers = append(g.offers, id)
	g.unbed(id)
}

// unbed gives back every bed slot id holds.
func (g *fakeGuestHost) unbed(id entity.EntityID) {
	for _, bid := range g.h.bldgIDs {
		if b := g.h.bldgs[bid]; b.SlotOf(uint64(id)) >= 0 {
			g.h.bldgs[bid] = b.Release(uint64(id))
		}
	}
}
