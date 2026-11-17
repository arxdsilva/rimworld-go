package modding

import (
	"reflect"
	"strings"
	"testing"
)

// tobj and arr build trees by hand, as encoding/json would decode them.
type tobj = map[string]any

func tarr(v ...any) []any { return v }

func TestTreeCopyIsDeep(t *testing.T) {
	in := tobj{"a": tobj{"b": tarr(1.0, tobj{"c": "x"})}, "n": nil, "f": true}
	out := treeCopy(in).(tobj)
	if !treeEqual(in, out) {
		t.Fatalf("copy differs: %v vs %v", out, in)
	}
	out["a"].(tobj)["b"].([]any)[1].(tobj)["c"] = "changed"
	out["a"].(tobj)["new"] = 1.0
	if in["a"].(tobj)["b"].([]any)[1].(tobj)["c"] != "x" || in["a"].(tobj)["new"] != nil {
		t.Fatalf("copy shares memory with its source: %v", in)
	}
	if treeCopyMap(nil) != nil {
		t.Fatal("nil map must copy to nil")
	}
}

func TestTreeEqual(t *testing.T) {
	cases := []struct {
		a, b any
		eq   bool
	}{
		{1.0, 1.0, true},
		{1.0, 2.0, false},
		{"a", "a", true},
		{nil, nil, true},
		{nil, false, false},
		{false, nil, false},
		{"1", 1.0, false},
		{tarr(1.0, "x"), tarr(1.0, "x"), true},
		{tarr(1.0, "x"), tarr("x", 1.0), false},
		{tarr(1.0), tarr(1.0, 1.0), false},
		{tobj{"a": 1.0}, tobj{"a": 1.0}, true},
		{tobj{"a": 1.0}, tobj{"b": 1.0}, false},
		{tobj{"a": nil}, tobj{}, false},
		{tobj{"a": tarr(tobj{"b": true})}, tobj{"a": tarr(tobj{"b": true})}, true},
		{tobj{}, tarr(), false},
		{1.0, tobj{}, false},
	}
	for i, c := range cases {
		if got := treeEqual(c.a, c.b); got != c.eq {
			t.Errorf("case %d: treeEqual(%v, %v) = %v, want %v", i, c.a, c.b, got, c.eq)
		}
	}
}

func TestTreeMerge(t *testing.T) {
	dst := tobj{
		"keep":  "x",
		"gone":  1.0,
		"list":  tarr(1.0, 2.0),
		"inner": tobj{"a": 1.0, "b": tobj{"c": 1.0, "d": 2.0}},
		"flip":  tobj{"a": 1.0},
	}
	src := tobj{
		"gone":   nil,
		"list":   tarr(3.0),
		"inner":  tobj{"b": tobj{"d": nil, "e": 3.0}, "z": "new"},
		"flip":   "scalar",
		"fresh":  tobj{"x": nil, "y": tobj{"q": nil, "r": 1.0}},
		"absent": nil,
	}
	srcCopy := treeCopy(src)
	treeMerge(dst, src)
	want := tobj{
		"keep":  "x",
		"list":  tarr(3.0),
		"inner": tobj{"a": 1.0, "b": tobj{"c": 1.0, "e": 3.0}, "z": "new"},
		"flip":  "scalar",
		"fresh": tobj{"y": tobj{"r": 1.0}},
	}
	if !treeEqual(dst, want) {
		t.Fatalf("merge = %v, want %v", dst, want)
	}
	if !treeEqual(src, srcCopy) {
		t.Fatal("merge changed its source")
	}
	dst["list"].([]any)[0] = "changed"
	dst["fresh"].(tobj)["y"].(tobj)["r"] = "changed"
	if !treeEqual(src, srcCopy) {
		t.Fatal("merge aliased its source")
	}
}

func TestTreePath(t *testing.T) {
	if segs, err := treePath(""); err != nil || segs != nil {
		t.Fatalf(`treePath("") = %v, %v`, segs, err)
	}
	segs, err := treePath("skills.0.Level_2")
	if err != nil || !reflect.DeepEqual(segs, []string{"skills", "0", "Level_2"}) {
		t.Fatalf("treePath = %v, %v", segs, err)
	}
	for _, bad := range []string{".a", "a.", "a..b", "a-b", "a b", "ä"} {
		if _, err := treePath(bad); err == nil {
			t.Errorf("treePath(%q) accepted a bad path", bad)
		}
	}
}

func TestTreeOverlap(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{"", "", true},
		{"", "a.b", true},
		{"a.b", "", true},
		{"a", "a", true},
		{"a", "a.b", true},
		{"a.b.c", "a.b", true},
		{"stats.hp", "stats.hpmax", false},
		{"a.b", "a.c", false},
		{"a", "b", false},
	}
	for _, c := range cases {
		if got := treeOverlap(c.a, c.b); got != c.want {
			t.Errorf("treeOverlap(%q, %q) = %v, want %v", c.a, c.b, got, c.want)
		}
	}
}

func TestTreeUpdate(t *testing.T) {
	set := func(v any) treeEdit {
		return func(any, bool) (any, bool, *treeError) { return v, false, nil }
	}
	del := func(any, bool) (any, bool, *treeError) { return nil, true, nil }
	body := tobj{"a": tobj{"list": tarr(tobj{"x": 1.0}, "y", "z")}, "s": "scalar"}

	if _, err := treeUpdate(body, []string{"a", "list", "0", "x"}, nil, set(2.0)); err != nil {
		t.Fatal(err)
	}
	if _, err := treeUpdate(body, []string{"a", "list", "1"}, nil, del); err != nil {
		t.Fatal(err)
	}
	if _, err := treeUpdate(body, []string{"a", "new"}, nil, set("n")); err != nil {
		t.Fatal(err)
	}
	want := tobj{"a": tobj{"list": tarr(tobj{"x": 2.0}, "z"), "new": "n"}, "s": "scalar"}
	if !treeEqual(body, want) {
		t.Fatalf("body = %v, want %v", body, want)
	}

	before := treeCopy(body)
	for _, c := range []struct {
		segs    []string
		missing bool
		msg     string
	}{
		{[]string{"nope", "x"}, true, "nope is missing"},
		{[]string{"a", "list", "5"}, true, "out of range"},
		{[]string{"a", "list", "k"}, false, "numeric index"},
		{[]string{"s", "x"}, false, "s is not an object or a list"},
	} {
		_, err := treeUpdate(body, c.segs, nil, set(1.0))
		if err == nil || err.missing != c.missing || !strings.Contains(err.Error(), c.msg) {
			t.Errorf("treeUpdate(%v) err = %v, want missing=%v %q", c.segs, err, c.missing, c.msg)
		}
	}
	if !treeEqual(body, before) {
		t.Fatalf("a failed update changed the body: %v", body)
	}
}

func TestTreeKeysSorted(t *testing.T) {
	m := tobj{}
	for _, k := range strings.Split("q w e r t y u i o p a s d f g h j k l z x c v b n m", " ") {
		m[k] = 1.0
	}
	keys := treeKeys(m)
	for i := 1; i < len(keys); i++ {
		if keys[i-1] >= keys[i] {
			t.Fatalf("keys not sorted: %v", keys)
		}
	}
}
