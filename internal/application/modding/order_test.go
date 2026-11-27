// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package modding

import (
	"math/rand/v2"
	"reflect"
	"strings"
	"testing"

	"rimworld-go/internal/application/pluginapi"
)

func mf(id string, edit ...func(*Manifest)) Manifest {
	m := Manifest{ID: id, Name: "Name " + id, Version: "1.0", Game: []string{"0.1"}, API: 1}
	for _, e := range edit {
		e(&m)
	}
	return m
}

func req(ids ...string) func(*Manifest)  { return func(m *Manifest) { m.Requires = ids } }
func aft(ids ...string) func(*Manifest)  { return func(m *Manifest) { m.After = ids } }
func bef(ids ...string) func(*Manifest)  { return func(m *Manifest) { m.Before = ids } }
func conf(ids ...string) func(*Manifest) { return func(m *Manifest) { m.Conflicts = ids } }

func ids(o Order) []string {
	var out []string
	for _, p := range o.Loaded {
		out = append(out, p.ID)
	}
	return out
}

func reasonOf(o Order, id string) string {
	for _, p := range o.Loaded {
		if p.ID == id {
			return p.Reason
		}
	}
	return "<missing>"
}

func msgFor(probs []Problem, plugin string) []string {
	var out []string
	for _, p := range probs {
		if p.Plugin == plugin {
			out = append(out, p.Msg)
		}
	}
	return out
}

func anyContains(msgs []string, sub string) bool {
	for _, m := range msgs {
		if strings.Contains(m, sub) {
			return true
		}
	}
	return false
}

func TestCheckManifestOK(t *testing.T) {
	m := mf("good_mod", func(m *Manifest) {
		m.URL = "https://example.com"
		m.Version = "1.2.3"
		m.Requires = []string{"core", "other"}
		m.After = []string{"core"}
	})
	if got := CheckManifest(m); len(got) != 0 {
		t.Fatalf("unexpected problems: %v", got)
	}
	if got := CheckManifest(mf("ok", func(m *Manifest) { m.Game = nil })); len(got) != 0 {
		t.Fatalf("empty game is not a manifest error: %v", got)
	}
}

func TestCheckManifestProblems(t *testing.T) {
	cases := []struct {
		name   string
		m      Manifest
		plugin string
		field  string
	}{
		{"id uppercase", mf("Bad"), "", "id"},
		{"id short", mf("a"), "", "id"},
		{"id long", mf(strings.Repeat("a", 49)), "", "id"},
		{"id empty", mf(""), "", "id"},
		{"id leading underscore", mf("_ab"), "", "id"},
		{"id core", mf("core"), "core", "id"},
		{"name empty", mf("abc", func(m *Manifest) { m.Name = "" }), "abc", "name"},
		{"name long", mf("abc", func(m *Manifest) { m.Name = strings.Repeat("é", 81) }), "abc", "name"},
		{"version empty", mf("abc", func(m *Manifest) { m.Version = "" }), "abc", "version"},
		{"version text", mf("abc", func(m *Manifest) { m.Version = "1.x" }), "abc", "version"},
		{"version 4 parts", mf("abc", func(m *Manifest) { m.Version = "1.2.3.4" }), "abc", "version"},
		{"game one part", mf("abc", func(m *Manifest) { m.Game = []string{"0.1", "1"} }), "abc", "game[1]"},
		{"game patch", mf("abc", func(m *Manifest) { m.Game = []string{"1.2.3"} }), "abc", "game[0]"},
		{"requires bad id", mf("abc", req("ok", "BAD")), "abc", "requires[1]"},
		{"requires self", mf("abc", req("abc")), "abc", "requires[0]"},
		{"requires dup", mf("abc", req("xyz", "xyz")), "abc", "requires[1]"},
		{"after self", mf("abc", aft("abc")), "abc", "after[0]"},
		{"before core", mf("abc", bef("core")), "abc", "before[0]"},
		{"conflicts core", mf("abc", conf("core")), "abc", "conflicts[0]"},
		{"conflicts dup", mf("abc", conf("xyz", "xyz")), "abc", "conflicts[1]"},
		{"url scheme", mf("abc", func(m *Manifest) { m.URL = "ftp://x" }), "abc", "url"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := CheckManifest(c.m)
			if len(got) != 1 {
				t.Fatalf("want 1 problem, got %v", got)
			}
			p := got[0]
			if p.File != "plugin.json" || p.Plugin != c.plugin || p.Field != c.field || p.Warn || p.Msg == "" {
				t.Fatalf("got %+v, want plugin %q field %q", p, c.plugin, c.field)
			}
		})
	}
}

