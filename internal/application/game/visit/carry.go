package visit

import (
	"fmt"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/pkg/mathx"
)

// Carrying the wounded home (MAP-23; wiki: Visitor: "When leaving, visitors
// will carry away wounded members of their faction."). A leaving visitor on
// its feet goes for the nearest downed friend of its faction that the colony
// isn't already looking after, lifts it and walks to the edge, where both
// leave. Enemies (turn.go) walk out alone: they run on the raid AI.
//
// No flap: a visitor keeps to the friend it chose (Rec.Fetch) until the walk
// ends, and one that fails (no way there, the friend got up or was taken)
// gives up on carrying for the rest of its visit (Rec.NoCarry) and just
// leaves.

// carryOff runs leaver r's side of it; true: its turn went to the fetch.
func (v Visits) carryOff(r *Rec, x, y int, j job.Record, now uint64) bool {
	h := v.h
	if t := h.Carrying(r.ID); t != 0 {
		if h.Status(t) != health.Downed {
			h.PutDown(r.ID) // it got up, or died
		}
		return false // on to the edge
	}
	switch {
	case now >= r.LeaveAt+giveUp:
		return false // the walk-out's own limit decides
	case r.Fetch != 0:
		return v.fetching(r, x, y, j)
	case r.NoCarry:
		return false
	}
	t, ok := v.woundedFor(r, x, y)
	if !ok {
		return false
	}
	tx, ty := h.Pos(t)
	sx, sy, ok := h.Beside(r.ID, tx, ty)
	h.Release(r.ID)
	if !ok || !h.StartJob(r.ID, job.Record{Kind: job.Move, TX: sx, TY: sy}) {
		r.NoCarry = true
		h.SetJob(r.ID, job.Record{})
		return false
	}
	r.Fetch = t
	return true
}

// fetching runs r's walk over to its friend: lift it on arrival.
func (v Visits) fetching(r *Rec, x, y int, j job.Record) bool {
	h := v.h
	t := r.Fetch
	if !v.free(t) {
		r.Fetch, r.NoCarry = 0, true
		h.Release(r.ID)
		h.SetJob(r.ID, job.Record{})
		return false
	}
	if j.Active() {
		return true
	}
	r.Fetch = 0
	tx, ty := h.Pos(t)
	done := j.State == job.Complete && max(mathx.Abs(x-tx), mathx.Abs(y-ty)) <= 1
	h.Release(r.ID)
	h.SetJob(r.ID, job.Record{})
	if !done {
		r.NoCarry = true
		return false
	}
	h.Pick(r.ID, t)
	h.Message(x, y, fmt.Sprintf("%s is carrying %s home.", h.Name(r.ID), h.Name(t)))
	return false
}

// woundedFor is the downed friend nearest r at (x, y) that nobody is seeing
// to, the lower id on a tie.
func (v Visits) woundedFor(r *Rec, x, y int) (entity.EntityID, bool) {
	h := v.h
	var best entity.EntityID
	bestD := 0
	recs := v.recs()
	for i := range recs {
		o := &recs[i]
		if o.ID == r.ID || o.Faction != r.Faction || o.Turned || !v.free(o.ID) || v.fetched(o.ID) {
			continue
		}
		ox, oy := h.Pos(o.ID)
		if d := max(mathx.Abs(x-ox), mathx.Abs(y-oy)); best == 0 || d < bestD || d == bestD && o.ID < best {
			best, bestD = o.ID, d
		}
	}
	return best, best != 0
}

// free reports visitor t down on the map and nobody's: not carried, claimed
// or in a bed.
func (v Visits) free(t entity.EntityID) bool {
	h := v.h
	return h.Present(t) && h.Status(t) == health.Downed && !h.Rescuing(t) && !h.GuestStep(t)
}

// fetched reports another leaver on its way to t.
func (v Visits) fetched(t entity.EntityID) bool {
	recs := v.recs()
	for i := range recs {
		if recs[i].Fetch == t {
			return true
		}
	}
	return false
}

// goHome takes id off the map, with whoever it carries.
func (v Visits) goHome(id entity.EntityID) {
	h := v.h
	if t := h.Carrying(id); t != 0 {
		h.PutDown(id)
		h.Depart(t)
	}
	h.Depart(id)
}
