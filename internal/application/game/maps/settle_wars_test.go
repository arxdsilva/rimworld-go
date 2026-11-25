// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package maps

import (
	"testing"

	"rimworld-go/internal/domain/faction"
)

// World settlement diplomacy follows the saved foreign war ledger, even
// after the player's relationship with a raiding faction changes.
func TestSettlementEnemiesArePersistentForeignWars(t *testing.T) {
	a := faction.Faction{ID: 1, Type: faction.Settlers, Relation: faction.Friendly, Enemies: []int{2}}
	b := faction.Faction{ID: 2, Type: faction.Drifters, Relation: faction.Friendly, Enemies: []int{1}}
	if !Enemies(a, b) || !Enemies(b, a) {
		t.Fatal("the allied-to-player foreign factions must remain at war")
	}
	b.Relation = faction.Hostile
	if !Enemies(a, b) {
		t.Fatal("the player's relationship changed the foreign war")
	}
	b.Enemies = nil
	if Enemies(a, b) {
		t.Fatal("one-sided war state caused a fight")
	}
	b.Enemies = []int{1}
	for _, change := range []func(*faction.Faction){
		func(f *faction.Faction) { f.Defeated = true },
		func(f *faction.Faction) { f.Type = faction.MechHive },
	} {
		c := b
		change(&c)
		if Enemies(a, c) {
			t.Fatalf("non-surface or defeated faction took part in settlement war: %+v", c)
		}
	}
}
