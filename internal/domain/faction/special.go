// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package faction

import "rimworld-go/internal/domain/animal"

// Special returns the immutable identity and standing of a hidden
// non-player faction. Negative IDs are reserved for these factions and
// cannot be mistaken for ordinary settlement indexes.
func Special(t Type) (Faction, bool) {
	if t != Swarm && t != AncientQuiet && t != AncientHostile && t != MechHive {
		return Faction{}, false
	}
	d := DefOf(t)
	f := Faction{ID: -int(t) - 1, Type: t, Name: d.Label, Goodwill: d.GoodwillMin}
	if d.Fixed && d.GoodwillMax <= HostileAt {
		f.Relation = Hostile
	}
	return f, true
}

func Specials() []Faction {
	out := make([]Faction, 0, 4)
	for _, t := range []Type{Swarm, AncientQuiet, AncientHostile, MechHive} {
		f, _ := Special(t)
		out = append(out, f)
	}
	return out
}

func Colony() Faction {
	return Faction{ID: -1, Type: Player, Name: DefOf(Player).Label, Relation: Friendly, Goodwill: 100}
}

func CreatureFaction(species animal.Species, tame bool) (Faction, bool) {
	if tame {
		return Colony(), true
	}
	t, ok := Creature(species)
	if !ok {
		return Faction{}, false
	}
	return Special(t)
}
