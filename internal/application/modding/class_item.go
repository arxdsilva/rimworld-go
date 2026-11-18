// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package modding

import (
	"bytes"
	"image/png"
	"slices"
	"strings"

	"rimworld-go/internal/domain/economy/item"
)

// ItemClass is the item class name in plugin files.
const ItemClass = item.Class

// Item field limits (PLG-026): what a def may hold.
const (
	itMaxLabel   = 60  // runes
	itMaxDesc    = 400 // runes
	itMaxValue   = 100_000
	itMaxMass    = 100_000 // grams
	itMaxStock   = 1000
	itMaxSmelt   = 1000
	itMaxTexture = 256 // pixels a side
)

// ItemPaths are the storage filter categories a plugin item may be filed
// under: the goods that need no code. Food, medicine, drugs, body parts,
// weapons, apparel, corpses, chunks, books, shells, crated buildings,
// carpets, wood and stone blocks each have behaviour of their own.
func ItemPaths() []string {
	return []string{"Manufactured/Components", "Manufactured/Textiles", "Raw resources/Metals",
		"Raw resources/Chemicals", "Raw resources/Brewing and leaves", "Items/Parts"}
}

// itFixed is why a patch may not change a core item's mass, stack limit or
// trader stock: saves and the game's own rules read them from the kind.
const itFixed = "a patch may not change a core item's mass, stack limit or trader stock (a plugin item sets its own)"

// itCoded are fields asking for behaviour a plugin item cannot have.
func itCoded() []string {
	return []string{"nutrition", "food", "medicine", "drug", "weapon", "apparel", "armor", "corpse", "stuff",
		"perishable", "flammability"}
}

// Textures finds a loaded plugin's texture (PLG-026): its PNG bytes by
// plugin id and slash path inside it ("textures/bell.png").
type Textures func(plugin, path string) ([]byte, bool)

// TexturesOf is the textures/**.png of found, by plugin id and path.
func TexturesOf(found []Plugin) Textures {
	m := map[string][]byte{}
	for _, p := range found {
		for _, b := range p.Textures {
			m[p.Manifest.ID+"\x00"+b.Path] = b.Data
		}
	}
	return func(plugin, path string) ([]byte, bool) {
		b, ok := m[plugin+"\x00"+path]
		return b, ok
	}
}

// itemClass is the item class (PLG-026) without plugin textures: a
// plugin item, which needs one, is refused (itemClassWith reads them).
func itemClass() class { return itemClassWith(nil) }

// itemClassWith is the item class reading plugin textures through tex.
func itemClassWith(tex Textures) class {
	return class{
		name: ItemClass,
		core: func() []Def { return EncodeItems(item.Core()) },
		decode: func(cat *Catalog, defs []Def) []Problem {
			b, probs := DecodeItems(defs, tex)
			if cat != nil {
				cat.Items = b
			}
			return probs
		},
	}
}

// EncodeItems turns a book's core kinds into trees under their keys, in
// Kind order: the label, mass and stack limit (fixed for core) and the value
// (a patch may change it). Trader stock is not encoded.
// Plugin kinds are not encoded (core's trees are what patches target).
func EncodeItems(b item.Book) []Def {
	out := make([]Def, 0, item.CoreKinds)
	for k := range item.CoreKinds {
		d := b.Def(item.Kind(k))
		out = append(out, Def{Class: ItemClass, Key: b.Key(item.Kind(k)), Plugin: Core, Body: map[string]any{
			"label": d.Label, "mass": float64(d.Mass), "value": float64(d.Value), "stack_limit": float64(d.Stack)}})
	}
	return out
}

// DecodeItems types item defs into a book: core's first, in Kind order (a
// patch may change their value only), then the plugins' simple goods,
// each with a label, a storage category of ItemPaths, a value and a
// texture tex finds and decodes as a PNG. A plugin good may set mass
// (grams, 1 to 100000; default 1000), stack_limit (1 to item.StackLimit;
// default item.StackLimit) and trader_stock (0 to 1000; default 0, a trader
// carries none). A bad def is a problem and is left out; kinds past
// item.MaxKinds are refused.
func DecodeItems(defs []Def, tex Textures) (item.Book, []Problem) {
	var list []item.Def
	var keys []string
	var probs []Problem
	core := item.Core()
	for _, d := range defs {
		id := itDecoder{bsDecoder: bsDecoder{def: d}, tex: tex}
		v := id.decode(core)
		if len(id.probs) == 0 && len(list) >= item.MaxKinds {
			id.fail("", "too many item kinds: a game holds at most %d", item.MaxKinds)
		}
		if len(id.probs) > 0 {
			probs = append(probs, id.probs...)
			continue
		}
		list, keys = append(list, v), append(keys, d.Key)
	}
	if len(list) < item.CoreKinds { // only keepCore's single-def checks get here
		return core, probs
	}
	return item.NewBook(list, keys), probs
}

