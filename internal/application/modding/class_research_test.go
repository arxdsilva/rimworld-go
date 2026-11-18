// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package modding

import (
	"fmt"
	"reflect"
	"slices"
	"strings"
	"testing"

	"rimworld-go/internal/domain/economy/recipe"
	"rimworld-go/internal/domain/research"
)

func rsSame(t *testing.T, got, want research.Book) {
	t.Helper()
	if got.Len() != want.Len() {
		t.Fatalf("%d projects, want %d", got.Len(), want.Len())
	}
	for i := range want.Len() {
		p := research.Project(i)
		if g, w := got.DefOf(p), want.DefOf(p); !reflect.DeepEqual(g, w) {
			t.Fatalf("project %d: %+v\nwant %+v", p, g, w)
		}
		if g, w := got.Key(p), want.Key(p); g != w {
			t.Fatalf("project %d: key %q, want %q", p, g, w)
		}
	}
}

// TestResearchCodecRoundTripsCore: decoding core's encoded defs (straight,
// and through JSON) gives core back, def for def and key for key.
func TestResearchCodecRoundTripsCore(t *testing.T) {
	t.Parallel()
	core := research.Core()
	defs := EncodeResearch(core)
	if len(defs) != core.Len() {
		t.Fatalf("%d defs for %d projects", len(defs), core.Len())
	}
	for i, d := range defs {
		if d.Class != ResearchClass || d.Plugin != Core || d.Key != core.Key(research.Project(i)) {
			t.Fatalf("def %d: %s by %q", i, d.Ref(), d.Plugin)
		}
	}
	for name, in := range map[string][]Def{"trees": defs, "json": bsJSON(t, defs)} {
		got, probs := DecodeResearch(in, recipe.Core())
		if len(probs) > 0 {
			t.Fatalf("%s: %v", name, probs)
		}
		rsSame(t, got, core)
	}
}

// rsDef is a plugin project def keyed "frontier.<name>".
func rsDef(name string, body map[string]any) Def {
	return Def{Class: ResearchClass, Key: "frontier." + name, Plugin: "frontier", File: "defs/research.json", Body: body}
}

func rsBody(extra ...any) map[string]any {
	b := map[string]any{"label": "Weaving", "cost": 400.0, "tech": "medieval"}
	for i := 0; i+1 < len(extra); i += 2 {
		b[extra[i].(string)] = extra[i+1]
	}
	return b
}

// rsRecipes is core's recipes plus the plugin recipe "frontier.mat".
func rsRecipes() recipe.Book {
	c := recipe.Core()
	return recipe.NewBook(append(slices.Clone(c.All()), recipe.Def{Name: "frontier.mat", Label: "Mat"}),
		append(c.Keys(), "frontier.mat"))
}

// TestResearchPluginDefFollowsCore: plugin projects follow core's under
// their keys, their prerequisites resolve to core and plugin projects
// (one defined later in the list too), and their recipe unlocks become
// unlock keys.
func TestResearchPluginDefFollowsCore(t *testing.T) {
	t.Parallel()
	defs := append(EncodeResearch(research.Core()),
		rsDef("weave", rsBody("desc", "Mats.", "bench", "hi_tech", "requires", []any{"core.traps", "frontier.loom"},
			"hidden", []any{"core.herbalism"}, "unlocks", []any{"frontier.mat", "recipe:make_straw_carpet"})),
		rsDef("loom", rsBody()))
	got, probs := DecodeResearch(bsJSON(t, defs), rsRecipes())
	if len(probs) > 0 {
		t.Fatal(probs)
	}
	n := research.NumProjects
	weave, loom := got.DefOf(n), got.DefOf(n+1)
	want := research.Def{Project: n, Name: "frontier.weave", Label: "Weaving", Desc: "Mats.", Cost: 400,
		TechLevel: research.Medieval, Bench: research.HiTechBench, Requires: []research.Project{research.Traps, n + 1},
		Hidden: []research.Project{research.Herbalism}, Unlocks: []string{"recipe:frontier.mat", "recipe:make_straw_carpet"}}
	if got.Len() != int(n)+2 || !reflect.DeepEqual(weave, want) || got.Key(n) != "frontier.weave" || loom.Name != "frontier.loom" {
		t.Fatalf("plugin projects %+v / %+v", weave, loom)
	}
	rsSame(t, research.NewBook(got.All()[:n], got.Keys()[:n]), research.Core())
	back, probs := DecodeResearch(EncodeResearch(got), rsRecipes())
	if len(probs) > 0 {
		t.Fatal(probs)
	}
	rsSame(t, back, got)
}

