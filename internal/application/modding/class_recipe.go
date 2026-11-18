// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package modding

import (
	"fmt"
	"slices"
	"strings"

	"rimworld-go/internal/domain/ai/work"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/recipe"
	"rimworld-go/internal/domain/pawn/skill"
)

// RecipeClass is the class name in defs files and patch targets.
const RecipeClass = recipe.Class

// Recipe field limits (PLG-025): what a def may hold.
const (
	rcMaxQty   = 10000
	rcMaxWork  = 1_000_000
	rcMaxSkill = 20
	rcMaxLabel = 60 // runes
)

// EncodeRecipes turns a book's recipes into trees under their keys, one
// per recipe in ID order (PLG-025). The name and ID are the key's and the
// position's, not fields. Zero fields are left out, except the required
// label, bench, work, skill and learn. Behaviour flags (rcBehaviour) are
// encoded too, so core round-trips exactly; a def cannot change them.
func EncodeRecipes(b recipe.Book) []Def {
	out := make([]Def, 0, b.Len())
	for i := range b.Len() {
		d, key := b.DefOf(recipe.ID(i)), b.Key(recipe.ID(i))
		plugin, _, _ := strings.Cut(key, ".")
		body := map[string]any{"label": d.Label, "bench": d.Bench.String(), "work": float64(d.Work),
			"skill": d.Skill.Key(), "learn": d.Learn.Key()}
		if len(d.Inputs) == 0 || d.Input != d.Inputs[0] {
			body["input"] = rcEncodeInput(d.Input)
		}
		if len(d.Inputs) > 0 {
			l := make([]any, len(d.Inputs))
			for j, in := range d.Inputs {
				l[j] = rcEncodeInput(in)
			}
			body["inputs"] = l
		}
		rcPut(body, "output", d.Output != 0, d.Output.String())
		rcPut(body, "out_qty", d.OutQty != 0, float64(d.OutQty))
		rcPut(body, "min_skill", d.MinSkill != 0, float64(d.MinSkill))
		rcPut(body, "extra", d.Extra != 0, d.Extra.String())
		rcPut(body, "extra_qty", d.ExtraQty != 0, float64(d.ExtraQty))
		if d.Also != nil {
			l := make([]any, len(d.Also))
			for j, k := range d.Also {
				l[j] = k.String()
			}
			body["also"] = l
		}
		rcPut(body, "builds", d.Builds != 0, d.Builds.String())
		for _, f := range []struct {
			name string
			on   bool
		}{{"by_species", d.BySpecies}, {"legacy", d.Legacy}, {"dry", d.Dry}, {"ferment", d.Ferment}, {"salvage", d.Salvage}} {
			rcPut(body, f.name, f.on, true)
		}
		out = append(out, Def{Class: RecipeClass, Key: key, Plugin: plugin, Body: body})
	}
	return out
}

func rcPut(body map[string]any, k string, set bool, v any) {
	if set {
		body[k] = v
	}
}

func rcEncodeInput(in recipe.Input) map[string]any {
	m := map[string]any{}
	rcPut(m, "kind", in.Kind != 0, in.Kind.String())
	rcPut(m, "qty", in.Qty != 0, float64(in.Qty))
	rcPut(m, "max", in.Max != 0, float64(in.Max))
	if l := in.Stuff.Keys(); len(l) > 0 {
		s := make([]any, len(l))
		for i, k := range l {
			s[i] = k
		}
		m["stuff"] = s
	}
	for _, f := range rcInputFlags(&in) {
		rcPut(m, f.name, *f.on, true)
	}
	return m
}

// rcFlag is one boolean input field: its name and where it lives.
type rcFlag struct {
	name string
	on   *bool
}

func rcInputFlags(in *recipe.Input) []rcFlag {
	return []rcFlag{{"any_raw", &in.AnyRaw}, {"any_dryable", &in.AnyDryable}, {"any_stone_blocks", &in.AnyStoneBlocks},
		{"smelt", &in.Smelt}, {"any_veg", &in.AnyVeg}, {"any_meat", &in.AnyMeat}, {"smelt_wear", &in.SmeltWear},
		{"wreck", &in.Wreck}, {"hops", &in.Hops}, {"hay", &in.Hay}}
}

// rcFlags is what of a recipe is code bound to particular recipes, not
// data: the yields from the body (by_species) or the wreck (salvage), the
// drying rack (dry, an input's any_dryable), the barrel (ferment), a
// sculpture set down (builds), smelting (an input's smelt, smelt_wear) and
// shredding (wreck), and the legacy butcher bills (legacy). A plugin
// recipe sets none; a patch leaves a core recipe's as they are.
type rcFlags struct {
	bySpecies, legacy, dry, ferment, salvage bool
	builds                                   construction.Kind
	smelt, smeltWear, wreck, dryable         bool
}

