// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package modding

import (
	"fmt"
	"slices"
	"strings"

	"rimworld-go/internal/domain/economy/recipe"
	"rimworld-go/internal/domain/research"
)

// ResearchClass is the research class name in plugin files.
const ResearchClass = research.Class

// Research field limits (PLG-026): what a def may hold.
const (
	rsMaxCost  = 1_000_000
	rsMaxLabel = 60   // runes
	rsMaxDesc  = 1000 // runes
)

// researchClass is the research class (PLG-026). Its plugin unlocks name
// recipes, so it decodes after the recipe class, against the run's recipes.
func researchClass() class {
	return class{
		name: ResearchClass,
		core: func() []Def { return EncodeResearch(research.Core()) },
		decode: func(cat *Catalog, defs []Def) []Problem {
			recipes := recipe.Core()
			if cat != nil {
				recipes = cat.Recipes
			}
			b, probs := DecodeResearch(defs, recipes)
			if cat != nil {
				cat.Research = b
			}
			return probs
		},
	}
}

// EncodeResearch turns a book's projects into trees under their keys, one
// per project in ID order (PLG-026). The name and Project are the key's
// and the position's, not fields; requires and hidden name project keys;
// unlocks are unlock keys ("build:<kind>", "crop:<plant>", "recipe:<name>").
// Empty fields are left out, except the required label, cost and tech.
func EncodeResearch(b research.Book) []Def {
	out := make([]Def, 0, b.Len())
	for i := range b.Len() {
		p := research.Project(i)
		d, key := b.DefOf(p), b.Key(p)
		plugin, _, _ := strings.Cut(key, ".")
		body := map[string]any{"label": d.Label, "cost": float64(d.Cost), "tech": d.TechLevel.Key()}
		rcPut(body, "desc", d.Desc != "", d.Desc)
		rcPut(body, "bench", d.Bench != research.SimpleBench, d.Bench.Key())
		rcPut(body, "requires", len(d.Requires) > 0, rsKeys(b, d.Requires))
		rcPut(body, "hidden", len(d.Hidden) > 0, rsKeys(b, d.Hidden))
		if len(d.Unlocks) > 0 {
			l := make([]any, len(d.Unlocks))
			for j, u := range d.Unlocks {
				l[j] = u
			}
			body["unlocks"] = l
		}
		out = append(out, Def{Class: ResearchClass, Key: key, Plugin: plugin, Body: body})
	}
	return out
}

func rsKeys(b research.Book, ps []research.Project) []any {
	out := make([]any, len(ps))
	for i, p := range ps {
		out[i] = b.Key(p)
	}
	return out
}

// rsTyped is one project def typed on its own: its prerequisites are still
// keys, resolved once the whole book is known.
type rsTyped struct {
	def              Def
	v                research.Def
	requires, hidden []string
}

// DecodeResearch types the research defs (abstract ones already dropped)
// in order and freezes them into a book (PLG-026): core's come first, so
// each keeps its Project. Typing is strict: an unknown field, a wrong JSON
// type, a fractional number, an unknown tech level, bench tier or recipe,
// or a value out of range is a problem, and its def is left out. A patch
// may change a core project's label, description, cost, tech level and
// bench, never its prerequisites or unlocks (the game's code and other
// saves rely on them). A plugin project's requires and hidden name project
// keys, core or plugin; its unlocks name recipes of recipes (a plugin
// recipe's name is its whole key). A plugin project that needs an unknown
// or left-out project, or whose prerequisites loop, is left out too.
// Projects past research.MaxBook are refused (a Project is a byte).
func DecodeResearch(defs []Def, recipes recipe.Book) (research.Book, []Problem) {
	var typed []rsTyped
	var probs []Problem
	gone := map[string]bool{} // keys left out, for the problems of what needs them
	for _, d := range defs {
		rd := rsDecoder{bsDecoder: bsDecoder{def: d}, recipes: recipes}
		t := rd.decode()
		if len(rd.probs) == 0 && len(typed) >= research.MaxBook {
			rd.fail("", "too many research projects: a game holds at most %d", research.MaxBook)
		}
		if len(rd.probs) > 0 {
			probs = append(probs, rd.probs...)
			gone[d.Key] = true
			continue
		}
		typed = append(typed, t)
	}
	typed, rp := rsResolve(typed, gone)
	probs = append(probs, rp...)
	list, keys := make([]research.Def, len(typed)), make([]string, len(typed))
	at := make(map[string]research.Project, len(typed))
	for i, k := range research.Core().Keys() { // core's keep their slots (keepCore)
		at[k] = research.Project(i)
	}
	for i, t := range typed {
		at[t.def.Key] = research.Project(i)
	}
	for i, t := range typed {
		list[i], keys[i] = t.v, t.def.Key
		list[i].Requires, list[i].Hidden = rsProjects(at, t.requires), rsProjects(at, t.hidden)
	}
	return research.NewBook(list, keys), probs
}