// TestResearchDecodeProblems: every strictness rule names its field, and a
// def with any problem is left out while the rest load.
func TestResearchDecodeProblems(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, field, msg string
		body             map[string]any
	}{
		{"unknown", "colour", "unknown field", rsBody("colour", "red")},
		{"nolabel", "label", "required", map[string]any{"cost": 1.0, "tech": "ultra"}},
		{"nocost", "cost", "required", map[string]any{"label": "x", "tech": "ultra"}},
		{"notech", "tech", "required", map[string]any{"label": "x", "cost": 1.0}},
		{"tech", "tech", "unknown tech level", rsBody("tech", "Medieval")},
		{"bench", "bench", "unknown bench tier", rsBody("bench", "lab")},
		{"zero", "cost", "out of range", rsBody("cost", 0.0)},
		{"frac", "cost", "not a whole number", rsBody("cost", 1.5)},
		{"long", "label", "characters", rsBody("label", strings.Repeat("x", rsMaxLabel+1))},
		{"reqtype", "requires", "want a list", rsBody("requires", "core.traps")},
		{"reqitem", "requires.0", "want a string", rsBody("requires", []any{1.0})},
		{"reqkey", "requires.0", "unknown research project", rsBody("requires", []any{"core.nothing"})},
		{"hidkey", "hidden.0", "unknown research project", rsBody("hidden", []any{"traps"})},
		{"recipe", "unlocks.0", "unknown recipe", rsBody("unlocks", []any{"frontier.nothing"})},
		{"build", "unlocks.0", "unlocks recipes", rsBody("unlocks", []any{"build:wall"})},
	}
	defs := append(EncodeResearch(research.Core()), rsDef("ok", rsBody()))
	for _, c := range cases {
		defs = append(defs, rsDef(c.name, c.body))
	}
	got, probs := DecodeResearch(defs, rsRecipes())
	if got.Len() != int(research.NumProjects)+1 || got.Key(research.NumProjects) != "frontier.ok" {
		t.Fatalf("only the good def loads: %v", got.Keys()[research.NumProjects:])
	}
	for _, c := range cases {
		ref := ResearchClass + "/frontier." + c.name
		if !slices.ContainsFunc(probs, func(p Problem) bool {
			return p.Def == ref && p.Field == c.field && strings.Contains(p.Msg, c.msg) && p.Plugin == "frontier"
		}) {
			t.Errorf("%s: no %s problem %q in %v", c.name, c.field, c.msg, probs)
		}
	}
}

// TestResearchPrerequisitesResolve: a project needing a left-out one is
// left out too, and projects whose prerequisites loop (and those needing
// them) are left out, each with a problem; the rest keep their order.
func TestResearchPrerequisitesResolve(t *testing.T) {
	t.Parallel()
	defs := append(EncodeResearch(research.Core()),
		rsDef("bad", rsBody("tech", "nope")),
		rsDef("needsbad", rsBody("requires", []any{"frontier.bad"})),
		rsDef("needsneeds", rsBody("hidden", []any{"frontier.needsbad"})),
		rsDef("a", rsBody("requires", []any{"frontier.b"})),
		rsDef("b", rsBody("requires", []any{"frontier.a"})),
		rsDef("self", rsBody("requires", []any{"frontier.self"})),
		rsDef("afterloop", rsBody("requires", []any{"core.traps", "frontier.a"})),
		rsDef("fine", rsBody("requires", []any{"frontier.last"})),
		rsDef("last", rsBody("requires", []any{"core.traps"})))
	got, probs := DecodeResearch(defs, recipe.Core())
	if keys := got.Keys()[research.NumProjects:]; !slices.Equal(keys, []string{"frontier.fine", "frontier.last"}) {
		t.Fatalf("kept %v; problems %v", keys, probs)
	}
	if fine := got.DefOf(research.NumProjects); !slices.Equal(fine.Requires, []research.Project{research.NumProjects + 1}) {
		t.Fatalf("fine requires %v", fine.Requires)
	}
	for name, msg := range map[string]string{"needsbad": "which was left out", "needsneeds": "which was left out",
		"a": "loop", "b": "loop", "self": "loop", "afterloop": "loop"} {
		if !slices.ContainsFunc(probs, func(p Problem) bool {
			return p.Def == ResearchClass+"/frontier."+name && strings.Contains(p.Msg, msg)
		}) {
			t.Errorf("%s: no %q problem in %v", name, msg, probs)
		}
	}
}

