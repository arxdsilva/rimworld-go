// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package modding

import (
	"slices"
	"strings"
	"testing"

	"rimworld-go/internal/domain/pawn/mood"
)

// firstKind is the first kind that is not code-valued and whose duration
// is above (remembered) or at zero (state).
func firstKind(t *testing.T, remembered bool) mood.Kind {
	t.Helper()
	core := mood.Core()
	for i := range core.Len() {
		k := mood.Kind(i)
		if !mood.CodeValued(k) && (core.DefOf(k).Duration > 0) == remembered {
			return k
		}
	}
	t.Fatalf("no data-driven thought with remembered=%v", remembered)
	return 0
}

func firstCodeValued(t *testing.T) mood.Kind {
	t.Helper()
	for i := range mood.Core().Len() {
		if mood.CodeValued(mood.Kind(i)) {
			return mood.Kind(i)
		}
	}
	t.Fatal("no code-valued thought")
	return 0
}

func thoughtOp(k mood.Kind, op, path string, value any) map[string]any {
	m := map[string]any{"op": op, "target": "thought/" + mood.Core().Key(k)}
	if path != "" {
		m["path"] = path
	}
	if value != nil {
		m["value"] = value
	}
	return m
}

// C1: core encodes and decodes back to itself.
func TestThoughtsRoundTrip(t *testing.T) {
	t.Parallel()
	core := mood.Core()
	enc := EncodeThoughts(core)
	if len(enc) != core.Len() {
		t.Fatalf("encoded %d, want %d", len(enc), core.Len())
	}
	b, probs := DecodeThoughts(enc)
	if len(probs) != 0 {
		t.Fatalf("problems: %v", probs)
	}
	if b.Len() != core.Len() {
		t.Fatalf("Len = %d", b.Len())
	}
	for i := range core.Len() {
		k := mood.Kind(i)
		if b.DefOf(k) != core.DefOf(k) || b.Key(k) != core.Key(k) {
			t.Fatalf("kind %d: %+v vs %+v", i, b.DefOf(k), core.DefOf(k))
		}
	}
}

// The encoded body leaves out defaults and never carries the group.
func TestEncodeThoughtsOmitsDefaults(t *testing.T) {
	t.Parallel()
	for _, d := range EncodeThoughts(mood.Core()) {
		if d.Class != ThoughtClass || d.Plugin != Core || !strings.HasPrefix(d.Key, "core.") {
			t.Fatalf("def %+v", d)
		}
		if _, ok := d.Body["label"]; !ok {
			t.Fatalf("%s has no label", d.Key)
		}
		for _, f := range []string{"group", "kind", "id"} {
			if _, ok := d.Body[f]; ok {
				t.Fatalf("%s encodes %s", d.Key, f)
			}
		}
		for f, def := range map[string]float64{"value": 0, "duration": 0, "stack_limit": 1, "stack_factor": 1000} {
			if v, ok := d.Body[f]; ok && v.(float64) == def {
				t.Fatalf("%s encodes default %s", d.Key, f)
			}
		}
	}
}

// C2: a core-only build is core; one patched def decodes alone.
func TestThoughtsBuildCoreAndSubset(t *testing.T) {
	t.Parallel()
	cat, probs := Build(nil, nil, "0.1.35")
	if len(probs) != 0 {
		t.Fatalf("problems: %v", probs)
	}
	core := mood.Core()
	for i := range core.Len() {
		if cat.Thoughts.DefOf(mood.Kind(i)) != core.DefOf(mood.Kind(i)) {
			t.Fatalf("kind %d moved", i)
		}
	}
	k := firstKind(t, true)
	d := EncodeThoughts(core)[k]
	d.Body["value"] = 3.0
	b, probs := DecodeThoughts([]Def{d})
	if len(probs) != 0 {
		t.Fatalf("problems: %v", probs)
	}
	for i := range core.Len() {
		want := core.DefOf(mood.Kind(i))
		if mood.Kind(i) == k {
			want.Value = 3
		}
		if b.DefOf(mood.Kind(i)) != want {
			t.Fatalf("kind %d: %+v, want %+v", i, b.DefOf(mood.Kind(i)), want)
		}
	}
}

