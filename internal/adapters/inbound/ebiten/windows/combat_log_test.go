// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package windows

import (
	"testing"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
)

func TestCombatRowsNewestFirst(t *testing.T) {
	t.Parallel()
	if got := CombatRows(nil); len(got) != 0 {
		t.Fatalf("empty log: %v", got)
	}
	got := CombatRows([]game.CombatEventView{{Tick: 10, Text: "a"}, {Tick: 20, Text: "b", Threat: true}})
	if len(got) != 2 || got[0].Text != "b" || got[1].Text != "a" {
		t.Fatalf("rows = %v", got)
	}
}

func TestThreatRowsOnlyThreatsNewestFirst(t *testing.T) {
	t.Parallel()
	log := []game.CombatEventView{
		{Tick: 10, Text: "raid", Threat: true},
		{Tick: 20, Text: "struck down"},
		{Tick: 30, Text: "siege", Threat: true},
		{Tick: 40, Text: "downed"},
	}
	got := ThreatRows(log)
	if len(got) != 2 || got[0].Text != "siege" || got[1].Text != "raid" {
		t.Fatalf("threats = %v", got)
	}
	for _, r := range got {
		if r.Text == "struck down" || r.Text == "downed" {
			t.Fatalf("fight entry leaked into Threats: %v", got)
		}
	}
	if got := ThreatRows([]game.CombatEventView{{Tick: 1, Text: "x"}}); len(got) != 0 {
		t.Fatalf("no threats: %v", got)
	}
}

// TestHistoryTabsDeclareAndPaint drives the History window through its five
// tabs: each tab declares its own list (or its empty label) and paints.
func TestHistoryTabsDeclareAndPaint(t *testing.T) {
	t.Parallel()
	sh, w := newFake()
	sh.env.Game = fakeReader{}
	sh.pawns = []game.PawnView{{Name: "Ada", Kills: 2, DamageDealt: 10, DamageTaken: 5}}
	log := []game.CombatEventView{{Tick: 5, Text: "A raid of 2 came", Threat: true}, {Tick: 9, Text: "Bo was struck down"}}
	img := ebiten.NewImage(1280, 720)
	list := map[int]string{1: "history.combat", 2: "history.threats", 3: "history.records"}
	for _, tab := range []int{1, 2, 3, 0, 4} {
		for _, withLog := range []bool{false, true} {
			sh.world.CombatLog = nil
			if withLog {
				sh.world.CombatLog = log
			}
			w.histTab = tab
			w.History().Launch()
			sh.kit.Begin()
			w.Declare()
			if id, ok := list[tab]; ok {
				_, has := sh.kit.Find(id)
				recordsAlwaysHaveARow := tab == 3
				if has != (withLog || recordsAlwaysHaveARow) {
					t.Fatalf("tab %d withLog=%v: list widget present = %v", tab, withLog, has)
				}
			}
			sh.kit.Paint(img)
		}
	}
}
