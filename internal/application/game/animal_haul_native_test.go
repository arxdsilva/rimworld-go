// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"rimworld-go/internal/application/game/herd"
	"rimworld-go/internal/domain/animal/training"
	"rimworld-go/internal/domain/building/zone"
	"rimworld-go/internal/domain/economy/item"
	"testing"
)

func TestAnimalHaulPhysicalSaveReplay(t *testing.T) {
	g := New(Config{Seed: 935, Width: 24, Height: 18})
	dog, err := g.SpawnAnimal("husky", 8, 8)
	if err != nil {
		t.Fatal(err)
	}
	g.TameAnimal(dog, "Carrier")
	g.zones.Paint(zone.KindStockpile, zone.NewStockpile(), zone.RectOf(14, 8, 16, 9), nil)
	_, err = g.SpawnStack(item.Wood, 5, 9, 8)
	if err != nil {
		t.Fatal(err)
	}
	saveTraining(g, dog, training.Maintenance{DecayAt: 1 << 40, HaulProgress: training.Needed(training.Haul) | herd.TrainToggled})
	for i := 0; i < 3000 && trainingAt(g, dog).Task.Carry == 0; i++ {
		g.Step(1)
	}
	task := trainingAt(g, dog).Task
	if task.Carry == 0 {
		t.Fatalf("no physical pickup: %+v", task)
	}
	st, ok := g.stacks.Get(task.Carry)
	if !ok || st.Carrier != dog || st.Qty != 5 {
		t.Fatalf("invalid carried stack: %+v", st)
	}
	loaded := saveLoad(t, g)
	for _, sim := range []*Game{g, loaded} {
		for i := 0; i < 3000 && trainingAt(sim, dog).Task.Source != 0; i++ {
			sim.Step(1)
		}
		if trainingAt(sim, dog).Task.Source != 0 {
			t.Fatal("haul failed to complete")
		}
		st, ok := sim.stacks.Get(task.Carry)
		if !ok || st.Carrier != 0 || st.Qty != 5 || st.X != task.X || st.Y != task.Y {
			t.Fatalf("invalid delivered stack: %+v", st)
		}
		if _, ok := sim.reserve.TileOwner(task.X, task.Y); ok {
			t.Fatal("destination claim leaked")
		}
	}
	if g.StateHash() != loaded.StateHash() {
		t.Fatal("haul save/replay diverged")
	}
}
