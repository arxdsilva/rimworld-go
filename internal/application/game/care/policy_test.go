package care

import (
	"testing"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/pawn/medcare"
)

func TestPolicyLevels(t *testing.T) {
	c, h := scene()
	dog := other
	h.animals[dog] = true
	if c.Level(doc) != medcare.Default(medcare.Colonists) || c.Level(dog) != medcare.Default(medcare.Animals) {
		t.Fatal("group defaults")
	}
	c.SetMedicalCare(doc, 99)                   // invalid level
	c.SetMedicalCare(77, uint8(medcare.NoCare)) // not a patient
	if _, own := c.Policy.Own(doc); own || c.Policy.Export() != nil {
		t.Fatal("bad SetMedicalCare changed something")
	}
	c.SetMedicalCare(dog, uint8(medcare.Best))
	c.Policy.SetDefault(int(medcare.Colonists), uint8(medcare.NoMeds))
	c.Policy.SetDefault(9, 1)
	c.Policy.SetDefault(0, 99)
	if c.Level(dog) != medcare.Best || c.Level(doc) != medcare.NoMeds || c.Policy.Defaults()[medcare.Colonists] != uint8(medcare.NoMeds) {
		t.Fatal("own level and a custom default")
	}
	c.Policy.SetDefault(int(medcare.Colonists), uint8(medcare.Default(medcare.Colonists)))
	if s := c.Policy.Export(); s == nil || s.Def != nil {
		t.Fatal("back at the defaults: no custom defaults saved")
	}
	c.SetSelfTend(77, false) // not a colonist
	c.SetSelfTend(doc, false)
	if c.Policy.SelfTendOff(77) || !c.Policy.SelfTendOff(doc) || c.SelfTendAllowed(doc) {
		t.Fatal("self-tend: colonists only")
	}
}

func TestPolicySaveRoundTrip(t *testing.T) {
	var p Policy
	if p.Export() != nil {
		t.Fatal("all defaults: nil save")
	}
	p.Set(9, uint8(medcare.NoMeds))
	p.Set(3, uint8(medcare.Best))
	p.SetDefault(int(medcare.Animals), uint8(medcare.NoCare))
	p.SetSelfTend(7, false)
	p.SetSelfTend(5, false)
	s := p.Export()
	if len(s.Per) != 2 || s.Per[0].ID != 3 || s.NoSelfTend[0] != 5 || len(s.Def) != int(medcare.NumGroups) {
		t.Fatalf("export in id order: %+v", s)
	}
	var q Policy
	q.Import(s)
	if q.Defaults() != p.Defaults() || !q.SelfTendOff(7) {
		t.Fatal("round trip")
	}
	if l, _ := q.Own(9); l != medcare.NoMeds {
		t.Fatal("own level lost")
	}
	q.Import(&PolicySave{Per: []PolicyEntry{{ID: 4, V: 99}}, Def: []uint8{99, 0, 0, 0}})
	if _, own := q.Own(4); own || q.Default(medcare.Colonists) != medcare.Default(medcare.Colonists) {
		t.Fatal("out-of-range saved levels fall back to the defaults")
	}
	q.Import(nil)
	if q.Export() != nil {
		t.Fatal("nil import: all defaults")
	}
}

