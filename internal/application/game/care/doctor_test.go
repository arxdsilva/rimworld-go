package care

import (
	"slices"
	"testing"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/pawn/medcare"
	"rimworld-go/internal/domain/sim/entity"
)

const (
	doc     entity.EntityID = 1
	patient entity.EntityID = 2
	other   entity.EntityID = 3
	bedID   entity.EntityID = 10
	herbal  entity.EntityID = 20
)

func scene() (*Care, *fakeHost) {
	h := newFakeHost()
	h.pawn(doc, 0, 0)
	h.pawn(patient, 5, 5)
	h.doctors[doc] = true
	return New(h, Roles{Prison: newFakePrison(h)}), h
}

func TestOutranksTieBreak(t *testing.T) {
	a := &patientRank{pid: 5, d: 3}
	if !a.outranks(nil) || !(&patientRank{pid: 4, d: 3}).outranks(a) || (&patientRank{pid: 4, d: 4}).outranks(a) ||
		!(&patientRank{pid: 9, bleed: 1}).outranks(a) || !(&patientRank{pid: 9, injury: 1}).outranks(a) {
		t.Fatal("outranks: bleeding, injury, distance, id")
	}
}

func TestRescueCarriesToTheMedicalBed(t *testing.T) {
	c, h := scene()
	h.downed(patient, 50)
	h.bed(bedID, construction.Building{Kind: construction.Bed, X: 8, Y: 8})
	h.bed(bedID+1, construction.Building{Kind: construction.Bed, X: 6, Y: 6, Medical: true})
	if !c.Doctor.Assign(doc) {
		t.Fatal("a downed colonist with a free bed: rescue")
	}
	j := h.jobs[doc]
	if j.Kind != job.Rescue || j.Target != patient {
		t.Fatalf("job %+v", j)
	}
	j.State = job.Do // beside the patient
	c.Doctor.DoRescue(doc, j)
	j = h.jobs[doc]
	if c.Carry.Patient(doc) != patient || j.Dest != bedID+1 || j.State != job.Goto {
		t.Fatalf("picked up for the medical bed: carry %d job %+v", c.Carry.Patient(doc), j)
	}
	h.pos[doc] = [2]int{7, 6}
	c.FollowCarriers()
	if h.pos[patient] != [2]int{7, 6} {
		t.Fatalf("the patient rides along: %v", h.pos[patient])
	}
	j.State = job.Do
	c.Doctor.DoRescue(doc, j)
	if h.jobs[doc].State != job.Complete || c.Carry.Len() != 0 || c.Rest.HeldBed(patient) != bedID+1 ||
		h.sleptAt[patient] != bedID+1 || h.pos[patient] != [2]int{6, 6} || h.log[len(h.log)-1] != "rescued" {
		t.Fatalf("laid in bed: job %+v held %v pos %v log %v", h.jobs[doc], c.Rest.HeldBed(patient), h.pos[patient], h.log)
	}
	if b := h.bldgs[bedID+1]; b.SlotOf(uint64(patient)) < 0 {
		t.Fatal("the bed is claimed for the patient")
	}
}

func TestRescueEnds(t *testing.T) {
	c, h := scene()
	h.downed(patient, 50)
	h.bed(bedID, construction.Building{Kind: construction.Bed, X: 8, Y: 8})
	rec := job.Record{Kind: job.Rescue, Target: patient, State: job.Do}

	h.owners[bedID] = other // someone holds the only bed
	c.Doctor.DoRescue(doc, rec)
	if j := h.jobs[doc]; j.State != job.Fail || j.Fail != job.Reserved {
		t.Fatalf("bed held: %+v", j)
	}
	delete(h.owners, bedID)
	h.noPath = true
	c.Doctor.DoRescue(doc, rec)
	if j := h.jobs[doc]; j.State != job.Fail || j.Fail != job.Unreachable {
		t.Fatalf("no way: %+v", j)
	}
	h.noPath = false
	c.Carry.Pick(doc, patient)
	c.Doctor.DoRescue(doc, job.Record{Kind: job.Rescue, Target: patient, State: job.Do, Dest: 99})
	if j := h.jobs[doc]; j.State != job.Fail || j.Fail != job.TargetGone || c.Carry.Len() != 0 {
		t.Fatalf("bed gone: %+v", j)
	}
	c.Carry.Pick(doc, patient)
	h.vital[patient] = health.Record{Status: health.Alive}
	c.Doctor.DoRescue(doc, rec)
	if j := h.jobs[doc]; j.State != job.Complete || c.Carry.Len() != 0 || h.pos[patient] != h.pos[doc] {
		t.Fatalf("got up: dropped where the rescuer stands: %+v", j)
	}
	h.vital[patient] = health.Record{Status: health.Downed, Cause: health.CauseInjury}
	h.prisoners[patient] = true
	if c.Doctor.Rescuable(patient) {
		t.Fatal("prisoners are tended where they lie")
	}
}