// rsProjects maps resolved keys to Projects (nil for none, as core's).
func rsProjects(at map[string]research.Project, keys []string) []research.Project {
	if len(keys) == 0 {
		return nil
	}
	out := make([]research.Project, len(keys))
	for i, k := range keys {
		out[i] = at[k]
	}
	return out
}

// rsResolve leaves out every plugin project that needs a project the book
// lacks (unknown, or itself left out: repeated until none is) or whose
// prerequisites loop (it can never be researched), each with a problem.
// Core projects need only core projects (a patch cannot change that), and
// every core project is in the book (keepCore).
func rsResolve(typed []rsTyped, gone map[string]bool) ([]rsTyped, []Problem) {
	var probs []Problem
	for changed := true; changed; {
		changed = false
		have := rsCoreKeys()
		for _, t := range typed {
			have[t.def.Key] = true
		}
		kept := typed[:0:0]
		for _, t := range typed {
			if p, ok := rsMissing(t, have, gone); ok {
				probs = append(probs, p)
				gone[t.def.Key], changed = true, true
				continue
			}
			kept = append(kept, t)
		}
		typed = kept
	}
	return rsAcyclic(typed, probs)
}

// rsMissing is the problem of a plugin project t that needs a project
// not in have (ok false: it needs none).
func rsMissing(t rsTyped, have, gone map[string]bool) (Problem, bool) {
	if t.def.Plugin == Core {
		return Problem{}, false
	}
	for _, f := range []struct {
		name string
		keys []string
	}{{"requires", t.requires}, {"hidden", t.hidden}} {
		for i, k := range f.keys {
			if have[k] {
				continue
			}
			rd := bsDecoder{def: t.def}
			msg := "unknown research project %q"
			if gone[k] {
				msg = "needs %q, which was left out"
			}
			rd.fail(fmt.Sprintf("%s.%d", f.name, i), msg, k)
			return rd.probs[0], true
		}
	}
	return Problem{}, false
}

// rsAcyclic leaves out the plugin projects that can never be researched
// because their prerequisites loop (or need such a loop): what is left
// after every project whose prerequisites can all be met is peeled off.
func rsAcyclic(typed []rsTyped, probs []Problem) ([]rsTyped, []Problem) {
	met := rsCoreKeys() // core's prerequisites never loop (the table, fixed)
	for changed := true; changed; {
		changed = false
		for _, t := range typed {
			if !met[t.def.Key] && rsAllMet(t, met) {
				met[t.def.Key], changed = true, true
			}
		}
	}
	kept := typed[:0:0]
	for _, t := range typed {
		if met[t.def.Key] {
			kept = append(kept, t)
			continue
		}
		rd := bsDecoder{def: t.def}
		rd.fail("requires", "its prerequisites loop (through %q): it could never be researched", rsLoopVia(t, met))
		probs = append(probs, rd.probs...)
	}
	return kept, probs
}

// rsCoreKeys is a set of every core project's key.
func rsCoreKeys() map[string]bool {
	keys := research.Core().Keys()
	out := make(map[string]bool, len(keys)+research.MaxBook)
	for _, k := range keys {
		out[k] = true
	}
	return out
}

func rsAllMet(t rsTyped, met map[string]bool) bool {
	for _, k := range slices.Concat(t.requires, t.hidden) {
		if !met[k] {
			return false
		}
	}
	return true
}

// rsLoopVia is t's first prerequisite that can never be met.
func rsLoopVia(t rsTyped, met map[string]bool) string {
	for _, k := range slices.Concat(t.requires, t.hidden) {
		if !met[k] {
			return k
		}
	}
	return ""
}

// rsDecoder types one research def (bsDecoder's helpers and problems).
type rsDecoder struct {
	bsDecoder
	recipes recipe.Book
}

