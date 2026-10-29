package care

import (
	"rimworld-go/internal/domain/ai/job"
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

// InjuredDowned: a pawn down for any reason a bed helps with (SIM-063):
// injury, blood loss, malnutrition, disease, toxins, heatstroke,
// hypothermia, an anaesthetic. Not a collapse from exhaustion
// (health.CauseNoBed), which is sleep where it falls, as in RimWorld; an
// anaesthetised patient already in bed is left out by rescueEligible.
func (d *Doctor) InjuredDowned(id entity.EntityID) bool {
	v, ok := d.h.Vital(id)
	return ok && v.Status == health.Downed && v.Cause != health.CauseNoBed
}

// Assign is the humanlike half of Doctor work, each step its own giver:
// rescue a downed patient to bed, then tend the bleeding, then everyone
// else (or itself, with no one else to help). Surgery and animals follow
// in the host's work assignment.
func (d *Doctor) Assign(id entity.EntityID) bool {
	return d.assignRescue(id) || d.assignTendWhere(id, true) || d.assignTendOther(id) || d.c.Feed.assign(id) // feed.go (SIM-064)
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
		if k := d.c.kindOf(pid); k == targetPrisoner || k == targetGuest || k == targetStranger {
			consider(pid)
		}
	}
	return best
}

// Rescuable: pid is rescueEligible with a safe bed to be carried to
// (SafeBedFor). Its care level does not matter: even "No care" is
// rescued. A downed prisoner is the warden's (prisonbed.go).
func (d *Doctor) Rescuable(pid entity.EntityID) bool {
	if !d.rescueEligible(pid) {
		return false
	}
	px, py, _ := d.h.Pos(pid)
	return d.c.SafeBedFor(pid, px, py, RescueBed) != 0
}

// assignRescue carries the most urgent downed colonist to a bed.
func (d *Doctor) assignRescue(id entity.EntityID) bool {
	best := d.bestPatient(id, func(pid entity.EntityID) (job.Record, bool) {
		return job.Record{Kind: job.Rescue, Target: pid}, d.Rescuable(pid)
	})
	return best != nil && d.h.StartJob(id, best.rec)
}

// DoRescue runs rescue job j of doctor id: on reaching the patient pick
// it up and head for a bed; on reaching the bed lay it in it (carryToBed).
func (d *Doctor) DoRescue(id entity.EntityID, j job.Record) {
	if !d.InjuredDowned(j.Target) {
		d.c.DropPatient(id)
		j.State = job.Complete
		d.h.SetJob(id, j)
		return
	}
	d.c.carryToBed(id, j, tripRescue)
}

// PatientDied: id died; whoever carried it lets go where it stands and
// stops (SIM-063). The host's Interrupt clears the carrier's job and its
// reservations; the patient is put down first so the carry index is clean
// whatever the host does.
func (d *Doctor) PatientDied(id entity.EntityID) {
	carrier := d.c.Carry.Carrier(id)
	if carrier == 0 {
		return
	}
	d.c.DropPatient(carrier)
	d.h.Interrupt(carrier, job.TargetGone)
}

// rescueEligible: pid is downed where a rescue helps and lies out of bed,
// a colonist or a guest the colony takes in (guest.go); Rescuable adds
// "a free bed exists", which the order menu shows as a reason instead.
func (d *Doctor) rescueEligible(pid entity.EntityID) bool {
	if !d.InjuredDowned(pid) || d.h.IsPrisoner(pid) || d.h.BedUnder(pid) != 0 {
		return false
	}
	return d.h.IsColonist(pid) || d.c.Guest.Rescuable(pid)
}
