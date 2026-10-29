package care

import (
	"slices"
	"testing"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/sim/command"
)

func TestInjuredDownedEveryCauseButCollapse(t *testing.T) {
	c, h := scene()
	for _, cause := range []health.Cause{health.CauseInjury, health.CauseBloodLoss, health.CauseNoFood, health.CauseDisease,
		health.CauseOverdose, health.CauseHeatstroke, health.CauseHypothermia, health.CauseAnesthetic, health.CauseInfection} {
		h.vital[patient] = health.Record{Status: health.Downed, Cause: cause}
		if !c.Doctor.InjuredDowned(patient) || !c.Doctor.rescueEligible(patient) {
			t.Errorf("cause %d: rescued", cause)
		}
	}
	h.vital[patient] = health.Record{Status: health.Downed, Cause: health.CauseNoBed}
	if c.Doctor.InjuredDowned(patient) {
		t.Error("collapsed asleep: sleeps where it fell, no rescue")
	}
	h.vital[patient] = health.Record{Status: health.Alive}
	if c.Doctor.InjuredDowned(patient) {
		t.Error("standing: no rescue")
	}
	h.vital[patient] = health.Record{Status: health.Downed, Cause: health.CauseAnesthetic}
	h.bed(bedID, construction.Building{Kind: construction.Bed, X: 5, Y: 5, Claimant: uint64(patient)})
	if c.Doctor.rescueEligible(patient) {
		t.Error("anaesthetised in bed: stays")
	}
}

func TestRescueStarvedColonistToItsOwnBed(t *testing.T) {
	c, h := scene()
	starved(h, patient)
	h.bed(bedID, construction.Building{Kind: construction.Bed, X: 6, Y: 5})
	h.bed(bedID+1, construction.Building{Kind: construction.Bed, X: 9, Y: 9, Claimant: uint64(patient)})
	if !c.Doctor.Assign(doc) || h.jobs[doc].Kind != job.Rescue {
		t.Fatalf("a starved colonist is rescued: %+v", h.jobs[doc])
	}
	j := h.jobs[doc]
	j.State = job.Do
	c.Doctor.DoRescue(doc, j)
	j = h.jobs[doc]
	if j.Dest != bedID+1 {
		t.Fatalf("to its own bed: %+v", j)
	}
	j.State = job.Do
	c.Doctor.DoRescue(doc, j)
	if h.jobs[doc].State != job.Complete || c.Rest.HeldBed(patient) != 0 || slices.Contains(h.log, "release") ||
		h.bldgs[bedID+1].SlotOf(uint64(patient)) < 0 {
		t.Fatalf("laid in its own bed: nothing held, nothing released: held %d log %v", c.Rest.HeldBed(patient), h.log)
	}
}

// TestRescueOtherBedHeldThenGivenBack: laid in a free bed not its own,
// the patient keeps its own claim; the stay ends when it is up.
func TestRescueOtherBedHeldThenGivenBack(t *testing.T) {
	c, h := scene()
	starved(h, patient)
	h.bed(bedID, construction.Building{Kind: construction.Bed, X: 6, Y: 5})
	own := construction.Building{Kind: construction.Bed, X: 9, Y: 9, Claimant: uint64(patient)}
	h.bed(bedID+1, own)
	h.owners[bedID+1] = other // its own bed is held for someone: the free one
	c.Carry.Pick(doc, patient)
	c.Doctor.DoRescue(doc, job.Record{Kind: job.Rescue, Target: patient, State: job.Do, Dest: bedID})
	if c.Rest.HeldBed(patient) != bedID || h.bldgs[bedID+1] != own || slices.Contains(h.log, "release") {
		t.Fatalf("held for the stay, own bed kept: held %d own %+v log %v", c.Rest.HeldBed(patient), h.bldgs[bedID+1], h.log)
	}
	h.vital[patient] = health.Record{Status: health.Alive}
	c.Rest.Update(patient)
	if c.Rest.HeldBed(patient) != 0 || h.bldgs[bedID].SlotOf(uint64(patient)) >= 0 || h.bldgs[bedID+1] != own {
		t.Fatalf("up again: the stay is given back, its own bed kept: %+v", h.bldgs[bedID])
	}
}

func TestRescueCarrierCutOffFromTheBestBed(t *testing.T) {
	h := newFakeHost()
	h.pawn(doc, 0, 0)
	h.pawn(patient, 5, 5)
	h.doctors[doc] = true
	ch := newCutHost(h)
	c := New(ch, Roles{Prison: newFakePrison(h)})
	h.downed(patient, 30)
	h.bed(bedID, construction.Building{Kind: construction.Bed, X: 6, Y: 6, Medical: true})
	h.bed(bedID+1, construction.Building{Kind: construction.Bed, X: 20, Y: 20})
	ch.cut[cutWay{doc, [2]int{6, 6}}] = true
	c.Doctor.DoRescue(doc, job.Record{Kind: job.Rescue, Target: patient, State: job.Do})
	if j := h.jobs[doc]; j.Dest != bedID+1 || c.Carry.Patient(doc) != patient {
		t.Fatalf("the carrier takes the best bed it can reach: %+v", j)
	}
	c.DropPatient(doc)
	ch.cut[cutWay{doc, [2]int{20, 20}}] = true
	c.Doctor.DoRescue(doc, job.Record{Kind: job.Rescue, Target: patient, State: job.Do})
	if j := h.jobs[doc]; j.State != job.Fail || j.Fail != job.Unreachable || c.Carry.Len() != 0 {
		t.Fatalf("safe beds, none the carrier can reach: unreachable: %+v", j)
	}
}

// TestRescueDoerCutOff: the doer alone can't get to the patient (the
// order menu's "cannot reach"; orders_test's noPath cuts every way).
func TestRescueDoerCutOff(t *testing.T) {
	h := newFakeHost()
	h.pawn(doc, 0, 0)
	h.pawn(patient, 5, 5)
	h.doctors[doc] = true
	ch := newCutHost(h)
	c := New(ch, Roles{Prison: newFakePrison(h)})
	h.downed(patient, 30)
	h.bed(bedID, construction.Building{Kind: construction.Bed, X: 8, Y: 8})
	ch.cut[cutWay{doc, [2]int{5, 5}}] = true
	if offer, why := c.judge(doc, command.OrderRescue, patient); !offer || why != ReasonUnreachable {
		t.Fatalf("offered, disabled as unreachable: %v %q", offer, why)
	}
}

func TestPatientDiedIsDropped(t *testing.T) {
	c, h := scene()
	h.downed(patient, 30)
	c.Doctor.PatientDied(patient) // no one carries it: nothing happens
	if len(h.log) != 0 {
		t.Fatalf("no carrier: %v", h.log)
	}
	c.Carry.Pick(doc, patient)
	h.jobs[doc] = job.Record{Kind: job.Rescue, Target: patient, State: job.Goto, Dest: bedID}
	h.pos[doc] = [2]int{3, 4}
	h.vital[patient] = health.Record{Status: health.Dead}
	c.Doctor.PatientDied(patient)
	if c.Carry.Len() != 0 || h.pos[patient] != [2]int{3, 4} || h.log[0] != "interrupt" {
		t.Fatalf("dropped on the carrier's tile, the carrier stopped: pos %v log %v", h.pos[patient], h.log)
	}
}