func rcBehaviour(d recipe.Def) rcFlags {
	f := rcFlags{bySpecies: d.BySpecies, legacy: d.Legacy, dry: d.Dry, ferment: d.Ferment, salvage: d.Salvage, builds: d.Builds}
	for _, in := range d.AppendInputs([]recipe.Input{d.Input}) {
		f.smelt, f.smeltWear = f.smelt || in.Smelt, f.smeltWear || in.SmeltWear
		f.wreck, f.dryable = f.wreck || in.Wreck, f.dryable || in.AnyDryable
	}
	return f
}

const rcBehaviourMsg = "by_species, legacy, dry, ferment, salvage, builds and an input's smelt, smelt_wear, wreck and any_dryable are the game's own behaviour"

// DecodeRecipes types the recipe defs (abstract ones already dropped) in
// order and freezes them into a book (PLG-025): core's come first, so each
// keeps its ID. Typing is strict: an unknown field, a wrong JSON type, a
// fractional number, an unknown item, bench, skill or work type, a value
// out of range or a changed behaviour flag is a problem, and its def is
// left out. Recipes past recipe.MaxBook are refused (an ID is a byte).
func DecodeRecipes(defs []Def) (recipe.Book, []Problem) { return DecodeRecipesWith(defs, item.Book{}) }

// DecodeRecipesWith is DecodeRecipes in a run whose item kinds are items
// (PLG-026): an input, output or extra may name a plugin item by its key,
// and the book smelts the plugin items (recipe.Book.WithItems).
func DecodeRecipesWith(defs []Def, items item.Book) (recipe.Book, []Problem) {
	var list []recipe.Def
	var keys []string
	var probs []Problem
	for _, d := range defs {
		rd := rcDecoder{bsDecoder: bsDecoder{def: d}, items: items}
		v := rd.decode()
		if len(rd.probs) == 0 && len(list) >= recipe.MaxBook {
			rd.fail("", "too many recipes: a game holds at most %d", recipe.MaxBook)
		}
		if len(rd.probs) > 0 {
			probs = append(probs, rd.probs...)
			continue
		}
		list, keys = append(list, v), append(keys, d.Key)
	}
	return recipe.NewBook(list, keys).WithItems(items), probs
}

// recipeClass is the recipe class (PLG-025). It types recipes against the
// catalog's item book, so the item class is built before it (PLG-026).
func recipeClass() class {
	return class{
		name: RecipeClass,
		core: func() []Def { return EncodeRecipes(recipe.Core()) },
		decode: func(cat *Catalog, defs []Def) []Problem {
			var items item.Book
			if cat != nil {
				items = cat.Items
			}
			b, probs := DecodeRecipesWith(defs, items)
			if cat != nil {
				cat.Recipes = b
			}
			return probs
		},
	}
}

// rcDecoder types one recipe def (bsDecoder's helpers and problems)
// against the run's item kinds.
type rcDecoder struct {
	bsDecoder
	items item.Book
}

// item parses an item a recipe names: a core kind's name or a plugin
// kind's key.
func (rd *rcDecoder) item(field string, v any) item.Kind {
	return rcRef(rd, field, v, "item", rd.items.Parse)
}

func (rd *rcDecoder) decode() recipe.Def {
	d := recipe.Def{Name: recipe.NameOf(rd.def.Key)}
	if rd.def.Class != RecipeClass {
		rd.fail("", "not a %s def", RecipeClass)
		return d
	}
	body := rd.def.Body
	for _, k := range rcSorted(body) {
		rd.field(&d, k, body[k])
	}
	for _, k := range []string{"label", "bench", "work", "skill", "learn"} {
		if _, ok := body[k]; !ok {
			rd.fail(k, "required")
		}
	}
	_, one := body["input"]
	_, many := body["inputs"]
	switch {
	case !one && !many:
		rd.fail("input", "required (or inputs)")
	case !one:
		if len(d.Inputs) > 0 {
			d.Input = d.Inputs[0]
		}
	}
	rd.behaviour(d)
	return d
}

func rcSorted(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	slices.Sort(out) // problems in a stable order
	return out
}

func (rd *rcDecoder) field(d *recipe.Def, k string, v any) {
	switch k {
	case "label":
		d.Label = rd.text(k, v, 1, rcMaxLabel)
	case "bench":
		d.Bench = rd.bench(k, v, true)
	case "also":
		d.Also = rd.benches(k, v)
	case "input":
		d.Input = rd.input(k, v)
	case "inputs":
		d.Inputs = rd.inputs(k, v)
	case "output":
		d.Output = rd.item(k, v)
	case "out_qty":
		d.OutQty = rd.num(k, v, 1, rcMaxQty)
	case "work":
		d.Work = rd.num(k, v, 1, rcMaxWork)
	case "skill":
		d.Skill = rcRef(rd, k, v, "work type", work.ParseType)
	case "learn":
		d.Learn = rcRef(rd, k, v, "skill", skill.Parse)
	case "min_skill":
		d.MinSkill = uint8(rd.num(k, v, 0, rcMaxSkill))
	case "extra":
		d.Extra = rd.item(k, v)
	case "extra_qty":
		d.ExtraQty = rd.num(k, v, 1, rcMaxQty)
	case "builds":
		d.Builds = rcRef(rd, k, v, "building", construction.Parse)
	case "by_species":
		d.BySpecies = rd.flag(k, v)
	case "legacy":
		d.Legacy = rd.flag(k, v)
	case "dry":
		d.Dry = rd.flag(k, v)
	case "ferment":
		d.Ferment = rd.flag(k, v)
	case "salvage":
		d.Salvage = rd.flag(k, v)
	default:
		rd.fail(k, "unknown field")
	}
}

