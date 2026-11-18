// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package modloader

import (
	"path/filepath"
	"testing"

	"rimworld-go/internal/application/modding"
	"rimworld-go/internal/domain/incident"
)

// The example plugin's incident runs the party's behaviour under its own
// key and numbers, and its patch raises the herd crossing's weight
// (PLG-026).
func TestExamplePluginIncidents(t *testing.T) {
	found, err := New(filepath.Join("..", "..", "..", "..", "mods")).Plugins()
	if err != nil {
		t.Fatal(err)
	}
	cat, probs := modding.Build(found, []string{"example_frontier"}, "0.1.35")
	for _, p := range probs {
		t.Errorf("problem: %s", p)
	}
	core, b := incident.Core(), cat.Incidents
	if b.Len() != core.Len()+1 {
		t.Fatalf("incidents Len = %d, want core %d + 1", b.Len(), core.Len())
	}
	id := incident.ID(core.Len())
	d := b.DefOf(id)
	if b.Key(id) != "example_frontier.settler_party" || d.Like != incident.Party || d.Name != "Frontier hoedown" || d.Weight != 8 || !d.Helpful {
		t.Fatalf("plugin incident %q: %+v", b.Key(id), d)
	}
	if got, was := b.DefOf(incident.HerdCrossing).Weight, core.DefOf(incident.HerdCrossing).Weight; got != 40 || was == 40 {
		t.Fatalf("herd crossing weight %d (core %d), want 40", got, was)
	}
}
