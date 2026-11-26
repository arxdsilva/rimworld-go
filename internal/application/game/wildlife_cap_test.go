// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"testing"

	"rimworld-go/internal/domain/animal"
)

// TestWildlifeCapCurve checks vanilla area density, with no headcount limit.
func TestWildlifeCapCurve(t *testing.T) {
	t.Parallel()
	for _, density := range []int{0, 10, 20, 40, 180, 370, 650} {
		prev := 0
		for side := 10; side <= 600; side++ {
			got := animal.EcosystemCapacity(density, side*side)
			if got < prev {
				t.Fatalf("density %d: weight cap fell at %d squared", density, side)
			}
			prev = got
		}
		if got, want := animal.EcosystemCapacity(density, 250*250), density*250*250/10000; got != want {
			t.Fatalf("density %d at 250 squared: %d, want %d", density, got, want)
		}
		if got, want := animal.EcosystemCapacity(density, 500*500), density*500*500/10000; got != want {
			t.Fatalf("density %d at 500 squared: %d, want %d", density, got, want)
		}
	}

}
