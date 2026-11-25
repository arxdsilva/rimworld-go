// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"testing"

	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/animal/training"
	"rimworld-go/internal/domain/sim/entity"
)

func TestAutomaticTrainingMaintenanceReplay(t *testing.T) {
	g := New(Config{Seed: 931, Width: 24, Height: 18})
	wolf, err := g.SpawnAnimal("timber_wolf", 8, 8)
	if err != nil {
		t.Fatal(err)
	}
	g.TameAnimal(wolf, "Tracker")
	a, _ := g.animals.Get(wolf)
	a.Trained = [training.LegacySkills]uint8{3, 3, 2}
	g.worldS().trainingMaintenance = map[entity.EntityID]training.Maintenance{wolf: {DecayAt: uint64(g.clock.Now()) + 1}}
	a.NextThink = 1 << 40
	g.animals.Set(wolf, a)
	loaded := saveLoad(t, g)
	for _, sim := range []*Game{g, loaded} {
		sim.Step(2 * animal.DecideEvery)
	}
	first, _ := g.animals.Get(wolf)
	second, ok := loaded.animals.Get(wolf)
	if !ok || first.Trained != second.Trained || g.worldS().trainingMaintenance[wolf] != loaded.worldS().trainingMaintenance[wolf] || first.Tame != second.Tame {
		t.Fatal("training maintenance diverged on v18 save/replay")
	}
	if first.Trained[training.Release] != 1 ||
		first.Trained[training.Guard] != 3 || first.Trained[training.Obedience] != 3 ||
		g.worldS().trainingMaintenance[wolf].DecayAt <= uint64(g.clock.Now()) {
		t.Fatalf("automatic tick changed the wrong training level: %+v", first)
	}
}

func TestUntendedWildAnimalBecomesWildOnDecisionTick(t *testing.T) {
	g := New(Config{Seed: 932, Width: 24, Height: 18})
	wolf, err := g.SpawnAnimal("timber_wolf", 8, 8)
	if err != nil {
		t.Fatal(err)
	}
	g.TameAnimal(wolf, "Wildling")
	a, _ := g.animals.Get(wolf)
	g.worldS().trainingMaintenance = map[entity.EntityID]training.Maintenance{wolf: {DecayAt: uint64(g.clock.Now()) + 1, TameLoss: 4}}
	a.NextThink = 1 << 40
	g.animals.Set(wolf, a)
	g.Step(2 * animal.DecideEvery)
	after, ok := g.animals.Get(wolf)
	if !ok || after.Tame || after.Name != "Wildling" || g.worldS().trainingMaintenance[wolf].TameLoss != 0 ||
		after.Bond != 0 || after.Master != 0 {
		t.Fatalf("feral state after fifth neglected stage: %+v", after)
	}
}

func TestTrainingMaintenanceSurvivesOffMapStash(t *testing.T) {
	g := New(Config{Seed: 933, Width: 24, Height: 18})
	pet, err := g.SpawnAnimal("timber_wolf", 9, 9)
	if err != nil {
		t.Fatal(err)
	}
	g.TameAnimal(pet, "Traveler")
	maintenance := training.Maintenance{DecayAt: uint64(g.clock.Now()) + 700, TameLoss: 2}
	g.worldS().trainingMaintenance = map[entity.EntityID]training.Maintenance{pet: maintenance}
	if !g.stashPawn(pet, 99) {
		t.Fatal("unable to stash trained animal")
	}
	loaded := saveLoad(t, g)
	if loaded.worldS().trainingMaintenance[pet] != maintenance {
		t.Fatal("maintenance state missing on off-map save")
	}
	if !loaded.unstashPawn(pet, 8, 8) {
		t.Fatal("unable to return saved animal")
	}
	if got, ok := loaded.animals.Get(pet); !ok || !got.Tame ||
		loaded.worldS().trainingMaintenance[pet] != maintenance {
		t.Fatalf("maintenance lost on animal return: %+v, exists=%v", got, ok)
	}
}

func TestTrainingMaintenanceClearedOnAnimalDeath(t *testing.T) {
	g := New(Config{Seed: 934, Width: 24, Height: 18})
	pet, err := g.SpawnAnimal("timber_wolf", 8, 8)
	if err != nil {
		t.Fatal(err)
	}
	g.TameAnimal(pet, "Shortlived")
	g.worldS().trainingMaintenance = map[entity.EntityID]training.Maintenance{
		pet: {DecayAt: uint64(g.clock.Now()) + 600, TameLoss: 2},
	}
	g.KillAnimal(pet)
	if _, ok := g.worldS().trainingMaintenance[pet]; ok {
		t.Fatal("dead animal retained orphan training maintenance")
	}
}