// C3: patches to every tuning field reach the catalog and are recorded.
func TestThoughtsPatchedThroughBuild(t *testing.T) {
	t.Parallel()
	k := firstKind(t, true)
	ops := []any{
		thoughtOp(k, "set", "label", "new line"),
		thoughtOp(k, "set", "value", 7.0),
		thoughtOp(k, "set", "duration", 1200.0),
		thoughtOp(k, "set", "stack_limit", 3.0),
		thoughtOp(k, "set", "stack_factor", 500.0),
	}
	cat, probs := Build([]Plugin{plugin("fixture", nil, ops)}, []string{"fixture"}, "0.1.35")
	if len(probs) != 0 {
		t.Fatalf("problems: %v", probs)
	}
	got := cat.Thoughts.DefOf(k)
	if got.Label != "new line" || got.Value != 7 || got.Duration != 1200 || got.StackLimit != 3 || got.StackFactor != 500 {
		t.Fatalf("def = %+v", got)
	}
	var touched int
	for _, d := range cat.Defs {
		if d.Class == ThoughtClass && d.Key == mood.Core().Key(k) {
			touched = len(d.Touched)
		}
	}
	if touched != len(ops) {
		t.Fatalf("Touched = %d, want %d", touched, len(ops))
	}
}

// C4: each bad field is a named problem and core's def is kept.
func TestThoughtsBadFields(t *testing.T) {
	t.Parallel()
	rem, state := firstKind(t, true), firstKind(t, false)
	cases := []struct {
		name  string
		k     mood.Kind
		op    map[string]any
		field string
		msg   string
	}{
		{"wrong type", rem, thoughtOp(rem, "set", "value", "high"), "value", "want a number"},
		{"fraction", rem, thoughtOp(rem, "set", "value", 1.5), "value", "whole number"},
		{"value high", rem, thoughtOp(rem, "set", "value", 101.0), "value", "out of range"},
		{"value low", rem, thoughtOp(rem, "set", "value", -101.0), "value", "out of range"},
		{"duration zero", rem, thoughtOp(rem, "set", "duration", 0.0), "duration", "out of range"},
		{"duration high", rem, thoughtOp(rem, "set", "duration", 7_200_001.0), "duration", "out of range"},
		{"stack_limit zero", rem, thoughtOp(rem, "set", "stack_limit", 0.0), "stack_limit", "out of range"},
		{"stack_limit high", rem, thoughtOp(rem, "set", "stack_limit", 21.0), "stack_limit", "out of range"},
		{"stack_factor zero", rem, thoughtOp(rem, "set", "stack_factor", 0.0), "stack_factor", "out of range"},
		{"stack_factor high", rem, thoughtOp(rem, "set", "stack_factor", 1001.0), "stack_factor", "out of range"},
		{"label long", rem, thoughtOp(rem, "set", "label", strings.Repeat("x", 41)), "label", "characters"},
		{"label type", rem, thoughtOp(rem, "set", "label", 4.0), "label", "want a string"},
		{"unknown field", rem, thoughtOp(rem, "set", "colour", "red"), "colour", "unknown field"},
		{"group", rem, thoughtOp(rem, "set", "group", "x"), "group", "own behaviour"},
		{"kind", rem, thoughtOp(rem, "set", "kind", 1.0), "kind", "own behaviour"},
		{"id", rem, thoughtOp(rem, "set", "id", 1.0), "id", "own behaviour"},
		{"duration on state", state, thoughtOp(state, "set", "duration", 100.0), "duration", "lasts while its condition holds"},
		{"removed duration", rem, thoughtOp(rem, "remove", "duration", nil), "duration", "needs a duration"},
		{"removed label", rem, thoughtOp(rem, "remove", "label", nil), "label", "required"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			cat, probs := Build([]Plugin{plugin("fixture", nil, []any{c.op})}, []string{"fixture"}, "0.1.35")
			// The typing problem names the def and field; a second one names
			// the plugin that last touched it.
			var hit, refused bool
			for _, p := range probs {
				ref := "thought/" + mood.Core().Key(c.k)
				if p.Def == ref && p.Field == c.field && strings.Contains(p.Msg, c.msg) {
					hit = true
				}
				if p.Plugin == "fixture" && p.Def == ref && strings.Contains(p.Msg, "game's own is kept") {
					refused = true
				}
			}
			if !hit || !refused {
				t.Fatalf("no problem for %s/%s %q: %v", c.name, c.field, c.msg, probs)
			}
			if cat.Thoughts.DefOf(c.k) != mood.Core().DefOf(c.k) {
				t.Fatalf("core's def not kept: %+v", cat.Thoughts.DefOf(c.k))
			}
		})
	}
}

// C5: a code-valued thought cannot be patched; unchanged it decodes.
func TestThoughtsCodeValuedRefused(t *testing.T) {
	t.Parallel()
	k := firstCodeValued(t)
	cat, probs := Build([]Plugin{plugin("fixture", nil, []any{thoughtOp(k, "set", "value", 5.0)})}, []string{"fixture"}, "0.1.35")
	var hit bool
	for _, p := range probs {
		hit = hit || strings.Contains(p.Msg, "cannot be patched")
	}
	if !hit {
		t.Fatalf("no refusal: %v", probs)
	}
	if cat.Thoughts.DefOf(k) != mood.Core().DefOf(k) {
		t.Fatal("core def not kept")
	}
	if _, probs := DecodeThoughts([]Def{EncodeThoughts(mood.Core())[k]}); len(probs) != 0 {
		t.Fatalf("unchanged core def: %v", probs)
	}
}

