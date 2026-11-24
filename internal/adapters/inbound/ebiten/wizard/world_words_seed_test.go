// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package wizard

import "testing"

func TestRandomSeedIsInRange(t *testing.T) {
	for range 200 {
		if s := RandomSeed(); s < 1 || s > 1_000_000_000 {
			t.Fatalf("RandomSeed %d outside 1..1e9", s)
		}
	}
}
