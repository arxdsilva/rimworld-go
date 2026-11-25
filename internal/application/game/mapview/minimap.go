// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package mapview

const (
	MinimapTerrain   uint8 = 0x3f
	MinimapPlant     uint8 = 0x40
	MinimapFloor     uint8 = 0x80
	MinimapStructure uint8 = 0xc0
	MinimapKind      uint8 = 0xc0
)

// MinimapCells builds an immutable minimap tile layer from native world view
// slices. The callbacks read each game's view types without copying them.
func MinimapCells[P, B any](terrainW int, terrain []uint8, mw, mh int, plants []P, buildings []B,
	plantXY func(*P) (int, int), buildingAt func(*B) (int, int, int, int, bool)) []uint8 {
	out := make([]uint8, mw*mh)
	if terrainW == mw {
		for i := range out {
			if i < len(terrain) {
				out[i] = terrain[i] & MinimapTerrain
			}
		}
	}
	set := func(x, y int, k uint8) {
		if x >= 0 && y >= 0 && x < mw && y < mh {
			i := y*mw + x
			if out[i]&MinimapKind < k || k == MinimapStructure {
				out[i] = out[i]&MinimapTerrain | k
			}
		}
	}
	for i := range plants {
		x, y := plantXY(&plants[i])
		set(x, y, MinimapPlant)
	}
	for i := range buildings {
		x, y, w, h, floor := buildingAt(&buildings[i])
		k := MinimapStructure
		if floor {
			k = MinimapFloor
		}
		for dy := 0; dy < max(h, 1); dy++ {
			for dx := 0; dx < max(w, 1); dx++ {
				set(x+dx, y+dy, k)
			}
		}
	}
	return out
}
