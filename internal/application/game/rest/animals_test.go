package rest

import (
	"testing"

	"rimworld-go/internal/domain/building/construction"
)

// TestAnimalBedChoice: a pet takes the nearest free animal bed in its
// area, keeps it night after night (a nearer free one never steals it),
// leaves colonists' beds and other pets' beds alone, and takes a bed
// whose holder is no longer a tame animal on the map.
func TestAnimalBedChoice(t *testing.T) {
	f := newFake()
	s := New(f)
	f.pos[5] = [2]int{0, 0}
	f.tame[5], f.tame[6] = true, true
	if _, _, ok := s.Animals.Bed(5, 0, 0); ok {
		t.Fatal("bed with none built")
	}
	f.bed(10, construction.Bed, 1, 0) // a colonist's bed: never
	f.bed(11, construction.AnimalBed, 6, 0)
	f.bed(12, construction.AnimalSleepingSpot, 9, 0)
	x, y, ok := s.Animals.Bed(5, 0, 0)
	if !ok || x != 6 || y != 0 || f.bldgs[11].Claimant != 5 || f.bldgs[10].Claimant != 0 {
		t.Fatalf("first night: %d,%d %v %+v", x, y, ok, f.bldgs[11])
	}
	f.bed(13, construction.AnimalSleepingSpot, 1, 1) // nearer, but it has a bed
	for night := 0; night < 3; night++ {
		if x, _, _ := s.Animals.Bed(5, 0, 0); x != 6 {
			t.Fatalf("night %d: swapped to %d", night, x)
		}
	}
	if x, _, ok := s.Animals.Bed(6, 2, 2); !ok || x != 1 || f.bldgs[13].Claimant != 6 {
		t.Fatalf("second pet took %d (%v)", x, ok) // the nearest free one, not the held bed
	}
	f.tame[6] = false // released or gone: its spot is free again
	b := f.bldgs[11]
	b.Forbidden = true
	f.bldgs[11] = b
	if x, _, ok := s.Animals.Bed(5, 0, 0); !ok || x != 1 || f.bldgs[11].Claimant != 0 || f.bldgs[13].Claimant != 5 {
		t.Fatalf("forbidden own bed: moved to %d (%v) %+v", x, ok, f.bldgs)
	}
	f.outside[[2]int{1, 1}] = true // outside its area now
	if x, _, _ := s.Animals.Bed(5, 0, 0); x != 9 {
		t.Fatalf("bed outside the area: %d", x)
	}
}

// TestAnimalBedSleepRate: a pet in its own animal bed rests by the bed's
// rest effectiveness over the ground's; anywhere else, or wild, base.
func TestAnimalBedSleepRate(t *testing.T) {
	f := newFake()
	s := New(f)
	f.bed(11, construction.AnimalBed, 6, 0)
	f.bed(12, construction.AnimalSleepingSpot, 9, 0)
	f.pos[5] = [2]int{6, 0}
	if got := s.Animals.SleepRate(5, true, 800); got != 800 {
		t.Fatalf("someone else's bed: %d", got)
	}
	b := f.bldgs[11]
	b.Claimant = 5
	f.bldgs[11] = b
	if got := s.Animals.SleepRate(5, true, 800); got != 1000 {
		t.Fatalf("own animal bed: %d", got)
	}
	if got := s.Animals.SleepRate(5, false, 800); got != 800 {
		t.Fatalf("wild: %d", got)
	}
	f.pos[5] = [2]int{3, 3}
	if got := s.Animals.SleepRate(5, true, 800); got != 800 {
		t.Fatalf("on the ground: %d", got)
	}
	b = f.bldgs[12]
	b.Claimant = 5
	f.bldgs[12] = b
	f.pos[5] = [2]int{9, 0}
	if got := s.Animals.SleepRate(5, true, 800); got != 800 {
		t.Fatalf("a spot is the ground's: %d", got)
	}
}
