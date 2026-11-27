// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package starlarkhost

import (
	"errors"
	"maps"
	"os"
	"regexp"
	"slices"
	"testing"

	"go.starlark.net/starlark"

	"rimworld-go/internal/application/modding"
	"rimworld-go/internal/application/pluginapi"
	"rimworld-go/internal/application/scripting"
	"rimworld-go/internal/domain/sim/command"
)

// docHook is a hook line of the scripting package doc: a tab-indented
// function name and its open parenthesis ("on_hour(ctx)").
var docHook = regexp.MustCompile(`(?m)^//\t([a-z_]+)\(`)

// Every ctx attribute dir(ctx) lists and every hook the scripting doc
// names has exactly one row; the rows are the contract's; place needs
// recipes or worldgen by event; a name with no row is denied.
func TestCapsTableComplete(t *testing.T) {
	rows := map[string]int{}
	for _, r := range attrCaps {
		rows[r.name]++
	}
	for n, k := range rows {
		if k != 1 {
			t.Errorf("attrCaps has %d rows for %q", k, n)
		}
	}
	for _, n := range (&ctxValue{c: &call{}}).AttrNames() {
		if rows[n] != 1 {
			t.Errorf("ctx attribute %q has %d attrCaps rows, want 1", n, rows[n])
		}
	}

	type want struct {
		cap    pluginapi.Capability
		writes bool
		hooks  []string
	}
	wantAttrs := map[string]want{
		"tick": {cap: pluginapi.Query}, "day": {cap: pluginapi.Query}, "hour": {cap: pluginapi.Query},
		"season": {cap: pluginapi.Query}, "colonists": {cap: pluginapi.Query}, "wealth": {cap: pluginapi.Query},
		"count": {cap: pluginapi.Query}, "rand": {cap: pluginapi.Query}, "pawns": {cap: pluginapi.Query},
		"pawn":    {cap: pluginapi.Query},
		"message": {cap: pluginapi.Commands, writes: true}, "letter": {cap: pluginapi.Commands, writes: true},
		"spawn":   {cap: pluginapi.Commands, writes: true},
		"animals": {cap: pluginapi.World}, "buildings": {cap: pluginapi.World},
		"map": {cap: pluginapi.World}, "world": {cap: pluginapi.World},
		"order": {cap: pluginapi.Jobs, writes: true, hooks: []string{"on_pawns", "on_pawn"}},
		"place": {writes: true, hooks: []string{"on_event"}},
	}
	if len(attrCaps) != len(wantAttrs) {
		t.Errorf("attrCaps has %d rows, want %d", len(attrCaps), len(wantAttrs))
	}
	for _, r := range attrCaps {
		w, ok := wantAttrs[r.name]
		if !ok {
			t.Errorf("attrCaps row %q is not in the contract", r.name)
			continue
		}
		if r.cap != w.cap || r.writes != w.writes || !slices.Equal(r.hooks, w.hooks) {
			t.Errorf("attrCaps %q = {%s writes=%v hooks=%v}, want {%s writes=%v hooks=%v}",
				r.name, r.cap, r.writes, r.hooks, w.cap, w.writes, w.hooks)
		}
		if (r.byEvent != nil) != (r.name == "place") {
			t.Errorf("attrCaps %q: byEvent set = %v, want only on place", r.name, r.byEvent != nil)
		}
	}

	// N3: place is one capability per event kind, never one for all.
	place, _ := ruleOf("place")
	wantPlace := map[scripting.EventKind]pluginapi.Capability{
		scripting.RecipeDone: pluginapi.Recipes, scripting.MapGenerated: pluginapi.Worldgen,
	}
	if !maps.Equal(place.byEvent, wantPlace) {
		t.Errorf("place byEvent = %v, want %v", place.byEvent, wantPlace)
	}
	for _, k := range scripting.EventKinds() {
		if _, ok := place.byEvent[k]; ok != (k == scripting.RecipeDone || k == scripting.MapGenerated) {
			t.Errorf("place in %s: allowed = %v", k, ok)
		}
	}

	wantHooks := map[string]pluginapi.Capability{
		"on_hour": pluginapi.Events, "on_day": pluginapi.Events, "on_incident": pluginapi.Events,
		"on_colonist_died": pluginapi.Events, "on_pawns": pluginapi.Events, "on_pawn": pluginapi.Events,
		"on_event": pluginapi.Events, "register_defs": pluginapi.Defs,
		"validate_defs": pluginapi.Validate, "validate_command": pluginapi.Validate,
		"panel": pluginapi.UI, "on_action": pluginapi.UI, "run_incident": pluginapi.Incidents,
	}
	if !maps.Equal(hookCaps, wantHooks) {
		t.Errorf("hookCaps = %v, want %v", hookCaps, wantHooks)
	}
	for _, h := range scripting.Hooks() {
		if hookCaps[h.String()] != pluginapi.Events {
			t.Errorf("hook %s needs %s, want events", h, hookCaps[h.String()])
		}
	}
	doc, err := os.ReadFile("../../../application/scripting/doc.go")
	if err != nil {
		t.Fatal(err)
	}
	found := docHook.FindAllSubmatch(doc, -1)
	if len(found) < len(scripting.Hooks()) {
		t.Fatalf("the scripting doc names %d hooks, want at least %d", len(found), len(scripting.Hooks()))
	}
	for _, m := range found {
		if _, ok := hookCaps[string(m[1])]; !ok {
			t.Errorf("the scripting doc's hook %q has no hookCaps row", m[1])
		}
	}
	for _, h := range readOnlyHooks {
		if _, ok := hookCaps[h]; !ok {
			t.Errorf("read-only hook %q has no hookCaps row", h)
		}
	}

	// A call with no hook and no capability may use no mapped name, and
	// says so with an error that turns the file off (not a missing field).
	c := &call{}
	for _, r := range attrCaps {
		var nsa starlark.NoSuchAttrError
		if err := c.allowed(r.name); err == nil || errors.As(err, &nsa) {
			t.Errorf("allowed(%q) on a zero call = %v, want a denial", r.name, err)
		}
	}
	for _, n := range []string{"", "nope", "Tick", "globals", "__class__"} {
		var nsa starlark.NoSuchAttrError
		if err := c.allowed(n); !errors.As(err, &nsa) {
			t.Errorf("allowed(%q) = %v, want a NoSuchAttrError", n, err)
		}
	}
}

