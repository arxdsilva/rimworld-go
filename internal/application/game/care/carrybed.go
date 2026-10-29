package care

import (
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/ai/pathing"
	"rimworld-go/internal/domain/sim/entity"
)

// bedTrip is which carry-to-bed job runs carryToBed: a rescue to a safe
// bed, a capture to a prisoner bed, or a warden taking a downed prisoner
// back to one (SIM-063).
type bedTrip uint8

const (
	tripRescue bedTrip = iota
	tripCapture
	tripPrisoner
)

// carryToBed runs the shared flow of carry-to-bed job j of carrier id
// (Doctor.DoRescue, DoCapture): with no bed yet (j.Dest 0) pick one,
// reserve it, path beside it, pick the patient up and set off; at the bed
// lay the patient in it. A capture disarms the captive as it picks it up
// and makes it a prisoner before laying it down; a rescue holds a bed
// that is not the patient's own for the stay (layInBed) and fires
// Rescued; a prisoner's trip only lays it down.
func (c *Care) carryToBed(id entity.EntityID, j job.Record, trip bedTrip) {
	h := c.h
	if j.Dest == 0 {
		var ok bool
		if j, ok = c.setOff(id, j, trip); !ok || j.State == job.Goto {
			return
		}
	}
	if b, ok := h.Building(j.Dest); !ok || trip != tripRescue && !b.ForPrisoners {
		c.DropPatient(id)
		j.State, j.Fail = job.Fail, job.TargetGone
		h.SetJob(id, j)
		return
	}
	t := j.Target
	c.Carry.Drop(id)
	if trip == tripCapture && !h.IsPrisoner(t) {
		c.pr.MakePrisoner(t, id, false)
	}
	c.layInBed(t, j.Dest, trip == tripRescue)
	if trip == tripRescue {
		h.Rescued(id, t)
	}
	j.State = job.Complete
	h.SetJob(id, j)
}

// tripBed is the bed carrier id at (x, y) takes patient t to on trip
// (0: none): for a rescue the safe bed the carrier can get beside, and
// unreachable true when safe beds exist but the carrier can reach none.
func (c *Care) tripBed(id, t entity.EntityID, x, y int, trip bedTrip) (bid entity.EntityID, unreachable bool) {
	if trip != tripRescue {
		bid, _ = c.pr.PrisonBedFor(t, x, y)
		return bid, false
	}
	q := rescueQuery(c, t, x, y)
	if bid = q.best(c); bid == 0 {
		return 0, false
	}
	q.reacher = id
	bid = q.best(c)
	return bid, bid == 0
}

// setOff reserves the trip's bed, paths carrier id beside it and picks
// the patient up. It stores and returns the job; false when it failed.
func (c *Care) setOff(id entity.EntityID, j job.Record, trip bedTrip) (job.Record, bool) {
	h := c.h
	t := j.Target
	x, y, _ := h.Pos(id)
	bid, unreachable := c.tripBed(id, t, x, y, trip)
	if unreachable {
		j.State, j.Fail = job.Fail, job.Unreachable
		h.SetJob(id, j)
		return j, false
	}
	if bid == 0 || !h.Claim(id, bid) {
		if trip != tripRescue {
			tx, ty, _ := h.Pos(t)
			h.Message(tx, ty, "No free prisoner bed for "+c.pr.PersonName(t)+".")
		}
		j.State, j.Fail = job.Fail, job.Reserved
		h.SetJob(id, j)
		return j, false
	}
	b, _ := h.Building(bid)
	sx, sy, ok := h.BesideSpot(id, b.X, b.Y)
	var path []pathing.Point
	if ok {
		path, ok = h.PathTo(id, sx, sy)
	}
	if !ok {
		j.State, j.Fail = job.Fail, job.Unreachable
		h.SetJob(id, j)
		return j, false
	}
	if trip == tripCapture {
		c.pr.DropGear(t) // a captive's weapon stays where it was taken
	}
	c.Carry.Pick(id, t)
	j.Dest, j.TX, j.TY, j.Path, j.PathI = bid, sx, sy, path, 0
	j.State = job.Goto
	if len(path) == 0 {
		j.State = job.Do
	}
	h.SetJob(id, j)
	return j, true
}

// layInBed puts patient in bed bid: its slot claimed, on its sleep cell.
// With hold (a rescue) its own bed stays its own: a medical bed, or any
// bed it did not already own, is held only for the stay and given back
// once it is up (Rest.Hold, SIM-063). Without hold (a prisoner's trip) it
// gives up the beds it held.
func (c *Care) layInBed(patient, bid entity.EntityID, hold bool) {
	h := c.h
	b, _ := h.Building(bid)
	switch {
	case !hold:
		h.ReleaseBed(patient)
	case IsMedicalBed(b) || b.SlotOf(uint64(patient)) < 0:
		c.Rest.Hold(patient, bid)
	}
	b, _ = h.Building(bid)
	b = b.Claim(uint64(patient))
	h.SetBuilding(bid, b)
	cx, cy := h.SleepCell(patient, b)
	h.Place(patient, cx, cy)
	h.SetSleptAt(patient, bid)
}
