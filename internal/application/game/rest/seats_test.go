// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package rest

import (
	"slices"
	"testing"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/pawn/mood"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/pawn/schedule"
	"rimworld-go/internal/domain/sim/entity"
)

func TestSeatKindsAndComfort(t *testing.T) {
	f := newFake()
	s := New(f)
	f.bed(1, construction.Chair, 0, 0)
	f.bed(2, construction.Stool, 1, 0)
	f.bed(3, construction.Armchair, 2, 0)
	f.bed(4, construction.Table, 3, 0)
	for _, c := range []struct {
		x, comfort int
		ok         bool
	}{{0, 700, true}, {1, 500, true}, {2, 800, true}, {3, 0, false}, {9, 0, false}} {
		if got, ok := s.Seats.ComfortAt(c.x, 0); got != c.comfort || ok != c.ok {
			t.Errorf("ComfortAt(%d) = %d %v", c.x, got, ok)
		}
	}
	if seatBonus(700) != 6 || seatBonus(500) != 4 || seatBonus(800) != 7 || seatBonus(0) != 0 {
		t.Error("seat bonus")
	}
}

func TestAtTablePrefersSeats(t *testing.T) {
	f := newFake()
	s := New(f)
	f.bed(1, construction.Table, 10, 10) // A2: the 1x2 table, (10, 10) and (10, 11)
	f.bldgIDs = append(f.bldgIDs, 99)    // a building gone from the store
	// a table without a free chair does not offer a dining seat.
	// A stool then an armchair introduce two usable spots.
	f.pawn(9, 10, 14)
	if _, _, ok := s.Seats.AtTable(9, 10, 14); ok {
		t.Fatal("table without a chair is not a reservable sitting surface")
	}
	f.bed(2, construction.Stool, 11, 11)
	if x, y, _ := s.Seats.AtTable(9, 10, 14); x != 11 || y != 11 {
		t.Fatalf("stool: %d,%d", x, y)
	}
	f.bed(3, construction.Armchair, 9, 11)
	if x, y, _ := s.Seats.AtTable(9, 10, 14); x != 9 || y != 11 {
		t.Fatalf("armchair: %d,%d", x, y)
	}
	// taken, unreachable or cluttered cells are passed over
	f.taken[[2]int{9, 11}] = 7
	f.unreachable[[2]int{11, 11}] = true
	f.bed(4, construction.EndTable, 10, 12)
	if _, _, ok := s.Seats.AtTable(9, 10, 14); ok {
		t.Fatal("all dining chairs blocked; cannot reserve bare table-side floor")
	}
	if _, _, ok := s.Seats.AtTable(9, 100, 100); ok {
		t.Fatal("a table out of reach")
	}
}

// TestBigTableSeatsAndCouch (A2): a 2x2 table offers a seat along each
// side cell (8, wiki: Table), and a couch is two seats: a sitter takes the
// nearer cell, a second the other.
func TestBigTableSeatsAndCouch(t *testing.T) {
	f := newFake()
	s := New(f)
	f.bed(1, construction.Table2x2, 10, 10)
	n := 0
	for _, c := range construction.SeatCells(construction.Table2x2, construction.South, 10, 10) {
		f.taken[c] = 50 // every seat taken
		n++
	}
	if n != 8 {
		t.Fatalf("%d seats round a 2x2 table, want 8", n)
	}
	if _, _, ok := s.Seats.AtTable(9, 10, 14); ok {
		t.Fatal("a full table still seats a diner")
	}
	f.pawn(2, 0, 0)
	s.Needs.m[2] = NeedRecord{Comfort: 20 * needs.LevelPct}
	f.bed(3, construction.Couch, 2, 0) // (2, 0) and (3, 0)
	if c, ok := s.Seats.ComfortAt(3, 0); !ok || c != 850 {
		t.Fatalf("couch comfort %d %v, want 850 (wiki: Couch)", c, ok)
	}
	if x, y, ok := s.Seats.best(2); !ok || x != 2 || y != 0 {
		t.Fatalf("first sitter: %d,%d %v", x, y, ok)
	}
	f.taken[[2]int{2, 0}] = 9
	if x, y, ok := s.Seats.best(2); !ok || x != 3 || y != 0 {
		t.Fatalf("second sitter: %d,%d %v, want the couch's other cell", x, y, ok)
	}
}

func TestSitDownPicksTheBestSeat(t *testing.T) {
	f := newFake()
	s := New(f)
	f.pawn(1, 0, 0)
	s.Needs.m[1] = NeedRecord{Comfort: 20 * needs.LevelPct}
	f.bed(2, construction.Stool, 2, 0)
	f.bed(3, construction.Armchair, 4, 0) // two steps farther, worth three
	f.bed(4, construction.Chair, 1, 0)
	b := f.bldgs[4]
	b.Forbidden = true
	f.bldgs[4] = b
	f.bed(5, construction.Chair, 40, 0) // too far
	if !s.Seats.SitDown(1, 100) {
		t.Fatal("no seat")
	}
	if j := f.jobs[1]; j.Kind != job.SitDown || j.TX != 4 || j.WorkLeft != SitTicks || f.reserved[[2]int{4, 0}] != 1 {
		t.Fatalf("job %+v reserved %v", j, f.reserved)
	}
	// comfortable enough, or in work hours: no
	s.Needs.m[1] = NeedRecord{Comfort: SeekComfort}
	if s.Seats.SitDown(1, 100) {
		t.Fatal("sat at the seek line")
	}
	s.Needs.m[1] = NeedRecord{Comfort: 0}
	f.act = schedule.Work
	if s.Seats.SitDown(1, 100) {
		t.Fatal("sat in work hours")
	}
}