// dir(ctx) is the legacy thirteen plus the groups' lists, sorted, each
// once (with the W0 groups empty: the legacy thirteen), and a misspelt
// attribute fails as before, spelling hint included.
func TestCtxAttrsUnchanged(t *testing.T) {
	t.Parallel()
	legacy := []string{"colonists", "count", "day", "hour", "letter", "message", "pawn", "pawns", "rand", "season", "spawn", "tick", "wealth"}
	want := slices.Concat(legacy, queryAttrs, orderAttrs)
	slices.Sort(want)
	if got := (&ctxValue{c: &call{}}).AttrNames(); !slices.Equal(got, want) || len(slices.Compact(slices.Clone(got))) != len(got) {
		t.Errorf("AttrNames() = %v, want %v, each once", got, want)
	}
	h := New([]Plugin{plugin("p", "scripts/a.star", "def on_hour(ctx):\n    ctx.mesage(\"x\")\n")})
	res := fire(h, scripting.OnHour)
	if len(res.Disabled) != 1 || res.Disabled[0].Msg != "ctx has no .mesage field or method (did you mean .message?)" {
		t.Errorf("Disabled = %+v, want the no-such-attribute error with its hint", res.Disabled)
	}
}

// A plugin that declares nothing runs with the legacy set; a declared
// set, even empty, is what it gets.
func TestGranted(t *testing.T) {
	t.Parallel()
	if got := (Plugin{ID: "a"}).Granted(); got != pluginapi.Legacy() {
		t.Errorf("legacy Granted = %b, want %b", got, pluginapi.Legacy())
	}
	if got := (Plugin{ID: "a", Declared: true}).Granted(); got != 0 {
		t.Errorf("declared-empty Granted = %b, want 0", got)
	}
	s := pluginapi.Set(1<<pluginapi.World | 1<<pluginapi.Jobs)
	if got := (Plugin{ID: "a", Caps: s, Declared: true}).Granted(); got != s {
		t.Errorf("declared Granted = %b, want %b", got, s)
	}
	if got := (Plugin{ID: "a", Caps: s}).Granted(); got != pluginapi.Legacy() {
		t.Errorf("undeclared Caps Granted = %b, want legacy", got)
	}
}

