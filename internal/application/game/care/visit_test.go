package care

import (
	"slices"
	"testing"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/pawn/recreation"
	"rimworld-go/internal/domain/sim/entity"
)

const (
	visitor1 entity.EntityID = 1
	bedded   entity.EntityID = 2
	visitAt  uint64          = 2400 // the pair (1, 2) has its start slot open here
)

// visitScene: visitor1 at (3, 3) bored (recreation 3000), friend bedded
// lying in a claimed bed at (8, 8) with a wound to tend, awake, friends.
func visitScene() (*Care, *fakeHost, *fakeSocial) {
	h := newFakeHost()
	h.pawn(visitor1, 3, 3)
	h.pawn(bedded, 8, 8)
	b := construction.Building{Kind: construction.Bed, X: 8, Y: 8}.Claim(uint64(bedded))
	h.bed(bedID, b)
	h.tend[bedded] = true
	h.now = visitAt
	s := newFakeSocial(h)
	s.friends(visitor1, bedded, 30)
	s.joy[visitor1], s.joy[bedded] = 3000, 4000
	return New(h, Roles{Social: s}), h, s
}

func TestAssignVisitStartsBesideAFriendInBed(t *testing.T) {
	c, h, _ := visitScene()
	if !c.Visits.Assign(visitor1) {
		t.Fatal("a bored colonist visits a friend lying awake in bed")
	}
	j := h.jobs[visitor1]
	if j.Kind != job.VisitSick || entity.EntityID(j.Count) != bedded || j.TX != 9 || j.TY != 8 || j.WorkLeft != VisitTicks || j.Target != 0 {
		t.Fatalf("visit job %+v: beside the bed, the patient in Count, no reservation of the patient", j)
	}
}

func TestAssignVisitRefusals(t *testing.T) {
	tests := []struct {
		name string
		edit func(h *fakeHost, s *fakeSocial)
	}{
		{"not a friend", func(h *fakeHost, s *fakeSocial) { s.friends(visitor1, bedded, VisitOpinion-1) }},
		{"patient dislikes the visitor", func(h *fakeHost, s *fakeSocial) { s.opinion[[2]entity.EntityID{bedded, visitor1}] = -1 }},
		{"patient asleep", func(h *fakeHost, s *fakeSocial) { s.asleep[bedded] = true }},
		{"visitor entertained", func(h *fakeHost, s *fakeSocial) { s.joy[visitor1] = recreation.Entertained }},
		{"patient not in bed", func(h *fakeHost, s *fakeSocial) { h.Place(bedded, 5, 5) }},
		{"nothing to tend and not downed", func(h *fakeHost, s *fakeSocial) { h.tend[bedded] = false }},
		{"start slot closed", func(h *fakeHost, s *fakeSocial) { h.now = visitAt + visitStart }},
		{"other pair parity", func(h *fakeHost, s *fakeSocial) { h.now = 0 }},
		{"visitor drafted", func(h *fakeHost, s *fakeSocial) { h.drafted[visitor1] = true }},
		{"visitor in a break", func(h *fakeHost, s *fakeSocial) { h.breaking = map[entity.EntityID]bool{visitor1: true} }},
		{"no path", func(h *fakeHost, s *fakeSocial) { h.noPath = true }},
	}
	for _, tc := range tests {
		c, h, s := visitScene()
		tc.edit(h, s)
		if c.Visits.Assign(visitor1) {
			t.Errorf("%s: no visit expected, got %+v", tc.name, h.jobs[visitor1])
		}
	}
	if c := New(newFakeHost(), Roles{}); c.Visits.Assign(visitor1) {
		t.Error("without the Social role nobody visits")
	}
}

// runVisit plays the visitor's job as the game does: beside the bed it
// does the visit once a tick until the job leaves its Do state.
func runVisit(c *Care, h *fakeHost, id entity.EntityID) int {
	h.Place(id, 9, 8)
	j := h.jobs[id]
	j.State = job.Do
	h.jobs[id] = j
	n := 0
	for ; h.jobs[id].State == job.Do && n < 5*VisitTicks; n++ {
		c.Do(id, h.jobs[id])
	}
	return n
}

func TestVisitRestoresRecreationOfBoth(t *testing.T) {
	c, h, s := visitScene()
	c.Visits.Assign(visitor1)
	if n := runVisit(c, h, visitor1); n != VisitTicks {
		t.Fatalf("the visit lasts %d ticks, ran %d", VisitTicks, n)
	}
	if got := s.joy[visitor1] - 3000; got != VisitGain || s.joy[bedded]-4000 != VisitGain {
		t.Fatalf("both gain %d (0.216 of the bar): visitor %d, patient %d", VisitGain, got, s.joy[bedded]-4000)
	}
	if s.chats != 1 || !slices.Contains(h.log, "chat 1+2") || h.jobs[visitor1].State != job.Complete {
		t.Fatalf("one chat at the bedside, job complete: chats %d, log %q, job %+v", s.chats, h.log, h.jobs[visitor1])
	}
}

func TestVisitEndsWhenThePatientFallsAsleep(t *testing.T) {
	c, h, s := visitScene()
	c.Visits.Assign(visitor1)
	h.Place(visitor1, 9, 8)
	j := h.jobs[visitor1]
	j.State = job.Do
	h.jobs[visitor1] = j
	for range 100 {
		c.Do(visitor1, h.jobs[visitor1])
	}
	s.asleep[bedded] = true
	c.Do(visitor1, h.jobs[visitor1])
	if h.jobs[visitor1].State != job.Complete || s.chats != 0 {
		t.Fatalf("the visit ends at once: %+v", h.jobs[visitor1])
	}
	if s.joy[visitor1] <= 3000 || s.joy[visitor1] > 3000+VisitGain/10 {
		t.Fatalf("it kept only the recreation earned so far: %d", s.joy[visitor1])
	}
}

// TestVisitDoesNotFlap lets the pair try to visit again and again for
// five days (the visitor always bored again): visits never run back to
// back, each starts in the pair's slot and waits a window after the last.
func TestVisitDoesNotFlap(t *testing.T) {
	c, h, s := visitScene()
	var starts, ends []uint64
	active := false
	for h.now = 0; h.now < 5*24000; h.now++ {
		s.joy[visitor1] = 3000
		if !active {
			if c.Visits.Assign(visitor1) {
				active = true
				starts = append(starts, h.now)
				h.Place(visitor1, 9, 8)
				j := h.jobs[visitor1]
				j.State = job.Do
				h.jobs[visitor1] = j
			}
			continue
		}
		c.Do(visitor1, h.jobs[visitor1])
		if h.jobs[visitor1].State == job.Complete {
			active = false
			ends = append(ends, h.now)
			h.Place(visitor1, 3, 3)
			delete(h.jobs, visitor1)
		}
	}
	if len(starts) < 5 {
		t.Fatalf("a bored friend keeps visiting: %d visits in five days", len(starts))
	}
	for i, st := range starts {
		if !visitSlot(st, visitor1, bedded) {
			t.Errorf("visit %d started outside the pair's slot at %d", i, st)
		}
		if i > 0 && st-ends[i-1] < visitWindow-VisitTicks {
			t.Errorf("visit %d started %d ticks after the last ended: a cooldown of at least %d", i, st-ends[i-1], visitWindow-VisitTicks)
		}
	}
}
