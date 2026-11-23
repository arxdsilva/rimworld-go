// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import "testing"

// TestSaveV20FriendlyFire: save v20 (SHT-007) keeps the difficulty's
// friendly-fire share; an older save, written before the setting existed,
// loads at 100 % (friendly fire as it always was), and a game without a
// setup loads unchanged.
func TestSaveV20FriendlyFire(t *testing.T) {
	if SaveVersion != 20 {
		t.Fatalf("SaveVersion %d, want 20", SaveVersion)
	}
	g := buildSetup(t, setupFor(t, "broken_hull", func(s *Setup) {
		s.MapSize = 60
		s.Difficulty.FriendlyFirePct = 37
	}))
	if got := loadAsVersion(t, g, SaveVersion).diff().FriendlyFirePct; got != 37 {
		t.Fatalf("a v20 save round-trips friendly fire %d, want 37", got)
	}
	if got := loadAsVersion(t, g, 19).diff().FriendlyFirePct; got != 100 {
		t.Fatalf("a v19 save loads friendly fire %d, want 100", got)
	}
	bare := New(Config{Seed: 3, Width: 20, Height: 12})
	if l := loadAsVersion(t, bare, 19); l.hasSetup() || l.StateHash() != loadAsVersion(t, bare, SaveVersion).StateHash() {
		t.Fatal("a v19 save without a setup no longer loads as it was")
	}
}
