package rest

import (
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/pawn/mood"
	"rimworld-go/internal/domain/sim/entity"
)

// niceRoomScore is the room score from which a bedroom is nice to wake in.
const niceRoomScore = 12

// Waking gives a pawn the thoughts of how and where it slept: in a bed or
// on the ground (a sleeping spot counts as the ground), outside under the
// sky, in an awful or a nice room.
type Waking struct{ h Host }

// After records how and where pawn id slept.
func (w *Waking) After(id entity.EntityID) {
	x, y := w.h.Pos(id)
	r := w.h.RoomAt(x, y)
	bed, _ := w.h.SleptAt(id)
	onFloor := bed == 0
	if b, ok := w.h.Building(bed); ok && b.Kind == construction.SleepingSpot {
		onFloor = true
	}
	if onFloor {
		if r < 0 && w.h.UnderSky(x, y) {
			w.h.Remember(id, mood.SleptOutside)
		} else {
			w.h.Remember(id, mood.SleptOnGround)
		}
		return
	}
	w.h.Remember(id, mood.SleptInBed)
	if r < 0 {
		if w.h.UnderSky(x, y) {
			w.h.Remember(id, mood.SleptOutside)
		}
		return
	}
	switch s := w.h.RoomScore(r); {
	case s < 0:
		w.h.Remember(id, mood.SleptInAwfulRoom)
	case s >= niceRoomScore:
		w.h.Remember(id, mood.SleptInNiceRoom)
	default:
		w.h.ForgetGroup(id, mood.SleptInNiceRoom)
	}
}
