// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package faction

import "rimworld-go/internal/domain/animal"

// Creature is the permanent allegiance of a non-human species.
// Wildlife has no faction; a pet's owner is stored in the living state.
func Creature(s animal.Species) (Type, bool) {
	switch {
	case animal.IsMech(s):
		return MechHive, true
	case animal.DefOf(s).Hive:
		return Swarm, true
	default:
		return Player, false
	}
}

// HiddenEnemies is the innate hostility of the mechanized and hive
// factions to every foreign faction, including one another.
func HiddenEnemies(a, b Type) bool {
	return a != b && (a == MechHive || a == Swarm || b == MechHive || b == Swarm)
}
