// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"reflect"
	"testing"

	"rimworld-go/internal/application/game/animalstate"
	"rimworld-go/internal/application/game/travel"
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/animal/training"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/sim/statehash"
)

func TestGroupedOffMapStashSurvivesSaveAndReturn(t *testing.T) {
	g := New(Config{Seed: 151, Width: 24, Height: 16})
	id, err := g.SpawnAnimal("husky", 8, 8)
	if err != nil {
		t.Fatal(err)
	}
	g.TameAnimal(id, "Courier")
	a, _ := g.animals.Get(id)
	a.Trained = [training.LegacySkills]uint8{3, 2, 1}
	a.Injury, a.Pregnant, a.Mated = 2, uint64(g.clock.Now())+500, 2
	g.animals.Set(id, a)
	if !g.stashPawn(id, 99) {
		t.Fatal("cannot move tame animal to world stash")
	}
	original := *g.worldSt.stashed[0].Beast
	info, state := animalSaveState(t, g)
	if len(state.WorldAnimalRecords) != 1 ||
		!reflect.ValueOf(*state.World.Stashed[0].Beast).IsZero() {
		t.Fatal("new stash save contains a legacy animal payload")
	}
	if !reflect.DeepEqual(original, *g.worldSt.stashed[0].Beast) {
		t.Fatal("save mutated live stashed animal")
	}
	loaded, err := loadAnimalStateFile(t, info, state)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.worldSt == nil || len(loaded.worldSt.stashed) != 1 ||
		!reflect.DeepEqual(*loaded.worldSt.stashed[0].Beast, original) {
		t.Fatal("grouped world stash lost animal state")
	}
	legacy := state
	legacyAnimalFormat(&legacy)
	oldInfo := info
	oldInfo.Version = SaveVersion
	former, err := loadAnimalStateFile(t, oldInfo, legacy)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(*former.worldSt.stashed[0].Beast, original) {
		t.Fatal("legacy v21 stash migration lost animal state")
	}
	if !loaded.unstashPawn(id, 9, 9) {
		t.Fatal("animal failed to return from grouped stash")
	}
	got, ok := loaded.animals.Get(id)
	if !ok || got.Trained != a.Trained || got.Pregnant != a.Pregnant ||
		got.Mated != a.Mated || got.Injury != a.Injury || got.Name != a.Name {
		t.Fatalf("returned animal state changed: %+v", got)
	}
}

func TestGroupedOffMapTransportSurvivesSave(t *testing.T) {
	cases := []struct {
		name  string
		build func(entity.EntityID, animal.State) *travel.Save
		read  func(*travel.Save) animal.State
	}{
		{"pod", func(id entity.EntityID, a animal.State) *travel.Save {
			return &travel.Save{Holds: []travel.PodHold{{Pod: 99, Hold: travel.Hold{
				Animals: []travel.Beast{{ID: id, State: a}},
			}}}}
		}, func(s *travel.Save) animal.State { return s.Holds[0].Hold.Animals[0].State }},
		{"caravan", func(id entity.EntityID, a animal.State) *travel.Save {
			return &travel.Save{Cargo: []travel.CaravanCargo{{Caravan: 99, Hold: travel.Hold{
				Animals: []travel.Beast{{ID: id, State: a}},
			}}}}
		}, func(s *travel.Save) animal.State { return s.Cargo[0].Hold.Animals[0].State }},
		{"flight", func(id entity.EntityID, a animal.State) *travel.Save {
			return &travel.Save{Flights: []travel.Flight{{ID: 99, Load: travel.Hold{
				Animals: []travel.Beast{{ID: id, State: a}},
			}}}}
		}, func(s *travel.Save) animal.State { return s.Flights[0].Load.Animals[0].State }},
		{"rope", func(id entity.EntityID, a animal.State) *travel.Save {
			return &travel.Save{Roped: []travel.RopedBeast{{Hauler: 99, Beast: travel.Beast{ID: id, State: a}}}}
		}, func(s *travel.Save) animal.State { return s.Roped[0].Beast.State }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			g := New(Config{Seed: 152, Width: 24, Height: 16})
			id, err := g.SpawnAnimal("husky", 8, 8)
			if err != nil {
				t.Fatal(err)
			}
			g.TameAnimal(id, "Passenger")
			a, _ := g.animals.Get(id)
			a.Injury, a.Trained[training.Guard] = 7, 3
			g.animals.Set(id, a)
			g.removeAnimal(id)
			g.worldS().ax9 = travel.Import(tc.build(id, a))
			info, state := animalSaveState(t, g)
			if state.World == nil || state.World.AX9 == nil || len(state.WorldAnimalRecords) != 1 ||
				!reflect.ValueOf(tc.read(state.World.AX9)).IsZero() {
				t.Fatalf("new %s save retained legacy animal data", tc.name)
			}
			if original := tc.read(g.worldSt.ax9.Export()); !reflect.DeepEqual(original, a) {
				t.Fatal("saving overwrote live transport animal")
			}
			h, err := loadAnimalStateFile(t, info, state)
			if err != nil {
				t.Fatal(err)
			}
			if got := tc.read(h.worldSt.ax9.Export()); !reflect.DeepEqual(got, a) {
				t.Fatalf("transport animal changed after grouped load: %+v", got)
			}
			old := state
			legacyAnimalFormat(&old)
			oldInfo := info
			oldInfo.Version = SaveVersion
			legacy, err := loadAnimalStateFile(t, oldInfo, old)
			if err != nil || !reflect.DeepEqual(tc.read(legacy.worldSt.ax9.Export()), a) {
				t.Fatalf("legacy v21 %s transport migration: %v", tc.name, err)
			}
		})
	}
}

