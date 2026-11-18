// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package inspector

import (
	"strings"
	"testing"

	"rimworld-go/internal/application/game"
)

func sourcesContent() game.Content {
	return game.Content{Sources: map[string]game.Source{
		game.SourceKey("backstory", "demo.drifter"):  {AddedBy: "demo"},
		game.SourceKey("recipe", "demo.trail_stew"):  {AddedBy: "demo"},
		game.SourceKey("recipe", "core.meal_simple"): {PatchedBy: []string{"demo", "other"}},
		game.SourceKey("trait", "demo.calm"):         {AddedBy: "demo"},
	}}
}

// A plugin's backstory says who added it; a core one says nothing (PLG-072).
func TestBackstorySourceLine(t *testing.T) {
	f, a := benchFixture(t)
	f.env.Content = sourcesContent()
	if got := a.sources().withSource("Drifts.", "backstory", "demo.drifter"); got != "Drifts.\nAdded by demo" {
		t.Fatalf("plugin backstory tip = %q", got)
	}
	if got := a.sources().withSource("Raised well.", "backstory", "core.farmhand"); got != "Raised well." {
		t.Fatalf("core backstory tip = %q", got)
	}
}

// Trait chips carry the note; the view's own slice is not mutated.
func TestTraitSourceLine(t *testing.T) {
	f, a := benchFixture(t)
	f.env.Content = sourcesContent()
	b := game.BioView{Traits: []game.TraitView{{Key: "demo.calm", Desc: "Calm."}, {Key: "core.tough", Desc: "Tough."}}}
	got := a.sources().traitsWithSource(b)
	if got.Traits[0].Desc != "Calm.\nAdded by demo" || got.Traits[1].Desc != "Tough." {
		t.Fatalf("traits = %+v", got.Traits)
	}
	if b.Traits[0].Desc != "Calm." {
		t.Fatal("the read model was mutated")
	}
}

// The bill picker's row tip and the bill dialog's title name the plugin.
func TestRecipeSourceLines(t *testing.T) {
	f, a := benchFixture(t)
	f.env.Content = sourcesContent()
	if got := a.Bills().pickTip(game.RecipeView{Name: "demo.trail_stew"}); got != "Added by demo" {
		t.Fatalf("plugin recipe tip = %q", got)
	}
	if got := a.Bills().pickTip(game.RecipeView{Name: "meal_simple"}); got != "Changed by demo, other" {
		t.Fatalf("patched core recipe tip = %q", got)
	}
	if got := a.Bills().pickTip(game.RecipeView{Name: "stone_block"}); got != "" {
		t.Fatalf("core recipe tip = %q", got)
	}
	title := a.Bills().billTitle(game.BillView{Recipe: "demo.trail_stew", Label: "Cook trail stew"})
	if !strings.Contains(title, "Added by demo") {
		t.Fatalf("bill title = %q", title)
	}
	if got := a.Bills().billTitle(game.BillView{Recipe: "stone_block", Label: "Cut blocks"}); got != "Bill: Cut blocks" {
		t.Fatalf("core bill title = %q", got)
	}
}
