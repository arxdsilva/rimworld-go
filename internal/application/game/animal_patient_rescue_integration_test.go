// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"testing"

	"rimworld-go/internal/application/game/care"
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/ai/work"
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/entity"
)

func animalRescueScene(t *testing.T, seed uint64, tame bool) (*Game, entity.EntityID, entity.EntityID, entity.EntityID) {
	t.Helper()
	g := New(Config{Seed: seed, Width: 24, Height: 14})
	pet, err := g.SpawnAnimal("husky", 16, 8)
	if err != nil {
		t.Fatal(err)
	}
	if tame {
		g.TameAnimal(pet, "Patient")
	}
	placeBuilt(t, g, construction.SleepingSpot, construction.South, 6, 7)
	placeBuilt(t, g, construction.AnimalSleepingSpot, construction.South, 3, 7)
	bed := g.bldgIDs[len(g.bldgIDs)-1]
	bruiseDown(t, g, pet)
	doc, err := g.SpawnNamedPawn(12, 8, "Vet")
	if err != nil {
		t.Fatal(err)
	}
	g.setPriority(doc, work.Doctor, 1)
	return g, doc, pet, bed
}

func TestDoctorCarriesDownedPetToAnimalBed(t *testing.T) {
	g, doc, pet, bed := animalRescueScene(t, 1101, true)
	if !g.care.Doctor.InjuredDowned(pet) || !g.care.Doctor.Rescuable(pet) {
		t.Fatal("tame downed animal should be rescuable")
	}
	if selected := g.care.SafeBedFor(pet, 16, 8, care.RescueBed); selected != bed {
		t.Fatalf("animal rescue selected a human bed: %d, want %d", selected, bed)
	}
	if !g.care.Doctor.Assign(doc) {
		t.Fatal("doctor did not get a rescue job")
	}
	if j, _ := g.jobs.Get(doc); j.Kind != job.Rescue || j.Target != pet {
		t.Fatalf("doctor chose another job: %+v", j)
	}
	carried := false
	for i := 0; i < 1800 && g.bedUnder(pet) != bed; i++ {
		g.Step(1)
		if g.care.Carry.Carrier(pet) == doc {
			carried = true
			if g.posOf(pet) != g.posOf(doc) {
				t.Fatal("animal patient detached while carried")
			}
		}
	}
	if !carried || g.bedUnder(pet) != bed || g.care.Carry.Carrier(pet) != 0 {
		t.Fatalf("animal rescue incomplete: carried=%v, bed=%d", carried, g.bedUnder(pet))
	}
	if g.care.Rest.HeldBed(pet) != bed {
		t.Fatal("temporary animal bed stay not recorded")
	}
	g.Step(1) // completed doctor job releases its reservation on retirement
	if _, held := g.reserve.EntityOwner(bed); held {
		t.Fatal("finished rescue holds bed reservation")
	}
	a, _ := g.animals.Get(pet)
	a.Injury = 0
	if !g.checkAnimalDowned(pet, &a, uint64(g.clock.Now())) || a.Downed {
		t.Fatal("recovered animal failed to stand")
	}
	g.animals.Set(pet, a)
	if g.care.Rest.HeldBed(pet) != 0 || g.bedUnder(pet) != 0 {
		t.Fatal("temporary animal bed stayed claimed after recovery")
	}
}

func TestWildAnimalExcludedFromMedicalRescue(t *testing.T) {
	g, doc, wild, _ := animalRescueScene(t, 1102, false)
	if g.care.Doctor.Rescuable(wild) {
		t.Fatal("wild animal entered colony rescue roster")
	}
	if g.care.Doctor.Assign(doc) {
		if j, _ := g.jobs.Get(doc); j.Kind == job.Rescue && j.Target == wild {
			t.Fatal("doctor assigned forbidden wild-animal rescue")
		}
	}
}

func TestPetDeathDuringDoctorCarryDropsPatientAndReservation(t *testing.T) {
	g, doc, pet, bed := animalRescueScene(t, 1103, true)
	if !g.care.Doctor.Assign(doc) {
		t.Fatal("rescue assignment failed")
	}
	for i := 0; i < 900 && g.care.Carry.Carrier(pet) != doc; i++ {
		g.Step(1)
	}
	if g.care.Carry.Carrier(pet) != doc {
		t.Fatal("doctor did not pick up pet")
	}
	g.KillAnimal(pet)
	if g.care.Carry.Patient(doc) != 0 || g.care.Carry.Carrier(pet) != 0 {
		t.Fatal("dead animal still in carry index")
	}
	if j, _ := g.jobs.Get(doc); j.Kind == job.Rescue && j.Active() {
		t.Fatalf("doctor still rescues dead animal: %+v", j)
	}
	if owner, held := g.reserve.EntityOwner(bed); held {
		t.Fatalf("dead animal bed reserved by %d", owner)
	}
	if g.care.Rest.HeldBed(pet) != 0 {
		t.Fatal("dead animal kept bed stay")
	}
}

