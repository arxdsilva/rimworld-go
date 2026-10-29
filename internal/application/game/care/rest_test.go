package care

import (
	"testing"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/needs"
)

// TestRestDoesNotFlap (no-flap): rest starts at the pain line and ends
// only at the far lower well line, and never before RestMin; between the
// two lines the patient stays where it is, in bed or up.
func TestRestDoesNotFlap(t *testing.T) {
	c, h := scene()
	r := c.Rest
	h.healing[patient] = PainInjury - 1
	r.Update(patient)
	if r.On(patient) {
		t.Fatal("below the pain line: no rest")
	}
	h.healing[patient] = PainInjury
	r.Update(patient)
	if !r.On(patient) {
		t.Fatal("at the pain line: rest")
	}
	flips := 0
	for tick := uint64(1); tick < 4*RestMin; tick++ {
		h.now = tick
		h.healing[patient] = RestWellInjury + 1 + int(tick%3) // between the lines
		was := r.On(patient)
		r.Update(patient)
		if r.On(patient) != was {
			flips++
		}
	}
	if flips != 0 || !r.On(patient) {
		t.Fatalf("between the lines: %d flips", flips)
	}
	h.healing[patient] = RestWellInjury
	r.Update(patient)
	if r.On(patient) {
		t.Fatal("well and past RestMin: up")
	}
	h.healing[patient] = PainInjury - 1
	for tick := uint64(0); tick < RestMin; tick++ {
		h.now++
		r.Update(patient)
		if r.On(patient) {
			t.Fatal("up again below the pain line: stays up")
		}
	}
}

func TestRestMinAndSickness(t *testing.T) {
	c, h := scene()
	r := c.Rest
	h.sickRest[patient] = true
	r.Update(patient)
	if !r.On(patient) || !r.NeedsBedRest(patient) || !r.MayUseMedicalBed(patient) {
		t.Fatal("sick: rest")
	}
	h.sickRest[patient], h.sickStill[patient] = false, true
	h.now = RestMin
	r.Update(patient)
	if !r.On(patient) {
		t.Fatal("still sick: stays in bed")
	}
	h.sickStill[patient] = false
	h.now = RestMin - 1
	r.Update(patient)
	h.now = RestMin
	r.Update(patient)
	if r.On(patient) {
		t.Fatal("well past RestMin: up")
	}
	h.surgery[patient] = true
	if !r.PatientRest(patient) || !r.WaitsForDoctor(patient) {
		t.Fatal("waiting for an operation: rest, and wait")
	}
	r.SetPawnBedRest(patient, false)
	r.SetPawnBedRest(77, false) // not a colonist
	if r.PatientRest(patient) || !r.Off(patient) || r.Off(77) {
		t.Fatal("rest when hurt off")
	}
	r.Start(patient, 1)
	r.Update(patient)
	if r.On(patient) {
		t.Fatal("turned off: a rest ends")
	}
	r.SetPawnBedRest(patient, true)
	if r.Off(patient) {
		t.Fatal("back on")
	}
}

func TestRestSlotTakesTheMedicalBed(t *testing.T) {
	c, h := scene()
	r := c.Rest
	h.healing[patient] = PainInjury
	h.bed(bedID, construction.Building{Kind: construction.Bed, X: 9, Y: 9, Medical: true})
	h.bed(bedID+1, construction.Building{Kind: construction.Bed, X: 5, Y: 6, Medical: true})
	h.owners[bedID+1] = other // held by a rescuer for someone else
	if r.Slot(patient, job.Record{}) {
		t.Fatal("not resting: no slot")
	}
	r.Update(patient)
	h.jobs[patient] = job.Record{Kind: job.Haul, State: job.Goto}
	if !r.Slot(patient, h.jobs[patient]) {
		t.Fatal("resting: to bed")
	}
	j := h.jobs[patient]
	if j.Kind != job.Sleep || j.Target != bedID || j.WorkLeft != needs.SleepWorkMin || r.HeldBed(patient) != bedID ||
		h.sleptAt[patient] != bedID || h.bldgs[bedID].SlotOf(uint64(patient)) < 0 || h.log[0] != "interrupt" {
		t.Fatalf("the free medical bed, held for the stay: %+v held %d", j, r.HeldBed(patient))
	}
	j.State = job.Do
	if !r.Slot(patient, j) {
		t.Fatal("in bed: stays")
	}
	for _, k := range []job.Kind{job.Eat, job.Warm} {
		if r.Slot(patient, job.Record{Kind: k, State: job.Goto}) {
			t.Fatalf("%v goes on", k)
		}
	}
	if r.Slot(patient, job.Record{Kind: job.Haul, Forced: true}) {
		t.Fatal("a player's order goes on")
	}
	h.hungry[patient] = true
	if !r.Slot(patient, job.Record{}) || h.jobs[patient].Kind != job.Eat {
		t.Fatal("hungry: up to eat")
	}
	h.hungry[patient] = false
	// well again: up, and the stay is given back once out of bed
	h.healing[patient] = 0
	h.now = RestMin
	h.jobs[patient] = job.Record{Kind: job.Sleep, Target: bedID, State: job.Do}
	r.Update(patient)
	if r.On(patient) || r.HeldBed(patient) != bedID {
		t.Fatal("up, but still lying in it: the stay holds")
	}
	h.jobs[patient] = job.Record{}
	r.Update(patient)
	if r.HeldBed(patient) != 0 || h.bldgs[bedID].SlotOf(uint64(patient)) >= 0 {
		t.Fatal("out of bed: the medical bed is given back")
	}
}

