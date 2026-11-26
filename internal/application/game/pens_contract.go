// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"rimworld-go/internal/application/game/mapview"
	"rimworld-go/internal/application/game/saveutil"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
	"slices"
)

// PLAY-015 pens contract (pens.go fills the behaviour).

// Player-facing pen settings and rendering fields belong to mapview.
type PenSpecies = mapview.PenSpecies
type PenSettings = mapview.PenSettings
type PenView = mapview.PenView

func EncodePenSettings(s PenSettings) string         { return mapview.EncodePenSettings(s) }
func DecodePenSettings(s string) (PenSettings, bool) { return mapview.DecodePenSettings(s) }

// penAnimalSave is one penned animal's unpenned clock (PLAY-015): Since
// is the tick it last lost its pen (roping or penning resets it).
type penAnimalSave struct {
	Animal entity.EntityID
	Since  uint64
}

// penSave is one pen marker's saved settings.
type penSave struct {
	Marker   entity.EntityID
	Settings PenSettings
	Edit     command.ConfigEdit
}

func recordConfigEdit(g *Game, c command.Command, id entity.EntityID, stamp command.ConfigEdit) {
	if stamp.Revision != 0 {
		g.configEdits = append(g.configEdits, command.ConfigMutation{Map: g.id, Owner: uint64(id), Edit: stamp, Command: c})
	}
}

// Configuration reads actual current native settings on the viewed map.
// Call on the simulation owner goroutine, between Step/ApplyPending calls.
func Configuration(g *Game) mapview.Configuration {
	v := mapview.Configuration{Map: g.id, Tick: uint64(g.Tick()), Mutations: slices.Clone(g.configEdits)}
	for _, id := range g.bldgIDs {
		b, ok := g.bldgs.Get(id)
		if !ok {
			continue
		}
		if construction.DefOf(b.Kind).Bench {
			_, spot, has := construction.Footprint(b.Kind, b.Facing, b.X, b.Y)
			rows, stamp := g.craft.Bills.Configuration(id)
			v.Benches = append(v.Benches, mapview.BenchConfig{ID: id, Kind: b.Kind, X: b.X, Y: b.Y, WorkSpot: spot, HasWorkSpot: has, Bills: rows, LastEdit: stamp})
		}
		if b.Kind == construction.FeedingTrough {
			t, init := g.troughs[id]
			m, _ := g.troughMarker(b)
			v.Troughs = append(v.Troughs, mapview.TroughConfig{ID: id, Pen: m, X: b.X, Y: b.Y, Filter: t.Filter, Initialized: init, LastEdit: t.Edit})
		}
	}
	for _, p := range g.penViews() {
		entry := mapview.PenConfig{PenView: mapview.DetachPen(p), LastEdit: g.pens[g.penIndex(p.ID)].Edit}
		c := g.penRegion(p.ID)
		for _, aid := range g.animalIDs {
			a, pos := g.animals.Ref(aid), g.positions.Ref(aid)
			if a != nil && !a.Dead() && pos != nil && g.inPenCells(c, pos.X, pos.Y) {
				entry.Occupants = append(entry.Occupants, aid)
			}
		}
		v.Pens = append(v.Pens, entry)
	}
	return v
}

func cloneSavePen(p penSave) penSave {
	p.Settings = saveutil.ClonePenSettings(p.Settings)
	return p
}
