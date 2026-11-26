// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package animalstate

import (
	"reflect"
	"testing"

	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/sim/entity"
)

func TestRecordsFollowStoreOrderAndRestore(t *testing.T) {
	s := New()
	a := animal.State{Species: animal.Labrador, Tame: true, Name: "A", Trained: [3]uint8{3, 2, 1}}
	b := animal.State{Species: animal.Goat, Tame: true, Pregnant: 77, Mated: 2}
	s.Set(20, a)
	s.Set(3, b)
	records := Export(s)
	if len(records) != 2 || records[0].ID != 20 || records[1].ID != 3 {
		t.Fatalf("store iteration changed: %+v", records)
	}
	if err := Check([]entity.EntityID{3, 20}, records); err != nil {
		t.Fatal(err)
	}
	restored := Import(records)
	for _, pair := range []struct {
		id    entity.EntityID
		state animal.State
	}{{20, a}, {3, b}} {
		got, ok := restored.Get(pair.id)
		if !ok || !reflect.DeepEqual(got, pair.state) {
			t.Errorf("animal %d changed: %+v", pair.id, got)
		}
	}
}

func TestCheckRejectsMissingDuplicateAndForeignAnimals(t *testing.T) {
	record := Record{ID: 8}
	for _, tc := range []struct {
		ids     []entity.EntityID
		records []Record
	}{
		{[]entity.EntityID{8}, nil},
		{[]entity.EntityID{8, 8}, []Record{record, record}},
		{[]entity.EntityID{0}, []Record{{ID: 0}}},
		{[]entity.EntityID{8}, []Record{{ID: 9}}},
		{[]entity.EntityID{8, 9}, []Record{record, record}},
	} {
		if err := Check(tc.ids, tc.records); err == nil {
			t.Errorf("accepted invalid animal records: ids=%v records=%v", tc.ids, tc.records)
		}
	}
}

func TestCheckGlobalRejectsAnimalInTwoLocations(t *testing.T) {
	if err := CheckGlobal([][]entity.EntityID{{1}, {2}}, []Record{{ID: 3}}); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		maps [][]entity.EntityID
		off  []Record
	}{
		{[][]entity.EntityID{{1}, {1}}, nil},
		{[][]entity.EntityID{{1}, {2}}, []Record{{ID: 2}}},
		{[][]entity.EntityID{{0}}, nil},
		{[][]entity.EntityID{{1}}, []Record{{ID: 0}}},
	} {
		if err := CheckGlobal(tc.maps, tc.off); err == nil {
			t.Errorf("accepted duplicate/invalid animal ownership: %+v", tc)
		}
	}
}