// The extension stubs answer nothing, so the game behaves as before W1.
func TestExtensionStubsInert(t *testing.T) {
	t.Parallel()
	h := New(nil)
	h.UseQueries(scripting.Queries{})
	ev := scripting.Event{Kind: scripting.Quadrum}
	if h.Subscribed(scripting.Quadrum) || h.RunsIncidents() || h.Validates() || h.HasPanels() {
		t.Error("a stub reports a live extension")
	}
	if r := h.Event(ev, &fakeWorld{}, nil); len(r.Commands)+len(r.Disabled) != 0 {
		t.Errorf("Event = %+v", r)
	}
	if r, ok := h.RunIncident(ev, &fakeWorld{}, nil); ok || len(r.Commands) != 0 {
		t.Errorf("RunIncident = %+v, %v", r, ok)
	}
	if p, why, _ := h.ValidateCommand(scripting.CommandView{Kind: "draft"}, ev, &fakeWorld{}, nil); p != "" || why != "" {
		t.Errorf("ValidateCommand = %q, %q", p, why)
	}
	if ps, _ := h.Panels(ev, &fakeWorld{}, nil); ps != nil {
		t.Errorf("Panels = %v", ps)
	}
	if r := h.Act(ev, &fakeWorld{}, nil); len(r.Commands) != 0 {
		t.Errorf("Act = %+v", r)
	}
	d := NewDefs()
	if fs, ps := d.Register(modding.Plugin{Manifest: modding.Manifest{ID: "a"}}); fs != nil || ps != nil {
		t.Errorf("Register = %v, %v", fs, ps)
	}
	if ps := d.Validate("a", nil); ps != nil {
		t.Errorf("Validate = %v", ps)
	}
}

// capSet is the set of cs.
func capSet(cs ...pluginapi.Capability) pluginapi.Set {
	var s pluginapi.Set
	for _, c := range cs {
		s |= pluginapi.Set(1) << c
	}
	return s
}

// allCaps grants every capability.
var allCaps = capSet(pluginapi.Query, pluginapi.Commands, pluginapi.Events, pluginapi.World, pluginapi.Defs,
	pluginapi.Validate, pluginapi.UI, pluginapi.Incidents, pluginapi.Jobs, pluginapi.Stats, pluginapi.Health,
	pluginapi.Recipes, pluginapi.Worldgen)

// declared is a plugin that declares caps, with files as path, source pairs.
func declared(id string, caps pluginapi.Set, files ...string) Plugin {
	p := plugin(id, files...)
	p.Caps, p.Declared = caps, true
	return p
}

// errText is err's message, "" for nil.
func errText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

