package modding

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

// tdef is a def of class backstory with key plugin.name.
func tdef(plugin, name, parent string, abstract bool, body tobj) Def {
	if parent != "" && !strings.Contains(parent, ".") {
		parent = plugin + "." + parent
	}
	return Def{Class: "backstory", Key: plugin + "." + name, Plugin: plugin,
		File: "defs/" + plugin + ".json", Abstract: abstract, Parent: parent, Body: body}
}

// tcopyDefs deep-copies defs, to compare an input before and after.
func tcopyDefs(defs []Def) []Def {
	out := make([]Def, len(defs))
	for i, d := range defs {
		d.Body = treeCopyMap(d.Body)
		d.Touched = append([]Touch(nil), d.Touched...)
		out[i] = d
	}
	return out
}

func TestInheritMergesParentChains(t *testing.T) {
	in := []Def{
		tdef("core", "base", "", true, tobj{
			"skills": tobj{"shooting": 2.0, "melee": 1.0},
			"tags":   tarr("a", "b"),
			"title":  "Base",
			"traits": tobj{"brave": true},
			"desc":   "inherited",
		}),
		tdef("core", "soldier", "base", false, tobj{
			"skills": tobj{"melee": 4.0, "medicine": nil},
			"tags":   tarr("c"),
			"desc":   nil,
		}),
		tdef("core", "sniper", "soldier", false, tobj{
			"skills": tobj{"shooting": 8.0},
			"traits": nil,
		}),
	}
	before := tcopyDefs(in)
	out, probs := Inherit(in)
	if len(probs) != 0 {
		t.Fatalf("problems: %v", probs)
	}
	if got := trefs(out); !reflect.DeepEqual(got, []string{"backstory/core.soldier", "backstory/core.sniper"}) {
		t.Fatalf("abstract defs must be dropped, order kept; got %v", got)
	}
	soldier := tobj{"skills": tobj{"shooting": 2.0, "melee": 4.0}, "tags": tarr("c"),
		"title": "Base", "traits": tobj{"brave": true}}
	if !treeEqual(out[0].Body, soldier) {
		t.Fatalf("soldier = %v, want %v", out[0].Body, soldier)
	}
	sniper := tobj{"skills": tobj{"shooting": 8.0, "melee": 4.0}, "tags": tarr("c"), "title": "Base"}
	if !treeEqual(out[1].Body, sniper) {
		t.Fatalf("sniper = %v, want %v", out[1].Body, sniper)
	}
	if out[1].Parent != "core.soldier" {
		t.Fatal("Parent must be kept for provenance")
	}
	if !reflect.DeepEqual(in, before) {
		t.Fatal("Inherit changed its input")
	}
	// Siblings and parents share nothing.
	out[0].Body["skills"].(tobj)["shooting"] = 99.0
	out[0].Body["tags"].([]any)[0] = "zz"
	if out[1].Body["skills"].(tobj)["melee"] != 4.0 || !reflect.DeepEqual(in, before) {
		t.Fatal("an output body aliases another def or the input")
	}
}

func TestInheritParentMayBeConcreteAndAnotherPlugin(t *testing.T) {
	in := []Def{
		tdef("core", "doctor", "", false, tobj{"skills": tobj{"medicine": 6.0}}),
		tdef("mod", "medic", "core.doctor", false, tobj{"title": "Medic"}),
	}
	out, probs := Inherit(in)
	if len(probs) != 0 || len(out) != 2 {
		t.Fatalf("out=%v probs=%v", trefs(out), probs)
	}
	if !treeEqual(out[1].Body, tobj{"skills": tobj{"medicine": 6.0}, "title": "Medic"}) {
		t.Fatalf("medic = %v", out[1].Body)
	}
}

func TestInheritSameKeyTwiceIsAnError(t *testing.T) {
	in := []Def{
		tdef("core", "cook", "", false, tobj{"v": 1.0}),
		tdef("a", "x", "", false, nil),
		{Class: "backstory", Key: "core.cook", Plugin: "b", File: "defs/b.json", Body: tobj{"v": 2.0}},
		{Class: "recipe", Key: "core.cook", Plugin: "b", File: "defs/b.json", Body: tobj{}},
	}
	out, probs := Inherit(in)
	if got := trefs(out); !reflect.DeepEqual(got, []string{"backstory/core.cook", "backstory/a.x", "recipe/core.cook"}) {
		t.Fatalf("out = %v", got)
	}
	if out[0].Body["v"] != 1.0 {
		t.Fatal("the first definition must win")
	}
	if out[1].Body == nil {
		t.Fatal("a nil body must come out as an empty object")
	}
	if len(probs) != 1 {
		t.Fatalf("problems = %v", probs)
	}
	p := probs[0]
	if p.Plugin != "b" || p.File != "defs/b.json" || p.Def != "backstory/core.cook" || p.Warn ||
		p.Msg != "already defined by core; use a replace patch to change it" {
		t.Fatalf("problem = %+v", p)
	}
}

