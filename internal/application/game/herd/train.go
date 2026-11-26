// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package herd

import (
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/animal/training"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
)

// Animal training's pure rules (ENG-003, M10), moved from the game
// package's train_animals.go (AX-7 paid its game lines with them); the
// sessions, duties and views stay there. Which skills are on lives in the
// high bit of animal.State.Trained: TrainToggled marks a skill the player
// flipped away from its default (Obedience on, the others off).

const (
	// TrainCooldown and TrainJitter: a session is due this long (plus up
	// to TrainJitter, by id) after the last.
	TrainCooldown = 3 * clock.TicksPerHour
	TrainJitter   = 600
	// TrainToggled: Trained[s] bit set when s is not at its default.
	TrainToggled uint8 = 0x80
)

var trainLabels = [training.NumSkills]string{training.Obedience: "Obedience", training.Guard: "Guard", training.Release: "Release", training.Haul: "Haul"}

// TrainLabel is a skill's display name.
func TrainLabel(s training.Skill) string {
	if s >= training.NumSkills {
		return ""
	}
	return trainLabels[s]
}

// CanLearn uses the species training class rather than combat heuristics.
func CanLearn(sp animal.Species, s training.Skill) bool {
	class := animal.TrainabilityOf(sp)
	switch s {
	case training.Obedience, training.Guard:
		return class >= animal.TrainSimple
	case training.Release:
		return class >= animal.TrainIntermediate
	case training.Haul:
		return class >= animal.TrainAdvanced
	}
	return false
}

// TrainDefault is whether s is on for a new animal.
func TrainDefault(s training.Skill) bool { return s == training.Obedience }

// TrainOn reports whether skill s is on for a.
func TrainOn(a animal.State, s training.Skill, m ...training.Maintenance) bool {
	if s >= training.NumSkills || !CanLearn(a.Species, s) {
		return false
	}
	return TrainDefault(s) != (trainingByte(a, s, m)&TrainToggled != 0)
}

// TrainSteps reads authoritative saved progress; maintenance settles decay.
func TrainSteps(a animal.State, s training.Skill, _ uint64, m ...training.Maintenance) uint8 {
	if s >= training.NumSkills {
		return 0
	}
	return trainingByte(a, s, m) &^ TrainToggled
}

// TrainLearned reports whether a knows s now.
func TrainLearned(a animal.State, s training.Skill, now uint64, m ...training.Maintenance) bool {
	return training.Learned(s, TrainSteps(a, s, now, m...))
}

// NextTrainSkill is the skill a's next session works on: the first on and
// unlearned one, Guard and Release only once Obedience is learned.
func NextTrainSkill(a animal.State, now uint64, m ...training.Maintenance) (training.Skill, bool) {
	for s := training.Skill(0); s < training.NumSkills; s++ {
		if !TrainOn(a, s, m...) || TrainLearned(a, s, now, m...) {
			continue
		}
		if s != training.Obedience && !TrainLearned(a, training.Obedience, now) {
			continue
		}
		return s, true
	}
	return 0, false
}

// TrainDue reports whether animal id's next session may start now.
func TrainDue(id entity.EntityID, a animal.State, now uint64) bool {
	return a.TrainDecay == 0 || now >= a.TrainDecay+TrainCooldown+uint64(id)%TrainJitter
}

// TrainingSlot is the byte owned by the legacy array or the global training
// owner, preserving the historical fixed-width Gob array.
func TrainingSlot(a *animal.State, m *training.Maintenance, s training.Skill) *uint8 {
	if s == training.Haul {
		return &m.HaulProgress
	}
	return &a.Trained[s]
}
func trainingByte(a animal.State, s training.Skill, m []training.Maintenance) uint8 {
	if s == training.Haul {
		if len(m) > 0 {
			return m[0].HaulProgress
		}
		return 0
	}
	return a.Trained[s]
}
