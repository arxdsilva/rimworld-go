// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"slices"
	"testing"

	"rimworld-go/internal/domain/sim/entity"
)

// tameOutcomes builds the same colony, draws extra unrelated values from
// the shared g.rng, then tames six animals and botches the taming of
// twelve timber wolves (30% revenge) at one tick: the pet names and which
// wolves were enraged.
func tameOutcomes(t *testing.T, extra int) (names []string, enraged []bool) {
	t.Helper()
	g := New(Config{Seed: 7, Width: 40, Height: 20})
	bareGround(g)
	ada := draftedPawn(t, g, 10, 10)
	fullNeeds(g, ada)
	var pets, wolves []entity.EntityID
	for i := range 6 {
		id, err := g.SpawnAnimal("deer", 20+i, 4)
		if err != nil {
			t.Fatal(err)
		}
		pets = append(pets, id)
	}
	for i := range 12 {
		id, err := g.SpawnAnimal("timber_wolf", 20+i, 16)
		if err != nil {
			t.Fatal(err)
		}
		wolves = append(wolves, id)
	}
	for range extra {
		g.rng.IntN(1000) // an unrelated system's draw
	}
	now := uint64(g.clock.Now())
	for _, id := range pets {
		herdHost{g}.Tamed(ada, id)
		names = append(names, animalOf(g, id).Name)
	}
	for _, id := range wolves {
		herdHost{g}.TameFailed(ada, id)
		enraged = append(enraged, animalOf(g, id).Enraged(now))
	}
	return names, enraged
}

// TestTameKeyedIgnoresUnrelatedDraws (#8, AC-1): the pet name and the
// revenge roll of a taming are keyed draws on (seed, tick, operation,
// animal), so k unrelated draws from the shared stream first leave every
// outcome as it was.
func TestTameKeyedIgnoresUnrelatedDraws(t *testing.T) {
	t.Parallel()
	names, enraged := tameOutcomes(t, 0)
	if !slices.Contains(enraged, true) || !slices.Contains(enraged, false) {
		t.Fatalf("setup: twelve wolves at 30%% should give both outcomes: %v", enraged)
	}
	for _, k := range []int{1, 3, 17} {
		n, e := tameOutcomes(t, k)
		if !slices.Equal(n, names) {
			t.Errorf("k=%d: pet names %v, want %v", k, n, names)
		}
		if !slices.Equal(e, enraged) {
			t.Errorf("k=%d: enraged %v, want %v", k, e, enraged)
		}
	}
}
