// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package modding

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"slices"
	"strings"
	"testing"

	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/recipe"
)

// itPNG is a w×h PNG.
func itPNG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	img.Set(0, 0, color.NRGBA{200, 120, 40, 255})
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// itTex is one plugin's textures.
func itTex(plugin string, files map[string][]byte) Textures {
	p := Plugin{Manifest: Manifest{ID: plugin}}
	for path, b := range files {
		p.Textures = append(p.Textures, Blob{Path: path, Data: b})
	}
	return TexturesOf([]Plugin{p})
}

// itBell is a plugin item def.
func itBell(body map[string]any) Def {
	full := map[string]any{"label": "copper bell", "category": "Manufactured/Components", "value": float64(40),
		"texture": "textures/bell.png"}
	for k, v := range body {
		if v == nil {
			delete(full, k)
			continue
		}
		full[k] = v
	}
	return Def{Class: ItemClass, Key: "frontier.bell", Plugin: "frontier", File: "defs/items.json", Body: full}
}

// Decoding core's encoded defs (straight and through JSON) gives core back.
func TestItemCodecRoundTripsCore(t *testing.T) {
	t.Parallel()
	core := item.Core()
	defs := EncodeItems(core)
	if len(defs) != item.CoreKinds || defs[item.Steel].Key != "core.steel" {
		t.Fatalf("%d defs; steel's key %q", len(defs), defs[item.Steel].Key)
	}
	for name, in := range map[string][]Def{"trees": defs, "json": bsJSON(t, defs)} {
		got, probs := DecodeItems(in, nil)
		if len(probs) > 0 {
			t.Fatalf("%s: %v", name, probs)
		}
		if !slices.Equal(got.Keys(), core.Keys()) {
			t.Fatalf("%s: keys differ", name)
		}
		for k := range item.CoreKinds {
			if got.Def(item.Kind(k)) != core.Def(item.Kind(k)) {
				t.Fatalf("%s: kind %d: %+v, want %+v", name, k, got.Def(item.Kind(k)), core.Def(item.Kind(k)))
			}
		}
	}
}

// A plugin item is a simple good with a texture of its own; whatever
// needs code, a bad category or a missing or broken texture is refused.
func TestItemCodecPluginItem(t *testing.T) {
	t.Parallel()
	tex := itTex("frontier", map[string][]byte{"textures/bell.png": itPNG(t, 16, 16),
		"textures/broken.png": []byte("not a png"), "textures/huge.png": itPNG(t, 300, 4)})
	core := EncodeItems(item.Core())
	b, probs := DecodeItems(append(core, itBell(map[string]any{"smelt": float64(3), "desc": "It rings."})), tex)
	bell := item.Kind(item.CoreKinds)
	if len(probs) > 0 || b.Len() != item.CoreKinds+1 || b.Key(bell) != "frontier.bell" {
		t.Fatalf("len %d, problems %v", b.Len(), probs)
	}
	want := item.Def{Label: "copper bell", Desc: "It rings.", Path: "Manufactured/Components", Mass: 1000, Value: 40,
		Stack: item.StackLimit, Texture: "textures/bell.png", Smelt: 3}
	if b.Def(bell) != want {
		t.Fatalf("bell = %+v", b.Def(bell))
	}
	for name, c := range map[string]struct {
		body  map[string]any
		field string
		msg   string
	}{
		"food":          {map[string]any{"nutrition": float64(5)}, "nutrition", "simple good"},
		"weapon":        {map[string]any{"weapon": true}, "weapon", "need code"},
		"category":      {map[string]any{"category": "Foods/Meals"}, "category", "need code"},
		"no texture":    {map[string]any{"texture": nil}, "texture", "required"},
		"missing":       {map[string]any{"texture": "textures/gone.png"}, "texture", "no such texture"},
		"broken":        {map[string]any{"texture": "textures/broken.png"}, "texture", "not a valid PNG"},
		"huge":          {map[string]any{"texture": "textures/huge.png"}, "texture", "300x4"},
		"outside":       {map[string]any{"texture": "../bell.png"}, "texture", "inside the plugin"},
		"stack":         {map[string]any{"stack_limit": float64(25)}, "stack_limit", "not moddable yet"},
		"mass":          {map[string]any{"mass": float64(700)}, "mass", "not moddable yet"},
		"unknown field": {map[string]any{"colour": "red"}, "colour", "unknown field"},
	} {
		got, probs := DecodeItems(append(EncodeItems(item.Core()), itBell(c.body)), tex)
		if got.Len() != item.CoreKinds || len(probs) == 0 || probs[0].Field != c.field || !strings.Contains(probs[0].Msg, c.msg) ||
			probs[0].Def != "item/frontier.bell" {
			t.Errorf("%s: len %d, problems %v", name, got.Len(), probs)
		}
	}
	// Another plugin's texture is not this one's; nil textures find none.
	if _, probs := DecodeItems(append(EncodeItems(item.Core()), itBell(nil)), itTex("other", map[string][]byte{"textures/bell.png": itPNG(t, 4, 4)})); len(probs) == 0 {
		t.Error("a texture from another plugin was taken")
	}
	if probs := itemClass().decode(nil, []Def{itBell(nil)}); len(probs) == 0 {
		t.Error("an item was taken with no textures at all")
	}
}

