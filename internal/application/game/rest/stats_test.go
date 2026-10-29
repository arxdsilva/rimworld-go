package rest

import (
	"slices"
	"testing"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/sim/entity"
)

// piece adds a building of kind k at (x, y) facing f.
func (f *fakeHost) piece(id entity.EntityID, k construction.Kind, fc construction.Facing, x, y int) {
	f.bed(id, k, x, y)
	b := f.bldgs[id]
	b.Facing = fc
	f.bldgs[id] = b
}

func TestRestEffect(t *testing.T) {
	f := newFake()
	s := New(f)
	f.bed(10, construction.Bed, 0, 0)
	f.bed(11, construction.Bedroll, 2, 0)
	f.bed(12, construction.AnimalBed, 4, 0)
	f.bed(13, construction.EndTable, 6, 0)
	cases := map[entity.EntityID]int{0: 800, 10: 1000, 11: 950, 12: 1000, 13: 800, 99: 800}
	for bed, want := range cases {
		if got := s.Stats.RestEffect(bed); got != want {
			t.Errorf("RestEffect(%d) = %d, want %d", bed, got, want)
		}
	}
}

// TestBedFacilities: an end table beside the head and a dresser in reach
// in the same room each add 0.05 comfort; one of each counts; a dresser
// out of reach, in another room or an end table at the foot adds none.
func TestBedFacilities(t *testing.T) {
	f := newFake()
	s := New(f)
	f.bed(10, construction.Bed, 5, 5) // facing south: head (5,5), foot (5,6)
	if v := s.Stats.Of(10); v != (BedView{Comfort: 750, RestEffect: 1000, Beauty: 1000}) || v.Links() != 0 {
		t.Fatalf("bare bed %+v", v)
	}
	f.bed(20, construction.EndTable, 4, 6) // beside the foot: no link
	if v := s.Stats.Of(10); v.EndTable != 0 {
		t.Fatalf("end table at the foot linked: %+v", v)
	}
	f.bed(21, construction.EndTable, 6, 5) // beside the head
	f.bed(22, construction.EndTable, 4, 5) // a second one counts nothing more
	v := s.Stats.Of(10)
	if v.EndTable == 0 || v.Comfort != 800 || v.Links() != LinkEndTable {
		t.Fatalf("end table: %+v", v)
	}
	f.piece(30, construction.Dresser, construction.South, 11, 5) // 6 tiles off: in reach
	f.piece(31, construction.Dresser, construction.South, 5, 0)  // also in reach: one counts
	v = s.Stats.Of(10)
	if v.Dresser == 0 || v.Comfort != 850 || v.Links() != LinkEndTable|LinkDresser {
		t.Fatalf("dresser: %+v", v)
	}
	f.rooms[[2]int{11, 5}], f.rooms[[2]int{12, 5}], f.rooms[[2]int{5, 0}], f.rooms[[2]int{6, 0}] = 7, 7, 7, 7 // other rooms
	if v := s.Stats.Of(10); v.Dresser != 0 || v.Comfort != 800 {
		t.Fatalf("dresser in another room linked: %+v", v)
	}
	delete(f.bldgs, 21)
	delete(f.bldgs, 22)
	f.bldgIDs = slices.DeleteFunc(f.bldgIDs, func(id entity.EntityID) bool { return id == 21 || id == 22 })
	clear(f.rooms)
	if v := s.Stats.Of(10); v.EndTable != 0 || v.Links() != LinkDresser {
		t.Fatalf("dresser only: %+v", v)
	}
	if v := s.Stats.Of(30); v != (BedView{Beauty: 5000}) {
		t.Fatalf("a dresser's own stats: %+v", v)
	}
	if v := s.Stats.Of(99); v != (BedView{}) {
		t.Fatalf("missing building: %+v", v)
	}
}

