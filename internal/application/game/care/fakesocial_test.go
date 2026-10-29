package care

import (
	"strconv"

	"rimworld-go/internal/domain/sim/entity"
)

// fakeSocial is care.Social over a fakeHost: opinions, awake pawns and
// recreation are maps; what care asked is logged on the host's log.
type fakeSocial struct {
	h       *fakeHost
	opinion map[[2]entity.EntityID]int
	asleep  map[entity.EntityID]bool
	joy     map[entity.EntityID]int
	chats   int
}

func newFakeSocial(h *fakeHost) *fakeSocial {
	return &fakeSocial{h: h, opinion: map[[2]entity.EntityID]int{}, asleep: map[entity.EntityID]bool{},
		joy: map[entity.EntityID]int{}}
}

// friends sets both opinions to o.
func (s *fakeSocial) friends(a, b entity.EntityID, o int) {
	s.opinion[[2]entity.EntityID{a, b}], s.opinion[[2]entity.EntityID{b, a}] = o, o
}

func (s *fakeSocial) Opinion(from, to entity.EntityID) int {
	return s.opinion[[2]entity.EntityID{from, to}]
}
func (s *fakeSocial) Awake(id entity.EntityID) bool { return !s.asleep[id] }
func (s *fakeSocial) Interact(a, b entity.EntityID) {
	s.chats++
	s.h.log = append(s.h.log, "chat "+strconv.Itoa(int(a))+"+"+strconv.Itoa(int(b)))
}
func (s *fakeSocial) Recreation(id entity.EntityID) int       { return s.joy[id] }
func (s *fakeSocial) Recreate(id entity.EntityID, amount int) { s.joy[id] += amount }

// fakeGear is care.Gear: a set of pawns with something to strip.
type fakeGear struct {
	h     *fakeHost
	worn  map[entity.EntityID]bool
	strip int
}

func (g *fakeGear) HasGear(t entity.EntityID) bool { return g.worn[t] }
func (g *fakeGear) StripGear(t entity.EntityID) bool {
	if !g.worn[t] {
		return false
	}
	delete(g.worn, t)
	g.strip++
	return true
}

// fakeGuests is care.Guests recording goodwill.
type fakeGuests struct {
	guests   map[entity.EntityID]uint64
	goodwill []int
}

func (g *fakeGuests) IsGuest(id entity.EntityID) bool     { _, ok := g.guests[id]; return ok }
func (g *fakeGuests) FactionOf(id entity.EntityID) uint64 { return g.guests[id] }
func (g *fakeGuests) Goodwill(faction uint64, delta int, reason string) {
	g.goodwill = append(g.goodwill, delta)
}
func (g *fakeGuests) Leave(id entity.EntityID)     {}
func (g *fakeGuests) JoinOffer(id entity.EntityID) {}
