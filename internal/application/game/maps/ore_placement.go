// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package maps

import (
	"rimworld-go/internal/application/game/travel"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/world/geology"
)

// OreSite is the map's authoritative terrain and resource placement port.
// A mineral site belongs to the map lifecycle; its host owns the tile stores.
type OreSite interface {
	LumpSize() (width, height int)
	LumpTaken() map[[2]int]bool
	LumpFree(x, y int) bool
	LumpVein(x, y int, ore geology.Ore)
	LumpStack(k item.Kind, qty, x, y int)
}

// PlaceLump builds a scanner site's ore vein, or drops goods without a vein.
func PlaceLump(h OreSite, k item.Kind, qty, count int) []int {
	if k == 0 || qty <= 0 {
		return nil
	}
	ore := travel.OreOfLump(k)
	n := 1
	w, height := h.LumpSize()
	if ore != geology.NoOre {
		n = travel.SiteCells(k, qty, count)
	}

	taken := h.LumpTaken()
	cells := LumpCells(w, height, n, func(x, y int) bool {
		return h.LumpFree(x, y) && !taken[[2]int{x, y}]
	})
	if ore == geology.NoOre {
		if len(cells) > 0 {
			h.LumpStack(k, qty, cells[0]%w, cells[0]/w)
		}
		return nil
	}
	for _, c := range cells {
		h.LumpVein(c%w, c/w, ore)
	}
	return cells
}