// TestDoubleBedEndTable: either head of a double bed takes an end table.
func TestDoubleBedEndTable(t *testing.T) {
	f := newFake()
	s := New(f)
	f.bed(10, construction.DoubleBedroll, 5, 5) // heads (5,5) and (6,5)
	f.bed(20, construction.EndTable, 7, 5)
	if v := s.Stats.Of(10); v.EndTable != 20 || v.Comfort != 730 || v.RestEffect != 950 {
		t.Fatalf("double bedroll %+v", v)
	}
}

// TestRestEffectivenessSetsSleepLength: the same rest takes longer to
// come back on the ground than in a bed, and longer in a bedroll.
func TestRestEffectivenessSetsSleepLength(t *testing.T) {
	ticks := func(bed entity.EntityID, k construction.Kind) int {
		f := newFake()
		s := New(f)
		f.pawn(1, 0, 0)
		if bed != 0 {
			f.bed(bed, k, 0, 0)
		}
		f.need[1] = needs.Pair{Hunger: needs.Full, Rest: needs.CriticalRest}
		j := job.Record{Kind: job.Sleep, State: job.Do, Target: bed, WorkLeft: needs.SleepWorkMin}
		for now := uint64(1); ; now++ {
			s.Do(1, j, now)
			if j = f.jobs[1]; j.State == job.Complete {
				return int(now)
			}
		}
	}
	ground, bed, roll := ticks(0, 0), ticks(10, construction.Bed), ticks(10, construction.Bedroll)
	if !(bed < roll && roll < ground) {
		t.Fatalf("sleep length bed %d, bedroll %d, ground %d", bed, roll, ground)
	}
	if want := bed * 1000 / 800; ground < want-5 || ground > want+5 {
		t.Fatalf("ground %d ticks, want about %d (0.8 of a bed)", ground, want)
	}
}

func TestForbiddenBedUnused(t *testing.T) {
	f := newFake()
	s := New(f)
	f.pawn(1, 0, 0)
	f.bed(10, construction.Bed, 1, 0)
	f.bed(11, construction.Bed, 8, 0)
	b := f.bldgs[10]
	b.Forbidden = true
	f.bldgs[10] = b
	if !s.Beds.SleepIn(1) || f.jobs[1].Target != 11 {
		t.Fatalf("took the forbidden bed: %+v", f.jobs[1])
	}
	b = f.bldgs[11]
	b.Forbidden = true
	f.bldgs[11] = b
	if s.Beds.SleepIn(1) {
		t.Fatal("slept in its own bed once forbidden")
	}
	// a forbidden double bed is no couple's
	f.partner[1], f.partner[2] = 2, 1
	f.pawn(2, 0, 1)
	f.bed(12, construction.DoubleBedroll, 3, 3)
	b = f.bldgs[12]
	b.Forbidden = true
	f.bldgs[12] = b
	if s.Beds.SleepIn(1) {
		t.Fatalf("couple used a forbidden double bed: %+v", f.jobs[1])
	}
	b.Forbidden = false
	f.bldgs[12] = b
	if !s.Beds.SleepIn(1) || f.jobs[1].Target != 12 {
		t.Fatalf("couple's double bedroll: %+v", f.jobs[1])
	}
}

func TestStoneBedRestsLess(t *testing.T) {
	f := newFake()
	s := New(f)
	f.bed(10, construction.Bed, 0, 0)
	b := f.bldgs[10]
	b.Stuff = item.GraniteBlocks
	f.bldgs[10] = b
	if got := s.Stats.RestEffect(10); got != 900 {
		t.Fatalf("stone bed %d", got)
	}
}

// TestStatsOfAllocatesNothing: the read model asks for every bed's stats
// each snapshot.
func TestStatsOfAllocatesNothing(t *testing.T) {
	f := newFake()
	s := New(f)
	f.bed(10, construction.DoubleBed, 5, 5)
	f.bed(20, construction.EndTable, 7, 5)
	if a := testing.AllocsPerRun(50, func() { s.Stats.Of(10) }); a != 0 {
		t.Fatalf("%v allocs", a)
	}
}
