// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"rimworld-go/internal/domain/world/site"
	"testing"

	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/world/biome"
)

func TestWildAnimalsNeverStartMating(t *testing.T) {
	g := New(Config{Seed: 31, Width: 30, Height: 20})
	m := spawnSexed(t, g, "deer", 12, 9, false, false, "Wild stag")
	f := spawnSexed(t, g, "deer", 13, 9, true, false, "Wild doe")
	now := uint64(g.clock.Now())
	keepFed(g, m)
	keepFed(g, f)
	for range 300 {
		male, _ := g.animals.Get(m)
		g.mateCheck(m, &male, now)
		if male.Mate != 0 {
			t.Fatal("wild deer selected a mate")
		}
	}
	setAnimal(g, m, func(a *animal.State) { a.Mate, a.MateLeft, a.MateUntil = f, 1, now+100 })
	setAnimal(g, f, func(a *animal.State) {
		a.Mate, a.MateLeft, a.MateUntil = m, 1, now+100
		a.Pregnant, a.Mated = now+1, 3
	})
	before := animalCount(g, animal.Deer)
	female, _ := g.animals.Get(f)
	g.tickLife(f, &female, now+2)
	g.animals.Set(f, female)
	male, _ := g.animals.Get(m)
	g.tickLife(m, &male, now+2)
	if female.Pregnant != 0 || female.Mated != 0 || female.Mate != 0 ||
		male.Mate != 0 || animalCount(g, animal.Deer) != before+3 {
		t.Fatalf("wild mating must stop but pending litter must survive: female %+v male %+v", female, male)
	}
}

func TestTemperateEcosystemBudget250(t *testing.T) {
	g := New(Config{Seed: 21, Width: 250, Height: 250, Biome: biome.TemperateForest})
	if c := g.wildlifeCap(); c != 2312 {
		t.Fatalf("temperate 250x250 capacity %d hundredths, want 2312 (23.12 weight)", c)
	}
	g.generate(21, site.Site{})
	g.seedGrass()
	g.spawnWildlife()
	initial := g.wildCount()
	if initial < g.startTarget()*3/4 || initial > g.wildlifeCap()+largestWildCost(g) {
		t.Fatalf("initial ecosystem weight %d outside expected budget %d", initial, g.wildlifeCap())
	}
	var calculated int
	for _, id := range g.animalIDs {
		a, ok := g.animals.Get(id)
		if ok && animal.CountsWild(a, uint64(g.clock.Now())) {
			calculated += animal.EcosystemWeight(a.Species)
		}
	}
	if calculated != initial {
		t.Fatalf("cached ecosystem weight %d, computed from wildlife %d", initial, calculated)
	}
	if len(g.animalIDs) >= 300 {
		t.Fatalf("250x250 ecosystem spawned old headcount-cap-sized herd: %d", len(g.animalIDs))
	}
	for day := 1; day <= 3; day++ {
		g.Step(clock.TicksPerDay)
		weight := g.wildCount()
		if weight*2 < g.wildlifeCap() || weight*2 > 3*g.wildlifeCap() {
			t.Fatalf("day %d: ecosystem weight %d diverged from budget %d", day, weight, g.wildlifeCap())
		}
	}
}

func TestLegacyWildlifeHeadcountCacheRebuiltAsWeight(t *testing.T) {
	g := New(Config{Seed: 9, Width: 30, Height: 20})
	id, err := g.SpawnAnimal("rat", 7, 6)
	if err != nil {
		t.Fatal(err)
	}
	if a, ok := g.animals.Get(id); !ok || a.Tame {
		t.Fatalf("fixture rat was not wild: %+v", a)
	}
	g.wl.Count, g.wl.Preds, g.wl.CountHour, g.wl.WeightSchema = 1, 0, 1, false
	h := saveLoad(t, g)
	if h.wl.WeightSchema {
		t.Fatal("legacy population cache was marked weighted")
	}
	if got := h.wildCount(); got != animal.EcosystemWeight(animal.Rat) {
		t.Fatalf("legacy headcount was read as ecosystem weight: %d", got)
	}
	if !h.wl.WeightSchema {
		t.Fatal("cache conversion did not persist in memory")
	}
}

func TestWildlifeCacheFormatMarkerDoesNotChangeStateHash(t *testing.T) {
	g := New(Config{Seed: 11, Width: 30, Height: 20})
	before := g.StateHash()
	g.wl.WeightSchema = !g.wl.WeightSchema
	if after := g.StateHash(); after != before {
		t.Fatalf("format marker affected state hash: %d -> %d", before, after)
	}
}

// A pair selected while tame must not conceive after either partner
// is released; already committed fertilization is a separate fact.
func TestMatingRevalidatesBothPartners(t *testing.T) {
	g := New(Config{Seed: 81, Width: 30, Height: 20})
	m := spawnSexed(t, g, "deer", 11, 10, false, true, "male")
	f := spawnSexed(t, g, "deer", 12, 10, true, true, "female")
	now := uint64(g.clock.Now())
	keepFed(g, m)
	keepFed(g, f)
	setAnimal(g, m, func(a *animal.State) {
		a.Mate, a.MateLeft, a.MateUntil = f, 1, now+100
	})
	setAnimal(g, f, func(a *animal.State) {
		a.Tame = false
		a.Mate, a.MateLeft, a.MateUntil = m, 1, now+100
	})
	male, _ := g.animals.Get(m)
	g.mateStep(m, &male, Position{X: 11, Y: 10}, now)
	female, _ := g.animals.Get(f)
	if male.Mate != 0 || female.Mate != 0 || female.Pregnant != 0 {
		t.Fatalf("mating committed after release: male %+v female %+v", male, female)
	}
}