func TestSitDownRetriesAfterAWhile(t *testing.T) {
	f := newFake()
	s := New(f)
	f.pawn(1, 0, 0)
	s.Needs.m[1] = NeedRecord{Comfort: 0}
	f.bed(2, construction.Stool, 3, 0)
	f.unreachable[[2]int{3, 0}] = true
	if s.Seats.SitDown(1, 100) || s.Needs.Of(1).SeatAt != 100+SeatRetry {
		t.Fatalf("unreachable seat: %+v", s.Needs.Of(1))
	}
	delete(f.unreachable, [2]int{3, 0})
	if s.Seats.SitDown(1, 100+SeatRetry-1) {
		t.Fatal("looked again before the retry")
	}
	f.noStart[1] = true
	if s.Seats.SitDown(1, 100+SeatRetry) || s.Needs.Of(1).SeatAt != 100+2*SeatRetry {
		t.Fatal("a job that would not start")
	}
	delete(f.noStart, 1)
	f.outside[[2]int{3, 0}] = true
	if s.Seats.SitDown(1, 100+2*SeatRetry) {
		t.Fatal("a seat outside the area")
	}
}

func TestSittingEnds(t *testing.T) {
	f := newFake()
	s := New(f)
	f.pawn(1, 2, 2)
	f.bed(20, construction.Stool, 2, 2)
	// comfort reaches the stool's 50 % and the sit ends
	s.Needs.m[1] = NeedRecord{Comfort: 40 * needs.LevelPct, ComfortStage: -1, BeautyStage: -1, OutdoorsStage: -1}
	f.jobs[1] = job.Record{Kind: job.SitDown, State: job.Do, TX: 2, TY: 2, WorkLeft: SitTicks}
	beats(s.Needs, 1, 0, 2)
	if f.jobs[1].State != job.Do {
		t.Fatalf("ended early: %+v", f.jobs[1])
	}
	beats(s.Needs, 1, 2*needs.StepTicks, 1)
	if f.jobs[1].State != job.Complete || s.Needs.Of(1).Comfort != 50*needs.LevelPct {
		t.Fatalf("at the stool's comfort: %+v %+v", f.jobs[1], s.Needs.Of(1))
	}
	// off the seat it gets up
	s.Needs.m[1] = NeedRecord{Comfort: 0, ComfortStage: -1, BeautyStage: -1, OutdoorsStage: -1}
	f.pos[1] = [2]int{5, 5}
	f.jobs[1] = job.Record{Kind: job.SitDown, State: job.Do, WorkLeft: SitTicks}
	beats(s.Needs, 1, 0, 1)
	if f.jobs[1].State != job.Complete {
		t.Fatal("off the seat")
	}
}

// TestSeatSeekingDoesNotFlap is the no-flap test for sitting: a pawn
// that idles all day sits once its comfort drops below SeekComfort, gets
// up at the seat's comfort, and does not sit again until it has fallen
// the whole gap back.
func TestSeatSeekingDoesNotFlap(t *testing.T) {
	f := newFake()
	s := New(f)
	f.pawn(1, 0, 0)
	f.bed(20, construction.Stool, 0, 0)
	s.Needs.m[1] = NeedRecord{Comfort: 31 * needs.LevelPct, ComfortStage: -1, BeautyStage: -1, OutdoorsStage: -1}
	var sits []uint64
	for now := uint64(0); now < 60000; now++ {
		j := f.jobs[1]
		if j.Kind == job.SitDown && j.State == job.Goto {
			j.State = job.Do // it stands on the seat already
			f.jobs[1] = j
		}
		if j.State == job.Complete || j.Kind == job.None {
			f.jobs[1] = job.Record{}
			if s.Seats.SitDown(1, now) {
				sits = append(sits, now)
				j := f.jobs[1]
				j.State = job.Do
				f.jobs[1] = j
			}
		}
		s.Needs.Tick(1, now)
	}
	if len(sits) < 2 {
		t.Fatalf("sat %d times in a day", len(sits))
	}
	// from the stool's 50 % back under 30 % takes over 12000 ticks
	for i := 1; i < len(sits); i++ {
		if gap := sits[i] - sits[i-1]; gap < 12000 {
			t.Fatalf("sat again after %d ticks: %v", gap, sits)
		}
	}
}