// TestPlainBedKeepsTheOldOne (SIM-063, was TestPlainBedReleasesTheOldOne):
// laid in a bed not its own, the patient keeps its own claim and holds
// this one only for the stay.
func TestPlainBedKeepsTheOldOne(t *testing.T) {
	c, h := scene()
	h.downed(patient, 50)
	h.bed(bedID, construction.Building{Kind: construction.Bed, X: 0, Y: 0})
	c.Carry.Pick(doc, patient)
	c.Doctor.DoRescue(doc, job.Record{Kind: job.Rescue, Target: patient, State: job.Do, Dest: bedID})
	if c.Rest.HeldBed(patient) != bedID || slices.Contains(h.log, "release") {
		t.Fatalf("a plain bed: held for the stay, its own bed kept: %v %v", c.Rest.HeldBed(patient), h.log)
	}
}

func TestSafeBedForRescueRanks(t *testing.T) {
	c, h := scene()
	h.downed(patient, 30) // needs tending: medical and hospital beds first (SIM-063)
	h.bed(bedID, construction.Building{Kind: construction.Bed, X: 1, Y: 0})
	if got := c.SafeBedFor(patient, 0, 0, RescueBed); got != bedID {
		t.Fatalf("the only bed: %d", got)
	}
	h.bed(bedID+1, construction.Building{Kind: construction.HospitalBed, X: 30, Y: 0})
	if got := c.SafeBedFor(patient, 0, 0, RescueBed); got != bedID+1 {
		t.Fatalf("a far hospital bed beats a near plain one: %d", got)
	}
	h.bed(bedID+2, construction.Building{Kind: construction.Bed, X: 40, Y: 0, Medical: true})
	if got := c.SafeBedFor(patient, 0, 0, RescueBed); got != bedID+2 {
		t.Fatalf("a medical bed first: %d", got)
	}
	h.bed(bedID+3, construction.Building{Kind: construction.Bed, X: 40, Y: 0, Medical: true})
	if got := c.SafeBedFor(patient, 0, 0, RescueBed); got != bedID+2 {
		t.Fatalf("a tie goes to the lowest id: %d", got)
	}
	h.owners[bedID+2], h.owners[bedID+3] = other, patient
	if got := c.SafeBedFor(patient, 0, 0, RescueBed); got != bedID+3 {
		t.Fatalf("a bed held by someone else is skipped: %d", got)
	}
	h.bldgs[bedID+3] = construction.Building{Kind: construction.Bed, X: 40, Y: 0, Medical: true, Claimant: uint64(other)}
	h.bed(bedID+4, construction.Building{Kind: construction.Wall, X: 0, Y: 0})
	h.bed(bedID+5, construction.Building{Kind: construction.Bed, ForPrisoners: true, Medical: true})
	if got := c.SafeBedFor(patient, 0, 0, RescueBed); got != bedID+1 {
		t.Fatalf("claimed, not a bed, or a prison bed: skipped (%d)", got)
	}
	h.prisoners[patient] = true
	if got := c.SafeBedFor(patient, 0, 0, RescueBed); got != bedID+5 {
		t.Fatalf("a prisoner takes the prison bed: %d", got)
	}
	if !IsMedicalBed(h.bldgs[bedID+5]) || IsMedicalBed(construction.Building{Kind: construction.Wall, Medical: true}) {
		t.Fatal("IsMedicalBed: a medical sleeping place only")
	}
}

