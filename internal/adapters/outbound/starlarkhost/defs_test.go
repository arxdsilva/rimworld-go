// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package starlarkhost

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"rimworld-go/internal/application/modding"
	"rimworld-go/internal/application/pluginapi"
)

func defsPlugin(id string, caps []string, scripts map[string]string) modding.Plugin {
	p := modding.Plugin{Manifest: modding.Manifest{ID: id, Capabilities: caps}}
	for path, src := range scripts {
		p.Scripts = append(p.Scripts, modding.Blob{Path: path, Data: []byte(src)})
	}
	return p
}

func TestRegisterDefsJSONShape(t *testing.T) {
	const src = `
def register_defs():
    return {"backstory": [{"key": "a.b", "n": 3, "ok": True, "tags": ["x", "y"], "sub": {"k": 1, "l": [2]}}]}
`
	files, probs := NewDefs().Register(defsPlugin("a", []string{"defs"}, map[string]string{"scripts/main.star": src}))
	if len(probs) != 0 || len(files) != 1 {
		t.Fatalf("files=%v probs=%v", files, probs)
	}
	if files[0].Path != "scripts/main.star#register_defs" {
		t.Fatalf("path %q", files[0].Path)
	}
	var want any
	if err := json.Unmarshal([]byte(`{"backstory":[{"key":"a.b","n":3,"ok":true,"tags":["x","y"],"sub":{"k":1,"l":[2]}}]}`), &want); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(files[0].Data, want) {
		t.Fatalf("got %#v\nwant %#v", files[0].Data, want)
	}
}

func TestRegisterDefsOrderAndGate(t *testing.T) {
	scripts := map[string]string{
		"scripts/b.star": "def register_defs():\n    return {\"t\": []}\n",
		"scripts/a.star": "def register_defs():\n    return {\"t\": []}\nx = 1\n",
		"scripts/c.star": "x = 1\n",
		"scripts/d.txt":  "ignored",
	}
	files, probs := NewDefs().Register(defsPlugin("a", []string{"defs"}, scripts))
	if len(probs) != 0 || len(files) != 2 || files[0].Path != "scripts/a.star#register_defs" || files[1].Path != "scripts/b.star#register_defs" {
		t.Fatalf("files=%v probs=%v", files, probs)
	}
	// Without the defs capability (and legacy has none) nothing runs.
	for _, caps := range [][]string{{"query"}, nil} {
		if f, p := NewDefs().Register(defsPlugin("a", caps, scripts)); f != nil || p != nil {
			t.Fatalf("caps %v ran: %v %v", caps, f, p)
		}
	}
	if pluginapi.Legacy().Has(pluginapi.Defs) {
		t.Fatal("legacy set must not grant defs")
	}
}

func TestRegisterDefsErrors(t *testing.T) {
	for name, src := range map[string]string{
		"not a dict":    "def register_defs():\n    return [1]\n",
		"none":          "def register_defs():\n    return None\n",
		"class not str": "def register_defs():\n    return {1: []}\n",
		"not a list":    "def register_defs():\n    return {\"c\": {}}\n",
		"not a def":     "def register_defs():\n    return {\"c\": [1]}\n",
		"float":         "def register_defs():\n    return {\"c\": [{\"k\": 1.5}]}\n",
		"none value":    "def register_defs():\n    return {\"c\": [{\"k\": None}]}\n",
		"int key":       "def register_defs():\n    return {\"c\": [{1: 2}]}\n",
		"nested key":    "def register_defs():\n    return {\"c\": [{\"k\": {1: 2}}]}\n",
		"big int":       "def register_defs():\n    return {\"c\": [{\"k\": 1 << 80}]}\n",
		"runtime error": "def register_defs():\n    return 1 // 0\n",
		"syntax":        "def register_defs(:\n",
		"load missing":  "load(\"lib/nope.star\", \"x\")\n",
		"cycle":         "def register_defs():\n    l = []\n    l.append(l)\n    return {\"c\": [{\"k\": l}]}\n",
	} {
		files, probs := NewDefs().Register(defsPlugin("a", []string{"defs"}, map[string]string{"scripts/m.star": src}))
		if len(files) != 0 || len(probs) != 1 {
			t.Errorf("%s: files=%v probs=%v", name, files, probs)
			continue
		}
		if probs[0].Plugin != "a" || !strings.HasPrefix(probs[0].File, "scripts/m.star") || probs[0].Msg == "" || probs[0].Warn {
			t.Errorf("%s: problem %+v", name, probs[0])
		}
	}
	// A bad module does not stop a good one.
	files, probs := NewDefs().Register(defsPlugin("a", []string{"defs"}, map[string]string{
		"scripts/a.star": "def register_defs():\n    return None\n",
		"scripts/b.star": "def register_defs():\n    return {\"c\": []}\n",
	}))
	if len(files) != 1 || len(probs) != 1 {
		t.Fatalf("files=%v probs=%v", files, probs)
	}
}

func TestRegisterDefsNoCtx(t *testing.T) {
	// register_defs takes no argument: a script expecting a ctx fails.
	_, probs := NewDefs().Register(defsPlugin("a", []string{"defs"}, map[string]string{
		"scripts/m.star": "def register_defs(ctx):\n    return {}\n",
	}))
	if len(probs) != 1 {
		t.Fatalf("probs=%v", probs)
	}
}

func testDefs() []modding.Def {
	return []modding.Def{
		{Class: "backstory", Key: "core.a", Plugin: "core", Body: map[string]any{"title": "A", "n": float64(3), "x": 1.5, "tags": []any{"p"}, "m": map[string]any{"k": true}, "z": nil, "ss": []string{"s"}, "i": 7, "i64": int64(8), "other": struct{}{}}},
		{Class: "recipe", Key: "p.b", Plugin: "p", Abstract: true, Parent: "p.base"},
	}
}