// TestCapsGate: every ctx attribute in every hook, with everything granted
// and with its own capability missing; the hand-written rows pin the
// messages and the scoped attributes.
func TestCapsGate(t *testing.T) {
	t.Parallel()
	rows := []struct {
		name, hook string
		kind       scripting.EventKind
		caps       pluginapi.Set
		want       string
	}{
		{"tick", "on_hour", 0, capSet(pluginapi.Query), ""},
		{"tick", "on_hour", 0, capsWithout(pluginapi.Query), "ctx.tick needs capability query"},
		{"pawns", "validate_command", 0, capSet(pluginapi.Query), ""},
		{"message", "on_day", 0, capSet(pluginapi.Commands), ""},
		{"message", "on_day", 0, capsWithout(pluginapi.Commands), "ctx.message needs capability commands"},
		{"spawn", "on_event", scripting.Quadrum, capSet(pluginapi.Commands), ""},
		{"animals", "on_hour", 0, pluginapi.Legacy(), "ctx.animals needs capability world"},
		{"world", "panel", 0, capSet(pluginapi.World), ""},
		{"order", "on_pawns", 0, capSet(pluginapi.Jobs), ""},
		{"order", "on_pawn", 0, capSet(pluginapi.Jobs), ""},
		{"order", "on_pawn", 0, capsWithout(pluginapi.Jobs), "ctx.order needs capability jobs"},
		{"order", "on_hour", 0, allCaps, "ctx.order is only for on_pawns and on_pawn"},
		{"order", "panel", 0, allCaps, "ctx.order is read-only in panel"},
		{"place", "on_event", scripting.RecipeDone, capSet(pluginapi.Recipes), ""},
		{"place", "on_event", scripting.RecipeDone, capSet(pluginapi.Worldgen), "ctx.place needs capability recipes"},
		{"place", "on_event", scripting.MapGenerated, capSet(pluginapi.Worldgen), ""},
		{"place", "on_event", scripting.MapGenerated, capSet(pluginapi.Recipes), "ctx.place needs capability worldgen"},
		{"place", "on_event", scripting.Quadrum, allCaps, "ctx.place is not available in on_event for quadrum"},
		{"place", "on_hour", 0, allCaps, "ctx.place is only for on_event"},
		{"letter", "register_defs", 0, allCaps, "ctx.letter is read-only in register_defs"},
	}
	for _, r := range rows {
		c := &call{hook: r.hook, caps: r.caps, ev: scripting.Event{Kind: r.kind}}
		if got := errText(c.allowed(r.name)); got != r.want {
			t.Errorf("%s in %s/%s with %b: %q, want %q", r.name, r.hook, r.kind, r.caps, got, r.want)
		}
	}

	// The sweep: every attribute, every hook (on_event in every kind).
	readOnly := map[string]bool{"panel": true, "validate_command": true, "validate_defs": true, "register_defs": true}
	for _, a := range attrCaps {
		for hook := range hookCaps {
			kinds := []scripting.EventKind{0}
			if hook == "on_event" {
				kinds = scripting.EventKinds()
			}
			for _, k := range kinds {
				c := &call{hook: hook, caps: allCaps, ev: scripting.Event{Kind: k}}
				scoped := (a.name == "order" && hook != "on_pawns" && hook != "on_pawn") ||
					(a.name == "place" && (hook != "on_event" || (k != scripting.RecipeDone && k != scripting.MapGenerated)))
				wantDenied := (a.writes && readOnly[hook]) || scoped
				if err := c.allowed(a.name); (err != nil) != wantDenied {
					t.Errorf("%s in %s/%s with every capability: %v, want denied=%v", a.name, hook, k, err, wantDenied)
				}
				if wantDenied {
					continue
				}
				need := a.cap
				if a.byEvent != nil {
					need = a.byEvent[k]
				}
				c.caps = capsWithout(need)
				want := "ctx." + a.name + " needs capability " + need.String()
				if got := errText(c.allowed(a.name)); got != want {
					t.Errorf("%s in %s/%s without %s: %q, want %q", a.name, hook, k, need, got, want)
				}
				c.caps = 0
				if c.allowed(a.name) == nil {
					t.Errorf("%s in %s/%s with no capability is allowed", a.name, hook, k)
				}
			}
		}
	}
}

// capsWithout is every capability but c.
func capsWithout(c pluginapi.Capability) pluginapi.Set { return allCaps &^ capSet(c) }