func TestTendFetchesMedicineAndTends(t *testing.T) {
	c, h := scene()
	h.downed(patient, 30)
	h.now = 1000
	h.stacks[herbal] = item.Stack{Kind: item.HerbalMedicine, Qty: 3, X: 2, Y: 2}
	if !c.Doctor.Assign(doc) {
		t.Fatal("a downed patient with no bed: tended where it lies")
	}
	j := h.jobs[doc]
	// RES-1: the game's job start holds the dose (a count claim), not care
	if j.Kind != job.Tend || j.Dest != herbal || j.TX != 2 {
		t.Fatalf("tend with a dose fetched first: %+v", j)
	}
	h.pos[doc] = [2]int{6, 5}
	c.Doctor.DoTend(doc, j)
	if h.jobs[doc].State != job.Complete || h.tended[patient] != 800 || h.stacks[herbal].Qty != 0 ||
		c.Doctor.TendedUntil(patient) != 1000+TendTicks || h.log[len(h.log)-1] != "tended" {
		t.Fatalf("tended with medicine: job %+v q %d until %d", h.jobs[doc], h.tended[patient], c.Doctor.TendedUntil(patient))
	}
}

func TestTendSteps(t *testing.T) {
	c, h := scene()
	h.downed(patient, 30)
	rec := job.Record{Kind: job.Tend, Target: patient, WorkLeft: TendWork, State: job.Do}
	h.pos[doc] = [2]int{6, 5}
	h.work = 0
	c.Doctor.DoTend(doc, rec)
	if h.jobs[doc].WorkLeft != TendWork {
		t.Fatal("no work done: the job waits")
	}
	h.work = 100
	c.Doctor.DoTend(doc, rec)
	if j := h.jobs[doc]; j.WorkLeft != TendWork-100 || j.State != job.Do {
		t.Fatalf("partial work: %+v", j)
	}
	h.pos[patient] = [2]int{20, 20}
	c.Doctor.DoTend(doc, rec)
	if j := h.jobs[doc]; j.State != job.Fail || j.Fail != job.TargetGone {
		t.Fatalf("the patient left: %+v", j)
	}
	h.tend[patient] = false
	c.Doctor.DoTend(doc, rec)
	if h.jobs[doc].State != job.Complete {
		t.Fatal("nothing to tend: done")
	}
}

func TestTendOrderAndSelfTend(t *testing.T) {
	c, h := scene()
	h.pawn(other, 9, 9)
	h.downed(patient, 30)
	h.downed(other, 10)
	h.bleed[other] = 5
	if !c.Doctor.Assign(doc) || h.jobs[doc].Target != other {
		t.Fatalf("the bleeding first: %+v", h.jobs[doc])
	}
	c2, h2 := scene()
	h2.tend[doc] = true
	if !c2.Doctor.Assign(doc) || h2.jobs[doc].Target != doc || h2.jobs[doc].WorkLeft != TendWork*3/2 {
		t.Fatalf("alone: it tends itself: %+v", h2.jobs[doc])
	}
	delete(h2.jobs, doc)
	c2.SetSelfTend(doc, false)
	if c2.Doctor.Assign(doc) {
		t.Fatal("self-tend off: no tend")
	}
	c2.SetSelfTend(doc, true)
	h2.pawn(other, 3, 3)
	h2.doctors[other] = true
	if !c2.Doctor.OtherDoctor(doc) || c2.Doctor.Assign(doc) {
		t.Fatal("another doctor around: no self-tend")
	}
	c2.Rest.Start(other, 0)
	if c2.Doctor.OtherDoctor(doc) {
		t.Fatal("a resting doctor tends no one else")
	}
	c2.SetMedicalCare(patient, uint8(medcare.NoCare))
	h2.downed(patient, 30)
	c2.Rest.Clear(other)
	if c2.Doctor.tendable(patient, false) {
		t.Fatal("No care: not tended")
	}
}
