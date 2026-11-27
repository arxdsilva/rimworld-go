// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package modding

import (
	"reflect"
	"strings"
	"testing"

	"rimworld-go/internal/domain/pawn/backstory"
)

// fakeDefs is a ScriptDefs counting what it is asked.
type fakeDefs struct {
	files    map[string][]File    // plugin id -> registered files
	regProbs map[string][]Problem // plugin id -> problems of Register
	valProbs map[string][]Problem // validator id -> problems of Validate
	asked    []string
	validate []string
	seen     map[string][]Def // validator id -> defs it was given
}

func (f *fakeDefs) Register(p Plugin) ([]File, []Problem) {
	f.asked = append(f.asked, p.Manifest.ID)
	return f.files[p.Manifest.ID], f.regProbs[p.Manifest.ID]
}

func (f *fakeDefs) Validate(v string, defs []Def) []Problem {
	f.validate = append(f.validate, v)
	if f.seen == nil {
		f.seen = map[string][]Def{}
	}
	f.seen[v] = defs
	return f.valProbs[v]
}

func withCaps(p Plugin, caps ...string) Plugin {
	p.Manifest.Capabilities = caps
	return p
}

func smDefs() map[string]any {
	return map[string]any{StatModClass: []any{
		map[string]any{"name": "base", "abstract": true, "stat": "work_speed", "add": 2.0},
		map[string]any{"name": "fast", "parent": "base", "mul": 1100.0},
		map[string]any{"name": "bad", "stat": "work_speed", "add": 1.5},
	}}
}

func smPatch() []any {
	return []any{map[string]any{"op": "set", "target": "stat_mod/pa.fast", "path": "add", "value": 7.0}}
}

// C1: a registered def is built like the same JSON file, inheritance,
// patches and errors included.
func TestBuildWithRegisteredDefsMatchJSON(t *testing.T) {
	t.Parallel()
	caps := []string{"defs", "stats"}
	viaJSON := withCaps(plugin("pa", smDefs(), nil), caps...)
	patcher := plugin("pb", nil, smPatch())
	patcher.Manifest.After = []string{"pa"}
	patcher.Manifest.Requires = []string{"pa"}
	wantCat, wantProbs := Build([]Plugin{viaJSON, patcher}, []string{"pa", "pb"}, "0.1.0")
	if len(wantCat.StatMods.Defs) != 1 || wantCat.StatMods.Defs[0].Add != 7 || wantCat.StatMods.Defs[0].Mul != 1100 {
		t.Fatalf("fixture did not build as expected: %+v / %v", wantCat.StatMods, wantProbs)
	}

	viaScript := withCaps(plugin("pa", nil, nil), caps...)
	fake := &fakeDefs{files: map[string][]File{"pa": {{Path: "defs/a.json", Data: smDefs()}}}}
	gotCat, gotProbs := BuildWith([]Plugin{viaScript, patcher}, []string{"pa", "pb"}, "0.1.0", fake)
	if !reflect.DeepEqual(gotCat, wantCat) {
		t.Fatalf("catalog differs:\n got %+v\nwant %+v", gotCat, wantCat)
	}
	if !reflect.DeepEqual(gotProbs, wantProbs) || !Errors(gotProbs) {
		t.Fatalf("problems differ:\n got %v\nwant %v", gotProbs, wantProbs)
	}
	if !reflect.DeepEqual(fake.asked, []string{"pa"}) {
		t.Fatalf("asked %v", fake.asked)
	}
}

// Register problems are added to the build's.
func TestBuildWithRegisterProblems(t *testing.T) {
	t.Parallel()
	p := withCaps(plugin("pa", nil, nil), "defs")
	want := Problem{Plugin: "pa", Msg: "boom"}
	fake := &fakeDefs{regProbs: map[string][]Problem{"pa": {want}}}
	_, probs := BuildWith([]Plugin{p}, []string{"pa"}, "0.1.0", fake)
	if !reflect.DeepEqual(probs, []Problem{want}) {
		t.Fatalf("problems = %v", probs)
	}
}

// C2: a plugin without the capability is never asked, nor is anyone when
// there is no ScriptDefs.
func TestBuildWithSkipsUndeclared(t *testing.T) {
	t.Parallel()
	good := withCaps(plugin("good", nil, nil), "defs", "validate")
	noCap := withCaps(plugin("nocap", nil, nil), "events")
	legacy := plugin("legacy", nil, nil) // no capabilities field: legacy set has no defs
	fake := &fakeDefs{}
	_, probs := BuildWith([]Plugin{good, noCap, legacy}, []string{"good", "nocap", "legacy"}, "0.1.0", fake)
	if len(probs) != 0 {
		t.Fatalf("problems: %v", probs)
	}
	if !reflect.DeepEqual(fake.asked, []string{"good"}) || !reflect.DeepEqual(fake.validate, []string{"good"}) {
		t.Fatalf("asked %v, validated %v", fake.asked, fake.validate)
	}
	if _, probs := BuildWith([]Plugin{good}, []string{"good"}, "0.1.0", nil); len(probs) != 0 {
		t.Fatalf("nil ScriptDefs: %v", probs)
	}
}