func TestCheckManifestAPIAndCaps(t *testing.T) {
	cases := []struct {
		name  string
		edit  func(*Manifest)
		field string // "" for no problem
		warn  bool
		msg   string
	}{
		{"api 1", func(m *Manifest) {}, "", false, ""},
		{"api absent", func(m *Manifest) { m.API = 0 }, "api", true, "declares no plugin API version; treated as 1"},
		{"api 2", func(m *Manifest) { m.API = 2 }, "api", false, "needs plugin API 2; this game has 1"},
		{"api -1", func(m *Manifest) { m.API = -1 }, "api", false, "not supported"},
		{"caps nil", func(m *Manifest) { m.Capabilities = nil }, "", false, ""},
		{"caps empty", func(m *Manifest) { m.Capabilities = []string{} }, "", false, ""},
		{"caps valid", func(m *Manifest) { m.Capabilities = []string{"query", "ui"} }, "", false, ""},
		{"caps unknown", func(m *Manifest) { m.Capabilities = []string{"query", "root"} }, "capabilities", false, `"root"`},
		{"caps dup", func(m *Manifest) { m.Capabilities = []string{"query", "query"} }, "capabilities", true, "listed twice"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := CheckManifest(mf("abc", c.edit))
			if c.field == "" {
				if len(got) != 0 {
					t.Fatalf("want none, got %v", got)
				}
				return
			}
			if len(got) != 1 {
				t.Fatalf("want 1 problem, got %v", got)
			}
			p := got[0]
			if p.Field != c.field || p.Warn != c.warn || p.File != "plugin.json" || p.Plugin != "abc" || !strings.Contains(p.Msg, c.msg) {
				t.Fatalf("got %+v, want field %q warn %v msg ~%q", p, c.field, c.warn, c.msg)
			}
		})
	}
}

func TestResolveRefusesNewerAPI(t *testing.T) {
	found := []Manifest{
		mf("newer", func(m *Manifest) { m.API = 2 }),
		mf("badcap", func(m *Manifest) { m.Capabilities = []string{"nope"} }),
		mf("legacy", func(m *Manifest) { m.API = 0 }),
		mf("dependant", req("newer")),
	}
	o, probs := Resolve(found, []string{"newer", "badcap", "legacy", "dependant"}, "0.1")
	if got := ids(o); !reflect.DeepEqual(got, []string{"core", "legacy"}) {
		t.Fatalf("loaded %v, want core and legacy", got)
	}
	if !anyContains(msgFor(probs, "newer"), "needs plugin API 2; this game has 1") {
		t.Errorf("no api refusal: %v", probs)
	}
	if !anyContains(msgFor(probs, "badcap"), `unknown capability "nope"`) {
		t.Errorf("no capability refusal: %v", probs)
	}
	var warns []Problem
	for _, p := range probs {
		if p.Plugin == "legacy" {
			warns = append(warns, p)
		}
	}
	if len(warns) != 1 || !warns[0].Warn || warns[0].Msg != "declares no plugin API version; treated as 1" {
		t.Errorf("legacy wants one warning, got %v", warns)
	}
}

func TestCapsOf(t *testing.T) {
	if s, legacy := CapsOf(Manifest{}); !legacy || s != pluginapi.Legacy() {
		t.Errorf("nil caps = %v %v, want legacy", s, legacy)
	}
	if s, legacy := CapsOf(Manifest{Capabilities: []string{}}); legacy || s != 0 {
		t.Errorf("empty caps = %v %v, want none", s, legacy)
	}
	s, legacy := CapsOf(Manifest{Capabilities: []string{"ui", "query", "bogus"}})
	if legacy || !s.Has(pluginapi.UI) || !s.Has(pluginapi.Query) || s.Has(pluginapi.Events) {
		t.Errorf("declared caps = %v %v", s.Names(), legacy)
	}
}

