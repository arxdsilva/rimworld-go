// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package industry

import "rimworld-go/internal/domain/building/construction"

// SkyClear checks the entire normalized rectangular footprint, without
// allocating a cell slice on every active scanner work tick.
func SkyClear(b construction.Building, roofed func(int, int) bool) bool {
	d := construction.DefOf(b.Kind)
	w, h := max(d.Size[0], 1), max(d.Size[1], 1)
	if d.Rotatable && (b.Facing == construction.East || b.Facing == construction.West) {
		w, h = h, w
	}
	for y := b.Y; y < b.Y+h; y++ {
		for x := b.X; x < b.X+w; x++ {
			if roofed(x, y) {
				return false
			}
		}
	}
	return true
}
