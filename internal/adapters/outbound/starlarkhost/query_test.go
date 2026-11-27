// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package starlarkhost

import (
	"fmt"
	"strings"
	"testing"

	"rimworld-go/internal/application/pluginapi"
	"rimworld-go/internal/application/scripting"
)

type qAnimals struct {
	n    *int
	rows []scripting.Animal
}

func (a qAnimals) Animals() []scripting.Animal { *a.n++; return a.rows }

type qBuildings struct {
	n    *int
	rows []scripting.BuildingCount
}

func (b qBuildings) Buildings() []scripting.BuildingCount { *b.n++; return b.rows }

type qWorld struct {
	n  *int
	fs []scripting.Faction
	ss []scripting.Site
}

func (w qWorld) Factions() []scripting.Faction { *w.n++; return w.fs }
func (w qWorld) Sites() []scripting.Site       { *w.n++; return w.ss }

type qMap struct{}

func (qMap) Size() (int, int) { return 4, 3 }
func (qMap) Terrain(x, y int) (string, bool) {
	if x < 0 || y < 0 || x >= 4 || y >= 3 {
		return "", false
	}
	return "core.soil", true
}
func (qMap) Temperature(x, y int) (int, bool) {
	if x < 0 || y < 0 || x >= 4 || y >= 3 {
		return 0, false
	}
	return 21 - x, true
}
func (qMap) Weather() string { return "rain" }

// queryOut runs src as on_hour (granted g, or legacy when zero and
// !declared) and returns the message texts.
func queryOut(t *testing.T, q scripting.Queries, g pluginapi.Set, declared bool, src string) ([]string, scripting.Result) {
	t.Helper()
	p := plugin("q", "scripts/q.star", src)
	p.Caps, p.Declared = g, declared
	h := New([]Plugin{p})
	h.UseQueries(q)
	r := fire(h, scripting.OnHour)
	var out []string
	for _, c := range r.Commands {
		out = append(out, c.Name)
	}
	return out, r
}

var worldSet = pluginapi.Set(1<<pluginapi.Commands | 1<<pluginapi.Events | 1<<pluginapi.World)

func TestQueryValues(t *testing.T) {
	t.Parallel()
	var n int
	q := scripting.Queries{
		Animals: qAnimals{&n, []scripting.Animal{{Species: "core.deer", Label: "Deer", Count: 3}, {Species: "core.wolf", Label: "Wolf", Count: 1, Predator: true}}},
		Map:     qMap{},
		World: qWorld{&n, []scripting.Faction{{Name: "Pirates", Goodwill: -80, Hostile: true}},
			[]scripting.Site{{Name: "Camp", Faction: "Pirates"}}},
		Buildings: qBuildings{&n, []scripting.BuildingCount{{Kind: "core.wall", Count: 12}}},
	}
	src := `
def on_hour(ctx):
    a = ctx.animals()
    ctx.message("%d %s %s %d %s %s" % (len(a), a[0].species, a[0].label, a[0].count, a[0].predator, a[1].predator))
    b = ctx.buildings()
    ctx.message("%s %d" % (b[0].kind, b[0].count))
    m = ctx.map
    ctx.message("%d %d %s %s %d" % (m.width, m.height, m.weather, m.terrain(1, 2), m.temperature(2, 0)))
    ctx.message("%s %s" % (m.terrain(9, 9), m.temperature(-1, 0)))
    f = ctx.world.factions()
    s = ctx.world.sites()
    ctx.message("%s %d %s %s %s" % (f[0].name, f[0].goodwill, f[0].hostile, s[0].name, s[0].faction))
`
	out, r := queryOut(t, q, worldSet, true, src)
	want := []string{"2 core.deer Deer 3 False True", "core.wall 12", "4 3 rain core.soil 19", "None None", "Pirates -80 True Camp Pirates"}
	if len(r.Disabled) != 0 || strings.Join(out, "|") != strings.Join(want, "|") {
		t.Fatalf("out %q disabled %+v, want %q", out, r.Disabled, want)
	}
}

