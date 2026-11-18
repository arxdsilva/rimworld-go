// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package modloader

import (
	"path/filepath"
	"testing"

	"rimworld-go/internal/application/modding"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/recipe"
	"rimworld-go/internal/domain/pawn/backstory"
	"rimworld-go/internal/domain/pawn/skill"
	"rimworld-go/internal/domain/pawn/stat"
	"rimworld-go/internal/domain/pawn/trait"
)

// The shipped example plugin loads with no problem through the whole
// pipeline: its backstories follow core's, its template is not one, and
// its patch reaches the core backstory it targets.
func TestExamplePluginBuilds(t *testing.T) {
	found, err := New(filepath.Join("..", "..", "..", "..", "mods")).Plugins()
	if err != nil {
		t.Fatal(err)
	}
	cat, probs := modding.Build(found, []string{"example_frontier"}, "0.1.35")
	for _, p := range probs {
		t.Errorf("problem: %s", p)
	}
	if !cat.Order.Has("example_frontier") {
		t.Fatalf("example not loaded: %+v", cat.Order)
	}
	core, b := backstory.Core(), cat.Backstories
	if b.Len() != core.Len()+3 {
		t.Fatalf("Len = %d, want core %d + 3", b.Len(), core.Len())
	}
	for id := backstory.ID(1); int(id) <= core.Len(); id++ {
		if b.Key(id) != core.Key(id) {
			t.Fatalf("core id %d moved: %q vs %q", id, b.Key(id), core.Key(id))
		}
	}
	first := backstory.ID(core.Len() + 1)
	if got := b.Key(first); got != "example_frontier.salt_flat_surveyor" {
		t.Fatalf("first plugin key = %q", got)
	}
	if d := b.DefOf(first); d.Slot != backstory.Adulthood || d.Weight != 6 || !d.Fits(backstory.TagNeolithic) {
		t.Fatalf("template fields not inherited: %+v", d)
	}
	var kid backstory.ID
	for id := backstory.ID(1); int(id) <= core.Len(); id++ {
		if core.Key(id) == "core.lighthouse_kid" {
			kid = id
		}
	}
	if kid == 0 {
		t.Fatal("core.lighthouse_kid not found")
	}
	if got := b.DefOf(kid).Skills[skill.Social]; got != 1 || core.DefOf(kid).Skills[skill.Social] != 0 {
		t.Fatalf("patch: Social = %d, core %d", got, core.DefOf(kid).Skills[skill.Social])
	}
	exampleRecipes(t, cat.Recipes)
	exampleTraits(t, cat.Traits)
	exampleItems(t, cat.Items, cat.Recipes)
}

// exampleItems: the example's trail bell follows core's kinds with its own
// texture, its recipe casts it from steel (so a smelter takes it back),
// and its patch raises wool's value (PLG-026).
func exampleItems(t *testing.T, b item.Book, r recipe.Book) {
	t.Helper()
	bell := item.Kind(item.CoreKinds)
	if b.Len() != item.CoreKinds+1 || b.Key(bell) != "example_frontier.trail_bell" || b.Label(bell) != "trail bell" ||
		b.Def(bell).Path != "Manufactured/Components" || b.Value(bell) != 18 || b.Def(bell).Mass != 250 || b.Def(bell).Stack != 75 || b.Def(bell).TraderStock != 6 || b.Def(bell).Texture != "textures/trail_bell.png" {
		t.Fatalf("plugin item: len %d, %q %+v", b.Len(), b.Key(bell), b.Def(bell))
	}
	id, ok := r.IDOf("example_frontier.cast_trail_bells")
	if d := r.DefOf(id); !ok || d.Output != bell || d.OutQty != 2 || d.Input != (recipe.Input{Kind: item.Steel, Qty: 10}) {
		t.Fatalf("bell recipe: %+v", d)
	}
	if got := r.SmeltYield(item.Stack{Kind: bell, Qty: 2}); got != 2 {
		t.Fatalf("2 bells smelt to %d steel, want 2 (a quarter of 5 each)", got)
	}
	if b.Value(item.Wool) != 4 || item.Wool.Value() != 3 {
		t.Fatalf("patch: wool value %d, core %d", b.Value(item.Wool), item.Wool.Value())
	}
}

// exampleRecipes: the example's recipe follows core's at the crafting spot,
// and its patch lowers the straw carpet's work (PLG-025).
func exampleRecipes(t *testing.T, r recipe.Book) {
	t.Helper()
	core := recipe.Core()
	if r.Len() != core.Len()+2 {
		t.Fatalf("recipes Len = %d, want core %d + 2", r.Len(), core.Len())
	}
	id := recipe.ID(core.Len())
	d := r.DefOf(id)
	if r.Key(id) != "example_frontier.lash_slat_carpet" || d.Bench != construction.CraftingSpot || d.Output != item.CarpetWood ||
		d.Input != (recipe.Input{Kind: item.Wood, Qty: 8}) || d.Work != 400 {
		t.Fatalf("plugin recipe %q: %+v", r.Key(id), d)
	}
	if l := r.For(construction.CraftingSpot); l[len(l)-1].ID != id {
		t.Fatal("the crafting spot does not offer the plugin recipe")
	}
	if got, was := r.DefOf(recipe.MakeStrawCarpet).Work, core.DefOf(recipe.MakeStrawCarpet).Work; got != 240 || was != 300 {
		t.Fatalf("patch: straw carpet work %d, core %d", got, was)
	}
}

// exampleTraits: the example's trait follows core's with its own numbers, and
// its patch raises Sunny's mood (PLG-026).
func exampleTraits(t *testing.T, b trait.Book) {
	t.Helper()
	core := trait.Core()
	if b.Len() != core.Len()+1 {
		t.Fatalf("traits Len = %d, want core %d + 1", b.Len(), core.Len())
	}
	id := trait.ID(core.Len() + 1)
	d := b.DefOf(id)
	if b.Key(id) != "example_frontier.trail_wise" || d.Label != "Trail-wise" || d.SkillBonus[skill.Animals] != 2 ||
		len(d.Mods) != 1 || d.Mods[0].Stat != stat.MoveSpeed || d.Mods[0].Mul != 1080 || !b.Conflicts(id, trait.FleetFooted) {
		t.Fatalf("plugin trait %q: %+v", b.Key(id), d)
	}
	if got, was := b.DefOf(trait.Sunny).Mods[0].Add, core.DefOf(trait.Sunny).Mods[0].Add; got != 10 || was != 8 {
		t.Fatalf("patch: Sunny mood %d, core %d", got, was)
	}
}

// Disabled, the example changes nothing: the book is core's.
func TestExamplePluginDisabledIsCore(t *testing.T) {
	found, err := New(filepath.Join("..", "..", "..", "..", "mods")).Plugins()
	if err != nil {
		t.Fatal(err)
	}
	cat, probs := modding.Build(found, nil, "0.1.35")
	if len(probs) != 0 || cat.Backstories.Len() != backstory.Core().Len() || cat.Recipes.Len() != recipe.Core().Len() || len(cat.Plugins) != 0 {
		t.Fatalf("disabled example changed the build: %v, %d defs", probs, cat.Backstories.Len())
	}
}
