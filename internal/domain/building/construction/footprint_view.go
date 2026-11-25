// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package construction

func FootprintView(k Kind, f Facing, x, y int) (w, h, sx, sy int, has bool) {
	cells, spot, ok := Footprint(k, f, x, y)
	maxX, maxY := x, y
	for _, c := range cells {
		if c[0] > maxX {
			maxX = c[0]
		}
		if c[1] > maxY {
			maxY = c[1]
		}
	}
	return maxX - x + 1, maxY - y + 1, spot[0], spot[1], ok
}