func TestQueryNilReaders(t *testing.T) {
	t.Parallel()
	src := `
def on_hour(ctx):
    m = ctx.map
    ctx.message("%d %d %d %d %d %s %s %s" % (len(ctx.animals()), len(ctx.buildings()), len(ctx.world.factions()), len(ctx.world.sites()), m.width, m.weather, m.terrain(0, 0), m.temperature(0, 0)))
`
	out, r := queryOut(t, scripting.Queries{}, worldSet, true, src)
	if len(r.Disabled) != 0 || len(out) != 1 || out[0] != "0 0 0 0 0  None None" {
		t.Fatalf("out %q disabled %+v", out, r.Disabled)
	}
}

func TestQueryBadArgs(t *testing.T) {
	t.Parallel()
	for _, call := range []string{"ctx.animals(1)", "ctx.buildings(1)", "ctx.world.factions(1)", "ctx.world.sites(1)", "ctx.map.terrain(1)", "ctx.map.temperature('a', 1)"} {
		_, r := queryOut(t, scripting.Queries{Map: qMap{}}, worldSet, true, "def on_hour(ctx):\n    "+call+"\n")
		if len(r.Disabled) != 1 {
			t.Errorf("%s: want the script disabled, got %+v", call, r.Disabled)
		}
	}
}

func TestQueryBatched(t *testing.T) {
	t.Parallel()
	var n int
	q := scripting.Queries{
		Animals:   qAnimals{&n, []scripting.Animal{{Species: "a"}}},
		Buildings: qBuildings{&n, []scripting.BuildingCount{{Kind: "k", Count: 1}}},
		World:     qWorld{&n, []scripting.Faction{{Name: "f"}}, []scripting.Site{{Name: "s"}}},
	}
	src := `
def on_hour(ctx):
    for i in range(5):
        ctx.animals()
        ctx.buildings()
        ctx.world.factions()
        ctx.world.sites()
`
	_, r := queryOut(t, q, worldSet, true, src)
	if len(r.Disabled) != 0 {
		t.Fatalf("disabled %+v", r.Disabled)
	}
	if n != 4 {
		t.Fatalf("readers were called %d times in one call, want 4 (once each)", n)
	}
}

func TestQueryCap(t *testing.T) {
	t.Parallel()
	var n int
	as := make([]scripting.Animal, scripting.MaxQueryRows+50)
	for i := range as {
		as[i] = scripting.Animal{Species: fmt.Sprintf("s%03d", i)}
	}
	q := scripting.Queries{Animals: qAnimals{&n, as}}
	out, r := queryOut(t, q, worldSet, true, "def on_hour(ctx):\n    a = ctx.animals()\n    ctx.message(\"%d %s\" % (len(a), a[-1].species))\n")
	want := fmt.Sprintf("%d s%03d", scripting.MaxQueryRows, scripting.MaxQueryRows-1)
	if len(r.Disabled) != 0 || len(out) != 1 || out[0] != want {
		t.Fatalf("out %q disabled %+v, want %q", out, r.Disabled, want)
	}
}

func TestQueryFrozenRows(t *testing.T) {
	t.Parallel()
	var n int
	q := scripting.Queries{Animals: qAnimals{&n, []scripting.Animal{{Species: "a"}}}}
	_, r := queryOut(t, q, worldSet, true, "def on_hour(ctx):\n    ctx.animals()[0].species = 'x'\n")
	if len(r.Disabled) != 1 {
		t.Fatalf("a row must be immutable: %+v", r.Disabled)
	}
}

func TestQueryNeedsWorld(t *testing.T) {
	t.Parallel()
	q := scripting.Queries{Map: qMap{}}
	noWorld := pluginapi.Set(1<<pluginapi.Commands | 1<<pluginapi.Events | 1<<pluginapi.Query)
	for _, attr := range []string{"ctx.animals", "ctx.buildings", "ctx.map", "ctx.world"} {
		_, r := queryOut(t, q, noWorld, true, "def on_hour(ctx):\n    "+attr+"\n")
		if len(r.Disabled) != 1 {
			t.Errorf("%s without world: want the call to error, got %+v", attr, r.Disabled)
		}
	}
	out, r := queryOut(t, q, worldSet, true, "def on_hour(ctx):\n    ctx.message(str(ctx.map.width))\n")
	if len(r.Disabled) != 0 || len(out) != 1 {
		t.Fatalf("with world: out %q disabled %+v", out, r.Disabled)
	}
}
