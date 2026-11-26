// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package roof

// CountRooms recounts roof-covered room cells using caller-owned scratch.
func CountRooms(counts []int, n int, grid []int, cells []Cell) []int {
	counts = counts[:0]
	for i := 0; i < n; i++ {
		counts = append(counts, 0)
	}
	if len(grid) == len(cells) {
		for i, r := range grid {
			if r >= 0 && cells[i].Roofed() {
				counts[r]++
			}
		}
	}
	return counts
}
