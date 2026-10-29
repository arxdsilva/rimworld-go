package care

import "rimworld-go/internal/domain/sim/entity"

// SIM-063 test doubles: a fakeHost whose ways can be cut one by one, and
// a Guests role of fixed answers.

// cutHost is a fakeHost where pawn cut[i].who can't get beside tile
// cut[i].at (every other way stays open).
type cutHost struct {
	*fakeHost
	cut map[cutWay]bool
}

type cutWay struct {
	who entity.EntityID
	at  [2]int
}

func newCutHost(h *fakeHost) *cutHost { return &cutHost{fakeHost: h, cut: map[cutWay]bool{}} }

func (h *cutHost) BesideSpot(id entity.EntityID, x, y int) (int, int, bool) {
	if h.cut[cutWay{id, [2]int{x, y}}] {
		return 0, 0, false
	}
	return h.fakeHost.BesideSpot(id, x, y)
}

// burning marks bed as a hazard (Reach.BedHazard).
func (h *fakeHost) burning(bed entity.EntityID) {
	if h.hazard == nil {
		h.hazard = map[entity.EntityID]bool{}
	}
	h.hazard[bed] = true
}

// s3Guests is care.Guests: guests by set with their faction (0: a
// stranger); the rest records nothing.
type s3Guests struct{ faction map[entity.EntityID]uint64 }

func (g s3Guests) IsGuest(id entity.EntityID) bool {
	_, ok := g.faction[id]
	return ok
}
func (g s3Guests) FactionOf(id entity.EntityID) uint64               { return g.faction[id] }
func (g s3Guests) Goodwill(faction uint64, delta int, reason string) {}
func (g s3Guests) Leave(id entity.EntityID)                          {}
func (g s3Guests) JoinOffer(id entity.EntityID)                      {}
