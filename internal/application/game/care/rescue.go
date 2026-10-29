package care

import (
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/ai/pathing"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/sim/entity"
)

// Doctor is Doctor work on humanlike patients (MED-03): carry a downed
// colonist to a bed (they heal faster there), tend wounds, and the
// drafted emergency tend. A tend has a quality (body.TendQuality, MED-02,
// rolled by the host): it stops the bleeding, speeds the healing of every
// wound it tends and slows an infection for its window. tendedUntil is
// when each tend stops showing as tended (TendTicks).
type Doctor struct {
	c           *Care
	h           Host
	tendedUntil map[entity.EntityID]uint64
}

// InjuredDowned: a pawn lying down from injury (or from exposure).
func (d *Doctor) InjuredDowned(id entity.EntityID) bool {
	v, ok := d.h.Vital(id)
	return ok && v.Status == health.Downed &&
		(v.Cause == health.CauseInjury || v.Cause == health.CauseHypothermia || v.Cause == health.CauseHeatstroke)
}

// Assign is the humanlike half of Doctor work, each step its own giver:
// rescue a downed patient to bed, then tend the bleeding, then everyone
// else (or itself, with no one else to help). Surgery and animals follow
// in the host's work assignment.
func (d *Doctor) Assign(id entity.EntityID) bool {
	return d.assignRescue(id) || d.assignTendWhere(id, true) || d.assignTendOther(id)
}

// patientRank is a doctor's candidate: the job it would start, how fast
// the patient bleeds, how hurt it is and how far away.
type patientRank struct {
	rec       job.Record
	pid       entity.EntityID
	bleed     int32
	injury, d int
}

// outranks: c comes before best: bleeding fastest, then worst hurt,
// then nearest, then lowest id (a deterministic tie-break).
func (c *patientRank) outranks(best *patientRank) bool {
	switch {
	case best == nil:
		return true
	case c.bleed != best.bleed:
		return c.bleed > best.bleed
	case c.injury != best.injury:
		return c.injury > best.injury
	case c.d != best.d:
		return c.d < best.d
	}
	return c.pid < best.pid
}

// bestPatient ranks every humanlike patient doctor id could look after
// (colonists, then prisoners; never one being carried or held by someone
// else) for which want gives a job, and returns the first by outranks,
// with the job's spot beside the patient filled in.
func (d *Doctor) bestPatient(id entity.EntityID, want func(pid entity.EntityID) (job.Record, bool)) *patientRank {
	x, y, _ := d.h.Pos(id)
	var best *patientRank
	consider := func(pid entity.EntityID) {
		if pid == id || d.c.Carry.Carrier(pid) != 0 {
			return
		}
		if _, taken := d.h.Owner(pid); taken {
			return
		}
		rec, ok := want(pid)
		if !ok {
			return
		}
		px, py, _ := d.h.Pos(pid)
		sx, sy, ok := d.h.BesideSpot(id, px, py)
		if !ok {
			return
		}
		rec.TX, rec.TY = sx, sy
		pv, _ := d.h.Vital(pid)
		c := &patientRank{rec: rec, pid: pid, injury: pv.Injury, d: manhattan(x, y, px, py), bleed: d.h.BleedPerDay(pid)}
		if c.outranks(best) {
			best = c
		}
	}
	for _, pid := range d.h.PawnIDs() {
		consider(pid)
	}
	for _, pid := range d.h.RaiderIDs() {
		if d.h.IsPrisoner(pid) {
			consider(pid)
		}
	}
	return best
}

// rescuable: pid is a downed colonist (prisoners are tended where they
// lie) out of bed with a free bed to be carried to. Its care level does
// not matter: even "No care" is rescued.
func (d *Doctor) rescuable(pid entity.EntityID) bool {
	if !d.InjuredDowned(pid) || d.h.IsPrisoner(pid) || d.h.BedUnder(pid) != 0 {
		return false
	}
	px, py, _ := d.h.Pos(pid)
	return d.c.SafeBedFor(pid, px, py, RescueBed) != 0
}

// assignRescue carries the most urgent downed colonist to a bed.
func (d *Doctor) assignRescue(id entity.EntityID) bool {
	best := d.bestPatient(id, func(pid entity.EntityID) (job.Record, bool) {
		return job.Record{Kind: job.Rescue, Target: pid}, d.rescuable(pid)
	})
	return best != nil && d.h.StartJob(id, best.rec)
}

// DoRescue runs rescue job j of doctor id: on reaching the patient pick
// it up and head for a bed; on reaching the bed lay it in it.
func (d *Doctor) DoRescue(id entity.EntityID, j job.Record) {
	h := d.h
	if !d.InjuredDowned(j.Target) {
		d.c.DropPatient(id)
		j.State = job.Complete
		h.SetJob(id, j)
		return
	}
	x, y, _ := h.Pos(id)
	if j.Dest == 0 {
		bid := d.c.SafeBedFor(j.Target, x, y, RescueBed)
		if bid == 0 || !h.Claim(id, bid) {
			j.State, j.Fail = job.Fail, job.Reserved
			h.SetJob(id, j)
			return
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
			return
		}
		d.c.Carry.Pick(id, j.Target)
		j.Dest, j.TX, j.TY, j.Path, j.PathI = bid, sx, sy, path, 0
		j.State = job.Goto
		if len(path) == 0 {
			j.State = job.Do
		}
		h.SetJob(id, j)
		if j.State == job.Goto {
			return
		}
	}
	if _, ok := h.Building(j.Dest); !ok {
		d.c.DropPatient(id)
		j.State, j.Fail = job.Fail, job.TargetGone
		h.SetJob(id, j)
		return
	}
	if dest, _ := h.Building(j.Dest); IsMedicalBed(dest) {
		d.c.Rest.Hold(j.Target, j.Dest) // held for the stay; its own bed stays its own
	} else {
		h.ReleaseBed(j.Target)
	}
	b, _ := h.Building(j.Dest)
	b = b.Claim(uint64(j.Target))
	h.SetBuilding(j.Dest, b)
	d.c.Carry.Drop(id)
	cx, cy := h.SleepCell(j.Target, b)
	h.Place(j.Target, cx, cy)
	h.SetSleptAt(j.Target, j.Dest)
	h.Rescued(id, j.Target)
	j.State = job.Complete
	h.SetJob(id, j)
}
