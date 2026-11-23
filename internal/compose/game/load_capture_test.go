// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"rimworld-go/internal/adapters/inbound/ebiten/ports"
	"rimworld-go/internal/adapters/outbound/filesave"
	appgame "rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/difficulty"
	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/scenario"
	"rimworld-go/internal/domain/world/biome"
	"rimworld-go/internal/domain/world/geology"
	"rimworld-go/internal/domain/world/site"
	"rimworld-go/pkg/randx"
)

// seededColony is a small Broken Hull colony whose planet seed phrase is phrase.
func seededColony(t *testing.T, phrase string) *appgame.Game {
	t.Helper()
	d, ok := scenario.Builtin("broken_hull")
	if !ok {
		t.Fatal("no broken_hull scenario")
	}
	s := &appgame.Setup{
		Scenario: d, Storyteller: "steady", Preset: difficulty.Seasoned, Difficulty: difficulty.Of(difficulty.Seasoned),
		Site: site.Site{Biome: biome.TemperateForest, Climate: biome.DefOf(biome.TemperateForest).Climate,
			Stones: []geology.Stone{geology.Granite, geology.Marble}},
		Factions: faction.Generate(faction.DefaultSlots(), randx.New(7)),
		MapSize:  60, Season: appgame.SeasonSpring, Seed: 4242,
	}
	s.World.SeedText = phrase
	s.Colonists = appgame.NewCandidatePool(s.Seed, s.Scenario.Apply()).Selected()
	b := appgame.NewColonyBuilder(appgame.Config{Setup: s})
	for i := 0; i < 1<<16; i++ {
		if _, _, done := b.Step(64); done {
			return b.Game()
		}
	}
	t.Fatal("the colony never finished building")
	return nil
}

// #120: the Load list shows the planet seed of a new save and none for an
// old one. With RIMWORLD_LOAD_CAPTURE_DIR set the two saves stay in that
// folder, for a capture run with -save-dir.
func TestLoadListShowsWorldSeed(t *testing.T) {
	dir := os.Getenv("RIMWORLD_LOAD_CAPTURE_DIR")
	if dir == "" {
		dir = t.TempDir()
	} else if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	h := saveHooks(dir, "fixture", runContent{})
	meta := ports.SaveMeta{Name: "Amber Hollow", Colony: "col-amber", ColonyName: "Amber Hollow", Base: "Amber Hollow", Season: "Aprimay"}
	if err := h.Save(seededColony(t, "amber-hollow"), meta); err != nil {
		t.Fatal(err)
	}
	closeSaves(h)
	old, err := os.ReadFile("../../application/game/testdata/ng1_v2.sav")
	if err != nil {
		t.Fatal(err)
	}
	oldPath := filepath.Join(dir, "Old Colony"+filesave.Ext)
	if err := os.WriteFile(oldPath, old, 0o644); err != nil {
		t.Fatal(err)
	}
	newPath := filepath.Join(dir, "Amber-Hollow"+filesave.Ext)
	for p, d := range map[string]time.Time{oldPath: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), newPath: time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)} {
		if err := os.Chtimes(p, d, d); err != nil {
			t.Fatal(err)
		}
	}
	slots, err := h.List()
	if err != nil || len(slots) != 2 {
		t.Fatalf("slots %+v, %v", slots, err)
	}
	for _, s := range slots {
		t.Logf("slot %q title %q world seed %q version %d", s.Name, s.Title, s.WorldSeed, s.Version)
		if s.Err != "" || s.Notice != "" {
			t.Errorf("%s: Err %q Notice %q, want none", s.Name, s.Err, s.Notice)
		}
		want := ""
		if s.Path == newPath {
			want = "amber-hollow"
		} else if s.Path != oldPath {
			t.Fatalf("unexpected slot %+v", s)
		}
		if s.WorldSeed != want {
			t.Errorf("%s: WorldSeed %q, want %q", s.Name, s.WorldSeed, want)
		}
		if _, _, err := h.Load(s.Path); err != nil {
			t.Errorf("%s: load: %v", s.Name, err)
		}
	}
}