func TestSleptInTheColdOrHeat(t *testing.T) {
	f := newFake()
	s := New(f)
	f.pawn(1, 0, 0)
	for _, c := range []struct {
		temp int
		want []mood.Kind
	}{
		{50, []mood.Kind{mood.SleptCold, mood.SleptOnGround}},
		{300, []mood.Kind{mood.SleptHot, mood.SleptOnGround}},
		{200, []mood.Kind{mood.SleptOnGround}},
	} {
		f.temp, f.thought = c.temp, nil
		s.Waking.After(1)
		if !slices.Equal(f.thought, c.want) {
			t.Errorf("at %d: %v, want %v", c.temp, f.thought, c.want)
		}
	}
}

func TestBedded(t *testing.T) {
	f := newFake()
	s := New(f)
	for id := entity.EntityID(1); id <= 6; id++ {
		f.pawn(id, 0, 0)
	}
	f.colonists[4] = false // a prisoner or guest gets no free bed
	f.bed(10, construction.DoubleBed, 0, 0)
	f.bed(11, construction.SleepingSpot, 5, 5)
	f.bed(12, construction.AnimalBed, 7, 7)
	f.bed(13, construction.Bed, 9, 9) // forbidden: no one's
	f.bed(14, construction.HospitalBed, 11, 11)
	f.bldgs[10] = construction.ClaimBed(construction.ClaimBed(f.bldgs[10], 1), 2)
	f.bldgs[11] = construction.ClaimBed(f.bldgs[11], 3)
	f.bldgs[12] = construction.ClaimBed(f.bldgs[12], 4) // an animal bed is no colonist's bed
	b := f.bldgs[13]
	b.Forbidden = true
	f.bldgs[13] = b
	b = f.bldgs[14]
	b.Medical = true // for patients only
	f.bldgs[14] = b
	f.bed(15, construction.Bed, 13, 13)
	b = f.bldgs[15]
	b.ForPrisoners = true
	f.bldgs[15] = b
	ids := []entity.EntityID{1, 2, 3, 4, 5, 6}
	if h := s.Beds.Bedded(ids); len(h) != 3 || !h[1] || !h[2] || !h[3] {
		t.Fatalf("bedded %v", h)
	}
	// the spot freed: the first bedless colonist in order gets it, one only
	f.bldgs[11] = construction.ReleaseBed(f.bldgs[11], 3)
	if h := s.Beds.Bedded(ids); len(h) != 3 || !h[1] || !h[2] || !h[3] || h[5] || h[6] {
		t.Fatalf("one free spot %v", h)
	}
	f.bldgs[10] = construction.ReleaseBed(f.bldgs[10], 2) // 2 now bedless too: two free slots for 2 and 3
	if h := s.Beds.Bedded(ids); len(h) != 3 || !h[1] || !h[2] || !h[3] || h[5] {
		t.Fatalf("two free slots %v", h)
	}
	f.bed(16, construction.Bedroll, 15, 15) // a third free slot reaches 5, never the prisoner 4
	if h := s.Beds.Bedded(ids); len(h) != 4 || h[4] || !h[5] || h[6] {
		t.Fatalf("two free slots %v", h)
	}
}

func TestColonyAIVanillaIngestSeatSearchUsesPawnRadiusAndRealSeat(t *testing.T) {
	f := newFake()
	s := New(f)
	f.pawn(9, 4, 4)
	f.bed(1, construction.Table, 35, 4)
	f.bed(2, construction.Chair, 34, 4) // 30 steps from pawn, reachable
	if x, y, ok := s.Seats.AtTable(9, 0, 0); !ok || x != 34 || y != 4 {
		t.Fatalf("expected seat within pawn-centered radius despite distant food: %d,%d %v", x, y, ok)
	}
	f.pawn(9, 1, 4)
	if _, _, ok := s.Seats.AtTable(9, 0, 0); ok {
		t.Fatal("chair outside pawn-centered radius should not be selected")
	}
	f.pawn(9, 4, 4)
	b := f.bldgs[2]
	b.Forbidden = true
	f.bldgs[2] = b
	if _, _, ok := s.Seats.AtTable(9, 4, 4); ok {
		t.Fatal("forbidden chair cannot be a valid dining seat; free table-side floor is not a chair")
	}
}

func TestColonyAIVanillaIngestChecksAllSeatsAfterEightBlocked(t *testing.T) {
	f := newFake()
	s := New(f)
	f.pawn(9, 10, 7)
	f.bed(1, construction.Table2x2, 10, 10)
	f.bed(2, construction.Table2x2, 16, 10)
	// First table has eight physically seated candidates, all unreachable.
	for i, c := range construction.SeatCells(construction.Table2x2, construction.South, 10, 10) {
		f.bed(entity.EntityID(100+i), construction.Stool, c[0], c[1])
		f.unreachable[c] = true
	}
	// The ninth candidate is reachable and is an allowed seat.
	f.bed(300, construction.Armchair, 16, 9)
	if x, y, ok := s.Seats.AtTable(9, 10, 11); !ok || x != 16 || y != 9 {
		t.Fatalf("8 blocked candidates hid the reachable ninth chair: %d,%d %v", x, y, ok)
	}
	f.outside[[2]int{16, 9}] = true
	if _, _, ok := s.Seats.AtTable(9, 10, 11); ok {
		t.Fatal("chair outside the resident's allowed area was selected")
	}
}
