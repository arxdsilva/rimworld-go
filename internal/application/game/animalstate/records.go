// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

// Package animalstate owns the authoritative nine-component animal
// registry and the grouped animal map and off-map persistence boundary.
package animalstate

import (
	"fmt"

	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/sim/entity"
)

type Record struct {
	ID    entity.EntityID
	State animal.Snapshot
}

func Export(store *Registry) []Record {
	out := make([]Record, 0, store.Len())
	store.All(func(id entity.EntityID, a animal.State) {
		out = append(out, Record{ID: id, State: animal.SnapshotFromState(a)})
	})
	return out
}

func Import(records []Record) *Registry {
	store := New()
	for _, r := range records {
		store.Set(r.ID, r.State.State())
	}
	return store
}

// Check verifies every saved animal record has exactly one registered ID.
// The stores can iterate in a different order from the public animal list.
func Check(ids []entity.EntityID, records []Record) error {
	if len(ids) != len(records) {
		return fmt.Errorf("animal records: %d IDs, %d records", len(ids), len(records))
	}
	registered := make(map[entity.EntityID]bool, len(ids))
	for _, id := range ids {
		if id == 0 || registered[id] {
			return fmt.Errorf("animal records: duplicate or zero registered ID %d", id)
		}
		registered[id] = true
	}
	seen := make(map[entity.EntityID]bool, len(records))
	for _, r := range records {
		if !registered[r.ID] || seen[r.ID] {
			return fmt.Errorf("animal records: unknown or duplicate record ID %d", r.ID)
		}
		seen[r.ID] = true
	}
	return nil
}

// CheckSave validates both native v21 wire representations: historical
// flat records with format 0, or new grouped records with format 1.
// Earlier versions used the flat records.
func CheckSave(version int, format uint8, ids []entity.EntityID, grouped []Record, legacy int) error {
	// The upstream source switched at v19. This Core already uses v21;
	// legacy v21 saves have AnimalFormat=0 and must remain readable.
	switch format {
	case 0:
		if len(grouped) != 0 {
			return fmt.Errorf("animal save format: v%d legacy data mixed with grouped records", version)
		}
		return nil
	case 1:
		if version < 21 || legacy != 0 {
			return fmt.Errorf("animal save format: v%d has incompatible grouped records", version)
		}
		return Check(ids, grouped)
	default:
		return fmt.Errorf("animal save format: unknown format %d", format)
	}
}

// CheckGlobal rejects duplicate animal ownership across maps and off-map
// stores. IDs remain global across the entire colony and the world.
func CheckGlobal(maps [][]entity.EntityID, offMap []Record) error {
	seen := make(map[entity.EntityID]bool)
	check := func(id entity.EntityID) error {
		if id == 0 || seen[id] {
			return fmt.Errorf("animal saved in multiple places: %d", id)
		}
		seen[id] = true
		return nil
	}
	for _, ids := range maps {
		for _, id := range ids {
			if err := check(id); err != nil {
				return err
			}
		}
	}
	for _, r := range offMap {
		if err := check(r.ID); err != nil {
			return err
		}
	}
	return nil
}
