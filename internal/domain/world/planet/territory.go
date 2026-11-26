// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package planet

import "rimworld-go/internal/domain/faction"

// Territory (#11): the planet's settlements, in global order, seed a
// faction.Territory. The game's planet keeps its own (ext.territory, built
// by finish and Unsettle); a view builds one on its read-only copy.
//
// These are package functions taking *Planet, not methods: Planet is at
// its 40-method limit (SOLID_GO.md §8).

// BuildTerritory builds t from p's settlements that pass standing (nil:
// every settlement still on p.Settlement), in global settlement order, so
// the seed index breaks ties. It only reads p.
func BuildTerritory(p *Planet, t *faction.Territory, standing func(tile int) bool) {
	seeds := make([]faction.Seed, 0, len(p.settlements))
	for k, s := range p.settlements {
		if p.Settlement[s.tile] != int32(k) { // a destroyed base has left its tile (MAP-05)
			continue
		}
		if standing != nil && !standing(int(s.tile)) {
			continue
		}
		seeds = append(seeds, faction.Seed{Tile: s.tile, Faction: int16(s.faction)})
	}
	t.Build(p, seeds, len(p.Factions), faction.TerritoryHops)
}

// TerritoryOwner is the faction index owning tile i in p's own territory,
// -1 when unclaimed, out of range or not built.
func TerritoryOwner(p *Planet, i int) int {
	return p.ext.territory.Owner(i)
}

// TerritoryTiles are the tiles faction f owns in p's own territory,
// ascending (shared, read-only).
func TerritoryTiles(p *Planet, f int) []int32 {
	return p.ext.territory.Tiles(f)
}

// TerritoryVersion is p's own territory's build count (0: not built).
func TerritoryVersion(p *Planet) uint64 {
	return p.ext.territory.Version()
}

// Unsettle clears p.Settlement for each settled tile given, then rebuilds
// p's own territory once. Unsettled and out-of-range tiles are ignored,
// and when nothing was settled nothing changes (no version bump).
func Unsettle(p *Planet, tiles ...int) {
	changed := false
	for _, t := range tiles {
		if t >= 0 && t < len(p.Settlement) && p.Settlement[t] >= 0 {
			p.Settlement[t] = -1
			changed = true
		}
	}
	if !changed {
		return
	}
	if p.ext.territory == nil {
		p.ext.territory = &faction.Territory{}
	}
	BuildTerritory(p, p.ext.territory, nil)
}
