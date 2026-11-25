// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package scenario

import "slices"

// fixedSettlers is how many settlers the fixed layout stands beside the
// start kit (DefaultColonists less the 3 of the start kit).
const fixedSettlers = DefaultColonists - MinColonists

// rect is an inclusive tile rectangle.
type rect struct{ x0, y0, x1, y1 int }

// siteRect is the colony site: the leveled rectangle around the map centre
// that the layout fills (soil, no plants).
func siteRect(cx, cy int) rect {
	return rect{cx - 52, cy - 30, cx + 47, cy + 29}
}

func (r rect) contains(x, y int) bool {
	return x >= r.x0 && x <= r.x1 && y >= r.y0 && y <= r.y1
}

// cellsOutside is how many cells of a w x h map lie outside the rectangle.
func (r rect) cellsOutside(w, h int) int {
	iw := min(r.x1, w-1) - max(r.x0, 0) + 1
	ih := min(r.y1, h-1) - max(r.y0, 0) + 1
	if iw < 0 {
		iw = 0
	}
	if ih < 0 {
		ih = 0
	}
	return w*h - iw*ih
}

// dist is the Chebyshev distance from a cell to the rectangle (0 inside).
func (r rect) dist(x, y int) int {
	dx := max(r.x0-x, 0, x-r.x1)
	dy := max(r.y0-y, 0, y-r.y1)
	return max(dx, dy)
}

type cell struct{ x, y int }

// extraOrder lists every cell of a w x h map outside the site, nearest the
// site first: ordered by Chebyshev distance to the site, then y, then x.
// The order is total, so it is the same on every call.
func extraOrder(w, h int, site rect) []cell {
	out := make([]cell, 0, site.cellsOutside(w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if !site.contains(x, y) {
				out = append(out, cell{x, y})
			}
		}
	}
	slices.SortFunc(out, func(a, b cell) int {
		if da, db := site.dist(a.x, a.y), site.dist(b.x, b.y); da != db {
			return da - db
		}
		if a.y != b.y {
			return a.y - b.y
		}
		return a.x - b.x
	})
	return out
}

// placeExtras walks order and calls spawn for each cell that occupied does
// not report taken, until want colonists stand. A spawn error skips the
// cell (not walkable); a placed cell becomes occupied. It returns how many
// were placed: fewer than want means the candidates ran out. The index k
// passed to spawn counts the extras placed so far (0 for the first).
func placeExtras(order []cell, want int, occupied func(x, y int) bool, spawn func(x, y, k int) error) int {
	taken := map[cell]bool{}
	placed := 0
	for _, c := range order {
		if placed >= want {
			break
		}
		if taken[c] || occupied(c.x, c.y) {
			continue
		}
		if err := spawn(c.x, c.y, placed); err != nil {
			continue
		}
		taken[c] = true
		placed++
	}
	return placed
}
