package care

import (
	"slices"
	"testing"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/sim/entity"
)

const (
	captive entity.EntityID = 4
	cellBed entity.EntityID = 11
)

// prisonScene: colonists doc (0, 0) and patient (5, 5), a downed raider
// at (3, 0), a plain bed at (1, 1) and a prisoner bed at (8, 8).
func prisonScene() (*Care, *fakeHost, *fakePrison) {
	h := newFakeHost()
	h.pawn(doc, 0, 0)
	h.pawn(patient, 5, 5)
	h.raiders = append(h.raiders, captive)
	h.pos[captive] = [2]int{3, 0}
	h.vital[captive] = health.Record{Status: health.Downed, Cause: health.CauseInjury}
	h.bed(bedID, construction.Building{Kind: construction.Bed, X: 1, Y: 1})
	h.bed(cellBed, construction.Building{Kind: construction.Bed, X: 8, Y: 8, ForPrisoners: true})
	pr := newFakePrison(h)
	pr.raiders[captive] = true
	pr.armed[captive] = true
	return New(h, Roles{Prison: pr}), h, pr
}

func TestCaptureCarriesToThePrisonBed(t *testing.T) {
	c, h, pr := prisonScene()
	if why := c.StartCapture(0, captive, true); why != "" {
		t.Fatalf("StartCapture: %q", why)
	}
	j := h.jobs[doc]
	if j.Kind != job.Capture || j.Target != captive || !j.Forced || j.TX != 4 || j.TY != 0 || h.log[0] != "interrupt" {
		t.Fatalf("the nearest colonist sets off, forced: %+v log %v", j, h.log)
	}
	h.owners[captive] = doc // the game's StartJob reserves the target
	j.State = job.Do        // beside the captive
	h.pos[doc] = [2]int{4, 0}
	c.DoCapture(doc, j)
	j = h.jobs[doc]
	if c.Carry.Patient(doc) != captive || j.Dest != cellBed || j.State != job.Goto || pr.armed[captive] {
		t.Fatalf("picked up, disarmed, for the prisoner bed: carry %d job %+v", c.Carry.Patient(doc), j)
	}
	j.State = job.Do
	c.DoCapture(doc, j)
	want := []string{"interrupt", "drop gear", "prisoner", "release"}
	if !slices.Equal(h.log, want) {
		t.Fatalf("log %v, want %v", h.log, want)
	}
	if h.jobs[doc].State != job.Complete || c.Carry.Len() != 0 || !h.prisoners[captive] ||
		h.sleptAt[captive] != cellBed || h.pos[captive] != [2]int{8, 8} || h.bldgs[cellBed].SlotOf(uint64(captive)) < 0 {
		t.Fatalf("laid in the prisoner bed: job %+v pos %v", h.jobs[doc], h.pos[captive])
	}
	// a prisoner bed that is medical is still not held for a stay
	if c.Rest.HeldBed(captive) != 0 {
		t.Fatal("a captive holds no medical bed")
	}
}

func TestStartCaptureRefuses(t *testing.T) {
	c, h, _ := prisonScene()
	if why := c.StartCapture(doc, patient, false); why != ReasonIncapable {
		t.Fatalf("a standing colonist is not capturable: %q", why)
	}
	h.owners[captive] = patient
	h.jobs[patient] = job.Record{Kind: job.Capture, Target: captive, State: job.Goto}
	if why := c.StartCapture(doc, captive, false); why != ReasonBusyTarget+"N2" {
		t.Fatalf("already held: %q", why)
	}
	delete(h.owners, captive)
	h.breaking = map[entity.EntityID]bool{doc: true}
	if why := c.StartCapture(doc, captive, false); why != ReasonIncapable {
		t.Fatalf("a breaking colonist takes no prisoner: %q", why)
	}
	if why := c.StartArrest(doc, doc, false); why != ReasonSelf {
		t.Fatalf("self: %q", why)
	}
	h.breaking = nil
	h.noPath = true
	if why := c.StartCapture(doc, captive, false); why != ReasonUnreachable {
		t.Fatalf("no way: %q", why)
	}
	h.noPath = false
	h.blocked[doc] = true // StartJob refuses
	if why := c.StartCapture(doc, captive, false); why != ReasonIncapable {
		t.Fatalf("job refused: %q", why)
	}
	h.blocked[doc] = false
	h.owners[cellBed] = patient
	h.log = nil
	if why := c.StartCapture(doc, captive, false); why != ReasonNoPrisonBed ||
		!slices.Equal(h.log, []string{"message: No free prisoner bed for N4."}) {
		t.Fatalf("no prisoner bed: %q %v", why, h.log)
	}
	if _, busy := h.jobs[doc]; busy {
		t.Fatal("a refused capture starts nothing")
	}
}

