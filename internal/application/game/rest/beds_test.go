package rest

import (
	"testing"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/pawn/health"
)

func TestSleepInPicksOwnBedThenNearest(t *testing.T) {
	f := newFake()
	s := New(f)
	f.pawn(1, 0, 0)
	f.bed(10, construction.Bed, 9, 0)
	f.bed(11, construction.Bed, 2, 0)
	if !s.Beds.SleepIn(1) {
		t.Fatal("no bed")
	}
	if f.slept[1] != 11 || f.bldgs[11].Claimant != 1 || f.jobs[1].Kind != job.Sleep || f.jobs[1].Target != 11 {
		t.Fatalf("nearest bed: slept %v job %+v", f.slept[1], f.jobs[1])
	}
	f.pos[1] = [2]int{9, 0}
	s.Beds.SleepIn(1) // keeps its own bed though another is nearer
	if f.slept[1] != 11 || f.bldgs[10].Claimant != 0 {
		t.Fatalf("own bed first: slept at %v", f.slept[1])
	}
}

func TestSleepInSkipsForbiddenBeds(t *testing.T) {
	f := newFake()
	s := New(f)
	f.pawn(1, 0, 0)
	f.bed(10, construction.Bed, 1, 0)
	b := f.bldgs[10]
	b.ForPrisoners = true
	f.bldgs[10] = b
	f.bed(11, construction.HospitalBed, 2, 0)
	b = f.bldgs[11]
	b.Medical = true
	f.bldgs[11] = b
	f.bed(12, construction.Bed, 3, 0)
	f.outside[[2]int{3, 0}] = true
	if s.Beds.SleepIn(1) {
		t.Fatalf("took a prison, medical or out-of-area bed: %+v", f.jobs[1])
	}
}

func TestSleepInOutOfReachReleases(t *testing.T) {
	f := newFake()
	s := New(f)
	f.pawn(1, 0, 0)
	f.bed(10, construction.Bed, 1, 0)
	f.noStart[1] = true
	if s.Beds.SleepIn(1) || f.bldgs[10].Claimant != 0 {
		t.Fatal("an unreachable bed stays held")
	}
}

func TestSleepInMovingGivesUpOldBed(t *testing.T) {
	f := newFake()
	s := New(f)
	f.pawn(1, 0, 0)
	f.bed(10, construction.Bed, 1, 0)
	f.bed(11, construction.Bed, 5, 0)
	b := f.bldgs[10]
	b.Claimant = 1
	f.bldgs[10] = b
	f.outside[[2]int{1, 0}] = true // its bed now lies outside its area
	if !s.Beds.SleepIn(1) || f.bldgs[10].Claimant != 0 || f.bldgs[11].Claimant != 1 {
		t.Fatalf("move: %+v %+v", f.bldgs[10], f.bldgs[11])
	}
}

func TestPartnerBed(t *testing.T) {
	f := newFake()
	s := New(f)
	f.pawn(1, 0, 0)
	f.pawn(2, 0, 0)
	f.partner[1], f.partner[2] = 2, 1
	f.bed(10, construction.Bed, 1, 0)
	f.bed(20, construction.DoubleBed, 6, 0)
	f.bed(21, construction.DoubleBed, 3, 0)
	b := f.bldgs[10]
	b.Claimant = 1
	f.bldgs[10] = b
	// a free double bed (nearest) for the two; its single bed is let go
	if !s.Beds.SleepIn(1) || f.slept[1] != 21 || f.bldgs[21].Claimant != 1 || f.bldgs[10].Claimant != 0 {
		t.Fatalf("free double: slept %v %+v", f.slept[1], f.bldgs[21])
	}
	// the partner joins the free slot of the double bed it holds
	if !s.Beds.SleepIn(2) || f.slept[2] != 21 || f.bldgs[21].Claimant2 != 2 {
		t.Fatalf("join: slept %v %+v", f.slept[2], f.bldgs[21])
	}
	// next night each keeps its slot
	s.Beds.SleepIn(1)
	s.Beds.SleepIn(2)
	if b := f.bldgs[21]; b.Claimant != 1 || b.Claimant2 != 2 {
		t.Fatalf("couple's bed flipped: %+v", b)
	}
}

