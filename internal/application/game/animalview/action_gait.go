// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package animalview

import (
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/sim/entity"
)

// ActionState is the state exposed to the action read model; eating,
// resting, and leaving animals use the standing wander action.
func ActionState(a animal.State) animal.State {
	switch a.Mode {
	case animal.Feed, animal.Leave, animal.Rest:
		a.Mode = animal.Wander
	}
	return a
}

// Female keeps the sex used by pre-needs saves consistent with art.
func Female(id entity.EntityID, a animal.State) bool {
	if !a.HasSex {
		return animal.ArtFemale(uint64(id))
	}
	return a.Female
}

// Gait is the step's recorded gait, with no gait shown while stationary.
func Gait(a animal.State, stepTick uint64, stepTicks int, now uint64, hasNext bool) string {
	if a.StepGait == animal.GaitNone || a.Downed ||
		a.Mode == animal.Rest && a.PathI >= len(a.Path) {
		return ""
	}
	if !hasNext && now >= stepTick+uint64(stepTicks) {
		return ""
	}
	return a.StepGait.String()
}
