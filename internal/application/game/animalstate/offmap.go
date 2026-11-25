// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package animalstate

import (
	"fmt"
	"reflect"
	"slices"

	"rimworld-go/internal/application/game/travel"
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/sim/entity"
)

// OffMapRef identifies an animal held in a world stash. The owner
// supplies a detached copy of the saved state before conversion.
type OffMapRef struct {
	ID    entity.EntityID
	State *animal.State
}

// CloneTravel copies every animal-bearing collection before the save
// layer substitutes record data. Runtime travel state remains untouched.
func CloneTravel(v *travel.Save) *travel.Save {
	if v == nil {
		return nil
	}
	out := *v
	out.Holds = slices.Clone(v.Holds)
	for i := range out.Holds {
		out.Holds[i].Hold.Animals = slices.Clone(v.Holds[i].Hold.Animals)
	}
	out.Cargo = slices.Clone(v.Cargo)
	for i := range out.Cargo {
		out.Cargo[i].Hold.Animals = slices.Clone(v.Cargo[i].Hold.Animals)
	}
	out.Flights = slices.Clone(v.Flights)
	for i := range out.Flights {
		out.Flights[i].Load.Animals = slices.Clone(v.Flights[i].Load.Animals)
	}
	out.Roped = slices.Clone(v.Roped)
	return &out
}

// visitTravel covers all animal payloads in a deterministic save order.
func visitTravel(v *travel.Save, use func(*travel.Beast) error) error {
	if v == nil {
		return nil
	}
	for i := range v.Holds {
		for j := range v.Holds[i].Hold.Animals {
			if err := use(&v.Holds[i].Hold.Animals[j]); err != nil {
				return err
			}
		}
	}
	for i := range v.Cargo {
		for j := range v.Cargo[i].Hold.Animals {
			if err := use(&v.Cargo[i].Hold.Animals[j]); err != nil {
				return err
			}
		}
	}
	for i := range v.Flights {
		for j := range v.Flights[i].Load.Animals {
			if err := use(&v.Flights[i].Load.Animals[j]); err != nil {
				return err
			}
		}
	}
	for i := range v.Roped {
		if err := use(&v.Roped[i].Beast); err != nil {
			return err
		}
	}
	return nil
}

// ExportWorld moves all off-map animal payloads into one grouped list.
// Legacy Beast state fields remain as empty location markers only.
func ExportWorld(stashed []OffMapRef, travelSave *travel.Save) ([]Record, error) {
	seen := make(map[entity.EntityID]bool)
	var records []Record
	add := func(id entity.EntityID, a *animal.State) error {
		if id == 0 || seen[id] {
			return fmt.Errorf("off-map animal: duplicate or zero ID %d", id)
		}
		seen[id] = true
		records = append(records, Record{ID: id, State: animal.SnapshotFromState(*a)})
		*a = animal.State{}
		return nil
	}
	for _, r := range stashed {
		if r.State != nil {
			if err := add(r.ID, r.State); err != nil {
				return nil, err
			}
		}
	}
	if err := visitTravel(travelSave, func(b *travel.Beast) error {
		return add(b.ID, &b.State)
	}); err != nil {
		return nil, err
	}
	return records, nil
}

// ImportWorld validates the v19 grouped off-map payloads and fills the
// existing world/travel state before the runtime's normal import.
func ImportWorld(grouped bool, stashed []OffMapRef, travelSave *travel.Save, records []Record) error {
	if !grouped {
		if len(records) != 0 {
			return fmt.Errorf("off-map animal: legacy save contains grouped records")
		}
		return nil
	}
	present := make(map[entity.EntityID]animal.Snapshot, len(records))
	for _, r := range records {
		if r.ID == 0 {
			return fmt.Errorf("off-map animal: zero record ID")
		}
		if _, duplicate := present[r.ID]; duplicate {
			return fmt.Errorf("off-map animal: duplicate record %d", r.ID)
		}
		present[r.ID] = r.State
	}
	used := make(map[entity.EntityID]bool, len(records))
	get := func(id entity.EntityID, a *animal.State) error {
		s, ok := present[id]
		if !ok || used[id] {
			return fmt.Errorf("off-map animal: missing or duplicated animal %d", id)
		}
		if !reflect.ValueOf(*a).IsZero() {
			return fmt.Errorf("off-map animal: mixed legacy and grouped data for %d", id)
		}
		used[id] = true
		*a = s.State()
		return nil
	}
	for _, r := range stashed {
		if r.State != nil {
			if err := get(r.ID, r.State); err != nil {
				return err
			}
		}
	}
	if err := visitTravel(travelSave, func(b *travel.Beast) error {
		return get(b.ID, &b.State)
	}); err != nil {
		return err
	}
	if len(used) != len(records) {
		return fmt.Errorf("off-map animal: %d unreferenced grouped records", len(records)-len(used))
	}
	return nil
}
