// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package devtools

import (
	"testing"

	"rimworld-go/internal/application/game"
)

type incidentGame struct{ list []game.PluginIncident }

func (g incidentGame) PluginIncidents() []game.PluginIncident { return g.list }

type fireLog struct {
	Tools
	keys []string
}

func (f *fireLog) FireIncident(key string) bool { f.keys = append(f.keys, key); return true }

// TestPluginIncidentsInDebugMenu (PLG-026): each plugin incident is a debug
// menu entry under its own label that fires it by key; none without any.
func TestPluginIncidentsInDebugMenu(t *testing.T) {
	t.Parallel()
	f := &fakeShell{layout: devLayouts()["idle 1280x720"]}
	f.env = Env{On: true}
	base := len(New(f).Actions(1, 1))
	f.env = Env{On: true, Game: incidentGame{[]game.PluginIncident{{Key: "p.dark_noon", Label: "Dark noon", Like: "Eclipse"}}}}
	acts := New(f).Actions(1, 1)
	if len(acts) != base+1 {
		t.Fatalf("%d entries, want %d", len(acts), base+1)
	}
	last := acts[len(acts)-1]
	if last.Label != "Event: Dark noon (like Eclipse)" {
		t.Fatalf("label %q", last.Label)
	}
	log := &fireLog{}
	if msg := last.Run(log); msg != "Dark noon (like Eclipse) fired" || len(log.keys) != 1 || log.keys[0] != "p.dark_noon" {
		t.Fatalf("run: %q %v", msg, log.keys)
	}
}
