// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"bytes"
	"fmt"
	"os"
	"slices"
	"testing"

	"rimworld-go/internal/domain/pawn/trait"
)

func TestUpstreamV20SaveFixtures(t *testing.T) {
	for _, ff := range []int{0, 37, 100} {
		t.Run(fmt.Sprint(ff), func(t *testing.T) {
			raw, err := os.ReadFile(fmt.Sprintf("testdata/upstream-v20-ff%d.sav", ff))
			if err != nil {
				t.Fatal(err)
			}
			g, info, err := Load(bytes.NewReader(raw))
			if err != nil {
				t.Fatal(err)
			}
			if info.Version != 20 || g.diff().FriendlyFirePercent() != ff {
				t.Fatalf("version %d, friendly fire %d; want 20, %d", info.Version, g.diff().FriendlyFirePercent(), ff)
			}
			want := []trait.ID{trait.Scrapper, trait.LightFooted}
			if len(g.sk.bios) != 1 || len(g.ng.setup.Colonists) != 3 {
				t.Fatal("missing historical pawn or candidate records")
			}
			for _, p := range g.sk.bios {
				if !slices.Equal(p.Traits, want) {
					t.Fatalf("live traits %v, want %v", p.Traits, want)
				}
			}
			for _, p := range g.ng.setup.Colonists {
				if !slices.Equal(p.Traits, want) {
					t.Fatalf("candidate traits %v, want %v", p.Traits, want)
				}
			}
			var buf bytes.Buffer
			if err := g.Save(&buf, "migration"); err != nil {
				t.Fatal(err)
			}
			reloaded, _, err := Load(&buf)
			if err != nil {
				t.Fatal(err)
			}
			if reloaded.StateHash() != g.StateHash() {
				t.Fatal("migrated game changed on reload")
			}
		})
	}
}
