// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package pathing

// ReachableSpot finds the first admissible target/adjacent cell in the
// native work ordering. Mining uses only the four cardinal neighbours.
func ReachableSpot(x, y int, cardinal bool, can func(int, int) bool) (int, int, bool) {
	if can(x, y) {
		return x, y, true
	}
	dirs := [8][2]int{{0, 1}, {1, 0}, {0, -1}, {-1, 0}, {1, 1}, {-1, 1}, {1, -1}, {-1, -1}}
	n := len(dirs)
	if cardinal {
		n = 4
	}
	for _, d := range dirs[:n] {
		nx, ny := x+d[0], y+d[1]
		if can(nx, ny) {
			return nx, ny, true
		}
	}
	return 0, 0, false
}
