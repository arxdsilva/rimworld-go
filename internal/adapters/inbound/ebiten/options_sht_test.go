// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package ebiten

import (
	"strings"
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/options"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/domain/difficulty"
	"rimworld-go/internal/domain/sim/command"
)

// TestStoryOptionsFriendlyFire (SHT-007): the in-game Custom difficulty
// dialog has the Friendly fire slider; dragging it to the left end and
// accepting sends SetDifficulty with FriendlyFire 0 and every other
// value unchanged.
func TestStoryOptionsFriendlyFire(t *testing.T) {
	t.Parallel()
	s := storySim()
	a, f := openStory(t, s)
	press(t, a, f, options.StCustomID)
	f.Frame(t, a)
	if a.uiModalTop() != options.StCustomMod {
		t.Fatalf("top modal = %q, want the custom dialog", a.uiModalTop())
	}
	w, ok := a.uiSt().Find("diff.f.ff")
	if !ok {
		t.Fatal("no Friendly fire slider in the in-game Custom difficulty dialog")
	}
	tr := uikit.SliderTrackRect(w.R)
	_, y := w.R.Center()
	f.clickAt(t, a, int(tr.X), int(y))
	press(t, a, f, "modal."+options.StCustomMod+".accept")
	cs := cmdsOf(s, command.SetDifficulty)
	if len(cs) != 1 || !strings.HasPrefix(cs[0].Name, "custom:") {
		t.Fatalf("difficulty commands = %+v, want one custom", cs)
	}
	got, err := difficulty.Decode(strings.TrimPrefix(cs[0].Name, "custom:"))
	want := difficulty.Of(difficulty.Settled)
	want.FriendlyFire = 0
	if err != nil || got != want {
		t.Fatalf("custom values = %+v (%v), want Settled with friendly fire 0", got, err)
	}
}
