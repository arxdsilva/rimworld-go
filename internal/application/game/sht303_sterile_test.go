// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"testing"

	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/body"
	"rimworld-go/internal/domain/pawn/skill"
)

// TestSHT303SterileSurgery (SHT-303): a patient lying in a hut floored with
// sterile tile gets the +60 cleanliness end of body.CleanFactor (1018
// permille), a hut floored with wood the plain 1000, so the in-game
// surgery chance is higher in the sterile room.
func TestSHT303SterileSurgery(t *testing.T) {
	t.Parallel()
	type result struct {
		chance int32
		clean  int32
		found  bool
	}
	run := func(kind, stuff string) result {
		g := New(Config{Seed: 1, Width: 16, Height: 12})
		id, _ := g.SpawnNamedPawn(9, 5, "Ada")
		doc, _ := g.SpawnNamedPawn(2, 10, "Doc") // outside the hut
		(*g.sk.skills[doc])[skill.Medicine].Level = 10
		hut(t, g, 6, 2, 12, 8)
		for y := 3; y < 8; y++ {
			for x := 7; x < 12; x++ {
				if !g.BuildNow(kind, stuff, "south", x, y) {
					t.Fatalf("%s at %d,%d", kind, x, y)
				}
			}
		}
		g.warmIndexes(true)
		c, fs := g.surgeryChanceOf(doc, id, body.InstallImplant, item.HerbalMedicine)
		r := result{chance: c}
		for _, f := range fs {
			if f.Label == "Cleanliness" {
				r.clean, r.found = f.Pct, true
			}
		}
		return r
	}
	plain, sterile := run("floor", "wood"), run("sterile tile", "steel")
	if !plain.found || !sterile.found {
		t.Fatalf("no Cleanliness factor: plain %+v sterile %+v", plain, sterile)
	}
	if plain.clean != 100 || sterile.clean != 101 {
		t.Errorf("cleanliness factor plain %d%% sterile %d%%, want 100%% and 101%% (1000 and 1018 permille, shown /10)", plain.clean, sterile.clean)
	}
	if sterile.chance <= plain.chance {
		t.Errorf("sterile chance %d not above plain %d", sterile.chance, plain.chance)
	}
}
