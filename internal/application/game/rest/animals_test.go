// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package rest

import (
	"testing"

	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/sim/entity"
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

func TestAnimalBedOwnerAndClinicalStay(t *testing.T) {
	f := newFake()
	s := New(f)
	f.tame[5], f.tame[6] = true, true
	f.bed(10, construction.AnimalBed, 2, 0)
	f.bed(11, construction.AnimalBed, 8, 0)
	f.bed(12, construction.AnimalBed, 1, 0)
	b := f.bldgs[12]
	b.Medical = true
	f.bldgs[12] = b
	s.Beds.AssignOwner(10, 5, 0)
	s.Beds.AssignOwner(11, 5, 0)
	if f.bldgs[10].Claimant != 0 || f.bldgs[11].Claimant != 5 {
		t.Fatal("animal transfer did not release the previous personal bed")
	}
	s.Beds.AssignOwner(11, 99, 0)
	s.Beds.AssignOwner(11, 6, 1)
	s.Beds.AssignOwner(12, 6, 0)
	if f.bldgs[11].Claimant != 5 || f.bldgs[12].Claimant != 0 {
		t.Fatal("invalid owner/slot/medical owner accepted")
	}
	x, _, ok := s.Animals.RestBed(5, 0, 0, false)
	if !ok || x != 8 {
		t.Fatal("healthy pet admitted to medical bed")
	}
	f.held = map[entity.EntityID]entity.EntityID{12: 6}
	if x, _, _ := s.Animals.RestBed(5, 0, 0, true); x != 8 {
		t.Fatal("medical bed reserved for another was stolen")
	}
	delete(f.held, 12)
	f.unreachable[[2]int{1, 0}] = true
	if x, _, _ := s.Animals.RestBed(5, 0, 0, true); x != 8 {
		t.Fatal("unreachable medical bed accepted")
	}
	delete(f.unreachable, [2]int{1, 0})
	if x, _, _ := s.Animals.RestBed(5, 0, 0, true); x != 1 || f.HeldBed(5) != 12 || f.bldgs[11].Claimant != 5 {
		t.Fatal("medical stay discarded personal ownership")
	}
	s.Beds.AssignOwner(10, 5, 0)
	s.Beds.AssignOwner(12, 0, 0)
	if f.bldgs[10].Claimant != 0 || f.bldgs[12].Claimant != 5 {
		t.Fatal("stale owner command evicted clinical patient")
	}
	f.pos[5] = [2]int{1, 0}
	if !s.Animals.MedicalLying(5) {
		t.Fatal("claimed clinical stay not recognised")
	}
	for range 3 {
		if s.Animals.EndMedicalRest(5, true, true, true) {
			t.Fatal("stable clinical rest flapped")
		}
	}
	if !s.Animals.EndMedicalRest(5, true, true, false) || f.bldgs[12].Claimant != 0 || f.HeldBed(5) != 0 || f.bldgs[11].Claimant != 5 {
		t.Fatal("recovery failed to release just the clinical stay")
	}
}