// The ctx gate as scripts meet it: a legacy plugin keeps its thirteen
// attributes; a declared plugin without query, or a legacy one reaching
// world, has its file turned off with the capability named.
func TestCapsGateTurnsTheFileOff(t *testing.T) {
	t.Parallel()
	legacy := "def on_hour(ctx):\n    ctx.message(\"%d %d %d %s %d %d %d %d\" % (ctx.tick, ctx.day, ctx.hour, ctx.season, ctx.colonists, ctx.wealth, ctx.count(\"core.wood\"), ctx.rand(5)))\n    ctx.letter(\"t\", \"x\")\n    ctx.spawn(\"core.wood\")\n    ctx.pawns()\n    ctx.pawn(1)\n"
	h := New([]Plugin{
		plugin("legacy", "scripts/a.star", legacy),
		declared("noquery", capSet(pluginapi.Events, pluginapi.Commands), "scripts/a.star", "def on_hour(ctx):\n    ctx.message(\"tick %d\" % ctx.tick)\n"),
		plugin("reach", "scripts/a.star", "def on_hour(ctx):\n    ctx.animals()\n"),
	})
	res := fire(h, scripting.OnHour)
	if len(res.Commands) != 3 || res.Commands[0].Kind != command.ScriptMessage {
		t.Errorf("legacy commands = %+v, want its message, letter and spawn", res.Commands)
	}
	want := []scripting.Problem{
		{Plugin: "noquery", File: "scripts/a.star", Line: 2, Msg: "ctx.tick needs capability query"},
		{Plugin: "reach", File: "scripts/a.star", Line: 2, Msg: "ctx.animals needs capability world"},
	}
	if !slices.Equal(res.Disabled, want) {
		t.Errorf("Disabled = %+v, want %+v", res.Disabled, want)
	}
	if r := fire(h, scripting.OnHour); len(r.Commands) != 3 || len(r.Disabled) != 0 {
		t.Errorf("the denied files stay off: %+v", r)
	}
}

// TestReadOnlyHookScope: every write attribute is denied in every
// read-only hook, whatever is granted; and a validate_command that writes
// is turned off.
func TestReadOnlyHookScope(t *testing.T) {
	t.Parallel()
	writes := 0
	for _, hook := range readOnlyHooks {
		for _, a := range attrCaps {
			c := &call{hook: hook, caps: allCaps, ev: scripting.Event{Kind: scripting.RecipeDone}}
			err := c.allowed(a.name)
			if !a.writes {
				if hook != "register_defs" && hook != "validate_defs" && err != nil {
					t.Errorf("read %s in %s: %v", a.name, hook, err)
				}
				continue
			}
			writes++
			if want := "ctx." + a.name + " is read-only in " + hook; errText(err) != want {
				t.Errorf("%s in %s: %q, want %q", a.name, hook, errText(err), want)
			}
		}
	}
	if writes != 5*len(readOnlyHooks) {
		t.Errorf("checked %d writes, want message, letter, spawn, order, place in each of %d hooks", writes, len(readOnlyHooks))
	}
	for _, attr := range []string{"message", "letter", "spawn", "order", "place"} {
		h := New([]Plugin{declared("v", allCaps, "scripts/v.star", "def validate_command(ctx, cmd):\n    ctx."+attr+"\n")})
		_, why, res := h.ValidateCommand(scripting.CommandView{Kind: "build"}, scripting.Event{Tick: 1}, &fakeWorld{}, nil)
		want := "ctx." + attr + " is read-only in validate_command"
		if why != "" || len(res.Disabled) != 1 || res.Disabled[0].Msg != want {
			t.Errorf("validate_command writing %s: %q, %+v, want %q", attr, why, res.Disabled, want)
		}
		if h.Validates() {
			t.Errorf("validate_command writing %s is still validating", attr)
		}
	}
}

