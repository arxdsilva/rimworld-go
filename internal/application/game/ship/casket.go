package ship

import (
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/sim/entity"
)

// Board sends colonist pawn to sleep in ship casket id; the reason it
// can't ("": on its way). One sleeper per casket (wiki: Ship).
func (s *State) Board(h Host, pawn, id entity.EntityID) string {
	b, ok := h.Building(id)
	switch {
	case !ok || b.Kind != construction.ShipCasket:
		return "not a ship stasis casket"
	case s.launched:
		return "the ship has already left"
	case s.Sleeper(id) != 0:
		return "someone already sleeps in it"
	case !h.Ready(pawn):
		return "only a colonist on their feet can board"
	}
	sx, sy, ok := h.Spot(pawn, b.X, b.Y)
	if !ok {
		return "no way there"
	}
	if !h.StartJob(pawn, job.Record{Kind: job.BoardShip, Target: id, TX: sx, TY: sy, Forced: true}) {
		return "someone else is on the way"
	}
	return ""
}

// Do is a boarding colonist at its casket: it lies down and the stasis
// takes it off the map.
func (s *State) Do(h Host, pawn entity.EntityID, j job.Record) {
	b, ok := h.Building(j.Target)
	if !ok || b.Kind != construction.ShipCasket || s.launched || s.Sleeper(j.Target) != 0 {
		j.State, j.Fail = job.Fail, job.TargetGone
		h.SetJob(pawn, j)
		return
	}
	j.State = job.Complete
	h.SetJob(pawn, j)
	if !h.Board(pawn, j.Target) {
		return
	}
	s.setSleeper(j.Target, pawn)
	h.Message(b.X, b.Y, h.Name(pawn)+" lies down in a ship stasis casket and sleeps.")
	h.Changed()
}

// Leave wakes the sleeper of ship casket id; the reason it can't ("":
// awake beside it).
func (s *State) Leave(h Host, id entity.EntityID) string {
	pawn := s.Sleeper(id)
	switch {
	case pawn == 0:
		return "nobody sleeps in it"
	case !h.Wake(pawn, id):
		return "no room beside it"
	}
	s.setSleeper(id, 0)
	h.Changed()
	return ""
}

// Prune drops the sleepers of caskets that are gone (destroyed or taken
// down): each wakes where the casket stood, or is lost with it.
func (s *State) Prune(h Host) {
	for i := 0; i < len(s.sleepers); {
		e := s.sleepers[i]
		if b, ok := h.Building(e.Casket); ok && b.Kind == construction.ShipCasket {
			i++
			continue
		}
		h.Wake(e.Pawn, e.Casket)
		s.sleepers = append(s.sleepers[:i], s.sleepers[i+1:]...)
		h.Changed()
	}
}

// CasketFree reports ship casket id standing, not marked for removal and
// empty: game/travel may carry a downed colonist into it.
func (s *State) CasketFree(h Host, id entity.EntityID) bool {
	b, ok := h.Building(id)
	return ok && b.Kind == construction.ShipCasket && !b.Deconstruct && !s.launched && s.Sleeper(id) == 0
}

// LayIn puts pawn, carried there by game/travel's CarryToCasket, to sleep
// in ship casket id; false when it can't (the carrier sets it down).
func (s *State) LayIn(h Host, pawn, id entity.EntityID) bool {
	if !s.CasketFree(h, id) || !h.Board(pawn, id) {
		return false
	}
	s.setSleeper(id, pawn)
	b, _ := h.Building(id)
	h.Message(b.X, b.Y, h.Name(pawn)+" was laid in a ship stasis casket.")
	h.Changed()
	return true
}