// TestResearchCorePatch: a patch may change a core project's numbers and
// words, never its prerequisites or unlocks.
func TestResearchCorePatch(t *testing.T) {
	t.Parallel()
	defs := EncodeResearch(research.Core())
	patch := func(p research.Project, edit func(map[string]any)) []Problem {
		d := defs[p]
		d.Body = treeCopyMap(d.Body)
		edit(d.Body)
		_, probs := DecodeResearch([]Def{d}, recipe.Core())
		return probs
	}
	if p := patch(research.Gunsmithing, func(b map[string]any) {
		b["cost"], b["label"], b["tech"], b["bench"], b["desc"] = 50.0, "Guns", "spacer", "hi_tech", "Bang."
	}); len(p) > 0 {
		t.Fatalf("plain patch refused: %v", p)
	}
	for name, p := range map[string][]Problem{
		"requires": patch(research.Gunsmithing, func(b map[string]any) { delete(b, "requires") }),
		"hidden":   patch(research.Gunsmithing, func(b map[string]any) { b["hidden"] = []any{"core.traps"} }),
		"unlocks":  patch(research.Traps, func(b map[string]any) { b["unlocks"] = []any{"recipe:make_straw_carpet"} }),
	} {
		if len(p) != 1 || p[0].Field != name || !strings.Contains(p[0].Msg, "a patch cannot change") {
			t.Errorf("%s: %v", name, p)
		}
	}
}

// TestResearchBookCap: past research.MaxBook a project is refused, not
// wrapped onto a byte Project another project holds.
func TestResearchBookCap(t *testing.T) {
	t.Parallel()
	defs := EncodeResearch(research.Core())
	for i := research.Core().Len(); i <= research.MaxBook; i++ {
		defs = append(defs, rsDef(fmt.Sprintf("p%d", i), rsBody()))
	}
	b, probs := DecodeResearch(defs, recipe.Core())
	if b.Len() != research.MaxBook || len(probs) != 1 || !strings.Contains(probs[0].Msg, "too many research projects") {
		t.Fatalf("Len %d, problems %v", b.Len(), probs)
	}
}

// TestBuildResearchPlugin: through the whole pipeline, a plugin's project
// unlocking its own recipe follows core's, and its patch changes a core
// project's cost; a patch removing a core project is refused.
func TestBuildResearchPlugin(t *testing.T) {
	t.Parallel()
	defs := map[string]any{
		"recipe": []any{map[string]any{"name": "mat", "label": "Mat", "bench": "crafting spot", "work": 100.0, "skill": "craft",
			"learn": "crafting", "input": map[string]any{"kind": "wood", "qty": 2.0}, "output": "wood carpet", "out_qty": 1.0}},
		"research": []any{map[string]any{"name": "weave", "label": "Weaving", "cost": 300.0, "tech": "neolithic",
			"requires": []any{"core.traps"}, "unlocks": []any{"frontier.mat"}}},
	}
	patches := []any{
		map[string]any{"op": "set", "target": "research/core.traps", "path": "cost", "value": 650.0},
		map[string]any{"op": "remove", "target": "research/core.herbalism"},
	}
	cat, probs := Build([]Plugin{plugin("frontier", defs, patches)}, []string{"frontier"}, "0.1.0")
	if len(probs) != 1 || !strings.Contains(probs[0].Msg, "cannot be removed") {
		t.Fatalf("problems %v", probs)
	}
	r := cat.Research
	p, ok := r.IDOf("frontier.weave")
	if !ok || p != research.NumProjects || r.DefOf(p).Unlocks[0] != "recipe:frontier.mat" ||
		!slices.Equal(r.DefOf(p).Requires, []research.Project{research.Traps}) {
		t.Fatalf("plugin project %v: %+v", ok, r.DefOf(p))
	}
	if r.DefOf(research.Traps).Cost != 650 || research.DefOf(research.Traps).Cost == 650 || r.DefOf(research.Herbalism).Name != "herbalism" {
		t.Fatal("the patch changes the run's book only; core's projects stay")
	}
}
