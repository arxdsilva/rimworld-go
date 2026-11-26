// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package faction

import (
	"rimworld-go/internal/domain/animal"
	"testing"
)

func TestCreatureAllegianceAndForeignHostility(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		species animal.Species
		typ     Type
		owned   bool
	}{
		{animal.HiveBeetle, Swarm, true}, {animal.HiveCrawler, Swarm, true},
		{animal.HiveBrute, Swarm, true}, {animal.Harrow, MechHive, true},
		{animal.Spindle, MechHive, true}, {animal.Bulwark, MechHive, true},
		{animal.Quill, MechHive, true}, {animal.Deer, Player, false},
		{animal.TimberWolf, Player, false}, {animal.Species(255), Player, false},
	} {
		typ, got := Creature(tc.species)
		if got != tc.owned || typ != tc.typ {
			t.Errorf("%v: faction %v, assigned %v", tc.species, typ, got)
		}
	}
	for a := Type(0); a < NumTypes; a++ {
		for b := Type(0); b < NumTypes; b++ {
			want := a != b && (a == Swarm || a == MechHive || b == Swarm || b == MechHive)
			if HiddenEnemies(a, b) != want {
				t.Fatalf("%v/%v foreign hostility mismatch", a, b)
			}
		}
	}
}