func TestHandlerIsTheNearestAbleColonist(t *testing.T) {
	c, h, _ := prisonScene()
	h.pawn(5, 3, 3) // as near as doc (3 tiles): the lower id wins
	h.pawn(6, 3, 1) // nearest
	h.vital[6] = health.Record{Status: health.Downed}
	if by, ok := c.handler(0, captive); !ok || by != doc {
		t.Fatalf("tie to the lower id, never a downed one: %d %v", by, ok)
	}
	h.breaking = map[entity.EntityID]bool{doc: true}
	if by, ok := c.handler(0, captive); !ok || by != 5 {
		t.Fatalf("a breaking colonist is skipped: %d %v", by, ok)
	}
	if _, ok := c.handler(captive, captive); ok {
		t.Fatal("a raider handles nothing")
	}
	h.breaking[5] = true
	h.breaking[patient] = true
	if _, ok := c.handler(0, captive); ok {
		t.Fatal("no able colonist")
	}
}

func TestCaptureEnds(t *testing.T) {
	c, h, pr := prisonScene()
	rec := job.Record{Kind: job.Capture, Target: captive, State: job.Do}

	c.DoCapture(doc, rec) // three tiles off, not carried
	if j := h.jobs[doc]; j.State != job.Fail || j.Fail != job.TargetGone {
		t.Fatalf("walked off: %+v", j)
	}
	h.pos[doc] = [2]int{2, 0}
	h.owners[cellBed] = patient
	c.DoCapture(doc, rec)
	if j := h.jobs[doc]; j.State != job.Fail || j.Fail != job.Reserved || h.log[len(h.log)-1] != "message: No free prisoner bed for N4." {
		t.Fatalf("bed held: %+v %v", j, h.log)
	}
	delete(h.owners, cellBed)
	h.noPath = true
	c.DoCapture(doc, rec)
	if j := h.jobs[doc]; j.State != job.Fail || j.Fail != job.Unreachable || c.Carry.Len() != 0 || !pr.armed[captive] {
		t.Fatalf("no way: %+v", j)
	}
	h.noPath = false
	c.Carry.Pick(doc, captive)
	b := h.bldgs[cellBed]
	b.ForPrisoners = false
	h.bldgs[cellBed] = b
	c.DoCapture(doc, job.Record{Kind: job.Capture, Target: captive, State: job.Do, Dest: cellBed})
	if j := h.jobs[doc]; j.State != job.Fail || j.Fail != job.TargetGone || c.Carry.Len() != 0 || h.prisoners[captive] {
		t.Fatalf("no longer a prisoner bed: %+v", j)
	}
	b.ForPrisoners = true
	h.bldgs[cellBed] = b
	h.prisoners[captive] = true // already a prisoner (an arrest): not made one again
	h.log = nil
	c.Carry.Pick(doc, captive)
	c.DoCapture(doc, job.Record{Kind: job.Capture, Target: captive, State: job.Do, Dest: cellBed})
	if !slices.Equal(h.log, []string{"release"}) || h.jobs[doc].State != job.Complete {
		t.Fatalf("an arrested prisoner is laid down: %v %+v", h.log, h.jobs[doc])
	}
	h.prisoners[captive] = false
	h.vital[captive] = health.Record{Status: health.Dead}
	c.Carry.Pick(doc, captive)
	c.DoCapture(doc, rec)
	if j := h.jobs[doc]; j.State != job.Complete || c.Carry.Len() != 0 || h.pos[captive] != h.pos[doc] {
		t.Fatalf("died: dropped where the captor stands: %+v", j)
	}
}

