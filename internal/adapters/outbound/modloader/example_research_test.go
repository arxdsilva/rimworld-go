// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package modloader

import (
	"path/filepath"
	"slices"
	"testing"

	"rimworld-go/internal/application/modding"
	"rimworld-go/internal/domain/research"
)

// The example plugin's project follows core's, needs Carpet making and
// unlocks the plugin's slat carpet recipe; its patch lowers Carpet
// making's cost (PLG-026).
func TestExamplePluginResearch(t *testing.T) {
	found, err := New(filepath.Join("..", "..", "..", "..", "mods")).Plugins()
	if err != nil {
		t.Fatal(err)
	}
	cat, probs := modding.Build(found, []string{"example_frontier"}, "0.1.35")
	for _, p := range probs {
		t.Errorf("problem: %s", p)
	}
	core, b := research.Core(), cat.Research
	if b.Len() != core.Len()+1 {
		t.Fatalf("research Len = %d, want core %d + 1", b.Len(), core.Len())
	}
	p := research.Project(core.Len())
	d := b.DefOf(p)
	if b.Key(p) != "example_frontier.slat_weaving" || d.Label != "Slat weaving" || d.Cost != 400 || d.TechLevel != research.Medieval ||
		!slices.Equal(d.Requires, []research.Project{research.CarpetMaking}) ||
		!slices.Equal(d.Unlocks, []string{"recipe:example_frontier.lash_slat_carpet"}) {
		t.Fatalf("plugin project %q: %+v", b.Key(p), d)
	}
	if _, ok := cat.Recipes.IDOf("example_frontier.lash_slat_carpet"); !ok {
		t.Fatal("the unlocked recipe is the plugin's")
	}
	var pr research.Progress
	if g, ok := b.Locks(&pr, "recipe:example_frontier.lash_slat_carpet"); !ok || g != p {
		t.Fatal("the plugin project gates the plugin recipe")
	}
	if got, was := b.DefOf(research.CarpetMaking).Cost, core.DefOf(research.CarpetMaking).Cost; got != 600 || was == 600 {
		t.Fatalf("carpet making cost %d (core %d), want 600", got, was)
	}
}
