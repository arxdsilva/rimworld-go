// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package modding

import (
	"slices"
	"strings"

	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/recipe"
	"rimworld-go/internal/domain/incident"
	"rimworld-go/internal/domain/pawn/backstory"
	"rimworld-go/internal/domain/pawn/trait"
	"rimworld-go/internal/domain/research"
)

// Catalog is the frozen content of one run: the load order, the loaded
// plugins' manifests, every moddable class's book, and every def's tree
// with its provenance (MASTER_SPEC §52).
type Catalog struct {
	Order       Order
	Plugins     []Manifest // the loaded plugins in load order, core excluded
	Backstories backstory.Book
	Recipes     recipe.Book
	Traits      trait.Book
	Research    research.Book
	Incidents   incident.Book
	Items       item.Book
	Defs        []Def // every typed def after inheritance and patches
}

// IDs is the load order's plugin ids, core first: what a save records.
// The zero Catalog is core alone.
func (c Catalog) IDs() []string {
	if len(c.Order.Loaded) == 0 {
		return []string{Core}
	}
	out := make([]string, len(c.Order.Loaded))
	for i, p := range c.Order.Loaded {
		out[i] = p.ID
	}
	return out
}

// class is one moddable definition class: its core defs as trees (in
// core order, which saves rely on) and how its typed defs become its book
// on the catalog. A new class is one entry here plus its codec file.
type class struct {
	name string
	core func() []Def
	// decode types defs (core's first, in core order, then the plugins')
	// and, when cat is not nil, stores the frozen book on it.
	decode func(cat *Catalog, defs []Def) []Problem
}

// classes are the moddable classes, in the order their books are built.
func classes() []class { return classesWith(nil) }

// classesWith are the classes reading plugin textures through tex. Items
// come before recipes: a recipe may name a plugin item (PLG-026).
func classesWith(tex Textures) []class {
	return []class{{
		name: BackstoryClass,
		core: func() []Def { return EncodeBackstories(backstory.Core()) },
		decode: func(cat *Catalog, defs []Def) []Problem {
			b, probs := DecodeBackstories(defs)
			if cat != nil {
				cat.Backstories = b
			}
			return probs
		},
	}, itemClassWith(tex), recipeClass(), traitClass(), researchClass(), incidentClass()}
}

// Classes are the moddable class names, in the order their books are built.
func Classes() []string {
	var out []string
	for _, c := range classes() {
		out = append(out, c.name)
	}
	return out
}

// Build runs the whole pipeline over the installed plugins (found, as a
// Source lists them) for the enabled ids:
//
//	resolve the load order
//	→ core's defs as trees, then each loaded plugin's defs files
//	→ inheritance → patches, in load order
//	→ type each class into its book
//
// A broken def or patch drops only itself; a plugin whose manifest or
// dependencies fail does not load (see Resolve). Every problem names its
// plugin, file, def and field.
func Build(found []Plugin, enabled []string, gameVersion string) (Catalog, []Problem) {
	manifests := make([]Manifest, len(found))
	byID := make(map[string]int, len(found))
	for i, p := range found {
		manifests[i] = p.Manifest
		byID[p.Manifest.ID] = i
	}
	order, rp := Resolve(manifests, enabled, gameVersion)
	// What the adapter could not read comes first, for every enabled plugin
	// whether it loads or not: a broken plugin.json is why a plugin fails,
	// so its line and column must reach the player (PLG-041).
	var probs []Problem
	for _, p := range found {
		if slices.Contains(enabled, p.Manifest.ID) {
			probs = append(probs, p.Problems...)
		}
	}
	probs = append(probs, rp...)
	cat := Catalog{Order: order}

	var defs []Def
	for _, c := range classes() {
		defs = append(defs, c.core()...)
	}
	var ops []Op
	for _, placed := range order.Loaded {
		if placed.ID == Core {
			continue
		}
		p := found[byID[placed.ID]] // Resolve loads only ids installed once
		cat.Plugins = append(cat.Plugins, p.Manifest)
		d, dp := CollectDefs(p)
		defs = append(defs, d...)
		probs = append(probs, dp...)
		o, op := ParseOps(p)
		ops = append(ops, o...)
		probs = append(probs, op...)
	}
	defs, probs = known(defs, probs)
	defs, ip := Inherit(defs)
	probs = append(probs, ip...)
	defs, ap := Apply(defs, ops, order.Has)
	probs = append(probs, ap...)

	for _, c := range classesWith(TexturesOf(found)) {
		typed, kp := keepCore(c, defs, ops)
		probs = append(probs, kp...)
		probs = append(probs, c.decode(&cat, typed)...)
		cat.Defs = append(cat.Defs, typed...)
	}
	return cat, probs
}

// keepCore is class c's defs after patches with every core def in its own
// place, first and in core order: saves keep definitions by index, so a
// core def a patch removed or broke would shift every later one. Such a
// patch is refused (the game's own def is kept) with a problem; plugin
// defs follow in load order.
func keepCore(c class, defs []Def, ops []Op) ([]Def, []Problem) {
	var probs []Problem
	core := c.core()
	patched := make(map[string]Def, len(core))
	var extra []Def
	for _, d := range defs {
		switch {
		case d.Class != c.name:
		case d.Plugin == Core:
			patched[d.Key] = d
		default:
			extra = append(extra, d)
		}
	}
	out := make([]Def, 0, len(core)+len(extra))
	for _, orig := range core {
		d, ok := patched[orig.Key]
		if !ok {
			probs = append(probs, Problem{Plugin: remover(ops, orig.Ref()), Def: orig.Ref(),
				Msg: "the game's own definitions cannot be removed (saves refer to them); the removal is ignored"})
			out = append(out, orig)
			continue
		}
		if dp := c.decode(nil, []Def{d}); Errors(dp) {
			probs = append(probs, dp...)
			probs = append(probs, Problem{Plugin: lastToucher(d), Def: d.Ref(),
				Msg: "the patched definition is invalid; the game's own is kept"})
			out = append(out, orig)
			continue
		}
		out = append(out, d)
	}
	return append(out, extra...), probs
}

// remover is the plugin of the last whole-def remove op on ref.
func remover(ops []Op, ref string) string {
	for i := len(ops) - 1; i >= 0; i-- {
		if o := ops[i]; o.Kind == "remove" && o.Path == "" && o.Target == ref {
			return o.Plugin
		}
	}
	return ""
}

// lastToucher is the plugin whose patch touched d last ("" for none).
func lastToucher(d Def) string {
	if n := len(d.Touched); n > 0 {
		return d.Touched[n-1].Plugin
	}
	return ""
}

// known drops defs of a class no book reads, each with a problem.
func known(defs []Def, probs []Problem) ([]Def, []Problem) {
	out := defs[:0:0]
	for _, d := range defs {
		if slices.Contains(Classes(), d.Class) {
			out = append(out, d)
			continue
		}
		probs = append(probs, Problem{Plugin: d.Plugin, File: d.File, Def: d.Ref(),
			Msg: "unknown definition class " + d.Class + " (known: " + strings.Join(Classes(), ", ") + ")"})
	}
	return out, probs
}

// Errors reports whether probs holds anything but warnings.
func Errors(probs []Problem) bool {
	return slices.ContainsFunc(probs, func(p Problem) bool { return !p.Warn })
}
