// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package hud

import (
	"strings"
	"testing"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/clock"
)

func TestSongChip(t *testing.T) {
	t.Parallel()
	if _, ok := SongChip(0, 100); ok {
		t.Fatal("no song at ends 0")
	}
	if _, ok := SongChip(100, 100); ok {
		t.Fatal("no song at ends == now")
	}
	c, ok := SongChip(1000+uint64(clock.TicksPerHour)*3, 1000)
	if !ok || c.Label != "Sky song, 3.0 h left" || c.Tip == "" {
		t.Fatalf("chip %+v", c)
	}
}

// planShell adds the calls Global.Plan makes that fakeShell leaves out.
type planShell struct{ *fakeShell }

func (planShell) PauseReason() string     { return "" }
func (planShell) Power() (string, string) { return "", "" }
func (planShell) RoofShown() bool         { return false }
func (planShell) T(s string) string       { return s }

func TestSongChipBesideEclipse(t *testing.T) {
	t.Parallel()
	now := uint64(3 * clock.TicksPerDay) // the fake shell's tick
	_, f := newFake()
	f.w = game.WorldView{SkyEvent: "eclipse", SkyEventEnds: now + 5000, SkySongEnds: now + 9000}
	plan := New(planShell{f}).Global().Plan()
	n := len(plan.Chips)
	if n < 2 || !strings.HasPrefix(plan.Chips[n-2].Label, "Eclipse") || !strings.HasPrefix(plan.Chips[n-1].Label, "Sky song") {
		t.Fatalf("chips %+v", plan.Chips)
	}
	f.w = game.WorldView{}
	for _, c := range New(planShell{f}).Global().Plan().Chips {
		if strings.HasPrefix(c.Label, "Sky song") || strings.HasPrefix(c.Label, "Eclipse") {
			t.Fatalf("calm sky: %+v", c)
		}
	}
}