func TestResolveCoreAlone(t *testing.T) {
	o, probs := Resolve(nil, []string{"core"}, "0.1.0")
	if len(probs) != 0 || len(o.Failed) != 0 || len(o.Loaded) != 1 {
		t.Fatalf("got %+v %v", o, probs)
	}
	if o.Loaded[0] != (Placed{ID: Core, Reason: "the game itself"}) {
		t.Fatalf("core placement %+v", o.Loaded[0])
	}
	if !o.Has("core") || o.Has("x") {
		t.Fatal("Has")
	}
}

func TestResolveBasicOrderAndReasons(t *testing.T) {
	found := []Manifest{
		mf("zeta"),
		mf("alpha", req("zeta")),
		mf("mid", aft("zeta")),
		mf("first", bef("zeta")),
		mf("solo"),
		mf("ghost", aft("absent"), bef("absent")), // ignored edges
		mf("off"),
	}
	en := []string{"alpha", "zeta", "mid", "first", "solo", "ghost", "core"}
	o, probs := Resolve(found, en, "0.1.4-dev")
	if len(probs) != 0 {
		t.Fatalf("problems: %v", probs)
	}
	want := []string{"core", "first", "ghost", "solo", "zeta", "alpha", "mid"}
	if !reflect.DeepEqual(ids(o), want) {
		t.Fatalf("order %v want %v", ids(o), want)
	}
	if o.Has("off") || !o.Has("zeta") {
		t.Fatal("Has")
	}
	wantReason := map[string]string{
		"first": "alphabetical",
		"ghost": "alphabetical",
		"solo":  "alphabetical",
		"zeta":  "after first (first asks to load before it)",
		"alpha": "after zeta (requires it)",
		"mid":   "after zeta (asks to load after it)",
	}
	for id, w := range wantReason {
		if g := reasonOf(o, id); g != w {
			t.Errorf("reason %s = %q want %q", id, g, w)
		}
	}
	if len(o.Failed) != 0 {
		t.Fatalf("failed %v", o.Failed)
	}
}

func TestResolveReasonPicksLastLoadedPredecessor(t *testing.T) {
	found := []Manifest{mf("aa"), mf("bb", req("aa")), mf("cc", req("aa", "bb"))}
	o, _ := Resolve(found, []string{"aa", "bb", "cc"}, "0.1")
	if g := reasonOf(o, "cc"); g != "after bb (requires it)" {
		t.Fatalf("got %q", g)
	}
}

func TestResolveAfterCore(t *testing.T) {
	o, _ := Resolve([]Manifest{mf("aa", aft("core"))}, []string{"aa"}, "0.1")
	if g := reasonOf(o, "aa"); g != "after core (asks to load after it)" {
		t.Fatalf("got %q", g)
	}
}

func TestResolveDuplicateInstall(t *testing.T) {
	found := []Manifest{mf("dup"), mf("dup"), mf("other", req("dup"))}
	o, probs := Resolve(found, []string{"dup", "other"}, "0.1")
	if o.Has("dup") || o.Has("other") {
		t.Fatalf("loaded %v", ids(o))
	}
	if !anyContains(msgFor(probs, "dup"), "installed twice") {
		t.Fatalf("problems %v", probs)
	}
	if !anyContains(msgFor(probs, "other"), "requires dup, which cannot load") {
		t.Fatalf("problems %v", probs)
	}
	if !reflect.DeepEqual(o.Failed, []string{"dup", "other"}) {
		t.Fatalf("failed %v", o.Failed)
	}
}

func TestResolveEnabledNotInstalled(t *testing.T) {
	o, probs := Resolve(nil, []string{"nope"}, "0.1")
	if len(probs) != 1 || probs[0].Plugin != "nope" || probs[0].Msg != "enabled but not installed" {
		t.Fatalf("%v", probs)
	}
	if !reflect.DeepEqual(o.Failed, []string{"nope"}) {
		t.Fatalf("failed %v", o.Failed)
	}
}

