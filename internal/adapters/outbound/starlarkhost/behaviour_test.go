// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package starlarkhost

import (
	"reflect"
	"slices"
	"strings"
	"testing"

	"rimworld-go/internal/application/pluginapi"
	"rimworld-go/internal/application/scripting"
)

// TestRunIncidentOwnPluginOnly: only the modules of the plugin the key
// names run, in path order, until one returns True; anything but True,
// or an error, is not handled.
func TestRunIncidentOwnPluginOnly(t *testing.T) {
	t.Parallel()
	inc := capSet(pluginapi.Incidents, pluginapi.Commands)
	h := New([]Plugin{
		declared("other", inc, "scripts/a.star", "def run_incident(ctx, i):\n    ctx.message(\"other \" + i.key)\n    return True\n"),
		declared("herd", inc,
			"scripts/1.star", "def run_incident(ctx, i):\n    ctx.message(\"1 \" + i.key + \" \" + i.category)\n    return None\n",
			"scripts/2.star", "def run_incident(ctx, i):\n    ctx.letter(\"Quiet\", i.key)\n    return True\n",
			"scripts/3.star", "def run_incident(ctx, i):\n    ctx.message(\"3\")\n    return True\n"),
		declared("truthy", inc, "scripts/a.star", "def run_incident(ctx, i):\n    ctx.message(\"t\")\n    return 1\n"),
		declared("broken", inc, "scripts/a.star", "def run_incident(ctx, i):\n    ctx.message(\"b\")\n    fail(\"no\")\n"),
	})
	if !h.RunsIncidents() {
		t.Fatal("RunsIncidents = false")
	}
	run := func(key string) (scripting.Result, bool) {
		return h.RunIncident(scripting.Event{Tick: 5, Incident: scripting.Incident{Key: key, Category: "nature"}}, &fakeWorld{}, nil)
	}
	res, ok := run("herd.quiet_herd")
	if !ok || !slices.Equal(msgs(res), []string{"1 herd.quiet_herd nature", "Quiet\nherd.quiet_herd"}) {
		t.Errorf("herd: %v %q, want handled by 2.star after 1.star, 3.star and other never called", ok, msgs(res))
	}
	if res, ok := run("truthy.x"); ok || !slices.Equal(msgs(res), []string{"t"}) {
		t.Errorf("1 is not True: %v %q", ok, msgs(res))
	}
	if res, ok := run("broken.x"); ok || len(res.Commands) != 0 || len(res.Disabled) != 1 {
		t.Errorf("an error is not handled and keeps no command: %v %+v", ok, res)
	}
	for _, key := range []string{"core.herd_crossing", "herd", "", ".x", "nobody.x"} {
		if res, ok := run(key); ok || len(res.Commands)+len(res.Disabled) != 0 {
			t.Errorf("%q: %v %+v, want nobody called", key, ok, res)
		}
	}
}

// TestValidateCommandBudget: the first non-empty reason wins with its
// plugin; None and "" pass; a script over its step budget, one returning
// another type and one with an oversize reason are turned off.
func TestValidateCommandBudget(t *testing.T) {
	t.Parallel()
	val := capSet(pluginapi.Validate, pluginapi.Query)
	h := New([]Plugin{
		declared("none", val, "scripts/a.star", "def validate_command(ctx, cmd):\n    return None\n"),
		declared("empty", val, "scripts/a.star", "def validate_command(ctx, cmd):\n    return \"\"\n"),
		declared("spin", val, "scripts/a.star", "def validate_command(ctx, cmd):\n    if cmd.kind == \"build\":\n        for i in range(10000000):\n            pass\n"),
		declared("int", val, "scripts/a.star", "def validate_command(ctx, cmd):\n    if cmd.kind == \"build\":\n        return 3\n"),
		declared("long", val, "scripts/a.star", "def validate_command(ctx, cmd):\n    if cmd.kind == \"build\":\n        return \"x\" * 501\n"),
		declared("walls", val, "scripts/a.star", `
def validate_command(ctx, cmd):
    if cmd.kind == "build" and cmd.name == "core.wall":
        return "no walls at %d,%d by %d on day %d" % (cmd.x, cmd.y, cmd.pawn, ctx.day)
`),
		declared("late", val, "scripts/a.star", "def validate_command(ctx, cmd):\n    return \"late\"\n"),
	})
	if !h.Validates() {
		t.Fatal("Validates = false")
	}
	ask := func(c scripting.CommandView) (string, string, scripting.Result) {
		return h.ValidateCommand(c, scripting.Event{Tick: 3}, &fakeWorld{}, nil)
	}
	p, why, res := ask(scripting.CommandView{Kind: "build", Pawn: 7, X: 2, Y: 9, Name: "core.wall"})
	if p != "walls" || why != "no walls at 2,9 by 7 on day 4" {
		t.Errorf("build wall: %q %q", p, why)
	}
	var off []string
	for _, d := range res.Disabled {
		off = append(off, d.Plugin+": "+d.Msg)
	}
	want := []string{
		"spin: Starlark computation cancelled: too many steps",
		"int: validate_command returned int, want a reason string or None",
		"long: validate_command: the reason is longer than 500 bytes",
	}
	if !slices.Equal(off, want) {
		t.Errorf("Disabled = %q,\nwant %q", off, want)
	}
	if p, why, res := ask(scripting.CommandView{Kind: "draft", Pawn: 7}); p != "late" || why != "late" || len(res.Disabled) != 0 {
		t.Errorf("draft: %q %q %+v, want late's reason", p, why, res)
	}
	for _, s := range []string{"spin", "int", "long"} {
		if strings.Contains(msgsOfPlugins(h), s) {
			t.Errorf("%s is still live", s)
		}
	}
}