func TestValidateDefs(t *testing.T) {
	const src = `
def validate_defs(defs):
    out = []
    for d in defs:
        if getattr(d, "class") == "recipe" and d.abstract and d.parent == "p.base" and d.plugin == "p":
            out.append({"def": "recipe/p.b", "msg": "bad recipe"})
        if getattr(d, "class") == "backstory" and d.fields["n"] == 3 and d.fields["m"]["k"] and d.fields["tags"][0] == "p" and d.fields["z"] == None and d.fields["x"] == 1.5:
            out.append({"msg": "saw core", "warn": True})
    return out
`
	d := NewDefs()
	d.Register(defsPlugin("v", []string{"validate"}, map[string]string{"scripts/v.star": src}))
	got := d.Validate("v", testDefs())
	want := []modding.Problem{
		{Plugin: "v", Def: "recipe/p.b", Msg: "bad recipe"},
		{Plugin: "v", Msg: "saw core", Warn: true},
	}
	if len(got) != 2 {
		t.Fatalf("got %+v", got)
	}
	// Order within one def loop follows the defs: backstory first.
	if got[0] != want[1] || got[1] != want[0] {
		t.Fatalf("got %+v", got)
	}
}

func TestValidateDefsGateAndFrozen(t *testing.T) {
	d := NewDefs()
	d.Register(defsPlugin("nocap", []string{"defs"}, map[string]string{"scripts/v.star": "def validate_defs(defs):\n    return [{\"msg\": \"x\"}]\n"}))
	if got := d.Validate("nocap", testDefs()); got != nil {
		t.Fatalf("ran without the capability: %v", got)
	}
	if got := d.Validate("unknown", nil); len(got) != 1 || !got[0].Warn {
		t.Fatalf("unknown validator: %v", got)
	}
	// The defs are frozen: a write is an error, shown as a warning.
	d.Register(defsPlugin("w", []string{"validate"}, map[string]string{"scripts/v.star": "def validate_defs(defs):\n    defs[0].fields[\"n\"] = 9\n    return []\n"}))
	got := d.Validate("w", testDefs())
	if len(got) != 1 || !got[0].Warn || !strings.Contains(got[0].Msg, "skipped") {
		t.Fatalf("frozen write: %v", got)
	}
}

func TestValidateDefsBadShape(t *testing.T) {
	for name, ret := range map[string]string{
		"not list":  "1",
		"not dict":  "[1]",
		"no msg":    "[{\"def\": \"a/b\"}]",
		"def int":   "[{\"def\": 1, \"msg\": \"m\"}]",
		"warn int":  "[{\"msg\": \"m\", \"warn\": 1}]",
		"extra key": "[{\"msg\": \"m\", \"wat\": 1}]",
	} {
		d := NewDefs()
		d.Register(defsPlugin("v", []string{"validate"}, map[string]string{"scripts/v.star": "def validate_defs(defs):\n    return " + ret + "\n"}))
		got := d.Validate("v", nil)
		if len(got) != 1 || !got[0].Warn || got[0].Plugin != "v" || got[0].File != "scripts/v.star" {
			t.Errorf("%s: %+v", name, got)
		}
	}
}

func TestDefsBudget(t *testing.T) {
	const spin = "def f():\n    n = 0\n    for i in range(1000000):\n        n += i\n    return n\n"
	// register_defs over the step limit: a problem, no file.
	d := newDefs(1000)
	files, probs := d.Register(defsPlugin("a", []string{"defs", "validate"}, map[string]string{
		"scripts/m.star": spin + "def register_defs():\n    f()\n    return {}\ndef validate_defs(defs):\n    f()\n    return []\n",
	}))
	if len(files) != 0 || len(probs) != 1 || !strings.Contains(probs[0].Msg, "too many steps") {
		t.Fatalf("register: files=%v probs=%v", files, probs)
	}
	got := d.Validate("a", nil)
	if len(got) != 1 || !got[0].Warn || !strings.Contains(got[0].Msg, "too many steps") {
		t.Fatalf("validate: %v", got)
	}
	// The first run over the limit is a load problem.
	files, probs = newDefs(1000).Register(defsPlugin("b", []string{"defs"}, map[string]string{
		"scripts/m.star": "x = len([i for i in range(1000000)])\n",
	}))
	if len(files) != 0 || len(probs) != 1 {
		t.Fatalf("load: files=%v probs=%v", files, probs)
	}
	// The same script under the real limit passes.
	if files, probs = NewDefs().Register(defsPlugin("c", []string{"defs"}, map[string]string{
		"scripts/m.star": "def register_defs():\n    return {\"c\": [{\"n\": len(range(100))}]}\n",
	})); len(files) != 1 || len(probs) != 0 {
		t.Fatalf("files=%v probs=%v", files, probs)
	}
}

func TestDefsLoad(t *testing.T) {
	d := NewDefs()
	p := defsPlugin("a", []string{"defs"}, map[string]string{
		"scripts/main.star":  "load(\"lib/u.star\", \"mk\")\ndef register_defs():\n    return {\"c\": [mk()]}\n",
		"scripts/lib/u.star": "def mk():\n    return {\"k\": \"v\"}\n",
	})
	files, probs := d.Register(p)
	if len(probs) != 0 || len(files) != 1 {
		t.Fatalf("files=%v probs=%v", files, probs)
	}
	// A second Register of the same plugin reuses the loaded modules.
	files2, _ := d.Register(p)
	if !reflect.DeepEqual(files, files2) {
		t.Fatal("second Register differs")
	}
}