func TestResolveBadManifestDoesNotLoad(t *testing.T) {
	bad := mf("badone", func(m *Manifest) { m.Name = "" })
	o, probs := Resolve([]Manifest{bad, mf("core")}, []string{"badone", "core"}, "0.1")
	if o.Has("badone") || len(o.Failed) != 1 || o.Failed[0] != "badone" {
		t.Fatalf("%+v", o)
	}
	if len(probs) != 1 || probs[0].Field != "name" {
		t.Fatalf("%v", probs)
	}
}

func TestResolveBeforeCoreIsError(t *testing.T) {
	o, probs := Resolve([]Manifest{mf("aa", bef("core"))}, []string{"aa"}, "0.1")
	if o.Has("aa") || len(probs) != 1 || probs[0].Field != "before[0]" {
		t.Fatalf("%+v %v", o, probs)
	}
}

func TestResolveRequiresCascade(t *testing.T) {
	found := []Manifest{
		mf("aa", req("bb")), mf("bb", req("cc")), mf("cc", req("missing")),
		mf("dd", req("notenabled")), mf("notenabled"),
		mf("ee"),
	}
	o, probs := Resolve(found, []string{"aa", "bb", "cc", "dd", "ee"}, "0.1")
	if !reflect.DeepEqual(ids(o), []string{"core", "ee"}) {
		t.Fatalf("loaded %v", ids(o))
	}
	if !reflect.DeepEqual(o.Failed, []string{"aa", "bb", "cc", "dd"}) {
		t.Fatalf("failed %v", o.Failed)
	}
	for plugin, sub := range map[string]string{
		"cc": "requires missing, which is not installed",
		"bb": "requires cc, which cannot load",
		"aa": "requires bb, which cannot load",
		"dd": "requires notenabled, which is not enabled",
	} {
		if !anyContains(msgFor(probs, plugin), sub) {
			t.Errorf("%s: want %q in %v", plugin, sub, msgFor(probs, plugin))
		}
	}
}

func TestResolveConflict(t *testing.T) {
	found := []Manifest{mf("aa", conf("bb")), mf("bb"), mf("cc", req("bb")), mf("dd", conf("zz")), mf("zz")}
	o, probs := Resolve(found, []string{"aa", "bb", "cc", "dd"}, "0.1")
	// zz is not enabled so dd's conflict does not fire.
	if !reflect.DeepEqual(ids(o), []string{"core", "dd"}) {
		t.Fatalf("loaded %v", ids(o))
	}
	if !anyContains(msgFor(probs, "aa"), "conflicts with bb; disable one of them") ||
		!anyContains(msgFor(probs, "bb"), "conflicts with aa; disable one of them") {
		t.Fatalf("%v", probs)
	}
	if !anyContains(msgFor(probs, "cc"), "requires bb") {
		t.Fatalf("conflict must cascade: %v", probs)
	}
}

func TestResolveGameVersion(t *testing.T) {
	found := []Manifest{mf("old", func(m *Manifest) { m.Game = []string{"0.9"} }), mf("none", func(m *Manifest) { m.Game = nil }), mf("fine")}
	en := []string{"old", "none", "fine"}

	o, probs := Resolve(found, en, "0.1.7-dev")
	if len(o.Failed) != 0 || len(o.Loaded) != 4 {
		t.Fatalf("warnings must not stop loading: %+v", o)
	}
	if len(probs) != 2 {
		t.Fatalf("%v", probs)
	}
	if !probs[0].Warn || probs[0].Plugin != "none" || probs[0].Msg != "declares no supported game version" {
		t.Fatalf("%+v", probs[0])
	}
	if !probs[1].Warn || probs[1].Plugin != "old" || probs[1].Msg != "made for game 0.9; this is 0.1" {
		t.Fatalf("%+v", probs[1])
	}

	for _, v := range []string{"", "dev"} {
		_, probs := Resolve(found, []string{"old", "fine"}, v)
		if len(probs) != 0 {
			t.Fatalf("version %q should skip the check: %v", v, probs)
		}
	}
}

