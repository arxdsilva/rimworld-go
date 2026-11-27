// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package starlarkhost

import (
	"slices"
	"testing"

	"rimworld-go/internal/application/pluginapi"
	"rimworld-go/internal/application/scripting"
	"rimworld-go/internal/testkit/allocs"
)

// eventEcho messages every field of the event it gets, tagged with tag.
func eventEcho(tag string) string {
	return `
def on_event(ctx, event):
    ctx.message("` + tag + ` " + str(event))
`
}

// TestEventOrderAndFilter: on_event runs for the subscribed kinds only,
// plugins in load order and files in path order; a kind its plugin may
// not take, an unknown name or a non-string is a warning and ignored.
func TestEventOrderAndFilter(t *testing.T) {
	t.Parallel()
	ev := capSet(pluginapi.Events, pluginapi.Commands)
	h := New([]Plugin{
		declared("b", ev,
			"scripts/a.star", `EVENTS = ["year", "quadrum", "recipe_done", "nope", 3, "quadrum"]`+eventEcho("b/a"),
			"scripts/c.star", `EVENTS = ("quadrum",)`+eventEcho("b/c")),
		declared("a", ev|capSet(pluginapi.Recipes, pluginapi.Worldgen),
			"scripts/x.star", `EVENTS = ["recipe_done", "map_generated", "research_done"]`+eventEcho("a/x"),
			"scripts/y.star", `EVENTS = "quadrum"`+eventEcho("a/y"),
			"scripts/z.star", `EVENTS = ["quadrum"]`),
		plugin("legacy", "scripts/l.star", `EVENTS = ["quadrum", "map_generated"]`+eventEcho("legacy")),
	})
	wantWarn := []scripting.Problem{
		{Plugin: "b", File: "scripts/a.star", Msg: "EVENTS lists recipe_done but plugin.json does not declare recipes; it is never delivered"},
		{Plugin: "b", File: "scripts/a.star", Msg: `EVENTS lists "nope", which is no event; ignored`},
		{Plugin: "b", File: "scripts/a.star", Msg: "EVENTS[4] is int, not an event name; ignored"},
		{Plugin: "a", File: "scripts/y.star", Msg: "EVENTS is string, not a list of event names; ignored"},
		{Plugin: "a", File: "scripts/z.star", Msg: "EVENTS is set but on_event is not defined; no event is delivered"},
		{Plugin: "legacy", File: "scripts/l.star", Msg: "EVENTS lists map_generated but plugin.json does not declare worldgen; it is never delivered"},
	}
	if got := h.Warnings(); !slices.Equal(got, wantWarn) {
		t.Errorf("Warnings =\n%+v\nwant\n%+v", got, wantWarn)
	}
	for _, k := range scripting.EventKinds() {
		if !h.Subscribed(k) { // each has a taker: a/x, b/a or b/c
			t.Errorf("Subscribed(%s) = false", k)
		}
	}
	only := New([]Plugin{declared("b", ev, "scripts/a.star", `EVENTS = ["year", "recipe_done"]`+eventEcho("b"))})
	if !only.Subscribed(scripting.Year) || only.Subscribed(scripting.Quadrum) || only.Subscribed(scripting.RecipeDone) {
		t.Error("Subscribed must be the allowed EVENTS only")
	}
	if r := only.Event(scripting.Event{Kind: scripting.RecipeDone}, &fakeWorld{}, nil); len(r.Commands) != 0 {
		t.Errorf("recipe_done without recipes was delivered: %+v", r)
	}
	// A long bad EVENTS is warned about maxEventWarns times, then counted.
	many := New([]Plugin{declared("m", ev, "scripts/a.star", `EVENTS = ["x"] * 100 + ["year"]`+eventEcho("m"))})
	if w := many.Warnings(); len(w) != maxEventWarns+1 || w[maxEventWarns].Msg != "EVENTS has 96 more entries ignored" || !many.Subscribed(scripting.Year) {
		t.Errorf("long EVENTS: %d warnings, last %+v", len(w), w[len(w)-1])
	}
	if h.Subscribed(0) || h.Subscribed(200) {
		t.Error("no event or an out-of-range kind is subscribed")
	}

	cases := []struct {
		ev   scripting.Event
		want []string
	}{
		{scripting.Event{Kind: scripting.Quadrum, Tick: 9},
			[]string{`b/a event(kind = "quadrum")`, `b/c event(kind = "quadrum")`, `legacy event(kind = "quadrum")`}},
		{scripting.Event{Kind: scripting.Year},
			[]string{`b/a event(kind = "year")`}},
		{scripting.Event{Kind: scripting.ResearchDone, Research: "core.smithing"},
			[]string{`a/x event(kind = "research_done", research = "core.smithing")`}},
		{scripting.Event{Kind: scripting.RecipeDone, Recipe: scripting.Craft{Key: "core.make_bow", Product: "core.bow", Count: 2, Worker: 41, X: 3, Y: 4}},
			[]string{`a/x event(count = 2, key = "core.make_bow", kind = "recipe_done", product = "core.bow", worker = 41, x = 3, y = 4)`}},
		{scripting.Event{Kind: scripting.MapGenerated, Map: scripting.MapGen{Seed: 1 << 40, W: 250, H: 200, Biome: "core.tundra"}},
			[]string{`a/x event(biome = "core.tundra", height = 200, kind = "map_generated", seed = 1099511627776, width = 250)`}},
	}
	for _, c := range cases {
		res := h.Event(c.ev, &fakeWorld{}, nil)
		if got := msgs(res); !slices.Equal(got, c.want) || len(res.Disabled) != 0 {
			t.Errorf("Event(%s) = %q %+v,\nwant %q", c.ev.Kind, got, res.Disabled, c.want)
		}
	}
}