func TestArrestableAndBeingCaptured(t *testing.T) {
	c, h, pr := prisonScene()
	pr.visitors[7] = true
	h.vital[7] = health.Record{Status: health.Alive}
	switch {
	case !c.Arrestable(patient), !c.Arrestable(7):
		t.Fatal("a colonist and a visitor may be arrested")
	case c.Arrestable(captive), c.Arrestable(99):
		t.Fatal("a raider or nobody may not")
	}
	h.vital[patient] = health.Record{Status: health.Dead}
	h.prisoners[7] = true
	if c.Arrestable(patient) || c.Arrestable(7) {
		t.Fatal("not the dead, not a prisoner")
	}
	if why := c.StartArrest(doc, captive, false); why != ReasonIncapable {
		t.Fatalf("arresting a raider: %q", why)
	}
	if c.BeingCaptured(captive) {
		t.Fatal("nobody holds it")
	}
	h.owners[captive] = doc
	h.jobs[doc] = job.Record{Kind: job.Rescue, Target: captive, State: job.Goto}
	if c.BeingCaptured(captive) {
		t.Fatal("a rescue is no capture")
	}
	h.jobs[doc] = job.Record{Kind: job.Arrest, Target: captive, State: job.Goto}
	if !c.BeingCaptured(captive) {
		t.Fatal("an arrester holds it")
	}
}

func TestArrestChasesRollsAndCarriesOff(t *testing.T) {
	c, h, pr := prisonScene()
	if why := c.StartArrest(doc, patient, true); why != "" {
		t.Fatalf("StartArrest: %q", why)
	}
	j := h.jobs[doc]
	if j.Kind != job.Arrest || !j.Forced {
		t.Fatalf("arrest job %+v", j)
	}
	j.State = job.Do
	c.DoArrest(doc, j) // patient at (5, 5), five tiles off
	j = h.jobs[doc]
	if j.State != job.Goto || j.Count != 1 || j.TX != 6 || j.TY != 5 || len(j.Path) != 1 {
		t.Fatalf("chases: %+v", j)
	}
	j.Count = arrestChase
	c.DoArrest(doc, j)
	if j := h.jobs[doc]; j.State != job.Fail || j.Fail != job.TargetGone {
		t.Fatalf("gives up after the chase: %+v", j)
	}
	h.noPath = true
	c.DoArrest(doc, job.Record{Kind: job.Arrest, Target: patient, State: job.Do})
	if j := h.jobs[doc]; j.State != job.Fail || j.Fail != job.TargetGone {
		t.Fatalf("no way: %+v", j)
	}
	h.noPath = false
	h.pos[doc] = [2]int{6, 5}
	c.DoArrest(doc, job.Record{Kind: job.Arrest, Target: doc + 50, State: job.Do})
	if j := h.jobs[doc]; j.State != job.Complete {
		t.Fatalf("nobody to arrest: %+v", j)
	}
	h.log = nil
	c.DoArrest(doc, job.Record{Kind: job.Arrest, Target: patient, State: job.Do})
	if want := []string{"message: N2 resisted arrest by N1.", "fight 2>1"}; !slices.Equal(h.log, want) {
		t.Fatalf("resisted: log %v", h.log)
	}
	if j := h.jobs[doc]; j.State != job.Fail || j.Fail != job.Interrupted || h.prisoners[patient] {
		t.Fatalf("resisted: %+v", j)
	}
	pr.giveIn = true
	h.log = nil
	h.jobs[patient] = job.Record{Kind: job.Wander, State: job.Goto}
	c.DoArrest(doc, job.Record{Kind: job.Arrest, Target: patient, State: job.Do, Forced: true})
	if want := []string{"interrupt", "arrested", "drop gear"}; !slices.Equal(h.log, want) {
		t.Fatalf("arrested: log %v", h.log)
	}
	j = h.jobs[doc]
	if _, busy := h.jobs[patient]; busy || !h.prisoners[patient] || j.Kind != job.Capture || !j.Forced ||
		j.Dest != cellBed || j.State != job.Goto || c.Carry.Patient(doc) != patient {
		t.Fatalf("carried off as a capture: %+v", j)
	}
}