// orders.go's mayUse and allowed agree on ctx.order and ctx.place in
// every hook, kind and capability set (QA relay: mayUse can go once this
// holds).
func TestCapsOrderPlaceAgreeWithMayUse(t *testing.T) {
	t.Parallel()
	sets := []pluginapi.Set{0, allCaps, capsWithout(pluginapi.Jobs), capSet(pluginapi.Jobs),
		capSet(pluginapi.Recipes), capSet(pluginapi.Worldgen), pluginapi.Legacy()}
	for _, name := range []string{"order", "place"} {
		r, _ := ruleOf(name)
		for hook := range hookCaps {
			for _, k := range append([]scripting.EventKind{0}, scripting.EventKinds()...) {
				for _, s := range sets {
					c := &call{hook: hook, caps: s, ev: scripting.Event{Kind: k}}
					if got, want := c.allowed(name) == nil, c.mayUse(r); got != want {
						t.Errorf("%s in %s/%s with %b: allowed=%v, mayUse=%v", name, hook, k, s, got, want)
					}
				}
			}
		}
	}
}

// TestUndeclaredHookNeverCalled: a hook whose capability the plugin does
// not declare gets one warning at New, with its line, and is never
// called; it is not a disabled file.
func TestUndeclaredHookNeverCalled(t *testing.T) {
	t.Parallel()
	src := `
def on_hour(ctx):
    ctx.message("hour")

def run_incident(ctx, incident):
    ctx.message("ran")
    return True

def validate_command(ctx, cmd):
    return "no"

def on_event(ctx, event):
    ctx.message("event")

EVENTS = ["quadrum"]
`
	h := New([]Plugin{
		declared("bare", capSet(pluginapi.Query, pluginapi.Commands), "scripts/a.star", src),
		declared("ok", capSet(pluginapi.Commands, pluginapi.Events), "scripts/a.star", "def on_hour(ctx):\n    ctx.message(\"ok\")\n"),
	})
	want := []scripting.Problem{
		{Plugin: "bare", File: "scripts/a.star", Line: 12, Msg: "defines on_event but plugin.json does not declare events; it is never called"},
		{Plugin: "bare", File: "scripts/a.star", Line: 2, Msg: "defines on_hour but plugin.json does not declare events; it is never called"},
		{Plugin: "bare", File: "scripts/a.star", Line: 5, Msg: "defines run_incident but plugin.json does not declare incidents; it is never called"},
		{Plugin: "bare", File: "scripts/a.star", Line: 9, Msg: "defines validate_command but plugin.json does not declare validate; it is never called"},
	}
	if got := h.Warnings(); !slices.Equal(got, want) {
		t.Errorf("Warnings = %+v,\nwant %+v", got, want)
	}
	if p := h.Problems(); len(p) != 0 {
		t.Errorf("an undeclared hook is no disabled file: %+v", p)
	}
	if h.Subscribed(scripting.Quadrum) || h.RunsIncidents() || h.Validates() {
		t.Error("an undeclared hook is reported live")
	}
	res := fire(h, scripting.OnHour)
	if len(res.Commands) != 1 || res.Commands[0].Name != "ok" || len(res.Disabled) != 0 {
		t.Errorf("on_hour: %+v, want only ok's message", res)
	}
	ev := scripting.Event{Kind: scripting.Quadrum, Incident: scripting.Incident{Key: "bare.x"}}
	if r := h.Event(ev, &fakeWorld{}, nil); len(r.Commands) != 0 {
		t.Errorf("on_event was called: %+v", r)
	}
	if r, ok := h.RunIncident(ev, &fakeWorld{}, nil); ok || len(r.Commands) != 0 {
		t.Errorf("run_incident was called: %+v %v", r, ok)
	}
	if p, why, _ := h.ValidateCommand(scripting.CommandView{Kind: "build"}, ev, &fakeWorld{}, nil); p != "" || why != "" {
		t.Errorf("validate_command was called: %q %q", p, why)
	}
	// A legacy plugin keeps its clock hooks with no warning.
	if w := New([]Plugin{plugin("l", "scripts/a.star", "def on_hour(ctx):\n    pass\n")}).Warnings(); len(w) != 0 {
		t.Errorf("legacy warnings = %+v", w)
	}
}