// The event struct is frozen; a failing on_event turns its file off and
// takes its kinds out of Subscribed once no live file wants them.
func TestEventFailureUnsubscribes(t *testing.T) {
	t.Parallel()
	ev := capSet(pluginapi.Events, pluginapi.Commands)
	h := New([]Plugin{
		declared("p", ev, "scripts/a.star", "EVENTS = [\"year\"]\ndef on_event(ctx, event):\n    event.kind = \"x\"\n"),
		declared("q", ev, "scripts/a.star", "EVENTS = [\"quadrum\"]\ndef on_event(ctx, event):\n    ctx.message(event.kind)\n"),
	})
	res := h.Event(scripting.Event{Kind: scripting.Year}, &fakeWorld{}, nil)
	if len(res.Disabled) != 1 || res.Disabled[0].Plugin != "p" || res.Disabled[0].Line != 3 {
		t.Fatalf("assigning to the event: %+v", res)
	}
	if h.Subscribed(scripting.Year) || !h.Subscribed(scripting.Quadrum) {
		t.Error("Subscribed must follow the live files")
	}
	if r := h.Event(scripting.Event{Kind: scripting.Year}, &fakeWorld{}, nil); len(r.Commands)+len(r.Disabled) != 0 {
		t.Errorf("a disabled file is called: %+v", r)
	}
	if got := msgs(h.Event(scripting.Event{Kind: scripting.Quadrum}, &fakeWorld{}, nil)); !slices.Equal(got, []string{"quadrum"}) {
		t.Errorf("q = %q", got)
	}
}

// A file that fails its first run is reported by whichever call comes
// first, an extension call too, and only once.
func TestExtensionsReportFirstRunFailures(t *testing.T) {
	t.Parallel()
	broken := plugin("bad", "scripts/a.star", "fail(\"boom\")\n")
	ev := scripting.Event{Kind: scripting.MapGenerated}
	calls := map[string]func(h *Host) scripting.Result{
		"Event": func(h *Host) scripting.Result { return h.Event(ev, &fakeWorld{}, nil) },
		"RunIncident": func(h *Host) scripting.Result {
			r, _ := h.RunIncident(scripting.Event{Incident: scripting.Incident{Key: "bad.x"}}, &fakeWorld{}, nil)
			return r
		},
		"ValidateCommand": func(h *Host) scripting.Result {
			_, _, r := h.ValidateCommand(scripting.CommandView{Kind: "draft"}, ev, &fakeWorld{}, nil)
			return r
		},
	}
	for name, call := range calls {
		h := New([]Plugin{broken})
		if r := call(h); len(r.Disabled) != 1 || r.Disabled[0].Msg != "fail: boom" {
			t.Errorf("%s first: %+v, want the load failure", name, r.Disabled)
		}
		if r := call(h); len(r.Disabled) != 0 {
			t.Errorf("%s again: %+v, want nothing", name, r.Disabled)
		}
		if r := fire(h, scripting.OnHour); len(r.Disabled) != 0 {
			t.Errorf("Fire after %s repeats the failure: %+v", name, r.Disabled)
		}
	}
}

// C4: the game asks these on every emit site; they read precomputed
// flags.
func TestSubscribedAndValidatesDoNotAllocate(t *testing.T) { // not parallel: AllocsPerRun
	h := New([]Plugin{declared("p", allCaps, "scripts/a.star",
		"EVENTS = [\"year\"]\ndef on_event(ctx, e):\n    pass\ndef validate_command(ctx, c):\n    return None\ndef run_incident(ctx, i):\n    return False\n")})
	if !h.Subscribed(scripting.Year) || !h.Validates() || !h.RunsIncidents() {
		t.Fatal("p subscribes, validates and runs incidents")
	}
	for name, f := range map[string]func(){
		"Subscribed":    func() { h.Subscribed(scripting.Year) },
		"Validates":     func() { h.Validates() },
		"RunsIncidents": func() { h.RunsIncidents() },
	} {
		if n := allocs.Fewest(20, f); n != 0 {
			t.Errorf("%s allocates %v", name, n)
		}
	}
}
