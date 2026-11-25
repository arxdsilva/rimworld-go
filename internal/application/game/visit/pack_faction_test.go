// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package visit

import (
	"slices"
	"testing"

	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/sim/entity"
)

func TestPackFactionLiveMembershipAndDeparture(t *testing.T) {
	t.Parallel()
	groups := []Group{
		{ID: 1, Kind: KindCaravan, Faction: 2, Members: []Member{{ID: 12, Role: RoleTrader}, {ID: 13, Role: RolePackAnimal}}},
		{ID: 2, Kind: KindVisitor, Faction: 1, Members: []Member{{ID: 21, Role: RoleVisitor}}},
	}
	if PackFaction(groups, 12) != 0 || PackFaction(groups, 21) != 0 || PackFaction(groups, 13) != 2 {
		t.Fatal("only an active pack animal inherits its saved visit group's faction")
	}
	if CargoLossGroup(groups, 12) != 0 || CargoLossGroup(groups, 13) != 1 {
		t.Fatal("only a pack member of a trading caravan can lose its cargo")
	}
	owners := CargoOwners(groups[0], 12, func(id entity.EntityID) bool { return id == 13 })
	if !slices.Equal(owners, []entity.EntityID{12, 13}) {
		t.Fatalf("cargo owners %v", owners)
	}
	groups[0].Members[1].Departed = true
	if PackFaction(groups, 13) != 0 || CargoLossGroup(groups, 13) != 0 {
		t.Fatal("departed pack retained faction or forfeited normally carried goods")
	}
	if owners = CargoOwners(groups[0], 12, func(entity.EntityID) bool { return true }); !slices.Equal(owners, []entity.EntityID{12}) {
		t.Fatalf("departed pack remained cargo owner: %v", owners)
	}
}

func TestPackEnemyUsesMutualWorldWar(t *testing.T) {
	t.Parallel()
	fs := []faction.Faction{
		{ID: 0, Type: faction.Settlers, Relation: faction.Neutral},
		{ID: 1, Type: faction.Clans, Relation: faction.Hostile},
	}
	groups := []Group{{ID: 5, Faction: 1, Kind: KindCaravan, Members: []Member{{ID: 40, Role: RolePackAnimal}}}}
	has := func(src int, kind faction.Type) bool { return PackEnemy(groups, fs, src, kind, 40) }
	if has(1, faction.NumTypes) || has(2, faction.NumTypes) || has(0, faction.Player) {
		t.Fatal("neutral pack treated as enemy to colony or peaceful NPC")
	}
	if !has(0, faction.MechHive) || !has(0, faction.Swarm) {
		t.Fatal("hidden hostile factions should target foreign pack")
	}
	fs[0].Enemies = []int{1}
	if has(2, faction.NumTypes) {
		t.Fatal("one-sided NPC war should not produce hostile targeting")
	}
	fs[1].Enemies = []int{0}
	if has(1, faction.NumTypes) || !has(2, faction.NumTypes) {
		t.Fatal("inter-faction war should be reciprocal and exempt pack's own people")
	}
	if has(3, faction.NumTypes) || PackEnemy(groups, fs, 0, faction.Player, 800) {
		t.Fatal("invalid source or missing pack cannot be hostile")
	}
	fs[0].Relation = faction.Hostile
	if !has(0, faction.Player) {
		t.Fatal("colony should treat pack from hostile world faction as enemy")
	}
}