func TestAnimalPatientRescueSaveReplayAndBedLoss(t *testing.T) {
	g, doc, pet, bed := animalRescueScene(t, 1104, true)
	if !g.care.Doctor.Assign(doc) {
		t.Fatal("rescue assignment failed")
	}
	for i := 0; i < 900 && g.care.Carry.Carrier(pet) != doc; i++ {
		g.Step(1)
	}
	if g.care.Carry.Carrier(pet) != doc {
		t.Fatal("doctor did not pick up pet")
	}
	loaded := saveLoad(t, g)
	if g.StateHash() != loaded.StateHash() || loaded.care.Carry.Carrier(pet) != doc {
		t.Fatal("active animal rescue changed after v18 save")
	}
	g.Step(4)
	loaded.Step(4)
	if g.StateHash() != loaded.StateHash() {
		t.Fatal("animal rescue diverged on replay")
	}
	loaded.removeBuilding(bed)
	loaded.Step(1)
	if loaded.care.Carry.Carrier(pet) != 0 || loaded.bedUnder(pet) != 0 {
		t.Fatal("removed animal bed left the patient in carry")
	}
}

func TestMedicalAnimalBedOwnerRescueHealingReplay(t *testing.T) {
	g, doc, pet, medical := animalRescueScene(t, 1101, true)
	// Retain the known reachable rescue route of the original native fixture.
	// A separate ordinary bed is the patient's persistent personal claim.
	personal := buildNow(t, g, construction.AnimalBed, item.Cloth, construction.South, 20, 8)
	assignOwner(g, personal, pet)
	if bedOf(g, personal).Claimant != uint64(pet) {
		t.Fatal("native animal owner command failed")
	}
	setMedical(g, medical, true)
	assignOwner(g, medical, pet)
	assignOwner(g, personal, doc)
	if bedOf(g, personal).Claimant != uint64(pet) || bedOf(g, medical).Claimant != 0 || !buildingView(g, medical).Medical {
		t.Fatal("medical flag or invalid owner contract failed")
	}
	if got := g.care.SafeBedFor(pet, 16, 8, care.RescueBed); got != medical {
		t.Fatalf("rescue chose %v not medical %v", got, medical)
	}
	if !g.care.Doctor.Assign(doc) {
		t.Fatal("doctor could not rescue animal")
	}
	for i := 0; i < 2400 && g.bedUnder(pet) != medical; i++ {
		g.Step(1)
	}
	if !g.rest.Animals.MedicalLying(pet) || g.care.Rest.HeldBed(pet) != medical || bedOf(g, personal).Claimant != uint64(pet) {
		t.Fatalf("clinical hold failed: bed=%v personal=%+v", g.bedUnder(pet), bedOf(g, personal))
	}
	loaded := saveLoad(t, g)
	if g.StateHash() != loaded.StateHash() || !loaded.rest.Animals.MedicalLying(pet) {
		t.Fatal("clinical hold did not survive save/load")
	}
	// Same actual wounded animal and time: a medical bed contributes the
	// existing LieBed instead of LieGround. Compare fixed-point wound work
	// (including fully healed HP), rather than only a rendered label.
	setMedical(loaded, medical, false)
	a, _ := g.animals.Get(pet)
	b, _ := loaded.animals.Get(pet)
	now := uint64(g.clock.Now())
	if g.stepAnimalBody(pet, &a, now) || loaded.stepAnimalBody(pet, &b, now) {
		t.Fatal("fixture unexpectedly died")
	}
	remaining := func(s *animal.State) int64 {
		var n int64
		for _, w := range s.Body.Wounds {
			if !w.Permanent {
				n += int64(w.Dmg)*24000 - int64(w.HealAcc)
			}
		}
		return n
	}
	if remaining(&a) >= remaining(&b) {
		t.Fatalf("clinical bed gave no actual healing benefit: %d vs %d", remaining(&a), remaining(&b))
	}
	g.care.Rest.RecoveredAnimal(pet)
	if bedOf(g, medical).Claimant != 0 || bedOf(g, personal).Claimant != uint64(pet) {
		t.Fatal("clinical recovery lost the original owner claim")
	}
}