// behaviour refuses a core recipe whose behaviour flags differ from the
// game's own (rcFlags), and a plugin recipe that sets one, makes nothing
// or gives both input and inputs.
func (rd *rcDecoder) behaviour(d recipe.Def) {
	if rd.def.Plugin == Core {
		if id, ok := recipe.Core().IDOf(rd.def.Key); ok {
			if rcBehaviour(d) != rcBehaviour(recipe.Core().DefOf(id)) {
				rd.fail("", "%s: a patch cannot change them", rcBehaviourMsg)
			}
			return
		}
	}
	if rcBehaviour(d) != (rcFlags{}) {
		rd.fail("", "%s: a plugin recipe cannot set them", rcBehaviourMsg)
	}
	if !rd.has("output") || !rd.has("out_qty") {
		rd.fail("output", "a plugin recipe needs an output and out_qty")
	}
	if rd.has("input") && rd.has("inputs") {
		rd.fail("input", "give input or inputs, not both")
	}
}

func (rd *rcDecoder) has(k string) bool {
	_, ok := rd.def.Body[k]
	return ok
}

func (rd *rcDecoder) flag(field string, v any) bool {
	b, ok := v.(bool)
	if !ok {
		rd.fail(field, "want a boolean, got %s", bsJSONType(v))
	}
	return b
}

// rcRef parses a string naming one of a domain's keys.
func rcRef[T any](rd *rcDecoder, field string, v any, what string, parse func(string) (T, bool)) T {
	var zero T
	s, ok := v.(string)
	if !ok {
		rd.fail(field, "want a string, got %s", bsJSONType(v))
		return zero
	}
	out, ok := parse(s)
	if !ok {
		rd.fail(field, "unknown %s %q", what, s)
		return zero
	}
	return out
}

// bench parses a bench kind a recipe can name: a bench, and as the main
// one its base kind (an electric twin runs its base bench's recipes:
// recipe.Base).
func (rd *rcDecoder) bench(field string, v any, main bool) construction.Kind {
	n := len(rd.probs)
	k := rcRef(rd, field, v, "bench", construction.Parse)
	switch {
	case len(rd.probs) > n:
	case !construction.DefOf(k).Bench:
		rd.fail(field, "%q is not a bench", k)
	case main && recipe.Base(k) != k:
		rd.fail(field, "%q runs %q's recipes: name that bench", k, recipe.Base(k))
	}
	return k
}

func (rd *rcDecoder) benches(field string, v any) []construction.Kind {
	l, ok := v.([]any)
	if !ok {
		rd.fail(field, "want a list, got %s", bsJSONType(v))
		return nil
	}
	if len(l) == 0 {
		return nil
	}
	out := make([]construction.Kind, len(l))
	for i, x := range l {
		out[i] = rd.bench(fmt.Sprintf("%s.%d", field, i), x, false)
	}
	return out
}

func (rd *rcDecoder) inputs(field string, v any) []recipe.Input {
	l, ok := v.([]any)
	if !ok {
		rd.fail(field, "want a list, got %s", bsJSONType(v))
		return nil
	}
	if len(l) == 0 {
		rd.fail(field, "empty: leave it out or list an input")
		return nil
	}
	out := make([]recipe.Input, len(l))
	for i, x := range l {
		out[i] = rd.input(fmt.Sprintf("%s.%d", field, i), x)
	}
	return out
}

func (rd *rcDecoder) input(field string, v any) recipe.Input {
	var in recipe.Input
	m, ok := v.(map[string]any)
	if !ok {
		rd.fail(field, "want an object, got %s", bsJSONType(v))
		return in
	}
	flags := rcInputFlags(&in)
	for _, k := range rcSorted(m) {
		x, f := m[k], field+"."+k
		switch k {
		case "kind":
			in.Kind = rd.item(f, x)
		case "qty":
			in.Qty = rd.num(f, x, 1, rcMaxQty)
		case "max":
			in.Max = rd.num(f, x, 1, rcMaxQty)
		case "stuff":
			in.Stuff = item.StuffCategory(rd.keys(f, x, func(s string) (uint64, bool) {
				c, ok := item.ParseStuff(s)
				return uint64(c), ok
			}))
		default:
			i := slices.IndexFunc(flags, func(fl rcFlag) bool { return fl.name == k })
			if i < 0 {
				rd.fail(f, "unknown field")
				continue
			}
			*flags[i].on = rd.flag(f, x)
		}
	}
	if _, ok := m["qty"]; !ok {
		rd.fail(field+".qty", "required")
	}
	if _, ok := m["kind"]; !ok && !slices.ContainsFunc(flags, func(fl rcFlag) bool { return *fl.on }) {
		rd.fail(field+".kind", "required (or a family such as any_raw)")
	}
	return in
}
