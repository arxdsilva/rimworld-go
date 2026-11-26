// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"bytes"
	"encoding/gob"
	"reflect"
	"testing"

	"rimworld-go/internal/application/game/animalstate"
	"rimworld-go/internal/application/game/savefmt"
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/animal/training"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/sim/statehash"
)

func animalSaveState(t *testing.T, g *Game) (SaveInfo, saveState) {
	t.Helper()
	var raw bytes.Buffer
	if err := EncodeSave(g, &raw, "animal migration"); err != nil {
		t.Fatal(err)
	}
	dec := gob.NewDecoder(&raw)
	var info SaveInfo
	var state saveState
	if err := dec.Decode(&info); err != nil {
		t.Fatal(err)
	}
	if err := dec.Decode(&state); err != nil {
		t.Fatal(err)
	}
	return info, state
}

func loadAnimalStateFile(t *testing.T, info SaveInfo, state saveState) (*Game, error) {
	t.Helper()
	var buf bytes.Buffer
	zw := savefmt.NewWriter(&buf)
	enc := gob.NewEncoder(zw)
	if err := enc.Encode(info); err != nil {
		t.Fatal(err)
	}
	if err := enc.Encode(state); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	g, _, err := Load(&buf)
	return g, err
}

func legacyAnimalFormat(s *saveState) {
	s.Animals = make([]entry[animal.State], 0, len(s.AnimalRecords))
	for _, r := range s.AnimalRecords {
		s.Animals = append(s.Animals, entry[animal.State]{ID: r.ID, V: r.State.State()})
	}
	s.AnimalFormat, s.AnimalRecords = 0, nil
	if s.World != nil {
		if err := animalstate.ImportWorld(true, worldAnimalRefs(s.World), s.World.AX9, s.WorldAnimalRecords); err != nil {
			panic(err)
		}
		s.WorldAnimalRecords = nil
	}
	for i := range s.ExtraMaps {
		legacyAnimalFormat(&s.ExtraMaps[i].State)
	}
}

func TestGroupedAnimalSaveLoadAndLegacyV21Migration(t *testing.T) {
	g := New(Config{Seed: 143, Width: 24, Height: 16})
	id, err := g.SpawnAnimal("husky", 7, 7)
	if err != nil {
		t.Fatal(err)
	}
	g.TameAnimal(id, "Parcel")
	a, _ := g.animals.Get(id)
	a.Injury = 2
	a.Trained = [training.LegacySkills]uint8{3, 1, 0}
	a.Mated = 2
	a.Pregnant = uint64(g.clock.Now()) + 300
	a.Body = animal.NewBody(a.Species)
	g.animals.Set(id, a)
	info, state := animalSaveState(t, g)
	if info.Version != SaveVersion || state.AnimalFormat != 1 ||
		len(state.Animals) != 0 || len(state.AnimalRecords) != 1 {
		t.Fatalf("writer: version=%d marker=%d legacy=%d grouped=%d",
			info.Version, state.AnimalFormat, len(state.Animals), len(state.AnimalRecords))
	}
	if got := state.AnimalRecords[0].State.State(); !reflect.DeepEqual(a, got) {
		t.Fatal("grouped record changed animal")
	}
	loaded, err := loadAnimalStateFile(t, info, state)
	if err != nil {
		t.Fatal(err)
	}
	if got, ok := loaded.animals.Get(id); !ok || !reflect.DeepEqual(a, got) {
		t.Fatalf("grouped load changed animal: %+v", got)
	}
	old := state
	legacyAnimalFormat(&old)
	oldInfo := info
	oldInfo.Version = SaveVersion
	legacy, err := loadAnimalStateFile(t, oldInfo, old)
	if err != nil {
		t.Fatalf("legacy v21 animal migration: %v", err)
	}
	if got, ok := legacy.animals.Get(id); !ok || !reflect.DeepEqual(a, got) {
		t.Fatalf("legacy animal changed: %+v", got)
	}
	upgradeInfo, upgradeState := animalSaveState(t, legacy)
	again, err := loadAnimalStateFile(t, upgradeInfo, upgradeState)
	if err != nil {
		t.Fatal(err)
	}
	if got, ok := again.animals.Get(id); !ok || !reflect.DeepEqual(a, got) {
		t.Fatalf("legacy v21 -> grouped v21 -> load changed animal: %+v", got)
	}
	if h1, h2 := statehash.OfFields(normalisedFields(info, &state)), statehash.OfFields(normalisedFields(oldInfo, &old)); h1 != h2 {
		t.Fatalf("wire migration changed logical hash: %x vs %x", h1, h2)
	}
}

func TestGroupedAnimalSaveExtraMap(t *testing.T) {
	g, _, _, _, _, deer := twoMaps(t)
	info, state := animalSaveState(t, g)
	if state.AnimalFormat != 1 || len(state.ExtraMaps) != 1 ||
		state.ExtraMaps[0].State.AnimalFormat != 1 ||
		len(state.ExtraMaps[0].State.AnimalRecords) != 1 {
		t.Fatal("extra map lost grouped animal")
	}
	h, err := loadAnimalStateFile(t, info, state)
	if err != nil || len(h.maps) != 2 || !h.maps[1].animals.Has(deer) {
		t.Fatalf("grouped extra-map load: %v", err)
	}
}

func TestGroupedAnimalSaveRejectsInvalidRecords(t *testing.T) {
	g := New(Config{Seed: 144, Width: 24, Height: 16})
	if _, err := g.SpawnAnimal("husky", 6, 6); err != nil {
		t.Fatal(err)
	}
	info, state := animalSaveState(t, g)
	cases := []struct {
		name string
		edit func(*saveState)
	}{
		{"missing marker", func(s *saveState) { s.AnimalFormat = 0 }},
		{"missing record", func(s *saveState) { s.AnimalRecords = nil }},
		{"duplicate", func(s *saveState) { s.AnimalRecords = append(s.AnimalRecords, s.AnimalRecords[0]) }},
		{"foreign", func(s *saveState) { s.AnimalRecords[0].ID = entity.EntityID(199) }},
		{"legacy overlap", func(s *saveState) { s.Animals = []entry[animal.State]{{ID: s.AnimalIDs[0]}} }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			corrupt := state
			corrupt.AnimalRecords = append([]animalstate.Record(nil), state.AnimalRecords...)
			tc.edit(&corrupt)
			if _, err := loadAnimalStateFile(t, info, corrupt); err == nil {
				t.Fatal("accepted corrupt grouped animal save")
			}
		})
	}
}
