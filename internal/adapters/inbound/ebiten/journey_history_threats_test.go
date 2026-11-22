// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package ebiten

import (
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/windows"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/economy/item"
)

// TestJourneyHistoryThreats (DEF 3.1): a raid lands, its announcement is a
// Threat entry in the combat log, the History window's Threats tab lists it,
// and a hit on a colonist shows in the Records Damage taken column.
func TestJourneyHistoryThreats(t *testing.T) {
	j := newJourney(t, jPawns([2]int{8, 4}, [2]int{9, 4}))
	j.g.RaidArmed(item.Rifle, item.Rifle)
	j.update(1) // pause-on-threat pauses the game on this update
	taken := func() bool {
		for _, p := range j.g.Snapshot() {
			if !p.Hostile && p.DamageTaken > 0 {
				return true
			}
		}
		return false
	}
	for i := 0; i < 40 && !taken(); i++ {
		j.tick(50)
	}

	if !j.clickID("tab.history") {
		t.Fatal("history tab disabled")
	}
	j.update(1)
	if j.a.uiModalTop() != windows.HistoryModalID {
		t.Fatalf("modal = %q", j.a.uiModalTop())
	}
	if !j.clickID("history.tabs.2") {
		t.Fatal("Threats tab disabled")
	}
	j.update(1)

	log := j.g.World().CombatLog
	threats := windows.ThreatRows(log)
	if len(threats) == 0 {
		t.Fatalf("no threat rows in %+v", log)
	}
	var threat bool
	for _, e := range log {
		if e.Threat && e.Text == threats[0].Text {
			threat = true
		}
	}
	if !threat {
		t.Fatalf("first threat row %q is not a Threat entry: %+v", threats[0].Text, log)
	}
	want := 0
	for _, e := range log {
		if e.Threat {
			want++
		}
	}
	if len(threats) != want || len(windows.CombatRows(log)) != len(log) {
		t.Fatalf("threat rows %d want %d; combat rows %d of %d", len(threats), want, len(windows.CombatRows(log)), len(log))
	}
	if !hasWidget(j.a, "history.threats") {
		t.Fatal("Threats tab declares no history.threats list")
	}
	j.draw(2)

	if !taken() {
		t.Fatal("no colonist took damage")
	}
	var found bool
	for _, r := range windows.RecordRows(j.g.Snapshot(), func(p game.PawnView) string { return p.Name }) {
		if r.Taken > 0 {
			found = true
		}
	}
	if !found {
		t.Fatal("Records lists no colonist with damage taken")
	}

	j.key("Escape", 0)
	if j.a.uiModalTop() == windows.HistoryModalID {
		t.Fatal("Escape did not close History")
	}
}
