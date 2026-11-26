// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package maps

import (
	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/world/planet"
)

// BuildPlanet rebuilds saved world geometry and removes faction bases
// destroyed since the original world generation.
func BuildPlanet(params planet.Params, tile int, factions []faction.Faction) *planet.Planet {
	if params.Coverage <= 0 {
		return nil
	}
	p := planet.Build(params)
	if tile < 0 || tile >= p.TileCount() {
		return nil
	}
	Prune(p, factions)
	return p
}