// C3: a validation error drops a plugin's def; on a core def it is a
// warning and the def stays; an unknown ref is a warning.
func TestBuildWithValidateDrops(t *testing.T) {
	t.Parallel()
	coreRef := "backstory/" + backstory.Core().Key(1)
	defs := map[string]any{StatModClass: []any{
		map[string]any{"name": "keep", "stat": "work_speed", "add": 1.0},
		map[string]any{"name": "drop", "stat": "work_speed", "add": 2.0},
	}}
	a := withCaps(plugin("pa", defs, nil), "stats")
	v := withCaps(plugin("pv", nil, nil), "validate")
	v.Manifest.After = []string{"pa"}
	fake := &fakeDefs{valProbs: map[string][]Problem{"pv": {
		{Plugin: "pv", Def: "stat_mod/pa.drop", Msg: "no"},
		{Plugin: "pv", Def: coreRef, Msg: "meh"},
		{Plugin: "pv", Def: "stat_mod/zzz.nope", Msg: "ghost"},
		{Plugin: "pv", Msg: "general", Warn: true},
	}}}
	cat, probs := BuildWith([]Plugin{a, v}, []string{"pa", "pv"}, "0.1.0", fake)
	if len(cat.StatMods.Defs) != 1 || cat.StatMods.Defs[0].Key != "pa.keep" {
		t.Fatalf("stat mods = %+v", cat.StatMods)
	}
	if cat.Backstories.Len() != backstory.Core().Len() {
		t.Fatalf("a core def was dropped: %d", cat.Backstories.Len())
	}
	byDef := map[string]Problem{}
	for _, p := range probs {
		byDef[p.Def+"|"+p.Msg[:min(len(p.Msg), 7)]] = p
	}
	if p := byDef["stat_mod/pa.drop|no (the"]; p.Warn {
		t.Fatalf("a plugin def's error must stay an error: %v", probs)
	}
	for _, key := range []string{coreRef + "|meh (th", "stat_mod/zzz.nope|ghost (", "|general"} {
		if p, ok := byDef[key]; !ok || !p.Warn {
			t.Fatalf("%s: want a warning, got %v in %v", key, p, probs)
		}
	}
	if !strings.Contains(byDef[coreRef+"|meh (th"].Msg, "kept") {
		t.Fatalf("core warning: %v", probs)
	}
	// The validator saw the plugin's defs.
	if len(fake.seen["pv"]) < 2 {
		t.Fatalf("validator saw %d defs", len(fake.seen["pv"]))
	}
}

// A validate-only plugin is registered (the host loads its scripts there)
// and its validate_defs is honoured; its registered files are not defs.
func TestBuildWithValidateOnlyPlugin(t *testing.T) {
	t.Parallel()
	defs := map[string]any{StatModClass: []any{map[string]any{"name": "x", "stat": "work_speed", "add": 1.0}}}
	a := withCaps(plugin("pa", defs, nil), "stats")
	v := withCaps(plugin("pv", nil, nil), "validate")
	v.Manifest.After = []string{"pa"}
	fake := &fakeDefs{
		files:    map[string][]File{"pv": {{Path: "defs/z.json", Data: smDefs()}}},
		valProbs: map[string][]Problem{"pv": {{Def: "stat_mod/pa.x", Msg: "no"}}},
	}
	cat, probs := BuildWith([]Plugin{a, v}, []string{"pa", "pv"}, "0.1.0", fake)
	if !reflect.DeepEqual(fake.asked, []string{"pv"}) || !reflect.DeepEqual(fake.validate, []string{"pv"}) {
		t.Fatalf("asked %v, validated %v", fake.asked, fake.validate)
	}
	if len(cat.StatMods.Defs) != 0 || len(probs) != 1 || probs[0].Warn {
		t.Fatalf("stat mods %+v, problems %v", cat.StatMods, probs)
	}
}

// A dropped def is not shown to the next validator, and validators run in
// load order.
func TestBuildWithValidateOrder(t *testing.T) {
	t.Parallel()
	defs := map[string]any{StatModClass: []any{map[string]any{"name": "x", "stat": "work_speed", "add": 1.0}}}
	a := withCaps(plugin("pa", defs, nil), "stats", "validate")
	b := withCaps(plugin("pb", nil, nil), "validate")
	b.Manifest.After = []string{"pa"}
	fake := &fakeDefs{valProbs: map[string][]Problem{"pa": {{Def: "stat_mod/pa.x", Msg: "x"}}}}
	_, probs := BuildWith([]Plugin{b, a}, []string{"pa", "pb"}, "0.1.0", fake)
	if !reflect.DeepEqual(fake.validate, []string{"pa", "pb"}) {
		t.Fatalf("order %v", fake.validate)
	}
	for _, d := range fake.seen["pb"] {
		if d.Class == StatModClass {
			t.Fatalf("b saw dropped def %s", d.Ref())
		}
	}
	if len(probs) != 1 || probs[0].Plugin != "pa" {
		t.Fatalf("problems %v", probs)
	}
}

// An empty problem plugin is the validator.
func TestBuildWithValidateFillsPlugin(t *testing.T) {
	t.Parallel()
	v := withCaps(plugin("pv", nil, nil), "validate")
	fake := &fakeDefs{valProbs: map[string][]Problem{"pv": {{Msg: "hello"}}}}
	_, probs := BuildWith([]Plugin{v}, []string{"pv"}, "0.1.0", fake)
	if len(probs) != 1 || probs[0].Plugin != "pv" || probs[0].Warn {
		t.Fatalf("problems %v", probs)
	}
}
