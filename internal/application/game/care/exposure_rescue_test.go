// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package care

import (
	"testing"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/pawn/exposure"
)

// AIL-1: a doctor carries a downed exposure victim lying in air that won't
// heal it (in a bed or not) to a bed whose air does, else to a spot whose
// air does, and waits ExposureRetry when there is neither.

const heatSev = exposure.Full / 100 * 80

// hotInOwnBed: patient downed by heatstroke in its own bed at (5, 5) in
// 33.4 °C air (the player's save: comfortable up to 26 °C here).
func hotInOwnBed() (*Care, *fakeHost) {
	c, h := scene()
	h.heatstroked(patient, heatSev)
	h.bed(bedID, construction.Building{Kind: construction.Bed, X: 5, Y: 5, Claimant: uint64(patient)})
	h.setAir(5, 5, 334)
	return c, h
}

// carry runs doctor's rescue job to its end: pick up, then arrive.
func carry(t *testing.T, c *Care, h *fakeHost) job.Record {
	t.Helper()
	j := h.jobs[doc]
	j.State = job.Do
	c.Doctor.DoRescue(doc, j)
	j = h.jobs[doc]
	if j.State == job.Fail {
		return j
	}
	h.pos[doc] = [2]int{j.TX, j.TY}
	c.FollowCarriers()
	j.State = job.Do
	c.Doctor.DoRescue(doc, j)
	return h.jobs[doc]
}

func TestExposureVictimInHotBedGoesToACoolBed(t *testing.T) {
	c, h := hotInOwnBed()
	h.bed(bedID+1, construction.Building{Kind: construction.Bed, X: 9, Y: 9})
	h.setAir(9, 9, 200)
	if !c.Doctor.Assign(doc) || h.jobs[doc].Kind != job.Rescue {
		t.Fatalf("a heatstroke victim in a hot bed is rescued: %+v", h.jobs[doc])
	}
	if j := carry(t, c, h); j.State != job.Complete || j.Dest != bedID+1 || h.pos[patient] != [2]int{9, 9} {
		t.Fatalf("laid in the cool bed: %+v at %v", j, h.pos[patient])
	}
	if h.ExposureWait(patient) != h.now+ExposureRetry {
		t.Fatalf("held from another exposure move: %d", h.ExposureWait(patient))
	}
}

func TestExposureVictimSkipsAHotBedForASpot(t *testing.T) {
	c, h := hotInOwnBed()
	h.bed(bedID+1, construction.Building{Kind: construction.Bed, X: 9, Y: 9})
	h.setAir(9, 9, 300) // too hot as well
	h.air.spot[patient] = [2]int{12, 12}
	if !c.Doctor.Assign(doc) {
		t.Fatal("no cool bed, a cool spot: rescued")
	}
	j := carry(t, c, h)
	if j.State != job.Complete || j.Dest != 0 || h.pos[patient] != [2]int{12, 12} || c.Carry.Patient(doc) != 0 {
		t.Fatalf("put down on the cool spot: %+v at %v", j, h.pos[patient])
	}
	h.setAir(12, 12, 200)
	if c.Doctor.Rescuable(patient) {
		t.Fatal("healing on the spot, every bed hot: left there")
	}
}

func TestExposureVictimWithNowhereCoolWaits(t *testing.T) {
	c, h := hotInOwnBed()
	if c.Doctor.Assign(doc) {
		t.Fatalf("nowhere cooler: no rescue, %+v", h.jobs[doc])
	}
	if h.ExposureWait(patient) != ExposureRetry {
		t.Fatalf("waits ExposureRetry: %d", h.ExposureWait(patient))
	}
	looked := h.air.spots
	for h.now = 1; h.now < ExposureRetry; h.now += 250 {
		if c.Doctor.Assign(doc) || h.air.spots != looked {
			t.Fatalf("tick %d: no new search while it waits", h.now)
		}
	}
	h.air.spot[patient] = [2]int{12, 12}
	if h.now = ExposureRetry; !c.Doctor.Assign(doc) {
		t.Fatal("after the wait a new cool spot is found")
	}
}

func TestExposureVictimHealingInBedStays(t *testing.T) {
	c, h := hotInOwnBed()
	h.setAir(5, 5, 250)
	h.bed(bedID+1, construction.Building{Kind: construction.Bed, X: 9, Y: 9})
	if c.Doctor.rescueEligible(patient) || c.Doctor.Assign(doc) {
		t.Fatal("air that heals: stays in its bed")
	}
}

// TestExposureRescueDoesNotFlap: the air of the room the patient lies in
// drifts across its comfortable limit and back every half hour for a
// day; doctors move it at most once an ExposureRetry, and never for a
// drift the bed margin absorbs.
func TestExposureRescueDoesNotFlap(t *testing.T) {
	c, h := hotInOwnBed()
	h.bed(bedID+1, construction.Building{Kind: construction.Bed, X: 9, Y: 9})
	h.setAir(9, 9, 200)
	h.air.spot[patient] = [2]int{12, 12}
	moves, half := 0, uint64(ExposureRetry/2)
	for step := range 48 {
		h.now = uint64(step) * half
		hot, cool := 334, 200
		if step%2 == 1 {
			hot, cool = cool, hot
		}
		h.setAir(5, 5, hot)
		h.setAir(9, 9, cool)
		h.setAir(12, 12, cool)
		h.jobs[doc] = job.Record{}
		clear(h.owners) // a finished job gives its reservations back
		if c.Doctor.Assign(doc) {
			if j := carry(t, c, h); j.State != job.Complete {
				t.Fatalf("step %d: %+v", step, j)
			}
			moves++
		}
	}
	if moves > 24 {
		t.Fatalf("moved %d times in a day: at most once an hour", moves)
	}
	c, h = hotInOwnBed()
	h.bed(bedID+1, construction.Building{Kind: construction.Bed, X: 9, Y: 9})
	h.setAir(9, 9, 200)
	moves = 0
	for step := range 48 {
		h.now = uint64(step) * half
		h.setAir(9, 9, 250+5*(step%2)) // drifts within the margin, never past 26 °C
		h.jobs[doc] = job.Record{}
		clear(h.owners)
		if c.Doctor.Assign(doc) {
			carry(t, c, h)
			moves++
		}
	}
	if moves != 1 {
		t.Fatalf("one move to the cool bed, none for a drift inside the range: %d", moves)
	}
}

