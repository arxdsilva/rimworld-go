// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package hud

import (
	"strings"
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/clock"
)

// TestSongChip: no chip unless the song plays; then hours left, Good tone.
func TestSongChip(t *testing.T) {
	t.Parallel()
	now := uint64(1000)
	for _, ends := range []uint64{0, 999, now} {
		if _, ok := SongChip(ends, now); ok {
			t.Fatalf("ends %d: chip shown", ends)
		}
	}
	c, ok := SongChip(now+3*uint64(clock.TicksPerHour), now)
	if !ok || c.Label != "Sky song, 3.0 h left" || c.Edge != uikit.LetterGood || c.Tip == "" {
		t.Fatalf("chip %+v ok %v", c, ok)
	}
}

// planShell adds the calls Global.Plan makes beyond fakeShell's.
type planShell struct{ *fakeShell }

func (planShell) PauseReason() string     { return "" }
func (planShell) RoofShown() bool         { return false }
func (planShell) Power() (string, string) { return "", "" }

// TestGlobalPlanShowsSongChip: Plan lists the chip while the song plays.
func TestGlobalPlanShowsSongChip(t *testing.T) {
	t.Parallel()
	f := &fakeShell{env: Env{Game: fakeGame{}, Colony: true}}
	g := New(planShell{f}).Global()
	has := func() bool {
		for _, c := range g.Plan().Chips {
			if strings.HasPrefix(c.Label, "Sky song") {
				return true
			}
		}
		return false
	}
	f.w = game.WorldView{SkySongEnds: uint64(f.Tick()) + uint64(clock.TicksPerHour)}
	if !has() {
		t.Fatal("song plays, no chip")
	}
	f.w = game.WorldView{}
	if has() {
		t.Fatal("no song, chip shown")
	}
}
