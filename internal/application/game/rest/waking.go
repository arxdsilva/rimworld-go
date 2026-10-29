package rest

import (
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/pawn/mood"
	"rimworld-go/internal/domain/sim/entity"
)

// Waking gives a pawn the thoughts of how and where it slept: in a bed or
// on the ground (a sleeping spot counts as the ground), outside under the
// sky, the impressiveness of the bedroom, barracks or hospital it woke in
// (BED-2, room_mood.go), and a bed shared with someone not its partner.
// Felt gives the same kind of room thought for a meal or a pastime.
type Waking struct{ h Host }

// Felt gives pawn id the impressiveness thought of the room it stands in
// for occasion o (a meal in a dining room, a pastime in a recreation
// room), when the room has one.
func (w *Waking) Felt(id entity.EntityID, o Occasion) {
	r := w.h.RoomAt(w.h.Pos(id))
	if r < 0 {
		return
	}
	role, tenths := w.h.RoomQuality(r)
	if k, v, label, ok := RoomMood(role, tenths, o); ok {
		w.h.RememberValue(id, k, v, label)
	}
}

// After records how and where pawn id slept.
func (w *Waking) After(id entity.EntityID) {
	x, y := w.h.Pos(id)
	r := w.h.RoomAt(x, y)
	bed, _ := w.h.SleptAt(id)
	b, inBed := w.h.Building(bed)
	if !inBed || b.Kind == construction.SleepingSpot {
		if r < 0 && w.h.UnderSky(x, y) {
			w.h.Remember(id, mood.SleptOutside)
		} else {
			w.h.Remember(id, mood.SleptOnGround)
		}
		return
	}
	w.h.Remember(id, mood.SleptInBed)
	w.sharedBed(id, b)
	if r < 0 {
		if w.h.UnderSky(x, y) {
			w.h.Remember(id, mood.SleptOutside)
		}
		return
	}
	role, tenths := w.h.RoomQuality(r)
	if k, v, label, ok := RoomMood(role, tenths, Slept); ok {
		w.h.RememberValue(id, k, v, label)
	} else {
		w.h.ForgetGroup(id, mood.SleptInBedroom) // a room with no ladder: no room thought
	}
}

// sharedBed gives id the shared-bed thought when the other slot of its
// double bed is held by someone who is not its partner.
func (w *Waking) sharedBed(id entity.EntityID, b construction.Building) {
	other := entity.EntityID(b.Claimant)
	if other == id {
		other = entity.EntityID(b.Claimant2)
	}
	if b.Kind.Sleepers() < 2 || other == 0 || other == id || w.h.Mate(id) == other {
		return
	}
	if v := sharedBedValue(w.h.Opinion(id, other)); v < 0 {
		w.h.RememberValue(id, mood.SharedBed, v, "")
	} else {
		w.h.ForgetGroup(id, mood.SharedBed)
	}
}
