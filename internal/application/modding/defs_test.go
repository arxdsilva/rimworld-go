// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package modding

import (
	"reflect"
	"strings"
	"testing"
)

// tplug is a plugin with the given defs and patches files.
func tplug(id string, defs, patches []File) Plugin {
	return Plugin{Manifest: Manifest{ID: id, Name: id, Version: "1.0"}, Defs: defs, Patches: patches}
}

// tprob finds the first problem whose message holds sub.
func tprob(t *testing.T, probs []Problem, sub string) Problem {
	t.Helper()
	for _, p := range probs {
		if strings.Contains(p.Msg, sub) {
			return p
		}
	}
	t.Fatalf("no problem mentions %q in %v", sub, probs)
	return Problem{}
}

// trefs is each def's Ref, in order.
func trefs(defs []Def) []string {
	out := make([]string, len(defs))
	for i, d := range defs {
		out[i] = d.Ref()
	}
	return out
}

func TestCollectDefsReadsClassesInSortedOrder(t *testing.T) {
	p := tplug("mymod", []File{
		{Path: "defs/a.json", Data: tobj{
			"recipe": tarr(tobj{"name": "stew", "work": 5.0}),
			"backstory": tarr(
				tobj{"name": "base", "abstract": true, "skills": tobj{"shooting": 2.0}},
				tobj{"name": "sniper", "parent": "base", "title": "Sniper"},
				tobj{"name": "medic", "parent": "core.doctor"},
			),
		}},
		{Path: "defs/b.json", Data: tobj{"backstory": tarr(tobj{"name": "cook"})}},
	}, nil)
	defs, probs := CollectDefs(p)
	if len(probs) != 0 {
		t.Fatalf("problems: %v", probs)
	}
	want := []string{"backstory/mymod.base", "backstory/mymod.sniper", "backstory/mymod.medic",
		"recipe/mymod.stew", "backstory/mymod.cook"}
	if got := trefs(defs); !reflect.DeepEqual(got, want) {
		t.Fatalf("refs = %v, want %v", got, want)
	}
	base, sniper, medic := defs[0], defs[1], defs[2]
	if !base.Abstract || base.Parent != "" || !treeEqual(base.Body, tobj{"skills": tobj{"shooting": 2.0}}) {
		t.Fatalf("base = %+v", base)
	}
	if sniper.Parent != "mymod.base" || sniper.Abstract || !treeEqual(sniper.Body, tobj{"title": "Sniper"}) {
		t.Fatalf("sniper = %+v", sniper)
	}
	if medic.Parent != "core.doctor" {
		t.Fatalf("a dotted parent must be taken as written, got %q", medic.Parent)
	}
	if base.Plugin != "mymod" || base.File != "defs/a.json" || defs[4].File != "defs/b.json" {
		t.Fatalf("provenance: %+v / %+v", base, defs[4])
	}
	if _, ok := base.Body["name"]; ok {
		t.Fatal("name must not stay in the body")
	}
}

func TestCollectDefsDoesNotAliasTheTree(t *testing.T) {
	data := tobj{"backstory": tarr(tobj{"name": "a", "skills": tobj{"x": 1.0}})}
	before := treeCopy(data)
	defs, _ := CollectDefs(tplug("m", []File{{Path: "defs/a.json", Data: data}}, nil))
	defs[0].Body["skills"].(tobj)["x"] = 9.0
	if !treeEqual(data, before) {
		t.Fatal("CollectDefs aliased its input tree")
	}
}

func TestCollectDefsProblems(t *testing.T) {
	p := tplug("m", []File{
		{Path: "defs/list.json", Data: tarr()},
		{Path: "defs/a.json", Data: tobj{
			"Bad-Class": tarr(),
			"notalist":  tobj{},
			"backstory": tarr(
				"scalar",
				tobj{"title": "no name"},
				tobj{"name": "Bad Name"},
				tobj{"name": 3.0},
				tobj{"name": "abs", "abstract": "yes"},
				tobj{"name": "par", "parent": 4.0},
				tobj{"name": "par2", "parent": "x.Bad"},
				tobj{"name": "par3", "parent": ".x"},
				tobj{"name": "ok"},
				tobj{"name": "ok", "title": "again"},
			),
		}},
		{Path: "defs/b.json", Data: tobj{"backstory": tarr(tobj{"name": "ok"})}},
	}, nil)
	defs, probs := CollectDefs(p)
	if got := trefs(defs); !reflect.DeepEqual(got, []string{"backstory/m.ok"}) {
		t.Fatalf("kept %v, want only the first m.ok", got)
	}
	if defs[0].Body["title"] != nil {
		t.Fatal("the later duplicate must be dropped, not the first")
	}
	check := func(sub, file, def, field string) {
		t.Helper()
		pr := tprob(t, probs, sub)
		if pr.Plugin != "m" || pr.File != file || pr.Def != def || pr.Field != field || pr.Warn {
			t.Errorf("%q: got %+v, want file=%s def=%s field=%s", sub, pr, file, def, field)
		}
	}
	check("defs file must be an object", "defs/list.json", "", "")
	check(`class "Bad-Class"`, "defs/a.json", "", "Bad-Class")
	check("must hold a list", "defs/a.json", "notalist", "")
	check("definition must be an object", "defs/a.json", "backstory", "[0]")
	check("name is required", "defs/a.json", "backstory", "[1]")
	check(`name "Bad Name"`, "defs/a.json", "backstory", "[2]")
	check("name 3 must", "defs/a.json", "backstory", "[3]")
	check("abstract must be", "defs/a.json", "backstory/m.abs", "abstract")
	check("parent 4 must", "defs/a.json", "backstory/m.par", "parent")
	check(`parent "x.Bad"`, "defs/a.json", "backstory/m.par2", "parent")
	check(`parent ".x"`, "defs/a.json", "backstory/m.par3", "parent")
	check("first in defs/a.json", "defs/a.json", "backstory/m.ok", "name")
	if n := len(probs); n != 13 {
		t.Fatalf("%d problems, want 13: %v", n, probs)
	}
	dups := 0
	for _, pr := range probs {
		if strings.Contains(pr.Msg, "defined twice") {
			dups++
		}
	}
	if dups != 2 {
		t.Fatalf("want a duplicate error in a.json and in b.json, got %d", dups)
	}
}

func TestCollectDefsDeterministicOverManyClasses(t *testing.T) {
	classes := tobj{}
	for _, c := range strings.Split("q w e r t y u i o p a s d f g h j k l z x c v b n m qq ww ee rr tt", " ") {
		classes[c] = tarr(tobj{"name": "x"}, tobj{}, tobj{"name": "x"})
		classes[strings.ToUpper(c)] = tarr()
	}
	p := tplug("m", []File{{Path: "defs/a.json", Data: classes}}, nil)
	defs0, probs0 := CollectDefs(p)
	for i := 0; i < 50; i++ {
		defs, probs := CollectDefs(p)
		if !reflect.DeepEqual(defs, defs0) || !reflect.DeepEqual(probs, probs0) {
			t.Fatalf("run %d differs", i)
		}
	}
	got := trefs(defs0)
	for i := 1; i < len(got); i++ {
		if got[i-1] >= got[i] {
			t.Fatalf("classes not in sorted order: %v", got)
		}
	}
}