func TestPartnerBedOwnDoubleAndRollback(t *testing.T) {
	f := newFake()
	s := New(f)
	f.pawn(1, 0, 0)
	f.partner[1] = 2
	f.bed(20, construction.DoubleBed, 6, 0)
	b := f.bldgs[20]
	b.Claimant2 = 1
	f.bldgs[20] = b
	if bid, slot, ok := s.Beds.partnerBed(1); !ok || bid != 20 || slot != 1 {
		t.Fatalf("own double: %v %v %v", bid, slot, ok)
	}
	// out of reach: the bed and the sleep record go back as they were
	f.noStart[1] = true
	if s.Beds.sleepInPartnerBed(1, 20, 0) {
		t.Fatal("started")
	}
	if _, ok := f.slept[1]; ok || f.bldgs[20].Claimant2 != 1 || f.bldgs[20].Claimant != 0 {
		t.Fatalf("not rolled back: %+v slept %v", f.bldgs[20], f.slept)
	}
	f.slept[1] = 7
	s.Beds.sleepInPartnerBed(1, 20, 0)
	if f.slept[1] != 7 {
		t.Fatal("old sleep record lost")
	}
	// bad slots and a held slot leave everything alone
	if s.Beds.sleepInPartnerBed(1, 20, 2) || s.Beds.sleepInPartnerBed(1, 99, 0) {
		t.Fatal("bad slot or bed accepted")
	}
	b = f.bldgs[20]
	b.Claimant = 5
	f.bldgs[20] = b
	if s.Beds.sleepInPartnerBed(1, 20, 0) {
		t.Fatal("took a held slot")
	}
}

func TestPartnerBedNone(t *testing.T) {
	f := newFake()
	s := New(f)
	f.pawn(1, 0, 0)
	if _, _, ok := s.Beds.partnerBed(1); ok {
		t.Fatal("no partner, yet a partner bed")
	}
	f.partner[1] = 2
	f.bed(20, construction.DoubleBed, 6, 0)
	b := f.bldgs[20]
	b.Claimant, b.Claimant2 = 2, 3 // partner's bed is full
	f.bldgs[20] = b
	if _, _, ok := s.Beds.partnerBed(1); ok {
		t.Fatal("a full partner bed")
	}
}

func TestAssignOwner(t *testing.T) {
	f := newFake()
	s := New(f)
	f.pawn(1, 0, 0)
	f.pawn(2, 0, 0)
	f.bed(10, construction.Bed, 1, 0)
	f.bed(20, construction.DoubleBed, 5, 0)
	s.Beds.AssignOwner(10, 1, 0)
	s.Beds.AssignOwner(20, 1, 0) // gives up the single bed
	if f.bldgs[10].Claimant != 0 || f.bldgs[20].Claimant != 1 {
		t.Fatalf("owner move: %+v %+v", f.bldgs[10], f.bldgs[20])
	}
	s.Beds.AssignOwner(20, 2, 0) // takes 1's slot
	if f.bldgs[20].Claimant != 2 {
		t.Fatal("slot not taken")
	}
	s.Beds.AssignOwner(20, 2, 0) // already its slot: nothing changes
	s.Beds.AssignOwner(20, 0, 0) // nobody
	if f.bldgs[20].Claimant != 0 {
		t.Fatal("slot not freed")
	}
	s.Beds.AssignOwner(20, 1, 5) // no such slot
	s.Beds.AssignOwner(10, 1, 1) // a single bed has one slot
	s.Beds.AssignOwner(99, 1, 0) // no bed
	f.vital[2] = health.Record{Status: health.Dead}
	s.Beds.AssignOwner(10, 2, 0) // the dead own nothing
	b := f.bldgs[10]
	b.Medical = true
	f.bldgs[10] = b
	s.Beds.AssignOwner(10, 1, 0) // a medical bed has no owner
	if f.bldgs[10].Claimant != 0 || f.bldgs[20].Claimant != 0 {
		t.Fatalf("invalid assignment took: %+v %+v", f.bldgs[10], f.bldgs[20])
	}
}
