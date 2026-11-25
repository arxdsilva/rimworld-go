// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package animalview

import (
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/sim/entity"
)

// Live composes the species definition and a live authoritative animal
// state. Scene-specific movement and UI details are attached by the game.
func Live(id entity.EntityID, a animal.State, x, y int, female, hostile bool, target entity.EntityID, now uint64) View {
	s := Of(a.Species)
	return View{
		ID: id, Species: a.Species.String(), Label: s.Label, Faction: s.Faction, Emblem: s.Emblem, X: x, Y: y,
		Attack: s.Attack, Female: female, Size: s.Size,
		Health: a.HP() * 100 / s.MaxHP, Mode: a.Mode.String(),
		Designated: a.Designated, Hostile: hostile,
		TargetID: target, Tame: a.Tame, Name: a.Name,
		TameMark: a.TameMark, Manhunter: a.Enraged(now),
	}
}