func TestEmergencyTend(t *testing.T) {
	c, h := scene()
	h.downed(patient, 30)
	d := c.Doctor
	if got := d.CanEmergencyTend(doc, patient); got != ReasonIncapable {
		t.Fatalf("not drafted: %q", got)
	}
	h.drafted[doc] = true
	h.tend[patient] = false
	if got := d.CanEmergencyTend(doc, patient); got != ReasonNothingToTend {
		t.Fatalf("nothing to tend: %q", got)
	}
	h.tend[doc] = true
	c.SetSelfTend(doc, false)
	if got := d.CanEmergencyTend(doc, doc); got != ReasonNoSelfTend {
		t.Fatalf("self-tend off: %q", got)
	}
	h.tend[patient] = true
	h.owners[patient] = other
	if got := d.CanEmergencyTend(doc, patient); got != ReasonBusyTarget+h.Name(other) {
		t.Fatalf("held: %q", got)
	}
	delete(h.owners, patient)
	h.noPath = true
	if got := d.CanEmergencyTend(doc, patient); got != ReasonUnreachable {
		t.Fatalf("no way: %q", got)
	}
	d.EmergencyTend(doc, patient) // refused: nothing happens
	if _, ok := h.jobs[doc]; ok {
		t.Fatal("a refused emergency tend started a job")
	}
	h.noPath = false
	h.stacks[herbal] = item.Stack{Kind: item.HerbalMedicine, Qty: 1, X: 1, Y: 1}
	h.stacks[herbal+1] = item.Stack{Kind: item.IndustrialMedicine, Qty: 1, X: 40, Y: 40} // out of reach
	d.EmergencyTend(doc, patient)
	j := h.jobs[doc]
	if j.Kind != job.EmergencyTend || !j.Forced || j.Dest != herbal || j.WorkLeft != TendWork || h.owners[herbal] != doc ||
		len(h.log) < 1 || h.log[0] != "interrupt" { // the queue is the game's (tick.go): a plain order replaced it already
		t.Fatalf("emergency tend: %+v log %v", j, h.log)
	}
	h.drafted[doc] = false
	d.DoEmergencyTend(doc, j)
	if h.jobs[doc].State != job.Complete {
		t.Fatal("undrafted: the emergency tend ends")
	}
	h.drafted[doc] = true
	h.pos[doc] = [2]int{6, 5}
	j.Dest, j.State = 0, job.Do
	d.DoEmergencyTend(doc, j)
	if h.jobs[doc].State != job.Complete || h.tended[patient] != 400 {
		t.Fatalf("drafted: tends like any tend (no dose: q 400): %+v", h.jobs[doc])
	}
}

func TestCareHelpers(t *testing.T) {
	c, h := scene()
	d := c.Doctor
	h.downed(patient, 30)
	if !d.InjuredDowned(patient) || d.InjuredDowned(doc) || !d.Lying(patient) || d.Lying(doc) {
		t.Fatal("downed patient lies; standing doctor does not")
	}
	h.jobs[doc] = job.Record{Kind: job.Sleep, State: job.Do}
	if !d.Lying(doc) {
		t.Fatal("asleep: lying")
	}
	h.vital[patient] = health.Record{Status: health.Downed, Cause: health.CauseHypothermia}
	if !d.InjuredDowned(patient) {
		t.Fatal("down from the cold counts")
	}
	d.MarkTended(patient, 50)
	m := d.ExportTended()
	d.Forget(patient)
	if d.TendedUntil(patient) != 0 || m[patient] != 50 {
		t.Fatal("Forget / Export copy")
	}
	d.ImportTended(nil)
	d.ImportTended(m)
	if d.TendedUntil(patient) != 50 {
		t.Fatal("ImportTended")
	}
	c.DropPatient(doc) // carries nothing: no-op
	c.Carry.Pick(doc, patient)
	c.DropPatient(doc)
	if h.pos[patient] != h.pos[doc] || c.Carry.Len() != 0 {
		t.Fatal("dropped where the carrier stands")
	}
}

// raider 4 is no prisoner

func TestSafeBedForRest(t *testing.T) {
	c, h := scene()
	h.bed(bedID, construction.Building{Kind: construction.Bed, X: 5, Y: 9, Medical: true})   // Chebyshev 4
	h.bed(bedID+1, construction.Building{Kind: construction.Bed, X: 8, Y: 8, Medical: true}) // Chebyshev 3
	h.bed(bedID+2, construction.Building{Kind: construction.HospitalBed, X: 5, Y: 6})        // not medical
	if got := c.SafeBedFor(patient, 5, 5, RestBed); got != bedID+1 {
		t.Fatalf("the nearest medical bed by tile distance: %d", got)
	}
	h.bldgs[bedID] = construction.Building{Kind: construction.Bed, X: 5, Y: 9, Medical: true, Claimant: uint64(patient)}
	if got := c.SafeBedFor(patient, 5, 5, RestBed); got != bedID {
		t.Fatalf("one it holds first: %d", got)
	}
	if got := c.SafeBedFor(patient, 5, 5, RescueBed); got != bedID {
		t.Fatalf("rescue: medical first, nearest by Manhattan (its own claim counts as free): %d", got)
	}
}
