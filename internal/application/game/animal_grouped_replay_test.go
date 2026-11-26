// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"testing"

	"rimworld-go/internal/domain/sim/entity"
)

// This exercises the same active component owner through 1,000 animals,
// a mid-simulation save, 120 ticks and a replay from that checkpoint.
func TestGroupedAnimalThousandReplay(t *testing.T) {
	g := New(Config{Seed: 171, Width: 128, Height: 128})
	ids := make([]entity.EntityID, 0, 1000)
	for i := range 1000 {
		id, err := g.SpawnAnimal("deer", 1+i%100, 1+i/100)
		if err != nil {
			t.Fatalf("spawn %d: %v", i, err)
		}
		ids = append(ids, id)
	}
	if g.animals.Len() != 1000 {
		t.Fatalf("animal count = %d", g.animals.Len())
	}
	g.Step(60)
	h := saveLoad(t, g)
	if g.StateHash() != h.StateHash() {
		t.Fatal("1,000 animals changed after grouped checkpoint")
	}
	for _, id := range ids {
		if !h.animals.Has(id) || h.animals.Ref(id) == nil {
			t.Fatalf("animal %d lost its group owner on load", id)
		}
	}
	g.Step(60)
	h.Step(60)
	if hash, replay := g.StateHash(), h.StateHash(); hash != replay {
		t.Fatalf("1,000 animal replay diverged after 120 ticks: %x vs %x", hash, replay)
	}
}