func TestResolveLoop(t *testing.T) {
	found := []Manifest{
		mf("cc", aft("bb")), mf("aa", aft("cc")), mf("bb", aft("aa")), // aa -> bb -> cc -> aa? see below
		mf("tail", req("aa")),
		mf("tail2", req("tail")),
		mf("fine"),
	}
	o, probs := Resolve(found, []string{"aa", "bb", "cc", "tail", "tail2", "fine"}, "0.1")
	if !reflect.DeepEqual(ids(o), []string{"core", "fine"}) {
		t.Fatalf("loaded %v", ids(o))
	}
	// bb after aa, cc after bb, aa after cc: aa -> bb -> cc -> aa.
	want := "load order loop: aa → bb → cc → aa"
	for _, id := range []string{"aa", "bb", "cc"} {
		if m := msgFor(probs, id); len(m) != 1 || m[0] != want {
			t.Errorf("%s: %v", id, m)
		}
	}
	if m := msgFor(probs, "tail"); len(m) != 1 || m[0] != "waits for aa, which is in a load order loop" {
		t.Errorf("tail: %v", m)
	}
	if m := msgFor(probs, "tail2"); len(m) != 1 || !strings.HasPrefix(m[0], "waits for tail") {
		t.Errorf("tail2: %v", m)
	}
	if !reflect.DeepEqual(o.Failed, []string{"aa", "bb", "cc", "tail", "tail2"}) {
		t.Fatalf("failed %v", o.Failed)
	}
}

func TestResolveRequiresBeforeLoopMix(t *testing.T) {
	// a requires b, b asks to load before... a's requirement cycle via before.
	found := []Manifest{mf("aa", req("bb")), mf("bb", bef("aa"), aft("aa"))}
	o, probs := Resolve(found, []string{"aa", "bb"}, "0.1")
	if len(o.Loaded) != 1 {
		t.Fatalf("loaded %v", ids(o))
	}
	if m := msgFor(probs, "aa"); len(m) != 1 || !strings.HasPrefix(m[0], "load order loop: aa → bb → aa") {
		t.Fatalf("%v", m)
	}
}

func TestResolveDeterministicUnderShuffle(t *testing.T) {
	found := []Manifest{
		mf("aa", req("bb")), mf("bb", aft("cc")), mf("cc"),
		mf("dd", conf("ee")), mf("ee"), mf("ff", req("missing")),
		mf("l1", aft("l2")), mf("l2", aft("l3")), mf("l3", aft("l1")),
		mf("tail", req("l1")), mf("dup"), mf("dup"),
		mf("warn", func(m *Manifest) { m.Game = []string{"9.9"} }),
		mf("nogame", func(m *Manifest) { m.Game = nil }),
		mf("Bad Id"), mf("noname", func(m *Manifest) { m.Name = "" }),
		mf("x1", bef("cc")), mf("x2", aft("warn")),
	}
	en := []string{"aa", "bb", "cc", "dd", "ee", "ff", "l1", "l2", "l3", "tail", "dup", "warn", "nogame", "Bad Id", "noname", "x1", "x2", "gone", "core"}
	wantO, wantP := Resolve(found, en, "0.1.0")
	if len(wantP) == 0 || len(wantO.Failed) == 0 {
		t.Fatal("fixture should produce problems")
	}
	rng := rand.New(rand.NewPCG(7, 11))
	for i := 0; i < 50; i++ {
		f := append([]Manifest(nil), found...)
		e := append([]string(nil), en...)
		rng.Shuffle(len(f), func(a, b int) { f[a], f[b] = f[b], f[a] })
		rng.Shuffle(len(e), func(a, b int) { e[a], e[b] = e[b], e[a] })
		gotO, gotP := Resolve(f, e, "0.1.0")
		if !reflect.DeepEqual(gotO, wantO) || !reflect.DeepEqual(gotP, wantP) {
			t.Fatalf("iteration %d differs:\n%+v\n%+v\n%v\n%v", i, gotO, wantO, gotP, wantP)
		}
	}
}
