// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"testing"

	"rimworld-go/internal/application/scripting"
)

func TestFireRivalRaidUsesNativeRaidAndReturnsResult(t *testing.T) {
	g, _ := raidGame(t)
	f := &fakeScripts{}
	UseScripts(g, f)
	if g.FireIncident("raid") {
		t.Fatal("legacy unqualified incident must not fire")
	}
	if !g.FireIncident("rival_raid") {
		t.Fatal("native rival raid incident was refused with live defenders")
	}
	if len(g.raidGroups) != 1 || g.raidGroups[0].Size <= 0 {
		t.Fatalf("native storyteller did not create a group: %+v", g.raidGroups)
	}
	if n, _ := g.Raiders(); n == 0 {
		t.Fatal("native rival raid did not spawn raiders")
	}
	if len(f.events) != 1 || f.events[0].Hook != scripting.OnIncident ||
		f.events[0].Incident != (scripting.Incident{Key: "core.rival_raid", Category: "threat"}) {
		t.Fatalf("expected exactly one native raid notification: %+v", f.events)
	}
}

func TestFireRivalRaidRefusesAnUndefendedMap(t *testing.T) {
	f := &fakeScripts{}
	g := New(Config{Seed: 11, Width: 30, Height: 20, Scripts: f})
	if g.FireIncident("rival_raid") || len(g.raidGroups) > 0 {
		t.Fatalf("undefended map accepted rival raid: groups=%+v", g.raidGroups)
	}
	if len(f.events) != 0 {
		t.Fatalf("refused raid notified scripts: %+v", f.events)
	}
}