// msgsOfPlugins lists the live modules' plugins.
func msgsOfPlugins(h *Host) string {
	var b strings.Builder
	for _, m := range h.mods {
		if !m.off {
			b.WriteString(m.plugin + " ")
		}
	}
	return b.String()
}

// TestExtensionsDeterministic: two hosts over the same plugins give the
// same Results for the same calls, and ctx.rand is keyed by the hook, so
// on_event and run_incident on one tick draw their own numbers.
func TestExtensionsDeterministic(t *testing.T) {
	t.Parallel()
	src := `
EVENTS = ["year", "quadrum"]
def on_event(ctx, event):
    ctx.message("event %s %d %d" % (event.kind, ctx.rand(1 << 30), ctx.rand(1 << 30)))
def run_incident(ctx, i):
    ctx.message("incident %d %d" % (ctx.rand(1 << 30), ctx.rand(1 << 30)))
    return True
def validate_command(ctx, cmd):
    return "no %d" % ctx.rand(1 << 30)
`
	run := func() []any {
		h := New([]Plugin{declared("d", allCaps, "scripts/a.star", src), declared("e", allCaps, "scripts/a.star", src)})
		ev := scripting.Event{Tick: 777, Seed: 42, Incident: scripting.Incident{Key: "d.x"}}
		var out []any
		for _, k := range []scripting.EventKind{scripting.Year, scripting.Quadrum} {
			ev.Kind = k
			out = append(out, h.Event(ev, &fakeWorld{}, nil))
		}
		ev.Kind = 0
		r, ok := h.RunIncident(ev, &fakeWorld{}, nil)
		p, why, vr := h.ValidateCommand(scripting.CommandView{Kind: "build", Name: "core.wall"}, ev, &fakeWorld{}, nil)
		p2, why2, _ := h.ValidateCommand(scripting.CommandView{Kind: "draft", Pawn: 1}, ev, &fakeWorld{}, nil)
		return append(out, r, ok, p, why, vr, p2, why2)
	}
	a, b := run(), run()
	if !reflect.DeepEqual(a, b) {
		t.Fatalf("two runs differ:\n%+v\n%+v", a, b)
	}
	year := msgs(a[0].(scripting.Result))
	quadrum := msgs(a[1].(scripting.Result))
	incident := msgs(a[2].(scripting.Result))
	if len(year) != 2 || len(quadrum) != 2 || len(incident) != 1 {
		t.Fatalf("year %q quadrum %q incident %q", year, quadrum, incident)
	}
	draws := func(s string) string { return s[strings.LastIndex(s[:strings.LastIndex(s, " ")], " ")+1:] }
	seen := map[string]string{}
	for _, s := range []string{year[0], year[1], quadrum[0], incident[0]} {
		if prev, ok := seen[draws(s)]; ok {
			t.Errorf("%q and %q draw the same numbers", prev, s)
		}
		seen[draws(s)] = s
	}
	if a[5] == a[8] {
		t.Errorf("two commands on one tick draw the same reason %q", a[5])
	}

	// Two crafts of one recipe on one tick, by other workers, draw their
	// own numbers.
	h := New([]Plugin{declared("d", allCaps, "scripts/a.star", strings.Replace(src, `"quadrum"]`, `"quadrum", "recipe_done"]`, 1))})
	craft := func(worker uint64) string {
		ev := scripting.Event{Tick: 777, Seed: 42, Kind: scripting.RecipeDone, Recipe: scripting.Craft{Key: "core.make_bow", Worker: worker}}
		got := msgs(h.Event(ev, &fakeWorld{}, nil))
		if len(got) != 1 {
			t.Fatalf("recipe_done by %d: %q", worker, got)
		}
		return got[0]
	}
	if one, two := craft(1), craft(2); one == two {
		t.Errorf("two workers' crafts draw the same numbers: %q", one)
	}
}
