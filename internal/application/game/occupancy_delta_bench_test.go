// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"strconv"
	"testing"

	"rimworld-go/internal/domain/sim/entity"
)

// BenchmarkOccupancyRebuildSparse exercises repeated unchanged occupancy
// sweeps and net-change bookkeeping in a colony with 64 separate animals.
func BenchmarkOccupancyRebuildSparse(b *testing.B) {
	for _, side := range []int{64, 500} {
		b.Run(strconv.Itoa(side), func(b *testing.B) {
			g := New(Config{Seed: 13, Width: side, Height: side})
			bareGround(g)
			step := (side - 8) / 8
			for i := range 64 {
				x, y := 4+i%8*step, 4+i/8*step
				if _, err := g.SpawnAnimal("dustbuck", x, y); err != nil {
					b.Fatal(err)
				}
			}
			g.rebuildOccupancy()
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				g.rebuildOccupancy()
			}
		})
	}
}

// BenchmarkOccupancyRebuildMoving covers repeated displacement rather
// than the unchanged-layer case above.
func BenchmarkOccupancyRebuildMoving(b *testing.B) {
	for _, side := range []int{64, 500} {
		b.Run(strconv.Itoa(side), func(b *testing.B) {
			g := New(Config{Seed: 14, Width: side, Height: side})
			bareGround(g)
			step := (side - 8) / 8
			ids := make([]entity.EntityID, 64)
			for i := range ids {
				x, y := 4+i%8*step, 4+i/8*step
				id, err := g.SpawnAnimal("dustbuck", x, y)
				if err != nil {
					b.Fatal(err)
				}
				ids[i] = id
			}
			g.rebuildOccupancy()
			shift := 0
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				shift ^= 1
				for i, id := range ids {
					g.positions.Set(id, Position{X: 4 + i%8*step + shift, Y: 4 + i/8*step})
				}
				g.rebuildOccupancy()
			}
		})
	}
}