// C6: a plugin's own thought is refused with the trigger message.
func TestThoughtsPluginDefRefused(t *testing.T) {
	t.Parallel()
	defs := map[string]any{"thought": []any{map[string]any{"name": "new_thought", "label": "hi", "value": 1.0, "duration": 100.0}}}
	cat, probs := Build([]Plugin{plugin("fixture", defs, nil)}, []string{"fixture"}, "0.1.35")
	var hit bool
	for _, p := range probs {
		hit = hit || (p.Def == "thought/fixture.new_thought" && strings.Contains(p.Msg, "nothing in the game can give a plugin's thought"))
	}
	if !hit {
		t.Fatalf("no refusal: %v", probs)
	}
	if cat.Thoughts.Len() != mood.Core().Len() {
		t.Fatalf("Len = %d", cat.Thoughts.Len())
	}
	var found bool
	for _, d := range cat.Defs {
		found = found || d.Key == "fixture.new_thought"
	}
	if !found {
		t.Fatal("refused def not in cat.Defs")
	}
}

// A non-thought def and a core-keyed def of another plugin both fail.
func TestDecodeThoughtsRefusals(t *testing.T) {
	t.Parallel()
	_, probs := DecodeThoughts([]Def{{Class: "trait", Key: "core.x"}})
	if len(probs) != 1 || !strings.Contains(probs[0].Msg, "not a thought def") {
		t.Fatalf("probs = %v", probs)
	}
	d := EncodeThoughts(mood.Core())[0]
	d.Plugin = "other"
	if _, probs = DecodeThoughts([]Def{d}); len(probs) != 1 {
		t.Fatalf("probs = %v", probs)
	}
}

// C7: the later plugin in load order wins, whatever the discovery order.
func TestThoughtsLoadOrderLastWins(t *testing.T) {
	t.Parallel()
	k := firstKind(t, true)
	a := plugin("aaa", nil, []any{thoughtOp(k, "set", "value", 1.0)})
	b := plugin("bbb", nil, []any{thoughtOp(k, "set", "value", 2.0)})
	b.Manifest.After = []string{"aaa"}
	var want int
	var wantProbs []Problem
	for i, found := range [][]Plugin{{a, b}, {b, a}} {
		cat, probs := Build(found, []string{"aaa", "bbb"}, "0.1.35")
		got := cat.Thoughts.DefOf(k).Value
		if got != 2 {
			t.Fatalf("value = %d, want 2", got)
		}
		if i == 0 {
			want, wantProbs = got, probs
		} else if got != want || !slices.Equal(probs, wantProbs) {
			t.Fatalf("order changed the result: %v vs %v", probs, wantProbs)
		}
	}
}

// Q1: every accepted bound decodes and lands in the book.
func TestThoughtsAcceptedBounds(t *testing.T) {
	t.Parallel()
	rem := firstKind(t, true)
	multi := func(n int) string { return strings.Repeat("é", n) } // 2 bytes per rune
	cases := []struct {
		name  string
		field string
		val   any
		check func(mood.Def) bool
	}{
		{"value max", "value", 100.0, func(d mood.Def) bool { return d.Value == 100 }},
		{"value min", "value", -100.0, func(d mood.Def) bool { return d.Value == -100 }},
		{"duration max", "duration", 7_200_000.0, func(d mood.Def) bool { return d.Duration == 7_200_000 }},
		{"duration min", "duration", 1.0, func(d mood.Def) bool { return d.Duration == 1 }},
		{"stack_limit max", "stack_limit", 20.0, func(d mood.Def) bool { return d.StackLimit == 20 }},
		{"stack_limit min", "stack_limit", 1.0, func(d mood.Def) bool { return d.StackLimit == 1 }},
		{"stack_factor max", "stack_factor", 1000.0, func(d mood.Def) bool { return d.StackFactor == 1000 }},
		{"stack_factor min", "stack_factor", 1.0, func(d mood.Def) bool { return d.StackFactor == 1 }},
		{"label 40 runes", "label", multi(40), func(d mood.Def) bool { return d.Label == multi(40) }},
		{"label 1 rune", "label", "é", func(d mood.Def) bool { return d.Label == "é" }},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			d := EncodeThoughts(mood.Core())[rem]
			d.Body[c.field] = c.val
			b, probs := DecodeThoughts([]Def{d})
			if len(probs) != 0 {
				t.Fatalf("problems: %v", probs)
			}
			if !c.check(b.DefOf(rem)) {
				t.Fatalf("not in the book: %+v", b.DefOf(rem))
			}
		})
	}
}
