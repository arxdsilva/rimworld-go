// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package herd

import (
	"testing"

	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/animal/training"
)

func TestTrainingMaintenancePreservesPrerequisiteFrontier(t *testing.T) {
	a := animal.State{Species: animal.TimberWolf, Tame: true,
		Trained: [training.LegacySkills]uint8{3, 3 | TrainToggled, 2}}
	interval := training.DecayInterval(animal.Wildness(a.Species))
	now := uint64(100)
	m := training.Maintenance{DecayAt: now + interval}
	choices := []struct {
		skill training.Skill
		want  uint8
	}{
		{training.Release, 1}, {training.Release, 0},
		{training.Guard, 2}, {training.Guard, 1}, {training.Guard, 0},
		{training.Obedience, 2},
	}
	for i, c := range choices {
		due := now + uint64(i+1)*interval
		if AdvanceTraining(&a, &m, due) {
			t.Fatalf("stage %d became wild with progress remaining", i)
		}
		if got := TrainSteps(a, c.skill, due); got != c.want {
			t.Fatalf("stage %d %d progress: %d, want %d", i, c.skill, got, c.want)
		}
		if c.skill != training.Obedience && TrainSteps(a, training.Obedience, due) != 3 {
			t.Fatal("advanced training did not protect prerequisite")
		}
		if a.Trained[training.Guard]&TrainToggled == 0 {
			t.Fatal("decay overwrote player training toggle")
		}
	}
}

func TestTrainingMaintenanceWildnessAndPenProtection(t *testing.T) {
	const first = uint64(100)
	wolf := animal.State{Species: animal.TimberWolf, Tame: true, Name: "Wildling"}
	interval := training.DecayInterval(animal.Wildness(wolf.Species))
	wolfMaintenance := training.Maintenance{}
	if AdvanceTraining(&wolf, &wolfMaintenance, first) || wolfMaintenance.DecayAt != first+interval {
		t.Fatal("wildness maintenance scheduled incorrectly")
	}
	for stage := 1; stage < 5; stage++ {
		if AdvanceTraining(&wolf, &wolfMaintenance, first+uint64(stage)*interval) || wolfMaintenance.TameLoss != uint8(stage) {
			t.Fatalf("tameness stage %d: got %d", stage, wolfMaintenance.TameLoss)
		}
	}
	if !AdvanceTraining(&wolf, &wolfMaintenance, first+5*interval) || wolfMaintenance.TameLoss != 5 {
		t.Fatal("fifth neglected tameness stage should become wild")
	}

	dog := animal.State{Species: animal.Labrador, Tame: true}
	dog.Trained[training.Obedience] = 3
	dogMaintenance := training.Maintenance{}
	AdvanceTraining(&dog, &dogMaintenance, first)
	for stage := uint64(1); stage <= 4; stage++ {
		if AdvanceTraining(&dog, &dogMaintenance, first+stage*training.DecayInterval(0)) {
			t.Fatal("domestic animals can lose training, but retain tameness")
		}
	}
	if !dog.Tame || dog.Trained[training.Obedience] != 0 || dogMaintenance.TameLoss != 0 {
		t.Fatalf("domestic animal state: %+v", dog)
	}

	goat := animal.State{Species: animal.Goat, Tame: true,
		Trained: [training.LegacySkills]uint8{3, 2, 1}}
	goatMaintenance := training.Maintenance{DecayAt: 1, TameLoss: 4}
	if AdvanceTraining(&goat, &goatMaintenance, 20*training.DecayMaxTicks) ||
		goatMaintenance.DecayAt != 1 || goatMaintenance.TameLoss != 4 || goat.Trained != [training.LegacySkills]uint8{3, 2, 1} {
		t.Fatal("penned animal maintenance changed training or tameness")
	}
}
