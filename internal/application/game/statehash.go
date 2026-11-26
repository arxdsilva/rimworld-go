// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"fmt"
	"reflect"
	"slices"

	"rimworld-go/internal/application/game/animalstate"
	"rimworld-go/internal/application/game/savefmt"
	"rimworld-go/internal/application/game/saveutil"
	"rimworld-go/internal/application/game/visit"
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/sim/statehash"
	"rimworld-go/internal/domain/world/temperature"
)

// StateHash hashes authoritative save fields without a gob round trip.
// Canonical normalization excludes representation-only fields and caches.
func (g *Game) StateHash() uint64 {
	info, s, err := snapshotForSave(g, savefmt.Stamp{Name: "statehash"}, nil)
	if err != nil {
		panic(fmt.Errorf("state hash: %w", err))
	}
	return statehash.OfFields(normalisedFields(info, s))
}

// normalisedFields folds old save representations into stable named fields.
// The header uses its v1 identity; per-map state is hashed recursively.
func normalisedFields(info SaveInfo, s *saveState) map[string]any {
	// The active animal registry writes grouped components. Hash the same
	// detached animal.State representation as the existing native Core.
	normal := *s
	if s.AnimalFormat == 1 {
		normal.Animals = make([]entry[animal.State], 0, len(s.AnimalRecords))
		for _, r := range s.AnimalRecords {
			normal.Animals = append(normal.Animals, entry[animal.State]{ID: r.ID, V: r.State.State()})
		}
		normal.AnimalFormat, normal.AnimalRecords = 0, nil
		if s.World != nil {
			world := *s.World
			world.Stashed = slices.Clone(s.World.Stashed)
			world.AX9 = animalstate.CloneTravel(s.World.AX9)
			if err := animalstate.ImportWorld(true, worldAnimalRefs(&world), world.AX9, s.WorldAnimalRecords); err != nil {
				panic(fmt.Errorf("state hash off-map animals: %w", err))
			}
			normal.World = &world
			normal.WorldAnimalRecords = nil
		}
	}
	normal.Bills = saveutil.NilEmptyMapValues(s.Bills)
	normal.OrderQueue = saveutil.NilEmptyMapValues(s.OrderQueue)
	normal.PrioList = saveutil.NilEmptyMapValues(s.PrioList)
	normal.Logs = saveutil.NilEmptyMapValues(s.Logs)
	normal.Deliveries = saveutil.NilEmptyMapValues(s.Deliveries)
	normal.RNGStreams = saveutil.NilEmptyMapValues(s.RNGStreams)
	s = &normal
	f := saveutil.ExportedFields(s, infoV1{1, info.Name, info.Date, info.Day, info.Colonists})
	// Grouped save records and their marker are wire representations.
	delete(f, "AnimalFormat")
	delete(f, "AnimalRecords")
	delete(f, "WorldAnimalRecords")
	// Preserve logical state-hash field names across v21's gob changes.
	f["Bios"], f["Skills"], f["SkillLive"] = s.Bios21, s.Skills21, s.SkillLive21
	f["NG1"], f["People"] = s.NG121, s.People21
	for _, key := range [...]string{"Bios21", "Skills21", "SkillLive21", "NG121", "People21"} {
		delete(f, key)
	}
	saveutil.HashLabor[legacyLaborMask, entry[legacyLaborMask]](f, s.LaborWide, s.PrioList)
	f["Blocked"] = s.blockedTiles()
	f["Roofs"] = saveutil.LegacyRoofHashCells(s.W, s.H, s.Terr, s.Bldgs, s.Roofs)
	wildlife := s.Wildlife
	wildlife.WeightSchema = false
	wildlife.Forage.Skip = saveutil.NilEmptyMapValues(wildlife.Forage.Skip)
	// Gob drops runtime creature indexes and behavior hosts.
	wildlife.herd, wildlife.rage = nil, nil
	wildlife.idx, wildlife.food, wildlife.ids = creatureIndex{}, foodIndex{}, nil
	f["Wildlife"] = wildlife
	saveutil.CanonicalHashFields(f, streamSeedKey)
	delete(f, "CmdSeq") // #8: pending envelopes carry their own sequence
	if s.Trade != nil {
		t := *s.Trade
		t.Version = 1 // the ledger's wire version is not gameplay state
		if t.Caravan != nil {
			c := *t.Caravan
			// The initial per-member manifest is derivable from saved Stock
			// and Members; only changes to that distribution affect replay.
			if reflect.DeepEqual(c.Cargo, visit.AssignCargo(c.Stock, c.Members)) {
				c.Cargo = nil
			}
			t.Caravan = &c
		}
		f["Trade"] = &t
	}
	if len(s.ExtraMaps) > 0 {
		hs := make([]extraHash, len(s.ExtraMaps))
		for i := range s.ExtraMaps {
			m := &s.ExtraMaps[i]
			hs[i] = extraHash{m.ID, m.Tile, m.Climate, statehash.OfFields(normalisedFields(SaveInfo{}, &m.State))}
		}
		f["ExtraMaps"] = hs
	}
	return f
}

// extraHash is an extra map in the hash: its fields' hash beside its id,
// tile and climate.
type extraHash struct {
	ID      uint32
	Tile    int
	Climate temperature.Climate
	State   uint64
}

// infoV1 is SaveInfo as version 1 wrote it (the hashed header).
type infoV1 struct {
	Version        int
	Name, Date     string
	Day, Colonists int
}
