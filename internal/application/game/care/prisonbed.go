package care

import (
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/sim/entity"
)

// PrisonBeds is the warden's "take a downed prisoner back to bed"
// (SIM-063): a prisoner downed outside a bed (InjuredDowned, so not one
// collapsed asleep) is carried to a prison bed, autonomously by a warden
// (Assign, which the game's warden work tries first) or by the player's
// order "Take <name> to bed" (Judge, Force). The trip is carryToBed
// without making anyone a prisoner. It is a one-shot job, so it needs no
// hysteresis: the prisoner stays reserved by its warden while carried.
type PrisonBeds struct{ c *Care }

func newPrisonBeds(c *Care) *PrisonBeds { return &PrisonBeds{c: c} }

// Lying: prisoner id is down outside a bed, where a warden would carry it
// back to one.
func (p *PrisonBeds) Lying(id entity.EntityID) bool {
	c := p.c
	return c.h.IsPrisoner(id) && c.Doctor.InjuredDowned(id) && c.h.BedUnder(id) == 0
}

// Assign gives warden id the job of carrying the nearest such prisoner
// (ties to the lowest id) that no one carries or holds, that it can get
// beside, to a free prison bed; false when there is none.
func (p *PrisonBeds) Assign(id entity.EntityID) bool {
	c := p.c
	h := c.h
	x, y, _ := h.Pos(id)
	var best entity.EntityID
	var rec job.Record
	bestD := 1 << 30
	for _, pid := range h.RaiderIDs() { // ascending: a tie keeps the lower id
		if pid == id || !p.Lying(pid) || c.Carry.Carrier(pid) != 0 {
			continue
		}
		if _, taken := h.Owner(pid); taken {
			continue
		}
		px, py, _ := h.Pos(pid)
		d := manhattan(x, y, px, py)
		if d >= bestD {
			continue
		}
		if _, ok := c.pr.PrisonBedFor(pid, px, py); !ok {
			continue
		}
		sx, sy, ok := h.BesideSpot(id, px, py)
		if !ok {
			continue
		}
		best, bestD = pid, d
		rec = job.Record{Kind: job.CarryToPrisonBed, Target: pid, TX: sx, TY: sy}
	}
	return best != 0 && h.StartJob(id, rec)
}

// Do runs carry job j of warden id: beside the prisoner pick it up and
// head for a prison bed; at the bed lay it down. A prisoner that got up,
// died or is no prisoner any more ends the job (dropped where the warden
// stands).
func (p *PrisonBeds) Do(id entity.EntityID, j job.Record) {
	c := p.c
	t := j.Target
	if !c.h.IsPrisoner(t) || !c.Doctor.InjuredDowned(t) {
		c.DropPatient(id)
		j.State = job.Complete
		c.h.SetJob(id, j)
		return
	}
	if j.Dest == 0 && c.Carry.Patient(id) != t && !c.adjacent(id, t) {
		j.State, j.Fail = job.Fail, job.TargetGone
		c.h.SetJob(id, j)
		return
	}
	c.carryToBed(id, j, tripPrisoner)
}

// Judge is the menu verdict on "Take <name> to bed" (forced.go's judge
// pattern: one verdict for the menu and the order). Offered for a
// prisoner down outside a bed; disabled when the doer can't act, someone
// else carries or holds the prisoner, no prison bed is free or the doer
// can't get beside it. Drafted or not, any able colonist may.
func (p *PrisonBeds) Judge(doer, target entity.EntityID) (bool, string) {
	c := p.c
	if !p.Lying(target) {
		return false, ReasonNoLonger
	}
	return true, firstWhy(c.doerWhy(doer, false), c.holdWhy(doer, target), c.prisonBedWhy(target), c.reachWhy(doer, target))
}

// Force starts the order once Judge allowed it (Care.Force checks first):
// doer drops what it does and sets off, holding the prisoner.
func (p *PrisonBeds) Force(doer, target entity.EntityID) string {
	return p.c.forceJob(doer, target, job.Record{Kind: job.CarryToPrisonBed, Target: target, Forced: true})
}
