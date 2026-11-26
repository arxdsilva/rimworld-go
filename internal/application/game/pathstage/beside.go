// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package pathstage

import "rimworld-go/pkg/mathx"

// NearestBeside picks the nearest reachable adjacent tile in the stable
// native direction order, or the target cell when all neighbours fail.
func NearestBeside(px, py, x, y int, stand, reach, on func(int, int) bool) (int, int, bool) {
	best, bx, by := 1<<30, 0, 0
	for _, d := range [8][2]int{{0, 1}, {1, 0}, {-1, 0}, {0, -1}, {1, 1}, {-1, 1}, {1, -1}, {-1, -1}} {
		nx, ny := x+d[0], y+d[1]
		if !stand(nx, ny) {
			continue
		}
		dd := mathx.Abs(px-nx) + mathx.Abs(py-ny)
		if dd < best && reach(nx, ny) {
			best, bx, by = dd, nx, ny
		}
	}
	if best == 1<<30 {
		return x, y, on(x, y) && reach(x, y)
	}
	return bx, by, true
}
