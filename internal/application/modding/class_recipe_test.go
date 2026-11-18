// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package modding

import (
	"reflect"
	"strings"
	"testing"

	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/recipe"
)

func rcSame(t *testing.T, got, want recipe.Book) {
	t.Helper()
	if got.Len() != want.Len() {
		t.Fatalf("%d recipes, want %d", got.Len(), want.Len())
	}
	for i := 0; i <= want.Len(); i++ {
		id := recipe.ID(i)
		if g, w := got.DefOf(id), want.DefOf(id); !reflect.DeepEqual(g, w) {
			t.Fatalf("id %d: %+v\nwant %+v", id, g, w)
		}
		if g, w := got.Key(id), want.Key(id); g != w {
			t.Fatalf("id %d: key %q, want %q", id, g, w)
		}
	}
}

// TestRecipeCodecRoundTripsCore: decoding core's encoded defs (straight,
// and through JSON) gives core back, def for def and key for key.
func TestRecipeCodecRoundTripsCore(t *testing.T) {
	t.Parallel()
	core := recipe.Core()
	defs := EncodeRecipes(core)
	if len(defs) != core.Len() {
		t.Fatalf("%d defs for %d recipes", len(defs), core.Len())
	}
	for i, d := range defs {
		if d.Class != RecipeClass || d.Plugin != Core || d.Key != core.Key(recipe.ID(i)) || d.Key != "core."+core.DefOf(recipe.ID(i)).Name {
			t.Fatalf("def %d: %s by %q", i, d.Ref(), d.Plugin)
		}
	}
	for name, in := range map[string][]Def{"trees": defs, "json": bsJSON(t, defs)} {
		got, probs := DecodeRecipes(in)
		if len(probs) > 0 {
			t.Fatalf("%s: %v", name, probs)
		}
		rcSame(t, got, core)
	}
}

// TestRecipeCoreKeysUnique: every core recipe's name is unique snake_case,
// so "core.<name>" keys it (PLG-025).
func TestRecipeCoreKeysUnique(t *testing.T) {
	t.Parallel()
	seen := map[string]bool{}
	for _, d := range recipe.Core().All() {
		if d.Name == "" || seen[d.Name] || strings.Trim(d.Name, "abcdefghijklmnopqrstuvwxyz0123456789_") != "" {
			t.Fatalf("recipe %d: name %q empty, repeated or not snake_case", d.ID, d.Name)
		}
		seen[d.Name] = true
		if id, ok := recipe.Core().IDOf("core." + d.Name); !ok || id != d.ID {
			t.Fatalf("IDOf(core.%s) = %d, %v", d.Name, id, ok)
		}
	}
}

// TestRecipeEncodeOmitsZero: unset optional fields are not encoded.
func TestRecipeEncodeOmitsZero(t *testing.T) {
	t.Parallel()
	body := EncodeRecipes(recipe.Core())[recipe.MakeStrawCarpet].Body
	for _, k := range []string{"inputs", "min_skill", "extra", "extra_qty", "also", "builds", "by_species", "legacy", "dry", "ferment", "salvage"} {
		if _, ok := body[k]; ok {
			t.Fatalf("zero %s encoded: %v", k, body)
		}
	}
	if in := body["input"].(map[string]any); len(in) != 2 || in["kind"] != "straw" || in["qty"] != 10.0 {
		t.Fatalf("input %v", in)
	}
}

// rcPlugin is a valid plugin recipe at the workbench.
func rcPlugin() map[string]any {
	return map[string]any{"label": "Carve bowl", "bench": "workbench", "work": 300.0, "skill": "craft", "learn": "crafting",
		"input": map[string]any{"kind": "wood", "qty": 5.0}, "output": "wood carpet", "out_qty": 1.0, "min_skill": 2.0,
		"also": []any{"crafting spot"}}
}

// TestRecipePluginDefFollowsCore: a plugin's recipe after core's takes ID
// Len() under its own key and name, and runs at its benches.
func TestRecipePluginDefFollowsCore(t *testing.T) {
	t.Parallel()
	defs := append(EncodeRecipes(recipe.Core()), Def{Class: RecipeClass, Key: "moremods.carve_bowl", Plugin: "moremods",
		File: "defs/r.json", Body: rcPlugin()})
	got, probs := DecodeRecipes(bsJSON(t, defs))
	if len(probs) > 0 {
		t.Fatal(probs)
	}
	n := recipe.Core().Len()
	id := recipe.ID(n)
	d := got.DefOf(id)
	if got.Len() != n+1 || got.Key(id) != "moremods.carve_bowl" || d.ID != id || d.Name != "moremods.carve_bowl" ||
		d.Bench != construction.Workbench || d.Output != item.CarpetWood || d.Work != 300 || d.MinSkill != 2 ||
		d.Input != (recipe.Input{Kind: item.Wood, Qty: 5}) {
		t.Fatalf("plugin def %+v (key %q)", d, got.Key(id))
	}
	for _, k := range []construction.Kind{construction.Workbench, construction.CraftingSpot} {
		if l := got.For(k); len(l) == 0 || l[len(l)-1].ID != id {
			t.Fatalf("%s does not run the plugin recipe", k)
		}
	}
	if l := recipe.Core().For(construction.Workbench); l[len(l)-1].ID == id {
		t.Fatal("core's book changed")
	}
	rcSame(t, recipe.NewBook(got.All()[:n], got.Keys()[:n]), recipe.Core())
}

