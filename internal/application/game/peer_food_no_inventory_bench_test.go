// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"fmt"
	"testing"
)

func BenchmarkSeekFoodNoInventory(b *testing.B) {
	for _, n := range []int{100, 500, 1000} {
		b.Run(fmt.Sprint(n), func(b *testing.B) {
			g := New(Config{Seed: 7, Width: 80, Height: 80})
			bareGround(g)
			for i := 0; i < n; i++ {
				if _, err := g.SpawnPawn(2+i%60, 2+i/60); err != nil {
					b.Fatal(err)
				}
			}
			if len(g.itemIDs) != 0 {
				b.Fatal("unexpected map stacks")
			}
			for _, id := range g.pawnIDs {
				if g.seekFood(id) {
					b.Fatal("unexpected food")
				}
			}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				for _, id := range g.pawnIDs {
					if g.seekFood(id) {
						b.Fatal("unexpected food")
					}
				}
			}
		})
	}
}
