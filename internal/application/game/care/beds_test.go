package care

import (
	"testing"

	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/sim/entity"
)

// starved lays id down from malnutrition: nothing to tend, no bed rest
// due, so no medical bed first.
func starved(h *fakeHost, id entity.EntityID) {
	h.vital[id] = health.Record{Status: health.Downed, Cause: health.CauseNoFood}
}

func TestSafeBedOwnBedBeforeAnyFree(t *testing.T) {
	c, h := scene()
	starved(h, patient)
	h.bed(bedID, construction.Building{Kind: construction.Bed, X: 6, Y: 5})                  // nearest
	h.bed(bedID+1, construction.Building{Kind: construction.Bed, X: 6, Y: 6, Medical: true}) // medical, not needed
	h.bed(bedID+2, construction.Building{Kind: construction.Bed, X: 30, Y: 30, Claimant: uint64(patient)})
	if got := c.SafeBedFor(patient, 5, 5, RescueBed); got != bedID+2 {
		t.Fatalf("no care needed: its own bed, however far: %d", got)
	}
	h.bldgs[bedID+2] = construction.Building{Kind: construction.Bed, X: 30, Y: 30, Claimant: uint64(other)}
	if got := c.SafeBedFor(patient, 5, 5, RescueBed); got != bedID {
		t.Fatalf("no bed of its own: the nearest free one, a medical bed no better: %d", got)
	}
	h.tend[patient] = true
	if got := c.SafeBedFor(patient, 5, 5, RescueBed); got != bedID+1 {
		t.Fatalf("something to tend: the medical bed first: %d", got)
	}
	h.tend[patient] = false
	h.healing[patient] = PainInjury
	if got := c.SafeBedFor(patient, 5, 5, RescueBed); got != bedID+1 {
		t.Fatalf("bed rest due: the medical bed first: %d", got)
	}
}

func TestSafeBedSkipsHazards(t *testing.T) {
	c, h := scene()
	h.downed(patient, 30)
	h.bed(bedID, construction.Building{Kind: construction.Bed, X: 6, Y: 6, Medical: true})
	h.bed(bedID+1, construction.Building{Kind: construction.Bed, X: 20, Y: 20})
	h.burning(bedID)
	if got := c.SafeBedFor(patient, 5, 5, RescueBed); got != bedID+1 {
		t.Fatalf("a burning or deadly bed is skipped: %d", got)
	}
	h.burning(bedID + 1)
	if got := c.SafeBedFor(patient, 5, 5, RescueBed); got != 0 {
		t.Fatalf("no safe bed at all: %d", got)
	}
	if c.Doctor.Rescuable(patient) {
		t.Fatal("no safe bed: not rescuable (tended where it lies)")
	}
}

func TestSafeBedReachableForAColonist(t *testing.T) {
	h := newFakeHost()
	h.pawn(doc, 0, 0)
	h.pawn(patient, 5, 5)
	ch := newCutHost(h)
	c := New(ch, Roles{Prison: newFakePrison(h)})
	h.downed(patient, 30)
	h.bed(bedID, construction.Building{Kind: construction.Bed, X: 6, Y: 6, Medical: true})
	h.bed(bedID+1, construction.Building{Kind: construction.Bed, X: 20, Y: 20})
	ch.cut[cutWay{patient, [2]int{6, 6}}] = true
	if got := c.SafeBedFor(patient, 5, 5, RescueBed); got != bedID+1 {
		t.Fatalf("a bed the patient can't be brought beside is skipped: %d", got)
	}
}

func TestSafeBedForGuestsAndPrisoners(t *testing.T) {
	h := newFakeHost()
	h.pawn(doc, 0, 0)
	h.outsider(visitor, 5, 5)
	h.outsider(raider, 5, 5)
	starved(h, visitor)
	c := New(h, Roles{Prison: newFakePrison(h), Guests: s3Guests{faction: map[entity.EntityID]uint64{visitor: 0}}})
	h.bed(bedID, construction.Building{Kind: construction.Bed, X: 6, Y: 5})
	h.bed(bedID+1, construction.Building{Kind: construction.Bed, X: 30, Y: 30, Medical: true})
	h.bed(bedID+2, construction.Building{Kind: construction.Bed, X: 5, Y: 6, ForPrisoners: true})
	h.bed(bedID+3, construction.Building{Kind: construction.Bed, X: 40, Y: 40, ForPrisoners: true, Medical: true})
	if got := c.SafeBedFor(visitor, 5, 5, RescueBed); got != bedID+1 {
		t.Fatalf("a stranger: a medical bed first, never a prison bed: %d", got)
	}
	if got := PrisonBedFor(c, raider, 5, 5); got != bedID+2 {
		t.Fatalf("a captive needing no care: the nearest prison bed: %d", got)
	}
	h.tend[raider] = true
	if got := PrisonBedFor(c, raider, 5, 5); got != bedID+3 {
		t.Fatalf("a captive to tend: the medical prison bed: %d", got)
	}
	h.prisoners[raider] = true
	if got := c.SafeBedFor(raider, 5, 5, RescueBed); got != bedID+3 {
		t.Fatalf("a prisoner: prison beds only: %d", got)
	}
	h.burning(bedID + 3)
	h.owners[bedID+2] = doc
	if got := PrisonBedFor(c, raider, 5, 5); got != 0 {
		t.Fatalf("burning, or held for someone else: none: %d", got)
	}
}
