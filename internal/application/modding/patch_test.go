package modding

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

// tops parses one patches file of plugin id and fails on any problem.
func tops(t *testing.T, id string, ops ...any) []Op {
	t.Helper()
	parsed, probs := ParseOps(tplug(id, nil, []File{{Path: "patches/p.json", Data: tarr(ops...)}}))
	if len(probs) != 0 {
		t.Fatalf("ParseOps problems: %v", probs)
	}
	return parsed
}

// tnone is a loads func under which no other plugin loads.
func tnone(string) bool { return false }

// tbody is the body of the def with ref in defs, or nil.
func tbody(defs []Def, ref string) tobj {
	for _, d := range defs {
		if d.Ref() == ref {
			return d.Body
		}
	}
	return nil
}

func TestParseOpsFields(t *testing.T) {
	data := tarr(
		tobj{"op": "set", "target": "backstory/x", "path": "skills.shooting", "value": 3.0},
		tobj{"op": "add", "target": "backstory/core.cook", "path": "tags", "value": "x",
			"if": "other", "missing": "third", "optional": true},
		tobj{"op": "remove", "target": "recipe/m.stew"},
		tobj{"op": "remove", "target": "recipe/m.stew", "path": "tags", "value": nil},
		tobj{"op": "merge", "target": "recipe/m.stew", "value": tobj{"a": 1.0}},
		tobj{"op": "replace", "target": "recipe/m.stew", "value": tobj{"a": 1.0}},
	)
	ops, probs := ParseOps(tplug("m", nil, []File{{Path: "patches/p.json", Data: data}}))
	if len(probs) != 0 {
		t.Fatalf("problems: %v", probs)
	}
	want := []Op{
		{Plugin: "m", File: "patches/p.json", Index: 0, Kind: "set", Target: "backstory/m.x", Path: "skills.shooting", Value: 3.0},
		{Plugin: "m", File: "patches/p.json", Index: 1, Kind: "add", Target: "backstory/core.cook", Path: "tags", Value: "x",
			If: "other", Missing: "third", Optional: true},
		{Plugin: "m", File: "patches/p.json", Index: 2, Kind: "remove", Target: "recipe/m.stew"},
		{Plugin: "m", File: "patches/p.json", Index: 3, Kind: "remove", Target: "recipe/m.stew", Path: "tags"},
		{Plugin: "m", File: "patches/p.json", Index: 4, Kind: "merge", Target: "recipe/m.stew", Value: tobj{"a": 1.0}},
		{Plugin: "m", File: "patches/p.json", Index: 5, Kind: "replace", Target: "recipe/m.stew", Value: tobj{"a": 1.0}},
	}
	if !reflect.DeepEqual(ops, want) {
		t.Fatalf("ops =\n%+v\nwant\n%+v", ops, want)
	}
	ops[4].Value.(tobj)["a"] = 2.0
	if data[4].(tobj)["value"].(tobj)["a"] != 1.0 {
		t.Fatal("ParseOps aliased the value tree")
	}
}

