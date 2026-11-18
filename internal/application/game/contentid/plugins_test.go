// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package contentid

import (
	"slices"
	"testing"

	"rimworld-go/internal/domain/economy/recipe"
	"rimworld-go/internal/domain/pawn/backstory"
)

// With no plugin the run's content is the build's own: same hash, ["core"].
func TestOfCoreIsCurrent(t *testing.T) {
	got, want := Of([]string{Core}, Books{Backstories: backstory.Core(), Recipes: recipe.Core()}), Current()
	if got.Hash != want.Hash || !slices.Equal(got.Plugins, want.Plugins) || got.Hash != Fingerprint() {
		t.Fatalf("Of(core) = %+v, Current = %+v", got, want)
	}
}

// A plugin's backstory changes the fingerprint and the plugin list (PLG-043).
func TestOfCoversPluginDefs(t *testing.T) {
	core := backstory.Core()
	list, keys := core.All(), make([]string, 0, core.Len()+1)
	for id := backstory.ID(1); int(id) <= core.Len(); id++ {
		keys = append(keys, core.Key(id))
	}
	list = append(list, backstory.Def{Slot: backstory.Adulthood, Title: "Ferry pilot", Short: "Pilot", Weight: 5})
	keys = append(keys, "demo.ferry_pilot")
	got := Of([]string{Core, "demo"}, Books{Backstories: backstory.NewBook(list, keys)})
	if got.Hash == Fingerprint() {
		t.Fatal("a plugin backstory left the fingerprint unchanged")
	}
	if !slices.Equal(got.Plugins, []string{Core, "demo"}) {
		t.Fatalf("Plugins = %v", got.Plugins)
	}
}

// A plugin's recipe, or a patched core one, changes the fingerprint (PLG-025).
func TestOfCoversPluginRecipes(t *testing.T) {
	core := recipe.Core()
	list, keys := append([]recipe.Def(nil), core.All()...), core.Keys()
	d := core.DefOf(recipe.Club)
	d.Name, d.Label = "demo.clubs", "Make clubs"
	added := Of([]string{Core, "demo"}, Books{Recipes: recipe.NewBook(append(list, d), append(keys, "demo.clubs"))})
	list[recipe.Club].Work++
	patched := Of([]string{Core, "demo"}, Books{Recipes: recipe.NewBook(list, keys)})
	if added.Hash == Fingerprint() || patched.Hash == Fingerprint() || added.Hash == patched.Hash {
		t.Fatalf("hashes %#x %#x, core %#x", added.Hash, patched.Hash, Fingerprint())
	}
}
