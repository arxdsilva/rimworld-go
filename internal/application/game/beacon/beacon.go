// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

// Package beacon is the cells an orbital trade beacon trades from. It
// imports nothing from the game package.
package beacon

// Diameter is a trade beacon's reach across, in cells.
const Diameter = 15 // wiki: Orbital trade beacon

// discRadius is the stub's reach (Euclidean cells), the old beacon radius.
const discRadius = 6

// Reach calls add with the index (y*w+x) of every cell of the w x h map a
// beacon at (bx, by) trades from, in ascending index order. blocked
// reports a cell the reach does not cross.
//
// Stub: the old radius-6 disc, ignoring blocked.
func Reach(w, h, bx, by int, blocked func(x, y int) bool, add func(i int)) {
	for y := max(0, by-discRadius); y <= min(h-1, by+discRadius); y++ {
		for x := max(0, bx-discRadius); x <= min(w-1, bx+discRadius); x++ {
			dx, dy := x-bx, y-by
			if dx*dx+dy*dy <= discRadius*discRadius {
				add(y*w + x)
			}
		}
	}
}
