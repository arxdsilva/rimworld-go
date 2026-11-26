// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package ebiten

import (
	"strings"
	"testing"

	"rimworld-go/internal/application/game"
)

// TestJourneyHUDSkySong: the player's client shows the Sky song status chip
// and the sky and supply letters of the positive incidents. The song's end
// is not stepped (about two days of ticks); it is proven in the hud and game
// packages.
func TestJourneyHUDSkySong(t *testing.T) {
	t.Parallel()
	g := ax4RiverColony(t)
	g.Step(4000)
	j := newJourney(t, jGame(g))
	j.update(2)

	hasLetter := func(prefix string) bool {
		for _, l := range j.world().Letters {
			if strings.HasPrefix(l.Title, prefix) {
				return true
			}
		}
		return false
	}
	songChip := func() bool {
		for _, c := range j.a.globalPlan().Chips {
			if strings.HasPrefix(c.Label, "Sky song") {
				return true
			}
		}
		return false
	}
	if songChip() {
		t.Fatal("a Sky song chip before the song")
	}

	if !g.FireIncident("sky_song") {
		t.Fatal("the sky song did not fit this colony")
	}
	j.update(1)
	if !songChip() {
		t.Fatalf("no Sky song chip: %+v", j.a.globalPlan().Chips)
	}
	if !hasLetter("Sky song") {
		t.Fatalf("no Sky song letter: %+v", lettersOf(j.world()))
	}
	if j.world().SkySongEnds == 0 {
		t.Fatal("the read model has no song end")
	}
	j.draw(2) // and it paints

	if !g.FireIncident("supply_drop") {
		t.Fatal("the supply drop did not fit this colony")
	}
	j.update(1)
	if !hasLetter("Supply drop") {
		t.Fatalf("no Supply drop letter: %+v", lettersOf(j.world()))
	}
	j.draw(2)
}

func lettersOf(w game.WorldView) []string {
	var out []string
	for _, l := range w.Letters {
		out = append(out, l.Title)
	}
	return out
}