func TestInheritMissingParent(t *testing.T) {
	in := []Def{
		tdef("m", "orphan", "nobody", false, nil),
		tdef("m", "heir", "orphan", false, nil),
		{Class: "recipe", Key: "m.base", Plugin: "m", Abstract: true, Body: tobj{}},
		tdef("m", "wrongclass", "base", false, nil), // m.base is a recipe, not a backstory
		tdef("m", "fine", "", false, nil),
	}
	out, probs := Inherit(in)
	if got := trefs(out); !reflect.DeepEqual(got, []string{"backstory/m.fine"}) {
		t.Fatalf("out = %v", got)
	}
	want := []string{
		"m: defs/m.json: backstory/m.orphan: parent: parent m.nobody is not a backstory definition",
		"m: defs/m.json: backstory/m.heir: parent: parent m.orphan could not be built",
		"m: defs/m.json: backstory/m.wrongclass: parent: parent m.base is not a backstory definition",
	}
	if got := tstrings(probs); !reflect.DeepEqual(got, want) {
		t.Fatalf("problems =\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func TestInheritCycles(t *testing.T) {
	in := []Def{
		tdef("m", "d", "a", false, nil), // inherits into the cycle
		tdef("m", "a", "b", false, nil),
		tdef("m", "b", "a", true, nil),
		tdef("m", "self", "self", false, nil),
		tdef("m", "e", "d", false, nil), // two steps from the cycle
		tdef("m", "ok", "", false, nil),
	}
	out, probs := Inherit(in)
	if got := trefs(out); !reflect.DeepEqual(got, []string{"backstory/m.ok"}) {
		t.Fatalf("out = %v", got)
	}
	want := []string{
		"m: defs/m.json: backstory/m.d: parent: parent m.a could not be built",
		"m: defs/m.json: backstory/m.a: parent: parent cycle: m.a -> m.b -> m.a",
		"m: defs/m.json: backstory/m.b: parent: parent cycle: m.a -> m.b -> m.a",
		"m: defs/m.json: backstory/m.self: parent: parent cycle: m.self -> m.self",
		"m: defs/m.json: backstory/m.e: parent: parent m.d could not be built",
	}
	if got := tstrings(probs); !reflect.DeepEqual(got, want) {
		t.Fatalf("problems =\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func TestInheritCopiesTouched(t *testing.T) {
	touched := make([]Touch, 1, 4)
	touched[0] = Touch{Plugin: "x", Op: "set"}
	in := []Def{tdef("m", "a", "", false, nil)}
	in[0].Touched = touched
	out, _ := Inherit(in)
	out[0].Touched = append(out[0].Touched, Touch{Plugin: "y"})
	if touched[:2][1].Plugin != "" {
		t.Fatal("Inherit shares the Touched backing array with its input")
	}
}

func TestInheritDeterministic(t *testing.T) {
	var in []Def
	for i := 0; i < 40; i++ {
		body := tobj{}
		for j := 0; j < 30; j++ {
			body[fmt.Sprintf("f%02d", j)] = tobj{"v": float64(i * j), "gone": nil}
		}
		parent := ""
		switch {
		case i%7 == 3:
			parent = "missing"
		case i%5 == 4:
			parent = fmt.Sprintf("d%02d", i+1) // a forward chain, some into cycles
		case i > 0:
			parent = fmt.Sprintf("d%02d", i-1)
		}
		in = append(in, tdef("m", fmt.Sprintf("d%02d", i), parent, i%3 == 0, body))
	}
	in = append(in, in[5]) // a duplicate
	out0, probs0 := Inherit(in)
	for i := 0; i < 50; i++ {
		out, probs := Inherit(in)
		if !reflect.DeepEqual(out, out0) || !reflect.DeepEqual(probs, probs0) {
			t.Fatalf("run %d differs", i)
		}
	}
	if len(probs0) == 0 || len(out0) == 0 {
		t.Fatalf("the fixture should both keep and fail defs: %d out, %d problems", len(out0), len(probs0))
	}
}

// tstrings renders problems for comparison.
func tstrings(probs []Problem) []string {
	out := make([]string, len(probs))
	for i, p := range probs {
		out[i] = p.String()
	}
	return out
}
