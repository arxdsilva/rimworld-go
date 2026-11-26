// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"bytes"
	"encoding/gob"
	"testing"

	"rimworld-go/internal/application/game/savefmt"
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/animal/training"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
)

// legacyTrainedWolfSave writes a v20 file (before training decay) with a
// tame wolf on the map and one off it, each with one obedience step
// learned 60 days before the save tick: v20 had no training-maintenance record and the old animal layout.
func legacyTrainedWolfSave(t *testing.T) (raw []byte, home, away entity.EntityID) {
	t.Helper()
	g := New(Config{Seed: 935, Width: 24, Height: 18})
	for i, name := range []string{"Oldtimer", "Traveler"} {
		wolf, err := g.SpawnAnimal("timber_wolf", 8+i, 8)
		if err != nil {
			t.Fatal(err)
		}
		g.TameAnimal(wolf, name)
		a, _ := g.animals.Get(wolf)
		a.Trained = [training.LegacySkills]uint8{}
		a.Trained[training.Obedience] = 1
		a.TrainDecay = max(uint64(g.clock.Now()), 1) // the last training session
		g.animals.Set(wolf, a)
		home, away = away, wolf
	}
	if !g.stashPawn(away, 99) { // off-map with a caravan
		t.Fatal("unable to stash trained animal")
	}
	info, s := animalSaveState(t, g)
	legacyAnimalFormat(&s)
	info.Version = 20
	s.TrainingMaintenance = nil
	s.Tick += 60 * clock.TicksPerDay // saved 60 days after that session
	var buf bytes.Buffer
	zw := savefmt.NewWriter(&buf)
	enc := gob.NewEncoder(zw)
	if err := enc.Encode(info); err != nil {
		t.Fatal(err)
	}
	if err := enc.Encode(s); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes(), home, away
}

func loadLegacyV20(t *testing.T, raw []byte) *Game {
	t.Helper()
	g, info, err := Load(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	if info.Version != 20 {
		t.Fatalf("fixture version %d, want 20", info.Version)
	}
	return g
}

// TestLegacyTrainedAnimalKeepsTamenessOnLoad: a pre-decay save starts the
// decay timer at the loaded tick, so the animal neither loses its step nor
// goes feral on its first decide slot after load.
func TestLegacyTrainedAnimalKeepsTamenessOnLoad(t *testing.T) {
	raw, wolf, away := legacyTrainedWolfSave(t)
	g := loadLegacyV20(t, raw)
	loadTick := uint64(g.clock.Now())
	before, ok := g.animals.Get(wolf)
	if !ok || !before.Tame || before.Trained[training.Obedience] != 1 {
		t.Fatalf("fixture animal: %+v", before)
	}
	interval := training.DecayInterval(animal.Wildness(before.Species))
	g.Step(2 * animal.DecideEvery)
	after, ok := g.animals.Get(wolf)
	if !ok || !after.Tame || after.Name != "Oldtimer" {
		t.Fatalf("pre-decay animal went feral on load: %+v", after)
	}
	if after.Trained != before.Trained {
		t.Fatalf("training lost on load: %v, want %v", after.Trained, before.Trained)
	}
	m := g.worldS().trainingMaintenance[wolf]
	if m.TameLoss != 0 || m.DecayAt != loadTick+interval {
		t.Fatalf("maintenance %+v, want no tame loss and DecayAt %d", m, loadTick+interval)
	}
	if m := g.worldS().trainingMaintenance[away]; m.TameLoss != 0 || m.DecayAt != loadTick+interval {
		t.Fatalf("off-map maintenance %+v, want DecayAt %d", m, loadTick+interval)
	}
	if !g.unstashPawn(away, 12, 8) {
		t.Fatal("unable to return the off-map animal")
	}
	g.Step(2 * animal.DecideEvery)
	if back, ok := g.animals.Get(away); !ok || !back.Tame || back.Trained != before.Trained {
		t.Fatalf("returned pre-decay animal lost training or tameness: %+v", back)
	}
}

// TestLegacyTrainingDecayLoadDeterministic: two loads of the same v20 file
// hash the same after stepping past the decay seed and decide slots.
func TestLegacyTrainingDecayLoadDeterministic(t *testing.T) {
	raw, _, _ := legacyTrainedWolfSave(t)
	first, second := loadLegacyV20(t, raw), loadLegacyV20(t, raw)
	if first.StateHash() != second.StateHash() {
		t.Fatal("two loads of one v20 file differ")
	}
	for _, g := range []*Game{first, second} {
		g.Step(5 * animal.DecideEvery)
	}
	if first.StateHash() != second.StateHash() {
		t.Fatal("two loads of one v20 file diverged after stepping")
	}
	again := saveLoad(t, first)
	if again.StateHash() != first.StateHash() {
		t.Fatal("migrated training maintenance changed on save and reload")
	}
}