func (rd *rsDecoder) decode() rsTyped {
	t := rsTyped{def: rd.def, v: research.Def{Name: research.NameOf(rd.def.Key)}}
	if rd.def.Class != ResearchClass {
		rd.fail("", "not a %s def", ResearchClass)
		return t
	}
	body := rd.def.Body
	for _, k := range rcSorted(body) {
		rd.field(&t, k, body[k])
	}
	for _, k := range []string{"label", "cost", "tech"} {
		if _, ok := body[k]; !ok {
			rd.fail(k, "required")
		}
	}
	if rd.def.Plugin == Core {
		rd.fixed(t)
		return t
	}
	for i, u := range t.v.Unlocks {
		t.v.Unlocks[i] = rd.unlock(fmt.Sprintf("unlocks.%d", i), u)
	}
	return t
}

func (rd *rsDecoder) field(t *rsTyped, k string, v any) {
	switch k {
	case "label":
		t.v.Label = rd.text(k, v, 1, rsMaxLabel)
	case "desc":
		t.v.Desc = rd.text(k, v, 0, rsMaxDesc)
	case "cost":
		t.v.Cost = rd.num(k, v, 1, rsMaxCost)
	case "tech":
		t.v.TechLevel = rsRef(rd, k, v, "tech level", research.ParseTechLevel)
	case "bench":
		t.v.Bench = rsRef(rd, k, v, "bench tier", research.ParseBenchTier)
	case "requires":
		t.requires = rd.strings(k, v)
	case "hidden":
		t.hidden = rd.strings(k, v)
	case "unlocks":
		t.v.Unlocks = rd.strings(k, v)
	default:
		rd.fail(k, "unknown field")
	}
}

// fixed refuses a core project whose prerequisites or unlocks differ from
// the game's own: a patch may change its numbers and words only.
func (rd *rsDecoder) fixed(t rsTyped) {
	core := research.Core()
	p, ok := core.IDOf(rd.def.Key)
	if !ok {
		rd.fail("", "the game has no research project %q", rd.def.Key)
		return
	}
	orig := core.DefOf(p)
	for _, f := range []struct {
		name      string
		got, want []string
	}{{"requires", t.requires, rsKeyStrings(core, orig.Requires)}, {"hidden", t.hidden, rsKeyStrings(core, orig.Hidden)},
		{"unlocks", t.v.Unlocks, orig.Unlocks}} {
		if !slices.Equal(f.got, f.want) {
			rd.fail(f.name, "a patch cannot change a core project's %s", f.name)
		}
	}
}

func rsKeyStrings(b research.Book, ps []research.Project) []string {
	var out []string
	for _, p := range ps {
		out = append(out, b.Key(p))
	}
	return out
}

// unlock is a plugin project's unlock: a recipe of the run, by name ("x"
// or "recipe:x"), as its unlock key "recipe:x".
func (rd *rsDecoder) unlock(field, s string) string {
	name := strings.TrimPrefix(s, "recipe:")
	if strings.Contains(name, ":") {
		rd.fail(field, "%q: a plugin project unlocks recipes, by name", s)
		return s
	}
	if !slices.ContainsFunc(rd.recipes.All(), func(d recipe.Def) bool { return d.Name == name }) {
		rd.fail(field, "unknown recipe %q", name)
	}
	return "recipe:" + name
}

// strings is a list of strings (nil for an empty one, as core's).
func (rd *rsDecoder) strings(field string, v any) []string {
	l, ok := v.([]any)
	if !ok {
		rd.fail(field, "want a list, got %s", bsJSONType(v))
		return nil
	}
	var out []string
	for i, x := range l {
		s, ok := x.(string)
		switch {
		case !ok:
			rd.fail(fmt.Sprintf("%s.%d", field, i), "want a string, got %s", bsJSONType(x))
			continue
		case s == "":
			rd.fail(fmt.Sprintf("%s.%d", field, i), "empty")
			continue
		}
		out = append(out, s)
	}
	return out
}

// rsRef parses a string naming one of a domain's keys.
func rsRef[T any](rd *rsDecoder, field string, v any, what string, parse func(string) (T, bool)) T {
	var zero T
	s, ok := v.(string)
	if !ok {
		rd.fail(field, "want a string, got %s", bsJSONType(v))
		return zero
	}
	out, ok := parse(s)
	if !ok {
		rd.fail(field, "unknown %s %q", what, s)
	}
	return out
}