func TestParseOpsProblems(t *testing.T) {
	cases := []struct {
		op    tobj
		field string
		msg   string
	}{
		{tobj{"op": "set", "target": "backstory/x", "path": "a", "value": 1.0, "zeta": 1.0, "alpha": 1.0}, "alpha", `unknown field "alpha"`},
		{tobj{"target": "backstory/x"}, "op", "op <nil> must be"},
		{tobj{"op": "move", "target": "backstory/x"}, "op", `op "move" must be`},
		{tobj{"op": "remove"}, "target", "target <nil> must be"},
		{tobj{"op": "remove", "target": "backstory"}, "target", "target"},
		{tobj{"op": "remove", "target": "Back/x"}, "target", "target"},
		{tobj{"op": "remove", "target": "backstory/X"}, "target", "target"},
		{tobj{"op": "remove", "target": "backstory/x", "path": 3.0}, "path", "path must be a string"},
		{tobj{"op": "remove", "target": "backstory/x", "path": "a..b"}, "path", "segment"},
		{tobj{"op": "remove", "target": "backstory/x", "if": 1.0}, "if", "if must be a plugin id"},
		{tobj{"op": "remove", "target": "backstory/x", "missing": ""}, "missing", "missing must be a plugin id"},
		{tobj{"op": "remove", "target": "backstory/x", "optional": "yes"}, "optional", "optional must be"},
		{tobj{"op": "set", "target": "backstory/x", "path": "a"}, "value", "value is required"},
		{tobj{"op": "add", "target": "backstory/x", "path": "a"}, "value", "value is required"},
		{tobj{"op": "merge", "target": "backstory/x"}, "value", "value is required"},
		{tobj{"op": "replace", "target": "backstory/x"}, "value", "value is required"},
		{tobj{"op": "set", "target": "backstory/x", "value": 1.0}, "path", "path is required"},
		{tobj{"op": "add", "target": "backstory/x", "value": 1.0}, "path", "path is required"},
		{tobj{"op": "remove", "target": "backstory/x", "value": 1.0}, "path", "path is required"},
		{tobj{"op": "merge", "target": "backstory/x", "value": tarr()}, "value", "must be an object"},
		{tobj{"op": "replace", "target": "backstory/x", "path": "a", "value": tobj{}}, "path", "path must be empty"},
		{tobj{"op": "replace", "target": "backstory/x", "value": "s"}, "value", "must be an object"},
		{tobj{"op": "replace", "target": "backstory/x", "value": tobj{"name": "y"}}, "value", "must not set name"},
		{tobj{"op": "replace", "target": "backstory/x", "value": tobj{"abstract": true}}, "value", "must not set abstract"},
		{tobj{"op": "replace", "target": "backstory/x", "value": tobj{"parent": "z"}}, "value", "must not set parent"},
	}
	for i, c := range cases {
		data := tarr(tobj{"op": "remove", "target": "backstory/ok"}, c.op)
		ops, probs := ParseOps(tplug("m", nil, []File{{Path: "patches/p.json", Data: data}}))
		if len(ops) != 1 || len(probs) != 1 {
			t.Errorf("case %d: %d ops, problems %v; want the bad op dropped with one problem", i, len(ops), probs)
			continue
		}
		p := probs[0]
		if p.Plugin != "m" || p.File != "patches/p.json" || p.Field != c.field || p.Warn ||
			!strings.HasPrefix(p.Msg, "patch 1 (") || !strings.Contains(p.Msg, c.msg) {
			t.Errorf("case %d: problem %+v, want field %q and %q", i, p, c.field, c.msg)
		}
	}
	_, probs := ParseOps(tplug("m", nil, []File{
		{Path: "patches/obj.json", Data: tobj{}},
		{Path: "patches/scalar.json", Data: tarr("x")},
	}))
	if len(probs) != 2 || probs[0].File != "patches/obj.json" || !strings.Contains(probs[0].Msg, "must be a list") ||
		probs[1].File != "patches/scalar.json" || !strings.Contains(probs[1].Msg, "must be an object") {
		t.Fatalf("file problems = %v", probs)
	}
}