// TestRecipeDecodeProblems: every strictness rule names its field, and a
// def with any problem is left out while the rest load.
func TestRecipeDecodeProblems(t *testing.T) {
	t.Parallel()
	in := func(b map[string]any) map[string]any { return b["input"].(map[string]any) }
	cases := []struct {
		name, field, msg string
		edit             func(map[string]any)
	}{
		{"unknown field", "colour", "unknown field", func(b map[string]any) { b["colour"] = "red" }},
		{"name is the key", "name", "unknown field", func(b map[string]any) { b["name"] = "bowl" }},
		{"label missing", "label", "required", func(b map[string]any) { delete(b, "label") }},
		{"label long", "label", "want 1 to 60", func(b map[string]any) { b["label"] = strings.Repeat("a", 61) }},
		{"bench missing", "bench", "required", func(b map[string]any) { delete(b, "bench") }},
		{"bench unknown", "bench", "unknown bench", func(b map[string]any) { b["bench"] = "anvil" }},
		{"bench not one", "bench", "not a bench", func(b map[string]any) { b["bench"] = "wall" }},
		{"bench electric", "bench", "name that bench", func(b map[string]any) { b["bench"] = "electric stove" }},
		{"also type", "also", "want a list", func(b map[string]any) { b["also"] = "stove" }},
		{"also unknown", "also.0", "unknown bench", func(b map[string]any) { b["also"] = []any{"anvil"} }},
		{"work missing", "work", "required", func(b map[string]any) { delete(b, "work") }},
		{"work type", "work", "want a number", func(b map[string]any) { b["work"] = "300" }},
		{"work fraction", "work", "not a whole number", func(b map[string]any) { b["work"] = 2.5 }},
		{"work low", "work", "out of range", func(b map[string]any) { b["work"] = 0.0 }},
		{"work high", "work", "out of range", func(b map[string]any) { b["work"] = 1000001.0 }},
		{"skill missing", "skill", "required", func(b map[string]any) { delete(b, "skill") }},
		{"skill unknown", "skill", "unknown work type", func(b map[string]any) { b["skill"] = "juggle" }},
		{"learn unknown", "learn", "unknown skill", func(b map[string]any) { b["learn"] = "dancing" }},
		{"min skill high", "min_skill", "out of range", func(b map[string]any) { b["min_skill"] = 21.0 }},
		{"output unknown", "output", "unknown item", func(b map[string]any) { b["output"] = "gold bar" }},
		{"output missing", "output", "needs an output", func(b map[string]any) { delete(b, "output") }},
		{"out qty missing", "output", "needs an output", func(b map[string]any) { delete(b, "out_qty") }},
		{"out qty high", "out_qty", "out of range", func(b map[string]any) { b["out_qty"] = 10001.0 }},
		{"extra qty low", "extra_qty", "out of range", func(b map[string]any) { b["extra_qty"] = 0.0 }},
		{"input missing", "input", "required (or inputs)", func(b map[string]any) { delete(b, "input") }},
		{"input type", "input", "want an object", func(b map[string]any) { b["input"] = "wood" }},
		{"input and inputs", "input", "not both", func(b map[string]any) { b["inputs"] = []any{map[string]any{"kind": "wood", "qty": 1.0}} }},
		{"inputs empty", "inputs", "empty", func(b map[string]any) { delete(b, "input"); b["inputs"] = []any{} }},
		{"inputs item", "inputs.1.kind", "unknown item", func(b map[string]any) {
			delete(b, "input")
			b["inputs"] = []any{map[string]any{"kind": "wood", "qty": 1.0}, map[string]any{"kind": "mithril", "qty": 1.0}}
		}},
		{"input kind missing", "input.kind", "required", func(b map[string]any) { delete(in(b), "kind") }},
		{"input qty missing", "input.qty", "required", func(b map[string]any) { delete(in(b), "qty") }},
		{"input qty low", "input.qty", "out of range", func(b map[string]any) { in(b)["qty"] = 0.0 }},
		{"input qty high", "input.qty", "out of range", func(b map[string]any) { in(b)["qty"] = 10001.0 }},
		{"input unknown field", "input.colour", "unknown field", func(b map[string]any) { in(b)["colour"] = "red" }},
		{"input flag type", "input.any_raw", "want a boolean", func(b map[string]any) { in(b)["any_raw"] = "yes" }},
		{"input stuff", "input.stuff.0", "unknown key", func(b map[string]any) { in(b)["stuff"] = []any{"glass"} }},
	}
	for _, flag := range []string{"by_species", "legacy", "dry", "ferment", "salvage"} {
		cases = append(cases, struct {
			name, field, msg string
			edit             func(map[string]any)
		}{"flag " + flag, "", "a plugin recipe cannot set them", func(b map[string]any) { b[flag] = true }})
	}
	for _, flag := range []string{"smelt", "smelt_wear", "wreck", "any_dryable"} {
		cases = append(cases, struct {
			name, field, msg string
			edit             func(map[string]any)
		}{"input flag " + flag, "", "a plugin recipe cannot set them", func(b map[string]any) { in(b)[flag] = true }})
	}
	cases = append(cases, struct {
		name, field, msg string
		edit             func(map[string]any)
	}{"builds", "", "a plugin recipe cannot set them", func(b map[string]any) { b["builds"] = "small sculpture" }})
	for _, c := range cases {
		body := rcPlugin()
		c.edit(body)
		good := Def{Class: RecipeClass, Key: "p.ok", Plugin: "p", Body: rcPlugin()}
		bad := Def{Class: RecipeClass, Key: "p.bad", Plugin: "p", File: "defs/r.json", Body: body}
		b, probs := DecodeRecipes([]Def{good, bad})
		if b.Len() != 1 || b.Key(0) != "p.ok" {
			t.Errorf("%s: bad def kept (%d recipes)", c.name, b.Len())
		}
		found := false
		for _, p := range probs {
			if p.Field == c.field && strings.Contains(p.Msg, c.msg) && p.Def == "recipe/p.bad" && p.File == "defs/r.json" {
				found = true
			}
		}
		if !found {
			t.Errorf("%s: no problem at %q with %q in %v", c.name, c.field, c.msg, probs)
		}
	}
}