func TestGroupedOffMapRejectsIncompleteOrMixedWorldRecords(t *testing.T) {
	g := New(Config{Seed: 153, Width: 24, Height: 16})
	id, err := g.SpawnAnimal("husky", 8, 8)
	if err != nil {
		t.Fatal(err)
	}
	g.TameAnimal(id, "Lost")
	if !g.stashPawn(id, 99) {
		t.Fatal("cannot stash animal")
	}
	info, state := animalSaveState(t, g)
	for _, tc := range []struct {
		name string
		edit func(*saveState)
	}{
		{"missing record", func(s *saveState) { s.WorldAnimalRecords = nil }},
		{"duplicate", func(s *saveState) { s.WorldAnimalRecords = append(s.WorldAnimalRecords, s.WorldAnimalRecords[0]) }},
		{"foreign record", func(s *saveState) { s.WorldAnimalRecords[0].ID = 999 }},
		{"legacy mixed", func(s *saveState) { *s.World.Stashed[0].Beast = animal.State{Species: animal.Labrador} }},
		{"missing world", func(s *saveState) { s.World = nil }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			corrupt := state
			world := *state.World
			world.Stashed = append([]stashRec(nil), state.World.Stashed...)
			animalMarker := *world.Stashed[0].Beast
			world.Stashed[0].Beast = &animalMarker
			corrupt.World = &world
			corrupt.WorldAnimalRecords = append([]animalstate.Record(nil), state.WorldAnimalRecords...)
			tc.edit(&corrupt)
			if _, err := loadAnimalStateFile(t, info, corrupt); err == nil {
				t.Fatal("accepted corrupt grouped off-map animal save")
			}
		})
	}
}

func TestGroupedOffMapHashMatchesLegacyV21Save(t *testing.T) {
	g := New(Config{Seed: 154, Width: 24, Height: 16})
	id, err := g.SpawnAnimal("husky", 8, 8)
	if err != nil {
		t.Fatal(err)
	}
	g.TameAnimal(id, "Hash")
	if !g.stashPawn(id, 99) {
		t.Fatal("could not stash animal")
	}
	info, grouped := animalSaveState(t, g)
	want := statehash.OfFields(normalisedFields(info, &grouped))

	legacy := grouped
	legacyWorld := *grouped.World
	legacyWorld.Stashed = append([]stashRec(nil), grouped.World.Stashed...)
	legacyWorld.AX9 = animalstate.CloneTravel(grouped.World.AX9)
	legacy.World = &legacyWorld
	legacyAnimalFormat(&legacy)
	oldInfo := info
	oldInfo.Version = SaveVersion
	if got := statehash.OfFields(normalisedFields(oldInfo, &legacy)); got != want {
		t.Fatalf("off-map wire conversion changed gameplay hash: grouped=%x legacy=%x", want, got)
	}
}
