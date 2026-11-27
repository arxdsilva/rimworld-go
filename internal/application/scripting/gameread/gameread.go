// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

// Package gameread answers the scripting.Queries readers from a game's
// exported reads (wild animals, the map's terrain, temperature and
// weather, factions, sites, building counts), each capped at
// scripting.MaxQueryRows. The game swaps to its home map during a script
// call, so every read here is the home map's. It sits outside game/ so it
// may import the game root; the composition root builds it.
package gameread

import (
	"cmp"
	"slices"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/application/scripting"
)

// reader is every Queries reader over one game. It holds no state of its
// own and caches nothing: a reader is valid only during Fire (and the
// other scripted calls), when the game has swapped to its home map.
type reader struct{ g *game.Game }

// New is the readers over g.
func New(g *game.Game) scripting.Queries {
	r := reader{g: g}
	return scripting.Queries{Animals: r, Map: r, World: r, Buildings: r}
}

// Animals is the wild species rows, species ascending, at most
// scripting.MaxQueryRows.
func (r reader) Animals() []scripting.Animal {
	if r.g == nil {
		return nil
	}
	return animalsOf(r.g.Wildlife())
}

// animalsOf sorts rows by species and caps them at MaxQueryRows.
func animalsOf(rows []game.WildlifeRow) []scripting.Animal {
	slices.SortFunc(rows, func(a, b game.WildlifeRow) int { return cmp.Compare(a.Species, b.Species) })
	out := make([]scripting.Animal, 0, min(len(rows), scripting.MaxQueryRows))
	for _, w := range rows {
		if len(out) == scripting.MaxQueryRows {
			break
		}
		out = append(out, scripting.Animal{Species: w.Species, Label: w.Label, Count: w.Count, Predator: w.Predator})
	}
	return out
}

// Size is the map's width and height in cells (0, 0 without a game).
func (r reader) Size() (w, h int) {
	if r.g == nil {
		return 0, 0
	}
	return r.g.Width(), r.g.Height()
}

// in reports that (x, y) is a cell of the map.
func (r reader) in(x, y int) bool {
	w, h := r.Size()
	return x >= 0 && y >= 0 && x < w && y < h
}

// Terrain is the cell's terrain key; ok is false off the map.
func (r reader) Terrain(x, y int) (string, bool) {
	if !r.in(x, y) {
		return "", false
	}
	return r.g.TerrainAt(x, y), true
}

// Temperature is the cell's temperature in whole degrees C; ok is false
// off the map.
func (r reader) Temperature(x, y int) (int, bool) {
	if !r.in(x, y) {
		return 0, false
	}
	return r.g.TemperatureAt(x, y), true
}

// Weather is the sky's state word ("" without a game).
func (r reader) Weather() string {
	if r.g == nil {
		return ""
	}
	return r.g.Weather().Kind.String()
}

// Factions are the known factions in the game's order; a hidden one (not
// yet met) is left out.
func (r reader) Factions() []scripting.Faction {
	if r.g == nil {
		return nil
	}
	return factionsOf(r.g.Factions())
}

// factionsOf maps the visible rows in order, at most MaxQueryRows.
func factionsOf(rows []game.FactionView) []scripting.Faction {
	var out []scripting.Faction
	for _, f := range rows {
		if f.Hidden {
			continue
		}
		if len(out) == scripting.MaxQueryRows {
			break
		}
		out = append(out, scripting.Faction{Name: f.Name, Goodwill: f.Goodwill, Hostile: f.Relation == "hostile"})
	}
	return out
}

// Sites are the world's named sites in the game's order.
func (r reader) Sites() []scripting.Site {
	if r.g == nil {
		return nil
	}
	return sitesOf(r.g.WorldSites())
}

// sitesOf maps rows in order, at most MaxQueryRows.
func sitesOf(rows []game.WorldSiteView) []scripting.Site {
	var out []scripting.Site
	for _, s := range rows {
		if len(out) == scripting.MaxQueryRows {
			break
		}
		out = append(out, scripting.Site{Name: s.Name, Faction: s.Faction})
	}
	return out
}

// Buildings are the home map's building counts, kind ascending (the
// game's own read; bounded there).
func (r reader) Buildings() []scripting.BuildingCount {
	if r.g == nil {
		return nil
	}
	return capBuildings(game.BuildingCounts(r.g))
}

// capBuildings keeps the first MaxQueryRows rows.
func capBuildings(rows []scripting.BuildingCount) []scripting.BuildingCount {
	return rows[:min(len(rows), scripting.MaxQueryRows)]
}