// TestRecipeCorePatchKeepsBehaviour: a patched core recipe decodes when its
// plain fields change, and is refused when a behaviour flag does, whichever
// way (set on a plain recipe, cleared on a flagged one).
func TestRecipeCorePatchKeepsBehaviour(t *testing.T) {
	t.Parallel()
	defs := EncodeRecipes(recipe.Core())
	patch := func(id recipe.ID, edit func(map[string]any)) []Problem {
		d := defs[id]
		d.Body = treeCopyMap(d.Body)
		edit(d.Body)
		_, probs := DecodeRecipes([]Def{d})
		return probs
	}
	if p := patch(recipe.SimpleMeal, func(b map[string]any) { b["work"] = 90.0; b["label"] = "Cook a meal" }); len(p) > 0 {
		t.Fatalf("plain patch refused: %v", p)
	}
	if p := patch(recipe.DryFood, func(b map[string]any) { b["work"] = 60.0 }); len(p) > 0 {
		t.Fatalf("plain patch of a flagged recipe refused: %v", p)
	}
	for name, p := range map[string][]Problem{
		"set dry":        patch(recipe.SimpleMeal, func(b map[string]any) { b["dry"] = true }),
		"clear dry":      patch(recipe.DryFood, func(b map[string]any) { delete(b, "dry") }),
		"clear species":  patch(recipe.ButcherAnimal, func(b map[string]any) { b["by_species"] = false }),
		"change builds":  patch(recipe.SculptSmall, func(b map[string]any) { b["builds"] = "grand sculpture" }),
		"set smelt":      patch(recipe.Club, func(b map[string]any) { b["input"].(map[string]any)["smelt"] = true }),
		"clear salvage":  patch(recipe.ShredMech, func(b map[string]any) { delete(b, "salvage") }),
		"clear ferment":  patch(recipe.FillBarrel, func(b map[string]any) { delete(b, "ferment") }),
		"unlegacy":       patch(recipe.ButcherDustbuck, func(b map[string]any) { delete(b, "legacy") }),
		"clear wreck in": patch(recipe.ShredMech, func(b map[string]any) { delete(b["input"].(map[string]any), "wreck") }),
	} {
		if len(p) != 1 || !strings.Contains(p[0].Msg, "a patch cannot change them") {
			t.Errorf("%s: %v", name, p)
		}
	}
}

// TestRecipeBookCap: past recipe.MaxBook a recipe is refused, not wrapped
// onto a byte ID another recipe holds.
func TestRecipeBookCap(t *testing.T) {
	t.Parallel()
	defs := EncodeRecipes(recipe.Core())
	for i := recipe.Core().Len(); i <= recipe.MaxBook; i++ {
		defs = append(defs, Def{Class: RecipeClass, Key: "p.r" + string(rune('a'+i%26)) + strings.Repeat("x", i/26), Plugin: "p", Body: rcPlugin()})
	}
	b, probs := DecodeRecipes(defs)
	if b.Len() != recipe.MaxBook || len(probs) != 1 || !strings.Contains(probs[0].Msg, "too many recipes") {
		t.Fatalf("len %d, problems %v", b.Len(), probs)
	}
}