func TestForcedRescueOfAnExposureVictimNamesNoBedOnlyWithNowhere(t *testing.T) {
	c, h := hotInOwnBed()
	if c.rescueBedWhy(patient) != ReasonNoBed {
		t.Fatal("nowhere cooler: no bed")
	}
	h.now = ExposureRetry
	h.air.spot[patient] = [2]int{12, 12}
	if why := c.rescueBedWhy(patient); why != "" {
		t.Fatalf("a cool spot: the order is open, %q", why)
	}
}

func TestPrisonBedIgnoresTheAir(t *testing.T) {
	c, h := scene()
	h.heatstroked(patient, heatSev)
	h.setAir(5, 5, 334)
	h.bed(bedID, construction.Building{Kind: construction.Bed, X: 9, Y: 9, ForPrisoners: true})
	h.setAir(9, 9, 334)
	if PrisonBedFor(c, patient, 5, 5) != bedID {
		t.Fatal("a prison bed is chosen whatever its air (capture, the warden)")
	}
}

func TestExposureSpotInvalidatedInFlight(t *testing.T) {
	c, h := hotInOwnBed()
	h.air.spot[patient] = [2]int{12, 12}
	if !c.Doctor.Assign(doc) {
		t.Fatal("no initial rescue")
	}
	j := h.jobs[doc]
	j.State = job.Do
	c.Doctor.DoRescue(doc, j)
	j = h.jobs[doc]
	if j.State != job.Goto || j.Dest != 0 || c.Carry.Patient(doc) != patient {
		t.Fatalf("not carrying to spot: %+v", j)
	}
	h.setAir(12, 12, 334)
	h.air.spot[patient] = [2]int{15, 15}
	h.pos[doc] = [2]int{j.TX, j.TY}
	c.FollowCarriers()
	j.State = job.Do
	c.Doctor.DoRescue(doc, j)
	if h.jobs[doc].State != job.Fail || c.Carry.Patient(doc) != 0 || h.pos[patient] != h.pos[doc] || h.ExposureWait(patient) > h.now {
		t.Fatalf("stale spot completed or retained patient/retry: %+v, pos=%v, wait=%d", h.jobs[doc], h.pos[patient], h.ExposureWait(patient))
	}
}

// A nonhealing bed is still admissible only when no healing destination
// exists and the patient is not already in a bed or healing air.
func TestLiveNoHealingSiteFallback(t *testing.T) {
	c, h := scene()
	h.heatstroked(patient, heatSev)
	h.setAir(5, 5, 334)
	h.bed(bedID, construction.Building{Kind: construction.Bed, X: 9, Y: 9})
	h.setAir(9, 9, 334)
	h.setAir(10, 9, 334)
	h.pos[doc] = [2]int{5, 5}
	if !c.Doctor.Assign(doc) {
		t.Fatal("no-healing-site fallback refused")
	}
	if j := carry(t, c, h); j.State != job.Complete || j.Dest != bedID {
		t.Fatalf("legitimate fallback invalidated: %+v", j)
	}
}

func TestGroundRescueReroutesLiveHazard(t *testing.T) {
	for _, alternate := range []bool{false, true} {
		c, h := hotInOwnBed()
		h.air.spot[patient] = [2]int{12, 12}
		if !c.Doctor.Assign(doc) {
			t.Fatal("no initial rescue")
		}
		j := h.jobs[doc]
		j.State = job.Do
		c.Doctor.DoRescue(doc, j)
		j = h.jobs[doc]
		if j.State != job.Goto || c.Carry.Patient(doc) != patient {
			t.Fatal("fixture did not pick up")
		}
		h.pos[doc] = [2]int{j.TX, j.TY}
		h.air.unsafe = map[[2]int]bool{h.pos[doc]: true}
		if alternate {
			h.air.spot[patient] = [2]int{15, 15}
		} else {
			delete(h.air.spot, patient)
		}
		c.FollowCarriers()
		j.State = job.Do
		c.Doctor.DoRescue(doc, j)
		for _, log := range h.log {
			if log == "rescued" {
				t.Fatal("unsafe initial landing emitted success")
			}
		}
		j = h.jobs[doc]
		if !alternate {
			if j.State != job.Fail || c.Carry.Patient(doc) != 0 || h.ExposureWait(patient) > h.now {
				t.Fatal("unsuccessful transfer retained carry or success cooldown")
			}
			continue
		}
		if j.State != job.Goto || c.Carry.Patient(doc) != patient || j.TX != 15 || j.TY != 15 {
			t.Fatalf("no live alternate transport: %+v", j)
		}
		h.pos[doc] = [2]int{j.TX, j.TY}
		c.FollowCarriers()
		j.State = job.Do
		c.Doctor.DoRescue(doc, j)
		if h.jobs[doc].State != job.Complete || h.pos[patient] != [2]int{15, 15} || c.Carry.Patient(doc) != 0 {
			t.Fatal("safe alternate did not finish")
		}
		if h.ExposureWait(patient) != h.now+ExposureRetry {
			t.Fatal("successful reroute lost anti-flap cooldown")
		}
	}
}
