// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package modding

import (
	"rimworld-go/internal/domain/pawn/mood"
)

// ThoughtClass is the thought class name in plugin files.
const ThoughtClass = mood.Class

// Thought field limits: what a patch may set.
const (
	thMaxLabel    = 40 // runes
	thMaxValue    = 100
	thMaxDuration = 7_200_000 // ticks, 120 days
	thMaxStack    = 20
	thMaxFactor   = 1000
)

// thBehaviourMsg is what a thought def cannot set: its place in the game's
// own rules.
const thBehaviourMsg = "are the game's own behaviour, written for particular thoughts: a patch cannot set them"

// thPluginMsg refuses a thought a plugin would add: nothing in the game can
// give it yet.
const thPluginMsg = "nothing in the game can give a plugin's thought yet (a thought needs a trigger, and plugins have no way to fire one); patch a core thought instead, as thought/core.<key>"

// thCodeMsg refuses a patch to a thought the game computes.
const thCodeMsg = "the game computes this thought's mood effect and line; it cannot be patched"

// EncodeThoughts turns a book's thoughts into trees under their keys, one
// per kind in kind order. The label is always present; the other numbers are
// left out when they hold their default (0, 0, 1, 1000). The group is the
// game's own and is not encoded.
func EncodeThoughts(b mood.Book) []Def {
	n := b.Len()
	out := make([]Def, 0, n)
	for i := range n {
		k := mood.Kind(i) // #nosec G115 -- i < Book.Len() <= numKinds, fits Kind
		d := b.DefOf(k)
		body := map[string]any{"label": d.Label}
		rcPut(body, "value", d.Value != 0, float64(d.Value))
		rcPut(body, "duration", d.Duration != 0, float64(d.Duration))
		rcPut(body, "stack_limit", d.StackLimit != 1, float64(d.StackLimit))
		rcPut(body, "stack_factor", d.StackFactor != 1000, float64(d.StackFactor))
		out = append(out, Def{Class: ThoughtClass, Key: b.Key(k), Plugin: Core, Body: body})
	}
	return out
}

// coreTunings is core's tuning of every thought in kind order.
func coreTunings() []mood.Tuning {
	core := mood.Core()
	list := make([]mood.Tuning, core.Len())
	for i := range list {
		d := core.DefOf(mood.Kind(i)) // #nosec G115 -- i < Len() <= numKinds, fits Kind
		list[i] = tuningOf(d)
	}
	return list
}

func tuningOf(d mood.Def) mood.Tuning {
	return mood.Tuning{Label: d.Label, Value: d.Value, Duration: d.Duration, StackLimit: d.StackLimit, StackFactor: d.StackFactor}
}

// DecodeThoughts types the thought defs and overlays each on core's book.
// Any subset of defs is valid; the rest stay core's. Typing is strict: an
// unknown field, a wrong JSON type, a fractional number or a value out of
// range is a problem, and its def is left out. A def that is not core's
// (a plugin's thought) is refused.
func DecodeThoughts(defs []Def) (mood.Book, []Problem) {
	list := coreTunings()
	var probs []Problem
	for _, d := range defs {
		td := thDecoder{bsDecoder: bsDecoder{def: d}}
		k, t := td.decode()
		if len(td.probs) > 0 {
			probs = append(probs, td.probs...)
			continue
		}
		list[k] = t
	}
	b, _ := mood.NewBook(list)
	return b, probs
}

// thDecoder types one thought def.
type thDecoder struct{ bsDecoder }

func (td *thDecoder) decode() (mood.Kind, mood.Tuning) {
	if td.def.Class != ThoughtClass {
		td.fail("", "not a %s def", ThoughtClass)
		return 0, mood.Tuning{}
	}
	k, ok := mood.Core().KindOf(td.def.Key)
	if td.def.Plugin != Core || !ok {
		td.fail("", "%s", thPluginMsg)
		return 0, mood.Tuning{}
	}
	coreDef := mood.Core().DefOf(k)
	t := mood.Tuning{StackLimit: 1, StackFactor: 1000}
	body := td.def.Body
	for _, f := range rcSorted(body) {
		td.field(&t, coreDef, f, body[f])
	}
	if _, ok := body["label"]; !ok {
		td.fail("label", "required")
	}
	if _, ok := body["duration"]; !ok && coreDef.Duration > 0 {
		td.fail("duration", "a remembered thought needs a duration")
	}
	if len(td.probs) == 0 && mood.CodeValued(k) && t != tuningOf(coreDef) {
		td.fail("", "%s", thCodeMsg)
	}
	return k, t
}

func (td *thDecoder) field(t *mood.Tuning, core mood.Def, f string, v any) {
	switch f {
	case "label":
		t.Label = td.text(f, v, 1, thMaxLabel)
	case "value":
		t.Value = td.num(f, v, -thMaxValue, thMaxValue)
	case "duration":
		if core.Duration == 0 {
			td.fail(f, "a state thought lasts while its condition holds")
			return
		}
		t.Duration = uint64(td.num(f, v, 1, thMaxDuration)) // #nosec G115 -- num rejects outside [1, thMaxDuration=7200000], positive
	case "stack_limit":
		t.StackLimit = uint8(td.num(f, v, 1, thMaxStack)) // #nosec G115 -- num rejects outside [1, thMaxStack=20], fits uint8
	case "stack_factor":
		t.StackFactor = int32(td.num(f, v, 1, thMaxFactor)) // #nosec G115 -- num rejects outside [1, thMaxFactor=1000], fits int32
	case "group", "kind", "id":
		td.fail(f, "group, kind and id "+thBehaviourMsg)
	default:
		td.fail(f, "unknown field")
	}
}

// thoughtClass is the thought class: patch-only, plugins add none.
func thoughtClass() class {
	return class{
		name: ThoughtClass,
		core: func() []Def { return EncodeThoughts(mood.Core()) },
		decode: func(cat *Catalog, defs []Def) []Problem {
			b, probs := DecodeThoughts(defs)
			if cat != nil {
				cat.Thoughts = b
			}
			return probs
		},
	}
}
