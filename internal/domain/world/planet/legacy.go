// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package planet

import (
	"fmt"

	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/world/biome"
	"rimworld-go/internal/domain/world/site"
	"rimworld-go/pkg/randx"
)

// saltLegacySite keys the landing tile a seed's colony stands on.
const saltLegacySite uint64 = 0x6c65676163

// LegacySite is the water a colony of biome b has when it is started
// without the new-game wizard: a world is generated from seed (as the
// wizard's default world) and one of its landable tiles of biome b is
// picked from the seed; the tile's river and lakes (its Site) are what
// the map has, so rivers and lakes appear on the share of maps they do on
// the planet. The zero Site (no water) when the world has no such tile.
func LegacySite(seed uint64, b biome.ID) site.Site {
	p := Build(Params{SeedText: fmt.Sprintf("colony-%d", seed), Coverage: 30, Rain: 2, Temp: 2, Pop: 1,
		Factions: faction.DefaultSlots(), Version: CurrentVersion})
	var tiles []int
	for i := 0; i < p.TileCount(); i++ {
		if ok, _ := p.ValidLanding(i); ok && p.Biome[i] == b {
			tiles = append(tiles, i)
		}
	}
	if len(tiles) == 0 {
		return site.Site{}
	}
	t := p.Site(tiles[randx.New(seed^saltLegacySite).IntN(len(tiles))])
	return site.Site{Biome: b, River: t.River, Lakes: t.Lakes}
}
