// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"testing"

	"rimworld-go/internal/domain/difficulty"
)

func TestFriendlyFireSettingSurvivesActualSaveAndLegacyRead(t *testing.T) {
	for _, tc := range []struct {
		name     string
		settings difficulty.Settings
		want     int
	}{
		{"explicit-zero", func() difficulty.Settings { s := difficulty.Of(difficulty.Seasoned); s.FriendlyFire = 0; return s }(), 0},
		{"legacy-with-no-field", difficulty.Settings{ThreatScale: 100}, difficulty.DefaultFriendlyFire},
		{"explicit-full", func() difficulty.Settings { s := difficulty.Of(difficulty.Seasoned); s.FriendlyFire = 100; return s }(), 100},
	} {
		t.Run(tc.name, func(t *testing.T) {
			g := buildSetup(t, setupFor(t, "broken_hull", func(s *Setup) {
				s.MapSize = 60
				s.Preset, s.Difficulty = difficulty.Custom, tc.settings
			}))
			stepUntilLanded(t, g)
			h := saveLoad(t, g)
			if got := h.diff().FriendlyFirePercent(); got != tc.want {
				t.Fatalf("loaded friendly fire: got %d, want %d", got, tc.want)
			}
			if h.StateHash() != g.StateHash() {
				t.Fatal("difficulty save/load state hash changed")
			}
		})
	}
}
