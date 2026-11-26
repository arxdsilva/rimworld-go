// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"fmt"
	"rimworld-go/internal/domain/economy/item"
	"testing"
)

// Measure a resident food-admission pass. Upstream has no peer fallback,
// so selected/op records the functional difference as well as elapsed time.
func BenchmarkPeerAdmission(b *testing.B) {
	for _, mode := range []string{"edible", "ineligible"} {
		for _, n := range []int{100, 500, 1000} {
			b.Run(fmt.Sprintf("%s/%d", mode, n), func(b *testing.B) {
				g := New(Config{Seed: 7, Width: 80, Height: 80})
				bareGround(g)
				for i := 0; i < n; i++ {
					if _, err := g.SpawnPawn(2+i%60, 2+i/60); err != nil {
						b.Fatal(err)
					}
				}
				donor := g.pawnIDs[0]
				sid, err := g.SpawnStack(item.Meal, 2, 2, 2)
				if err != nil {
					b.Fatal(err)
				}
				if !senseOf(g).Gear.Stow(donor, sid, 2) {
					b.Fatal("pack setup")
				}
				if len(g.itemIDs) != 0 {
					b.Fatal("map food remains")
				}
				if mode == "ineligible" {
					g.setDraft(donor, true)
				}
				selected := 0
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					// Expire negative retry cooldown before each cold admission pass.
					g.clock.Advance(foodRetry + 1)
					for _, id := range g.pawnIDs {
						if g.seekFood(id) {
							selected++
						}
					}
				}
				b.ReportMetric(float64(selected)/float64(b.N), "selected/op")
			})
		}
	}
}

// Production variants omit the test-only scan/A* cross-checks, not the
// actual region checks. The checked benchmarks above remain unchanged.
func BenchmarkProductionPeerAdmission(b *testing.B) {
	SetIndexSelfCheck(false)
	b.Cleanup(func() { SetIndexSelfCheck(true) })
	BenchmarkPeerAdmission(b)
}

func BenchmarkProductionNoInventory(b *testing.B) {
	SetIndexSelfCheck(false)
	b.Cleanup(func() { SetIndexSelfCheck(true) })
	BenchmarkSeekFoodNoInventory(b)
}
