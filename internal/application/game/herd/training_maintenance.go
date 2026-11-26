// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package herd

import (
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/animal/training"
)

// OutermostSkill identifies the training branch that may lose progress.
// Release shields Guard, which shields Obedience.
func OutermostSkill(a animal.State, m ...training.Maintenance) (training.Skill, bool) {
	for _, skill := range [...]training.Skill{training.Haul, training.Release, training.Guard, training.Obedience} {
		if TrainSteps(a, skill, 0, m...) > 0 {
			return skill, true
		}
	}
	return 0, false
}

// AdvanceTraining settles due maintenance against the saved training state.
// True means the fifth untamed stage was reached; the game owns the
// social consequences and colony-ownership transition.
func AdvanceTraining(a *animal.State, m *training.Maintenance, now uint64) bool {
	if !a.Tame || animal.RosterOf(a.Species).Penned {
		return false
	}
	wildness := animal.Wildness(a.Species)
	interval := training.DecayInterval(wildness)
	if m.DecayAt == 0 {
		if _, trained := OutermostSkill(*a, *m); !trained && wildness <= 101 {
			return false
		}
		base := now
		if a.TrainDecay > 0 && a.TrainDecay <= now {
			base = a.TrainDecay
		}
		m.DecayAt = base + interval
	}
	for now >= m.DecayAt {
		if skill, trained := OutermostSkill(*a, *m); trained {
			progress := *TrainingSlot(a, m, skill)
			*TrainingSlot(a, m, skill) = progress&TrainToggled | (progress &^ TrainToggled) - 1
		} else if wildness > 101 {
			m.TameLoss++
			if m.TameLoss >= 5 {
				return true
			}
		} else {
			m.DecayAt = 0
			return false
		}
		m.DecayAt += interval
	}
	return false
}

// NeedsMaintenance is true only for a non-penned animal that can lose a
// trained step or tameness under the native decay rules.
func NeedsMaintenance(a animal.State, m ...training.Maintenance) bool {
	if animal.RosterOf(a.Species).Penned {
		return false
	}
	_, trained := OutermostSkill(a, m...)
	return trained || animal.Wildness(a.Species) > 101
}

// MaintenanceDue reserves ordinary handler time only for animals whose
// maintenance can actually decay, and never when the animal is starving.
func MaintenanceDue(a animal.State, m training.Maintenance, now uint64) bool {
	if !NeedsMaintenance(a, m) || a.Food <= 0 || m.DecayAt == 0 {
		return false
	}
	return m.TameLoss > 0 || now+training.DecayInterval(animal.Wildness(a.Species))/2 >= m.DecayAt
}