func TestApplySet(t *testing.T) {
	defs := []Def{tdef("core", "a", "", false, tobj{
		"skills": tobj{"shooting": 1.0},
		"list":   tarr(tobj{"lvl": 1.0}, "b"),
		"s":      "scalar",
	})}
	ops := tops(t, "m",
		tobj{"op": "set", "target": "backstory/core.a", "path": "skills.shooting", "value": 5.0},
		tobj{"op": "set", "target": "backstory/core.a", "path": "skills.new", "value": tobj{"x": 1.0}},
		tobj{"op": "set", "target": "backstory/core.a", "path": "list.0.lvl", "value": 7.0},
		tobj{"op": "set", "target": "backstory/core.a", "path": "list.1", "value": "c"},
		tobj{"op": "set", "target": "backstory/core.a", "path": "title", "value": "T"},
		// errors: missing intermediate, scalar intermediate, out-of-range index
		tobj{"op": "set", "target": "backstory/core.a", "path": "nope.x", "value": 1.0},
		tobj{"op": "set", "target": "backstory/core.a", "path": "s.x", "value": 1.0},
		tobj{"op": "set", "target": "backstory/core.a", "path": "list.5", "value": 1.0},
	)
	out, probs := Apply(defs, ops, tnone)
	want := tobj{
		"skills": tobj{"shooting": 5.0, "new": tobj{"x": 1.0}},
		"list":   tarr(tobj{"lvl": 7.0}, "c"),
		"s":      "scalar",
		"title":  "T",
	}
	if !treeEqual(out[0].Body, want) {
		t.Fatalf("body = %v, want %v", out[0].Body, want)
	}
	wantProbs := []string{
		"m: patches/p.json: backstory/core.a: nope.x: patch 5 (set): nope is missing",
		"m: patches/p.json: backstory/core.a: s.x: patch 6 (set): s.x: s is not an object or a list",
		"m: patches/p.json: backstory/core.a: list.5: patch 7 (set): list.5: index 5 is out of range (the list has 2 items)",
	}
	if got := tstrings(probs); !reflect.DeepEqual(got, wantProbs) {
		t.Fatalf("problems =\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(wantProbs, "\n"))
	}
	if len(out[0].Touched) != 5 {
		t.Fatalf("touches = %v, want one per applied op", out[0].Touched)
	}
	if out[0].Touched[0] != (Touch{Plugin: "m", File: "patches/p.json", Op: "set", Field: "skills.shooting"}) {
		t.Fatalf("touch = %+v", out[0].Touched[0])
	}
	ops[1].Value.(tobj)["x"] = 99.0
	if out[0].Body["skills"].(tobj)["new"].(tobj)["x"] != 1.0 {
		t.Fatal("set aliased the op's value")
	}
}

func TestApplyAdd(t *testing.T) {
	defs := []Def{tdef("core", "a", "", false, tobj{
		"tags":   tarr("x"),
		"skills": tobj{"shooting": 1.0},
		"s":      "scalar",
		"n":      nil,
	})}
	ops := tops(t, "m",
		tobj{"op": "add", "target": "backstory/core.a", "path": "tags", "value": "y"},
		tobj{"op": "add", "target": "backstory/core.a", "path": "tags", "value": tobj{"o": 1.0}},
		tobj{"op": "add", "target": "backstory/core.a", "path": "skills", "value": tobj{"melee": 2.0, "art": 3.0}},
		// errors
		tobj{"op": "add", "target": "backstory/core.a", "path": "skills", "value": tobj{"cook": 1.0, "shooting": 9.0}},
		tobj{"op": "add", "target": "backstory/core.a", "path": "skills", "value": "x"},
		tobj{"op": "add", "target": "backstory/core.a", "path": "absent", "value": "x"},
		tobj{"op": "add", "target": "backstory/core.a", "path": "s", "value": "x"},
		tobj{"op": "add", "target": "backstory/core.a", "path": "n", "value": "x"},
	)
	out, probs := Apply(defs, ops, tnone)
	want := tobj{
		"tags":   tarr("x", "y", tobj{"o": 1.0}),
		"skills": tobj{"shooting": 1.0, "melee": 2.0, "art": 3.0},
		"s":      "scalar",
		"n":      nil,
	}
	if !treeEqual(out[0].Body, want) {
		t.Fatalf("body = %v, want %v (a failed add must change nothing)", out[0].Body, want)
	}
	wantMsgs := []string{
		"patch 3 (add): skills.shooting already set; use set",
		"patch 4 (add): skills is an object: add needs an object value",
		"patch 5 (add): absent: field missing; use set",
		"patch 6 (add): s is not a list or an object; use set",
		"patch 7 (add): n is not a list or an object; use set",
	}
	if len(probs) != len(wantMsgs) {
		t.Fatalf("problems = %v", probs)
	}
	for i, p := range probs {
		if p.Msg != wantMsgs[i] {
			t.Errorf("problem %d = %q, want %q", i, p.Msg, wantMsgs[i])
		}
	}
}

func TestApplyRemove(t *testing.T) {
	defs := []Def{
		tdef("core", "a", "", false, tobj{
			"tags":   tarr("x", tobj{"k": 1.0}, "x", "y"),
			"skills": tobj{"shooting": 1.0, "melee": 2.0},
			"list":   tarr("p", "q", "r"),
			"s":      "scalar",
		}),
		tdef("core", "gone", "", false, tobj{}),
		tdef("core", "c", "", false, tobj{}),
	}
	ops := tops(t, "m",
		tobj{"op": "remove", "target": "backstory/core.a", "path": "tags", "value": "x"},
		tobj{"op": "remove", "target": "backstory/core.a", "path": "tags", "value": tobj{"k": 1.0}},
		tobj{"op": "remove", "target": "backstory/core.a", "path": "skills.melee"},
		tobj{"op": "remove", "target": "backstory/core.a", "path": "list.1"},
		tobj{"op": "remove", "target": "backstory/core.gone"},
		// optional misses are silent
		tobj{"op": "remove", "target": "backstory/core.a", "path": "nope", "optional": true},
		tobj{"op": "remove", "target": "backstory/core.a", "path": "tags", "value": "zz", "optional": true},
		// errors
		tobj{"op": "remove", "target": "backstory/core.a", "path": "nope"},
		tobj{"op": "remove", "target": "backstory/core.a", "path": "tags", "value": "zz"},
		tobj{"op": "remove", "target": "backstory/core.a", "path": "s", "value": "scalar", "optional": true},
		tobj{"op": "set", "target": "backstory/core.gone", "path": "x", "value": 1.0},
		tobj{"op": "remove", "target": "backstory/core.gone"},
	)
	out, probs := Apply(defs, ops, tnone)
	if got := trefs(out); !reflect.DeepEqual(got, []string{"backstory/core.a", "backstory/core.c"}) {
		t.Fatalf("out = %v", got)
	}
	want := tobj{"tags": tarr("y"), "skills": tobj{"shooting": 1.0}, "list": tarr("p", "r"), "s": "scalar"}
	if !treeEqual(out[0].Body, want) {
		t.Fatalf("body = %v, want %v", out[0].Body, want)
	}
	wantMsgs := []string{
		"patch 7 (remove): nope is missing",
		"patch 8 (remove): tags holds no item equal to the value",
		"patch 9 (remove): s is not a list: remove with a value removes list items",
		"patch 10 (set): target backstory/core.gone does not exist",
		"patch 11 (remove): target backstory/core.gone does not exist",
	}
	if len(probs) != len(wantMsgs) {
		t.Fatalf("problems = %v", probs)
	}
	for i, p := range probs {
		if p.Msg != wantMsgs[i] || p.Warn {
			t.Errorf("problem %d = %+v, want %q", i, p, wantMsgs[i])
		}
	}
	if len(out[0].Touched) != 4 {
		t.Fatalf("touches = %v: silent optional misses must not touch", out[0].Touched)
	}
}

func TestApplyMergeAndReplace(t *testing.T) {
	defs := []Def{
		tdef("core", "a", "", false, tobj{
			"skills": tobj{"shooting": 1.0, "melee": 2.0, "sub": tobj{"x": 1.0}},
			"tags":   tarr("x"),
			"title":  "A",
			"s":      "scalar",
		}),
		tdef("core", "b", "", false, tobj{"old": 1.0}),
	}
	ops := tops(t, "m",
		tobj{"op": "merge", "target": "backstory/core.a", "path": "skills",
			"value": tobj{"melee": nil, "sub": tobj{"y": 2.0}, "art": 3.0}},
		tobj{"op": "merge", "target": "backstory/core.a",
			"value": tobj{"tags": tarr("z"), "title": nil, "extra": tobj{"e": nil, "f": 1.0}}},
		tobj{"op": "replace", "target": "backstory/core.b", "value": tobj{"new": tarr(1.0)}},
		// errors
		tobj{"op": "merge", "target": "backstory/core.a", "path": "s", "value": tobj{}},
		tobj{"op": "merge", "target": "backstory/core.a", "path": "nope", "value": tobj{}},
	)
	out, probs := Apply(defs, ops, tnone)
	wantA := tobj{
		"skills": tobj{"shooting": 1.0, "sub": tobj{"x": 1.0, "y": 2.0}, "art": 3.0},
		"tags":   tarr("z"),
		"s":      "scalar",
		"extra":  tobj{"f": 1.0},
	}
	if !treeEqual(out[0].Body, wantA) {
		t.Fatalf("a = %v, want %v", out[0].Body, wantA)
	}
	if !treeEqual(out[1].Body, tobj{"new": tarr(1.0)}) {
		t.Fatalf("b = %v", out[1].Body)
	}
	if len(probs) != 2 || probs[0].Msg != "patch 3 (merge): s is not an object" ||
		probs[1].Msg != "patch 4 (merge): nope is missing; use set" {
		t.Fatalf("problems = %v", probs)
	}
	if out[1].Touched[0] != (Touch{Plugin: "m", File: "patches/p.json", Op: "replace"}) {
		t.Fatalf("replace touch = %+v", out[1].Touched)
	}
	ops[2].Value.(tobj)["new"].([]any)[0] = 5.0
	if out[1].Body["new"].([]any)[0] != 1.0 {
		t.Fatal("replace aliased the op's value")
	}
}

func TestApplyConditionsAndOptionalTargets(t *testing.T) {
	defs := []Def{tdef("core", "a", "", false, tobj{"v": 0.0})}
	loads := func(id string) bool { return id == "present" }
	ops := tops(t, "m",
		tobj{"op": "set", "target": "backstory/core.a", "path": "if_yes", "value": 1.0, "if": "present"},
		tobj{"op": "set", "target": "backstory/core.a", "path": "if_no", "value": 1.0, "if": "absent"},
		tobj{"op": "set", "target": "backstory/core.a", "path": "missing_yes", "value": 1.0, "missing": "absent"},
		tobj{"op": "set", "target": "backstory/core.a", "path": "missing_no", "value": 1.0, "missing": "present"},
		tobj{"op": "set", "target": "backstory/core.a", "path": "both", "value": 1.0, "if": "present", "missing": "absent"},
		// a skipped op raises nothing, even with a missing target
		tobj{"op": "set", "target": "backstory/core.nothere", "path": "x", "value": 1.0, "if": "absent"},
		tobj{"op": "set", "target": "backstory/core.nothere", "path": "x", "value": 1.0, "optional": true},
		tobj{"op": "set", "target": "backstory/nothere", "path": "x", "value": 1.0},
	)
	out, probs := Apply(defs, ops, loads)
	want := tobj{"v": 0.0, "if_yes": 1.0, "missing_yes": 1.0, "both": 1.0}
	if !treeEqual(out[0].Body, want) {
		t.Fatalf("body = %v, want %v", out[0].Body, want)
	}
	if len(probs) != 1 || probs[0].Def != "backstory/m.nothere" || probs[0].Field != "" ||
		probs[0].Msg != "patch 7 (set): target backstory/m.nothere does not exist" {
		t.Fatalf("problems = %v", probs)
	}
	if len(out[0].Touched) != 3 {
		t.Fatalf("touches = %v", out[0].Touched)
	}
}

func TestApplyConflictWarnings(t *testing.T) {
	defs := []Def{
		tdef("core", "a", "", false, tobj{"skills": tobj{"shooting": 1.0, "hp": 1.0, "hpmax": 1.0}, "tags": tarr()}),
		tdef("core", "b", "", false, tobj{"x": 1.0}),
	}
	var ops []Op
	ops = append(ops, tops(t, "one",
		tobj{"op": "set", "target": "backstory/core.a", "path": "skills.shooting", "value": 2.0},
		tobj{"op": "set", "target": "backstory/core.a", "path": "skills.shooting", "value": 3.0}, // same plugin: no warning
		tobj{"op": "set", "target": "backstory/core.a", "path": "skills.hp", "value": 3.0},
		tobj{"op": "add", "target": "backstory/core.a", "path": "tags", "value": "one"},
		tobj{"op": "replace", "target": "backstory/core.b", "value": tobj{"y": 1.0}},
	)...)
	ops = append(ops, tops(t, "two",
		tobj{"op": "set", "target": "backstory/core.a", "path": "skills.shooting", "value": 4.0},       // same path
		tobj{"op": "merge", "target": "backstory/core.a", "path": "skills", "value": tobj{"art": 1.0}}, // a prefix
		tobj{"op": "set", "target": "backstory/core.a", "path": "skills.hpmax", "value": 1.0},
		tobj{"op": "add", "target": "backstory/core.a", "path": "tags", "value": "two"}, // add never conflicts
		tobj{"op": "set", "target": "backstory/core.b", "path": "z", "value": 1.0},      // under a replace
	)...)
	ops = append(ops, tops(t, "three",
		tobj{"op": "set", "target": "backstory/core.a", "path": "skills.hpmax", "value": 2.0},
	)...)
	out, probs := Apply(defs, ops, tnone)
	want := []string{
		"two: patches/p.json: backstory/core.a: skills.shooting: warning: patch 0 (set): skills.shooting was changed by one; two overrides it",
		"two: patches/p.json: backstory/core.a: skills: warning: patch 1 (merge): skills.hp was changed by one; two overrides it",
		"two: patches/p.json: backstory/core.b: z: warning: patch 4 (set): z was changed by one; two overrides it",
		"three: patches/p.json: backstory/core.a: skills.hpmax: warning: patch 0 (set): skills.hpmax was changed by two; three overrides it",
	}
	if got := tstrings(probs); !reflect.DeepEqual(got, want) {
		t.Fatalf("problems =\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	for _, p := range probs {
		if !p.Warn {
			t.Fatalf("a conflict must be a warning: %+v", p)
		}
	}
	if got := out[0].Body["skills"].(tobj)["shooting"]; got != 4.0 {
		t.Fatalf("the later plugin must win, got %v", got)
	}
	if !treeEqual(out[0].Body["tags"], tarr("one", "two")) {
		t.Fatalf("tags = %v", out[0].Body["tags"])
	}
}

func TestApplyWholeBodyConflict(t *testing.T) {
	defs := []Def{tdef("core", "a", "", false, tobj{"x": 1.0})}
	var ops []Op
	ops = append(ops, tops(t, "one", tobj{"op": "set", "target": "backstory/core.a", "path": "x", "value": 2.0})...)
	ops = append(ops, tops(t, "two", tobj{"op": "replace", "target": "backstory/core.a", "value": tobj{"y": 1.0}})...)
	ops = append(ops, tops(t, "three", tobj{"op": "merge", "target": "backstory/core.a", "value": tobj{"z": 1.0}})...)
	_, probs := Apply(defs, ops, tnone)
	want := []string{
		"two: patches/p.json: backstory/core.a: warning: patch 0 (replace): x was changed by one; two overrides it",
		"three: patches/p.json: backstory/core.a: warning: patch 0 (merge): the whole def was changed by two; three overrides it",
	}
	if got := tstrings(probs); !reflect.DeepEqual(got, want) {
		t.Fatalf("problems =\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func TestApplyDoesNotMutateInput(t *testing.T) {
	touched := make([]Touch, 1, 8)
	touched[0] = Touch{Plugin: "early", Op: "set", Field: "x"}
	defs := []Def{
		tdef("core", "a", "", false, tobj{"skills": tobj{"s": 1.0}, "tags": tarr("x", "y"), "l": tarr(tobj{"k": 1.0})}),
		tdef("core", "b", "", false, tobj{"v": 1.0}),
	}
	defs[0].Touched = touched
	ops := tops(t, "m",
		tobj{"op": "set", "target": "backstory/core.a", "path": "skills.s", "value": 2.0},
		tobj{"op": "set", "target": "backstory/core.a", "path": "l.0.k", "value": 2.0},
		tobj{"op": "add", "target": "backstory/core.a", "path": "tags", "value": "z"},
		tobj{"op": "remove", "target": "backstory/core.a", "path": "tags", "value": "x"},
		tobj{"op": "merge", "target": "backstory/core.a", "value": tobj{"skills": tobj{"t": 1.0}}},
		tobj{"op": "replace", "target": "backstory/core.b", "value": tobj{"w": 1.0}},
		tobj{"op": "remove", "target": "backstory/core.b"},
	)
	defsBefore := tcopyDefs(defs)
	opsBefore := make([]Op, len(ops))
	for i, o := range ops {
		o.Value = treeCopy(o.Value)
		opsBefore[i] = o
	}
	out, probs := Apply(defs, ops, tnone)
	if len(probs) != 0 || len(out) != 1 {
		t.Fatalf("out=%v probs=%v", trefs(out), probs)
	}
	if !reflect.DeepEqual(defs, defsBefore) {
		t.Fatalf("Apply changed its input defs:\n%+v\nwas\n%+v", defs, defsBefore)
	}
	if !reflect.DeepEqual(ops, opsBefore) {
		t.Fatal("Apply changed its ops")
	}
	if touched[:2][1] != (Touch{}) {
		t.Fatal("Apply wrote into the input's Touched backing array")
	}
	if len(out[0].Touched) != 6 || out[0].Touched[0].Plugin != "early" {
		t.Fatalf("touches = %+v: earlier touches must be kept", out[0].Touched)
	}
}

func TestApplyDeterministic(t *testing.T) {
	var defs []Def
	var raw []any
	for i := 0; i < 30; i++ {
		body := tobj{"list": tarr()}
		for j := 0; j < 40; j++ {
			body[fmt.Sprintf("f%02d", j)] = tobj{"v": float64(j)}
		}
		addMany := tobj{}
		for j := 0; j < 40; j++ {
			k := fmt.Sprintf("k%02d", j)
			addMany[k] = 1.0
			body["f00"].(tobj)[k] = 0.0 // every key already set
			if j >= 20 {
				body["f01"].(tobj)[k] = 0.0 // the upper half already set
			}
		}
		defs = append(defs, tdef("core", fmt.Sprintf("d%02d", i), "", false, body))
		raw = append(raw,
			tobj{"op": "add", "target": fmt.Sprintf("backstory/core.d%02d", i), "value": addMany, "path": "f00"},
			tobj{"op": "add", "target": fmt.Sprintf("backstory/core.d%02d", i), "value": addMany, "path": "f01"},
			tobj{"op": "merge", "target": fmt.Sprintf("backstory/core.d%02d", i), "value": tobj{"f05": nil, "f06": tobj{"w": 1.0}}},
			tobj{"op": "set", "target": fmt.Sprintf("backstory/core.d%02d", (i*7)%30), "path": "f02.v", "value": float64(i)},
			tobj{"op": "remove", "target": fmt.Sprintf("backstory/core.d%02d", (i*11)%31)},
		)
	}
	var ops []Op
	ops = append(ops, tops(t, "aa", raw[:len(raw)/2]...)...)
	ops = append(ops, tops(t, "bb", raw[len(raw)/2:]...)...)
	out0, probs0 := Apply(defs, ops, tnone)
	for i := 0; i < 50; i++ {
		out, probs := Apply(defs, ops, tnone)
		if !reflect.DeepEqual(out, out0) || !reflect.DeepEqual(probs, probs0) {
			t.Fatalf("run %d differs", i)
		}
	}
	if len(probs0) < 40 {
		t.Fatalf("the fixture should raise many problems, got %d", len(probs0))
	}
	if !strings.Contains(probs0[0].Msg, "f00.k00 already set") || !strings.Contains(probs0[1].Msg, "f01.k20 already set") {
		t.Fatalf("add must report the first existing key in sorted order: %v", probs0[:2])
	}
}
