// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package herd

import (
	"rimworld-go/internal/application/game/animalview"
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/animal/training"
)

// TrainViews is a's training rows for the view: every skill, with whether
// it can be learned, is on, and its steps.
func TrainViews(a animal.State, now uint64, m ...training.Maintenance) ([]bool, []animalview.Training) {
	learned := make([]bool, training.NumSkills)
	rows := make([]animalview.Training, training.NumSkills)
	for s := training.Skill(0); s < training.NumSkills; s++ {
		learned[s] = TrainLearned(a, s, now, m...)
		rows[s] = animalview.Training{Skill: int(s), Label: TrainLabel(s), Allowed: CanLearn(a.Species, s),
			On: TrainOn(a, s, m...), Learned: learned[s], Steps: int(TrainSteps(a, s, now, m...)), Needed: int(training.Needed(s))}
	}
	return learned, rows
}
