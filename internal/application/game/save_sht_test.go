// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"testing"

	"rimworld-go/internal/application/game/savelegacy"
	"rimworld-go/internal/domain/pawn/trait"
	"rimworld-go/internal/domain/sim/entity"
)

// TestSaveV20FriendlyFire: save v20 (SHT-007) keeps the difficulty's
// friendly-fire share; an older save, written before the setting existed,
// loads at 100 % (friendly fire as it always was), and a game without a
// setup loads unchanged.
func TestSaveV20FriendlyFire(t *testing.T) {
	if SaveVersion != 21 {
		t.Fatalf("SaveVersion %d, want 21", SaveVersion)
	}
	g := buildSetup(t, setupFor(t, "broken_hull", func(s *Setup) {
		s.MapSize = 60
		s.Difficulty.FriendlyFire = 37
	}))
	if got := loadAsVersion(t, g, SaveVersion).diff().FriendlyFire; got != 37 {
		t.Fatalf("a current save round-trips friendly fire %d, want 37", got)
	}
	if got := loadAsVersion(t, g, 19).diff().FriendlyFire; got != 100 {
		t.Fatalf("a v19 save loads friendly fire %d, want 100", got)
	}
	// Re-encode the original v20 wire shape, not merely a v21 header.
	for _, ff := range []int{0, 37, 100} {
		old := loadAsVersionWith(t, g, 20, func(s *saveState) {
			p := savelegacy.Pawn{First: "Legacy", Traits: []trait.ID{trait.Scrapper, trait.LightFooted}}
			s.NG1 = &savelegacy.SetupState{Setup: savelegacy.Setup{
				MapSize: 60, Storyteller: "steady", Difficulty: savelegacy.DifficultySettings{FriendlyFirePct: ff},
				Colonists: []savelegacy.Pawn{p},
			}}
			s.NG121 = nil
			s.Bios = map[entity.EntityID]savelegacy.Pawn{71: p}
			s.Bios21 = nil
		})
		if got := old.diff().FriendlyFire; got != ff {
			t.Fatalf("v20 friendly fire %d, want %d", got, ff)
		}
		if p, ok := old.sk.bios[71]; !ok || len(p.Traits) != 2 || p.Traits[0] != trait.Scrapper || p.Traits[1] != trait.LightFooted {
			t.Fatalf("v20 live bio traits lost: %v (present %v)", p.Traits, ok)
		}
		for _, p := range old.ng.setup.Colonists {
			if len(p.Traits) != 2 || p.Traits[0] != trait.Scrapper || p.Traits[1] != trait.LightFooted {
				t.Fatalf("v20 traits lost: %v", p.Traits)
			}
		}
	}
	bare := New(Config{Seed: 3, Width: 20, Height: 12})
	if l := loadAsVersion(t, bare, 19); l.hasSetup() || l.StateHash() != loadAsVersion(t, bare, SaveVersion).StateHash() {
		t.Fatal("a v19 save without a setup no longer loads as it was")
	}
}
