package care

import (
	"slices"
	"testing"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/sim/command"
)

// wardenScene: warden doc at (0, 0), the colony's prisoner raider downed
// on (5, 5), a prison bed on (8, 8) and a colonist bed on (6, 6).
func wardenScene() (*Care, *fakeHost) {
	c, h := scene()
	h.outsider(raider, 5, 5)
	h.prisoners[raider] = true
	h.vital[raider] = health.Record{Status: health.Downed, Cause: health.CauseInjury, Injury: 30}
	h.bed(bedID, construction.Building{Kind: construction.Bed, X: 6, Y: 6})
	h.bed(bedID+1, construction.Building{Kind: construction.Bed, X: 8, Y: 8, ForPrisoners: true})
	return c, h
}

func TestWardenCarriesADownedPrisonerToBed(t *testing.T) {
	c, h := wardenScene()
	if !c.PrisonBeds.Assign(doc) {
		t.Fatal("a downed prisoner out of bed: the warden goes")
	}
	j := h.jobs[doc]
	if j.Kind != job.CarryToPrisonBed || j.Target != raider || j.Forced || j.TX != 6 || j.TY != 5 {
		t.Fatalf("job beside the prisoner: %+v", j)
	}
	j.State = job.Do
	h.pos[doc] = [2]int{6, 5} // beside the prisoner
	c.PrisonBeds.Do(doc, j)
	j = h.jobs[doc]
	if c.Carry.Patient(doc) != raider || j.Dest != bedID+1 || j.State != job.Goto || h.owners[bedID+1] != doc {
		t.Fatalf("picked up for the prison bed: %+v", j)
	}
	j.State = job.Do
	c.PrisonBeds.Do(doc, j)
	if h.jobs[doc].State != job.Complete || c.Carry.Len() != 0 || h.pos[raider] != [2]int{8, 8} ||
		h.bldgs[bedID+1].SlotOf(uint64(raider)) < 0 || h.sleptAt[raider] != bedID+1 {
		t.Fatalf("laid in the prison bed: job %+v pos %v", h.jobs[doc], h.pos[raider])
	}
	if slices.Contains(h.log, "prisoner") || slices.Contains(h.log, "rescued") || slices.Contains(h.log, "drop gear") {
		t.Fatalf("no capture, no rescue: %v", h.log)
	}
}

func TestWardenCarrySkips(t *testing.T) {
	for name, set := range map[string]func(c *Care, h *fakeHost){
		"standing": func(_ *Care, h *fakeHost) { h.vital[raider] = health.Record{Status: health.Alive} },
		"collapsed asleep": func(_ *Care, h *fakeHost) {
			h.vital[raider] = health.Record{Status: health.Downed, Cause: health.CauseNoBed}
		},
		"no prisoner": func(_ *Care, h *fakeHost) { delete(h.prisoners, raider) },
		"in bed": func(_ *Care, h *fakeHost) {
			h.bldgs[bedID+1] = construction.Building{Kind: construction.Bed, X: 5, Y: 5, ForPrisoners: true, Claimant: uint64(raider)}
		},
		"carried": func(c *Care, _ *fakeHost) { c.Carry.Pick(other, raider) },
		"held":    func(_ *Care, h *fakeHost) { h.owners[raider] = other },
		"no prison bed": func(_ *Care, h *fakeHost) {
			h.bldgs[bedID+1] = construction.Building{Kind: construction.Bed, X: 8, Y: 8}
		},
		"unreachable": func(_ *Care, h *fakeHost) { h.noPath = true },
	} {
		c, h := wardenScene()
		set(c, h)
		if c.PrisonBeds.Assign(doc) {
			t.Errorf("%s: no carry, got %+v", name, h.jobs[doc])
		}
	}
}

func TestWardenCarryNearestPrisoner(t *testing.T) {
	c, h := wardenScene()
	h.outsider(visitor, 1, 1)
	h.prisoners[visitor] = true
	h.vital[visitor] = health.Record{Status: health.Downed, Cause: health.CauseBloodLoss}
	if !c.PrisonBeds.Assign(doc) || h.jobs[doc].Target != visitor {
		t.Fatalf("the nearest downed prisoner first: %+v", h.jobs[doc])
	}
}

