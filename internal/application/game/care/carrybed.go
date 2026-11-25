// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package care

import (
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/ai/pathing"
	"rimworld-go/internal/domain/building/construction"
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
	if j.Dest == 0 && c.Carry.Patient(id) == 0 {
		var ok bool
		if j, ok = c.setOff(id, j, trip); !ok || j.State == job.Goto {
			return
		}
	}
	if j.Dest == 0 {
		x, y, _ := h.Pos(id)
		if trip == tripRescue {
			if !h.AirHeals(j.Target, x, y, 0) {
				failCarry(c, id, j, trip)
				return
			}
			if !h.GroundSafe(j.Target, id, x, y) {
				// A new physical hazard may be avoidable without putting the
				// patient down on the unsafe tile. Keep the actual carry relation.
				if !redirectGround(c, id, j) {
					failCarry(c, id, j, trip)
				}
				return
			}
		}
		laySpot(c, id, j)
		return
	}
	if !arrivalBedOK(c, id, j.Target, j.Dest, trip) {
		failCarry(c, id, j, trip)
		return
	}
	t := j.Target
	c.Carry.Drop(id)
	if trip == tripCapture && !h.IsPrisoner(t) {
		c.pr.MakePrisoner(t, id)
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
	q := tripQuery(c, id, t, x, y, trip)
	if bid = q.healBest(c); bid == 0 {
		return 0, false
	}
	q.reacher = id
	bid = q.healBest(c)
	return bid, bid == 0
}

func tripQuery(c *Care, carrier, patient entity.EntityID, x, y int, trip bedTrip) bedQuery {
	q := rescueQuery(c, patient, x, y)
	q.carrier = carrier
	if trip != tripRescue {
		q.prison, q.heal, q.animal = true, false, false
	}
	return q
}

// arrivalBedOK rechecks the physical destination against live ownership,
// bed type, forbidden/hazard status and clinical conditions before putting
// the patient down. Existing carry jobs never silently claim a now invalid bed.
func arrivalBedOK(c *Care, carrier, patient, bid entity.EntityID, trip bedTrip) bool {
	h := c.h
	b, ok := h.Building(bid)
	if !ok {
		return false
	}
	q := tripQuery(c, carrier, patient, b.X, b.Y, trip)
	q.reacher = carrier
	// For BedUnder's structural check (carrier zero), exposure retry
	// remains the Doctor's separate decision.
	if !q.eligible(c, bid, b) {
		return false
	}
	if carrier == 0 {
		q.heal = false
	}
	if q.bedWhy(c, bid, b) == "" {
		return true
	}
	if !q.heal || q.best(c) != 0 || !q.fallbackAllowed(c) {
		return false
	}
	q.heal = false
	return q.bedWhy(c, bid, b) == ""
}

// failCarry ends the obsolete transport at the carrier's actual tile.
// The normal terminal-job cleanup releases its reservations. A failed
// exposure transfer must not retain the successful-move cooldown.
func failCarry(c *Care, id entity.EntityID, j job.Record, trip bedTrip) {
	c.DropPatient(id)
	if trip == tripRescue && c.h.ExposureWait(j.Target) > c.h.Now() {
		c.h.SetExposureWait(j.Target, c.h.Now())
	}
	j.State, j.Fail = job.Fail, job.TargetGone
	c.h.SetJob(id, j)
}

// tripWhy is the shared destination pre-admission check for forced,
// autonomous rescue, capture and warden transport. It has no job effects.
func tripWhy(c *Care, doer, target entity.EntityID, trip bedTrip) string {
	px, py, _ := c.h.Pos(target)
	bid, blocked := c.tripBed(doer, target, px, py, trip)
	if bid != 0 {
		return ""
	}
	if blocked {
		return ReasonUnreachable
	}
	if trip == tripRescue && c.Doctor.exposureStuck(target) {
		if x, y, ok := c.h.HealingSpotFor(target, doer); ok {
			if _, ok = c.h.PathTo(doer, x, y); ok {
				return ""
			}
			return ReasonUnreachable
		}
	}
	if why := tripQuery(c, doer, target, px, py, trip).refusal(c); why != "" {
		return why
	}
	if trip != tripRescue {
		return ReasonNoPrisonBed
	}
	return ReasonNoBed
}

// setOff reserves the trip's bed, paths carrier id beside it and picks
// the patient up. It stores and returns the job; false when it failed.
func (c *Care) setOff(id entity.EntityID, j job.Record, trip bedTrip) (job.Record, bool) {
	h := c.h
	t := j.Target
	x, y, _ := h.Pos(id)
	bid, unreachable := c.tripBed(id, t, x, y, trip)
	if bid == 0 && !unreachable && trip == tripRescue && c.Doctor.exposureStuck(t) {
		return setOffToSpot(c, id, j)
	}
	if unreachable {
		j.State, j.Fail = job.Fail, job.Unreachable
		h.SetJob(id, j)
		return j, false
	}
	if bid == 0 || !h.Claim(id, bid) {
		if trip != tripRescue {
			tx, ty, _ := h.Pos(t)
			h.Message(tx, ty, "No free prisoner bed for %s.", c.pr.PersonName(t))
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
	holdExposure(c, t, trip)
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
	case IsMedicalBed(b) || construction.BedSlot(b, uint64(patient)) < 0:
		c.Rest.Hold(patient, bid)
	}
	b, _ = h.Building(bid)
	b = construction.ClaimBed(b, uint64(patient))
	h.SetBuilding(bid, b)
	cx, cy := h.SleepCell(patient, b)
	h.Place(patient, cx, cy)
	h.SetSleptAt(patient, bid)
}

// setOffToSpot carries exposure victim j.Target, with no bed free whose
// air heals it, to the nearest spot whose air does (AIL-1; wiki: Ailments,
// treated by moving to more comfortable temperatures). The trip keeps
// Dest 0: carrying with no bed is what marks it. With no spot, or no way
// there, the patient waits ExposureRetry.
func setOffToSpot(c *Care, id entity.EntityID, j job.Record) (job.Record, bool) {
	h := c.h
	sx, sy, ok := h.HealingSpotFor(j.Target, id)
	var path []pathing.Point
	if ok {
		path, ok = h.PathTo(id, sx, sy)
	}
	if !ok {
		h.SetExposureWait(j.Target, h.Now()+ExposureRetry)
		j.State, j.Fail = job.Fail, job.Unreachable
		h.SetJob(id, j)
		return j, false
	}
	holdExposure(c, j.Target, tripRescue)
	c.Carry.Pick(id, j.Target)
	j.TX, j.TY, j.Path, j.PathI, j.State = sx, sy, path, 0, job.Goto
	if len(path) == 0 {
		j.State = job.Do
	}
	h.SetJob(id, j)
	return j, true
}

// redirectGround replans an in-flight ground rescue through the same live
// admission policy, without teleporting, re-picking or emitting success.
func redirectGround(c *Care, id entity.EntityID, j job.Record) bool {
	h := c.h
	x, y, ok := h.HealingSpotFor(j.Target, id)
	if !ok || !h.GroundSafe(j.Target, id, x, y) {
		return false
	}
	path, ok := h.PathTo(id, x, y)
	if !ok || len(path) == 0 {
		return false
	}
	j.TX, j.TY, j.Path, j.PathI, j.State = x, y, path, 0, job.Goto
	h.SetJob(id, j)
	return true
}

// holdExposure: a rescue lifting patient t out of air that won't heal its
// exposure holds it from another exposure move for ExposureRetry, so air
// drifting across its comfortable limit can't send doctors back and forth
// (AIL-1 hysteresis, with healMargin).
func holdExposure(c *Care, t entity.EntityID, trip bedTrip) {
	h := c.h
	if x, y, _ := h.Pos(t); trip == tripRescue && !h.AirHeals(t, x, y, 0) {
		h.SetExposureWait(t, h.Now()+ExposureRetry)
	}
}

// laySpot puts the patient carrier id brought to a spot down there.
func laySpot(c *Care, id entity.EntityID, j job.Record) {
	h := c.h
	c.DropPatient(id)
	h.Rescued(id, j.Target)
	j.State = job.Complete
	h.SetJob(id, j)
}
