// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package saveutil

import (
	"slices"

	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/world/roof"
	"rimworld-go/internal/domain/world/terrain"
)

// LegacyRoofHashCells retains the historical replay-hash representation of
// the derived Supported bit. The actual roof grid, building/terrain sources,
// roof plans, roof existence, and pending support loss remain authoritative.
// Recompute only this redundant bit with the historical square kernel; the
// live solver and persisted roof grid use circular support. This works for
// every map in a snapshot, without mutating or warming the live map.
func LegacyRoofHashCells[E EntryShape[construction.Building]](w, h int, land terrain.Map, buildings []E, cells []uint8) []uint8 {
	if w <= 0 || h <= 0 || len(cells) != w*h || len(land.Cells) != w*h {
		return cells
	}
	support := make([]bool, len(cells))
	for i, t := range land.Cells {
		support[i] = terrain.IsRockWall(t)
	}
	for _, e := range buildings {
		_, b := entryParts[construction.Building](e)
		k := b.Kind
		if k != construction.Column && k != construction.Wall && !k.IsDoor() && !k.WallMounted() {
			continue
		}
		footprint, _, _ := construction.Footprint(k, b.Facing, b.X, b.Y)
		for _, xy := range footprint {
			if x, y := xy[0], xy[1]; x >= 0 && y >= 0 && x < w && y < h {
				support[y*w+x] = true
			}
		}
	}
	// Separable square dilation: a horizontal sliding window, then a
	// vertical one. Each cell is visited twice per axis, independent of
	// the support radius; no per-cell raycasts or allocation per tile.
	horizontal := make([]bool, len(cells))
	r := roof.SupportRange
	for y := 0; y < h; y++ {
		row := support[y*w : (y+1)*w]
		n := 0
		for x := 0; x < min(w, r+1); x++ {
			if row[x] {
				n++
			}
		}
		for x := 0; x < w; x++ {
			horizontal[y*w+x] = n > 0
			if a := x - r; a >= 0 && row[a] {
				n--
			}
			if a := x + r + 1; a < w && row[a] {
				n++
			}
		}
	}
	out := slices.Clone(cells)
	for x := 0; x < w; x++ {
		n := 0
		for y := 0; y < min(h, r+1); y++ {
			if horizontal[y*w+x] {
				n++
			}
		}
		for y := 0; y < h; y++ {
			i := y*w + x
			out[i] &^= uint8(roof.Supported)
			if n > 0 {
				out[i] |= uint8(roof.Supported)
			}
			if a := y - r; a >= 0 && horizontal[a*w+x] {
				n--
			}
			if a := y + r + 1; a < h && horizontal[a*w+x] {
				n++
			}
		}
	}
	return out
}