// itDecoder types one item def.
type itDecoder struct {
	bsDecoder
	tex Textures
}

func (id *itDecoder) decode(core item.Book) item.Def {
	var d item.Def
	if id.def.Class != ItemClass {
		id.fail("", "not an %s def", ItemClass)
		return d
	}
	if id.def.Plugin == Core {
		return id.decodeCore(core)
	}
	d.Mass, d.Stack = item.None.MassGrams(), item.None.StackLimit() // defaults: what every Kind method answers for it
	body := id.def.Body
	for _, k := range rcSorted(body) {
		id.field(&d, k, body[k])
	}
	for _, k := range []string{"label", "category", "value", "texture"} {
		if _, ok := body[k]; !ok {
			id.fail(k, "required")
		}
	}
	return d
}

// decodeCore types a core kind's (patched) tree: only its numbers may
// differ from the game's own.
func (id *itDecoder) decodeCore(core item.Book) item.Def {
	k, ok := core.KindOf(id.def.Key)
	if !ok {
		id.fail("", "the game has no item %q", id.def.Key)
		return item.Def{}
	}
	d := core.Def(k)
	for _, f := range rcSorted(id.def.Body) {
		v := id.def.Body[f]
		switch f {
		case "label":
			if s, _ := v.(string); s != d.Label {
				id.fail(f, "the game's own items keep their names (a translation renames them)")
			}
		case "mass", "stack_limit", "trader_stock":
			if n, ok := v.(float64); !ok || int(n) != map[string]int{"mass": d.Mass, "stack_limit": d.Stack, "trader_stock": d.TraderStock}[f] {
				id.fail(f, "%s", itFixed)
			}
		case "value":
			d.Value = id.num(f, v, 0, itMaxValue)
		default:
			id.fail(f, "a patch may change an item's value only")
		}
	}
	return d
}

func (id *itDecoder) field(d *item.Def, k string, v any) {
	switch k {
	case "label":
		d.Label = id.text(k, v, 1, itMaxLabel)
	case "desc":
		d.Desc = id.text(k, v, 0, itMaxDesc)
	case "category":
		d.Path = id.text(k, v, 1, 80)
		if !slices.Contains(ItemPaths(), d.Path) {
			id.fail(k, "%q: a plugin item goes under one of %s (the other categories need code)", d.Path,
				strings.Join(ItemPaths(), ", "))
		}
	case "mass":
		d.Mass = id.num(k, v, 1, itMaxMass)
	case "stack_limit":
		d.Stack = id.num(k, v, 1, item.StackLimit)
	case "trader_stock":
		d.TraderStock = id.num(k, v, 0, itMaxStock)
	case "value":
		d.Value = id.num(k, v, 0, itMaxValue)
	case "smelt":
		d.Smelt = id.num(k, v, 0, itMaxSmelt)
	case "texture":
		d.Texture = id.texture(k, v)
	default:
		if slices.Contains(itCoded(), k) {
			id.fail(k, "a plugin item is a simple good: food, medicine, drugs, weapons, apparel, corpses and building materials need code")
			return
		}
		id.fail(k, "unknown field")
	}
}

// texture checks path names a PNG in the def's own plugin that decodes,
// at most itMaxTexture pixels a side.
func (id *itDecoder) texture(field string, v any) string {
	path := id.text(field, v, 1, 200)
	if !strings.HasPrefix(path, "textures/") || !strings.HasSuffix(path, ".png") || strings.Contains(path, "..") {
		id.fail(field, "%q: want a textures/….png path inside the plugin", path)
		return path
	}
	var data []byte
	ok := false
	if id.tex != nil {
		data, ok = id.tex(id.def.Plugin, path)
	}
	if !ok {
		id.fail(field, "%q: no such texture in the plugin", path)
		return path
	}
	img, err := png.Decode(bytes.NewReader(data))
	switch {
	case err != nil:
		id.fail(field, "%q is not a valid PNG: %v", path, err)
	case img.Bounds().Dx() < 1 || img.Bounds().Dy() < 1 || img.Bounds().Dx() > itMaxTexture || img.Bounds().Dy() > itMaxTexture:
		id.fail(field, "%q is %dx%d pixels, want 1 to %d a side", path, img.Bounds().Dx(), img.Bounds().Dy(), itMaxTexture)
	}
	return path
}
