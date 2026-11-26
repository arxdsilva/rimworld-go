// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"slices"
	"testing"

	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/sim/entity"
)

func TestForeignFactionTargetingUsesLiveCreatureIndex(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 17, Width: 42, Height: 30})
	mech, _ := g.SpawnAnimal("spindle", 8, 10)
	otherMech, _ := g.SpawnAnimal("harrow", 10, 10)
	hive, _ := g.SpawnAnimal("hive_crawler", 15, 10)
	otherHive, _ := g.SpawnAnimal("hive_brute", 16, 10)
	wild, _ := g.SpawnAnimal("deer", 12, 9)
	raider := spawnRaiderAt(t, g, 17, 12, 0)
	targets := func(id entity.EntityID, x, y int) []entity.EntityID {
		seen := g.combat.Tactics.RaidFoes(id, x, y, 20)
		out := make([]entity.EntityID, len(seen))
		for i, s := range seen {
			out[i] = s.ID
		}
		return out
	}
	got := targets(mech, 8, 10)
	if !slices.Contains(got, hive) || !slices.Contains(got, otherHive) ||
		!slices.Contains(got, raider) || slices.Contains(got, otherMech) || slices.Contains(got, wild) {
		t.Fatalf("mech enemy roster %v", got)
	}
	got = targets(hive, 15, 10)
	if !slices.Contains(got, mech) || !slices.Contains(got, otherMech) ||
		!slices.Contains(got, raider) || slices.Contains(got, otherHive) || slices.Contains(got, wild) {
		t.Fatalf("hive enemy roster %v", got)
	}
	got = targets(raider, 17, 12)
	if !slices.Contains(got, mech) || !slices.Contains(got, hive) || slices.Contains(got, wild) {
		t.Fatalf("human raid enemy roster %v", got)
	}
	if got = targets(0, 17, 12); len(got) != 0 {
		t.Fatalf("guard mode incorrectly targets foreign fighters: %v", got)
	}
	a, _ := g.animals.Get(hive)
	a.Dormant = true
	g.animals.Set(hive, a)
	a, _ = g.animals.Get(otherHive)
	a.Injury = animal.DefOf(a.Species).MaxHP
	g.animals.Set(otherHive, a)
	got = targets(mech, 8, 10)
	if slices.Contains(got, hive) || slices.Contains(got, otherHive) {
		t.Fatalf("dormant or dead insect was still combat target: %v", got)
	}
	reloaded := saveLoad(t, g)
	got = func() []entity.EntityID {
		seen := reloaded.combat.Tactics.RaidFoes(mech, 8, 10, 20)
		out := make([]entity.EntityID, len(seen))
		for i, s := range seen {
			out[i] = s.ID
		}
		return out
	}()
	if slices.Contains(got, hive) || slices.Contains(got, otherHive) ||
		!slices.Contains(got, raider) {
		t.Fatalf("restored source-specific hostility %v", got)
	}
}

func TestMechAcquiresForeignHiveWithoutColonist(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 19, Width: 40, Height: 30})
	mech, err := g.SpawnAnimal("spindle", 8, 15)
	if err != nil {
		t.Fatal(err)
	}
	hive, err := g.SpawnAnimal("hive_crawler", 14, 15)
	if err != nil {
		t.Fatal(err)
	}
	friend, err := g.SpawnAnimal("harrow", 9, 15)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for range 30 {
		g.Step(1)
		if a, ok := g.animals.Get(mech); ok && a.Target == hive {
			found = true
			break
		}
	}
	if !found {
		a, _ := g.animals.Get(mech)
		t.Fatalf("mech targeted %d, want foreign hive %d (same-faction mech %d)", a.Target, hive, friend)
	}
	g2 := saveLoad(t, g)
	if g.StateHash() != g2.StateHash() {
		t.Fatal("foreign combat target is not replay-stable")
	}
}