// A patch may change a core item's value, never its name, mass or stack
// limit.
func TestItemCodecCorePatch(t *testing.T) {
	t.Parallel()
	patched := func(f string, v any) []Def {
		defs := EncodeItems(item.Core())
		for i := range defs {
			body := map[string]any{}
			for k, x := range defs[i].Body {
				body[k] = x
			}
			defs[i].Body = body
		}
		defs[item.Steel].Body[f] = v
		defs[item.Rifle].Body["stack_limit"] = float64(1)
		return defs
	}
	b, probs := DecodeItems(patched("value", float64(3)), nil)
	if len(probs) > 0 || b.Value(item.Steel) != 3 || b.Def(item.Steel).Mass != item.Steel.MassGrams() {
		t.Fatalf("value patch: %v %+v", probs, b.Def(item.Steel))
	}
	for f, v := range map[string]any{"label": "iron", "texture": "textures/x.png", "mass": float64(1), "stack_limit": float64(10)} {
		if _, probs := DecodeItems(patched(f, v), nil); len(probs) != 1 || probs[0].Field != f {
			t.Errorf("patching %s: %v", f, probs)
		}
	}
}

// Plugin kinds past item.MaxKinds are refused.
func TestItemCodecCap(t *testing.T) {
	t.Parallel()
	tex := itTex("frontier", map[string][]byte{"textures/bell.png": itPNG(t, 4, 4)})
	defs := EncodeItems(item.Core())
	for i := range item.MaxKinds - item.CoreKinds + 2 {
		d := itBell(nil)
		d.Key = "frontier.bell" + strings.Repeat("x", i)
		defs = append(defs, d)
	}
	b, probs := DecodeItems(defs, tex)
	if b.Len() != item.MaxKinds || len(probs) != 2 || !strings.Contains(probs[0].Msg, "at most") {
		t.Fatalf("len %d, problems %v", b.Len(), probs)
	}
}

// A plugin recipe names a plugin item by its key, as input and output;
// without the item book the key is unknown. The book smelts the item by
// the steel its recipe takes.
func TestRecipeCodecPluginItems(t *testing.T) {
	t.Parallel()
	tex := itTex("frontier", map[string][]byte{"textures/bell.png": itPNG(t, 4, 4)})
	items, probs := DecodeItems(append(EncodeItems(item.Core()), itBell(nil)), tex)
	if len(probs) > 0 {
		t.Fatal(probs)
	}
	bell := item.Kind(item.CoreKinds)
	cast := Def{Class: RecipeClass, Key: "frontier.cast_bell", Plugin: "frontier", Body: map[string]any{
		"label": "Cast a bell", "bench": "workbench", "work": float64(300), "skill": "craft", "learn": "crafting",
		"input": map[string]any{"kind": "steel", "qty": float64(8)}, "output": "frontier.bell", "out_qty": float64(2)}}
	ring := Def{Class: RecipeClass, Key: "frontier.ring", Plugin: "frontier", Body: map[string]any{
		"label": "Ring", "bench": "crafting spot", "work": float64(100), "skill": "craft", "learn": "crafting",
		"input": map[string]any{"kind": "frontier.bell", "qty": float64(1)}, "output": "wood", "out_qty": float64(1)}}
	defs := append(EncodeRecipes(recipe.Core()), cast, ring)
	b, probs := DecodeRecipesWith(defs, items)
	if len(probs) > 0 {
		t.Fatal(probs)
	}
	n := recipe.Core().Len()
	if b.DefOf(recipe.ID(n)).Output != bell || b.DefOf(recipe.ID(n+1)).Input.Kind != bell {
		t.Fatalf("plugin recipes: %+v / %+v", b.DefOf(recipe.ID(n)), b.DefOf(recipe.ID(n+1)))
	}
	if !b.DefOf(recipe.ID(n)).At(construction.Workbench) {
		t.Fatal("the bell is not cast at a workbench")
	}
	// 8 steel makes 2 bells: 4 a bell, a quarter back each.
	smelt := recipe.Core().DefOf(recipe.SmeltWeapon).Input
	if y := b.SmeltYield(item.Stack{Kind: bell, Qty: 4}); y != 4 || !b.Takes(smelt, bell) || !b.TakesStack(smelt, item.Stack{Kind: bell, Qty: 1}) {
		t.Fatalf("4 bells smelt to %d steel; taken %v", y, b.Takes(smelt, bell))
	}
	if smelt.Accepts(bell) || recipe.Core().Takes(smelt, bell) {
		t.Fatal("core's book smelts a plugin kind")
	}
	if _, probs := DecodeRecipes(defs); len(probs) != 2 || !strings.Contains(probs[0].Msg, "unknown item") {
		t.Fatalf("without the item book: %v", probs)
	}
}
