// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package modloader

import (
	"path/filepath"
	"testing"

	"rimworld-go/internal/application/modding"
	"rimworld-go/internal/domain/pawn/mood"
)

// The example plugin softens one core thought and leaves the rest core's.
func TestExamplePluginPatchesThought(t *testing.T) {
	found, err := New(filepath.Join("..", "..", "..", "..", "mods")).Plugins()
	if err != nil {
		t.Fatal(err)
	}
	cat, probs := modding.Build(found, []string{"example_frontier"}, "0.1.35")
	for _, p := range probs {
		t.Errorf("problem: %s", p)
	}
	core := mood.Core()
	if cat.Thoughts.Len() != core.Len() {
		t.Fatalf("Len = %d", cat.Thoughts.Len())
	}
	for i := range core.Len() {
		k := mood.Kind(i)
		want := core.DefOf(k)
		if k == mood.SleptOutside {
			want.Value = -2
		}
		if got := cat.Thoughts.DefOf(k); got != want {
			t.Errorf("%s: %+v, want %+v", core.Key(k), got, want)
		}
	}
}