func TestRestLieDownFallbacks(t *testing.T) {
	c, h := scene()
	r := c.Rest
	h.surgery[patient] = true
	if !r.LieDown(patient) || h.log[0] != "hospital" {
		t.Fatal("waiting for surgery: a hospital bed first")
	}
	h.surgery[patient] = false
	if !r.LieDown(patient) || h.jobs[patient].Target != 99 {
		t.Fatal("no medical bed: its own bed")
	}
	h.noBeds = true
	if r.LieDown(patient) {
		t.Fatal("no bed and no doctor coming: left as it was")
	}
	h.tend[patient] = true // doc doctors
	if !r.LieDown(patient) || h.jobs[patient].Target != 0 || r.RetryAt(patient) != RestRetry {
		t.Fatalf("a doctor is coming: on the floor, bed looked for again later: %+v", h.jobs[patient])
	}
	r.Start(patient, 0)
	if !r.Slot(patient, job.Record{Kind: job.Sleep, State: job.Do}) {
		t.Fatal("on the floor for a doctor: stays")
	}
	h.tend[patient] = false
	h.jobs[patient] = job.Record{}
	if r.Slot(patient, job.Record{}) || r.On(patient) {
		t.Fatal("no bed, nobody coming: rest ends until the retry")
	}
	if r.PatientRest(patient) {
		t.Fatal("before the retry: no rest")
	}
	h.now = RestRetry
	r.Update(patient)
	if r.RetryAt(patient) != 0 {
		t.Fatal("the retry passes")
	}
}

func TestRestSelfTendInBed(t *testing.T) {
	c, h := scene()
	r := c.Rest
	h.doctors[patient] = true
	h.tend[patient] = true
	h.stacks[herbal] = item.Stack{Kind: item.HerbalMedicine, Qty: 1}
	r.Start(patient, 0)
	j := job.Record{Kind: job.Sleep, Target: bedID, State: job.Do, WorkLeft: needs.SleepWorkMin}
	if r.SelfTendDue(patient, j) {
		t.Fatal("just lay down: not yet")
	}
	j.WorkLeft = needs.SleepWorkMin - selfTendSettle // lain there a while
	h.owners[patient] = doc
	if r.SelfTendDue(patient, j) {
		t.Fatal("held by a doctor: no self-tend")
	}
	delete(h.owners, patient)
	h.doctors[doc] = true
	if r.SelfTendDue(patient, j) {
		t.Fatal("another doctor: no self-tend")
	}
	h.doctors[doc] = false
	if !r.Slot(patient, j) {
		t.Fatal("in bed: stays")
	}
	if got := h.jobs[patient]; got.Kind != job.Tend || got.Target != patient || got.Dest != herbal || h.owners[herbal] != patient {
		t.Fatalf("tends itself where it lies, with medicine: %+v", got)
	}
	if !r.Slot(patient, h.jobs[patient]) {
		t.Fatal("tending itself: goes on")
	}
}

func TestSetBedMedical(t *testing.T) {
	c, h := scene()
	r := c.Rest
	h.pawn(other, 1, 1)
	h.healing[patient] = PainInjury
	h.bed(bedID, construction.Building{Kind: construction.DoubleBed, Claimant: uint64(patient), Claimant2: uint64(other)})
	h.jobs[other] = job.Record{Kind: job.Sleep, Target: bedID, State: job.Do}
	r.SetBedMedical(bedID, true)
	b := h.bldgs[bedID]
	if !b.Medical || b.SlotOf(uint64(patient)) < 0 || b.SlotOf(uint64(other)) >= 0 || r.HeldBed(patient) != bedID ||
		len(h.log) != 1 || h.log[0] != "interrupt" {
		t.Fatalf("the patient keeps its slot, the healthy one gets up: %+v %v", b, h.log)
	}
	r.SetBedMedical(bedID, true) // no change
	r.SetBedMedical(77, true)    // no such bed
	h.bed(bedID+1, construction.Building{Kind: construction.Bed, Claimant: uint64(other)})
	h.prisoners[other] = true
	r.SetBedMedical(bedID+1, true)
	if h.bldgs[bedID+1].Claimant != uint64(other) {
		t.Fatal("a prisoner keeps its bed")
	}
	r.Hold(patient, bedID+1) // a new stay ends the old one
	if h.bldgs[bedID].SlotOf(uint64(patient)) >= 0 {
		t.Fatal("the old stay's slot is freed")
	}
	r.SetBedMedical(bedID, false)
	if h.bldgs[bedID].Medical {
		t.Fatal("off again")
	}
}

func TestRestSaveRoundTrip(t *testing.T) {
	c, _ := scene()
	r := c.Rest
	if r.Export() != nil {
		t.Fatal("nothing to keep: nil")
	}
	r.SetPawnBedRest(doc, false)
	r.Start(patient, 7)
	r.Hold(patient, bedID)
	r.noBed(doc)
	s := r.Export()
	var back Rest
	back.Import(s)
	if since, _ := back.Since(patient); since != 7 || !back.Off(doc) || back.HeldBed(patient) != bedID || back.RetryAt(doc) != RestRetry {
		t.Fatalf("round trip: %+v", s)
	}
	r.Clear(patient)
	if r.On(patient) || r.HeldBed(patient) != 0 {
		t.Fatal("Clear")
	}
	back.Import(nil)
	if back.Export() != nil {
		t.Fatal("nil import: nothing")
	}
}