func TestWardenCarryEnds(t *testing.T) {
	c, h := wardenScene()
	rec := job.Record{Kind: job.CarryToPrisonBed, Target: raider, State: job.Do}
	c.PrisonBeds.Do(doc, rec)
	if j := h.jobs[doc]; j.State != job.Fail || j.Fail != job.TargetGone {
		t.Fatalf("not beside the prisoner: %+v", j)
	}
	c.Carry.Pick(doc, raider)
	h.vital[raider] = health.Record{Status: health.Alive}
	c.PrisonBeds.Do(doc, rec)
	if j := h.jobs[doc]; j.State != job.Complete || c.Carry.Len() != 0 || h.pos[raider] != h.pos[doc] {
		t.Fatalf("got up: dropped where the warden stands: %+v", j)
	}
	h.vital[raider] = health.Record{Status: health.Downed, Cause: health.CauseInjury}
	h.pos[doc] = [2]int{5, 4}
	c.Carry.Pick(doc, raider)
	h.bldgs[bedID+1] = construction.Building{Kind: construction.Bed, X: 8, Y: 8}
	c.PrisonBeds.Do(doc, job.Record{Kind: job.CarryToPrisonBed, Target: raider, State: job.Do, Dest: bedID + 1})
	if j := h.jobs[doc]; j.State != job.Fail || j.Fail != job.TargetGone || c.Carry.Len() != 0 {
		t.Fatalf("the bed is no prison bed any more: %+v", j)
	}
	c.PrisonBeds.Do(doc, job.Record{Kind: job.CarryToPrisonBed, Target: raider, State: job.Do})
	if j := h.jobs[doc]; j.State != job.Fail || j.Fail != job.Reserved {
		t.Fatalf("no prison bed: %+v", j)
	}
}

func TestTakePrisonerToBedOrder(t *testing.T) {
	c, h := wardenScene()
	h.drafted[doc] = true
	op, ok := optionFor(c.PawnOrders(doc, 5, 5), command.OrderCarryToPrisonBed, raider)
	if !ok || op.Label != "Take N4 to bed" || op.Disabled {
		t.Fatalf("the menu offers it, drafted too: %+v %v", op, labels(c.PawnOrders(doc, 5, 5)))
	}
	if _, ok := optionFor(c.PawnOrders(doc, 5, 5), command.OrderCarryToPrisonBed, patient); ok {
		t.Fatal("a colonist is no prisoner")
	}
	h.owners[raider] = other
	if offer, why := c.PrisonBeds.Judge(doc, raider); !offer || why != ReasonBusyTarget+"N3" {
		t.Fatalf("held by someone: %v %q", offer, why)
	}
	delete(h.owners, raider)
	h.bldgs[bedID+1] = construction.Building{Kind: construction.Bed, X: 8, Y: 8}
	if offer, why := c.PrisonBeds.Judge(doc, raider); !offer || why != ReasonNoPrisonBed {
		t.Fatalf("no prison bed: %v %q", offer, why)
	}
	if why := c.Force(doc, command.OrderCarryToPrisonBed, raider); why != ReasonNoPrisonBed {
		t.Fatalf("Force agrees with the menu: %q", why)
	}
	h.bldgs[bedID+1] = construction.Building{Kind: construction.Bed, X: 8, Y: 8, ForPrisoners: true}
	if why := c.Force(doc, command.OrderCarryToPrisonBed, raider); why != "" {
		t.Fatalf("ordered: %q", why)
	}
	if j := h.jobs[doc]; j.Kind != job.CarryToPrisonBed || !j.Forced || h.owners[raider] != doc || !h.drafted[doc] {
		t.Fatalf("a forced carry holding the prisoner, still drafted: %+v", j)
	}
	h.vital[raider] = health.Record{Status: health.Alive}
	if offer, why := c.PrisonBeds.Judge(doc, raider); offer || why != ReasonNoLonger {
		t.Fatalf("standing: hidden: %v %q", offer, why)
	}
}
