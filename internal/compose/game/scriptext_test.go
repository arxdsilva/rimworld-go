// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"slices"
	"strings"
	"testing"

	"rimworld-go/internal/adapters/outbound/wasmhost"
	appgame "rimworld-go/internal/application/game"
	"rimworld-go/internal/application/modding"
	"rimworld-go/internal/application/scripting"
	"rimworld-go/internal/domain/sim/clock"
)

// capPlugin is a loaded plugin with one Starlark file and declared capabilities.
func capPlugin(id string, caps []string, src string) modding.Plugin {
	p := starPlugin(id, "scripts/main.star", src)
	p.Manifest.Capabilities = caps
	return p
}

func msgs(r scripting.Result) []string {
	var out []string
	for _, c := range r.Commands {
		out = append(out, c.Name)
	}
	return out
}

// TestScriptMuxExtensions is the forwarding table: absent segments give a
// literal nil, present ones are asked in load order and merged.
func TestScriptMuxExtensions(t *testing.T) {
	t.Parallel()
	if testMux(t).extensions() != nil {
		t.Fatal("no plugin: want a literal nil")
	}
	if testMux(t, wasmPlugin("w", "scripts/a.wasm", tallyWasm(t))).extensions() != nil {
		t.Fatal("Wasm only: want a literal nil")
	}
	m := testMux(t,
		capPlugin("a", []string{"events", "commands", "incidents", "validate", "ui"}, `
EVENTS = ["quadrum"]
def on_event(ctx, event):
    ctx.message("a " + event.kind)
def run_incident(ctx, i):
    ctx.message("a-inc")
    return True
def validate_command(ctx, cmd):
    return "no from a"
def panel(ctx):
    return {"title": "A", "lines": ["x"], "actions": [{"id": "go", "label": "Go"}]}
def on_action(ctx, action):
    ctx.message("act " + action)
`),
		capPlugin("b", []string{"events", "commands", "incidents", "validate", "ui"}, `
EVENTS = ["quadrum", "year"]
def on_event(ctx, event):
    ctx.message("b " + event.kind)
def run_incident(ctx, i):
    ctx.message("b-inc")
    return True
def validate_command(ctx, cmd):
    return "no from b"
def panel(ctx):
    return {"title": "B"}
`),
		wasmPlugin("w", "scripts/a.wasm", tallyWasm(t)))
	x := m.extensions()
	if x == nil {
		t.Fatal("no extensions")
	}
	if !x.Subscribed(scripting.Quadrum) || !x.Subscribed(scripting.Year) || x.Subscribed(scripting.MapGenerated) {
		t.Fatal("Subscribed")
	}
	w := fixedWorld{day: 1}
	if got := msgs(x.Event(scripting.Event{Kind: scripting.Quadrum, Seed: 1}, w, nil)); !slices.Equal(got, []string{"a quadrum", "b quadrum"}) {
		t.Fatalf("quadrum: %v", got)
	}
	if got := msgs(x.Event(scripting.Event{Kind: scripting.Year, Seed: 1}, w, nil)); !slices.Equal(got, []string{"b year"}) {
		t.Fatalf("year: %v", got)
	}
	if !x.RunsIncidents() {
		t.Fatal("RunsIncidents")
	}
	res, ok := x.RunIncident(scripting.Event{Seed: 1, Incident: scripting.Incident{Key: "a.x"}}, w, nil)
	if !ok || !slices.Equal(msgs(res), []string{"a-inc"}) {
		t.Fatalf("first replacement wins: %v %v", ok, msgs(res))
	}
	if !x.Validates() {
		t.Fatal("Validates")
	}
	pl, why, _ := x.ValidateCommand(scripting.CommandView{Kind: "spawn"}, scripting.Event{Seed: 1}, w, nil)
	if pl != "a" || !strings.Contains(why, "no from a") {
		t.Fatalf("first denial wins: %q %q", pl, why)
	}
	if !x.HasPanels() {
		t.Fatal("HasPanels")
	}
	ps, _ := x.Panels(scripting.Event{Seed: 1}, w, nil)
	if len(ps) != 2 || ps[0].Plugin != "a" || ps[1].Plugin != "b" {
		t.Fatalf("panels in load order: %+v", ps)
	}
	act := x.Act(scripting.Event{Seed: 1, Plugin: "a", Action: "go"}, w, nil)
	if !slices.Equal(msgs(act), []string{"act go"}) {
		t.Fatalf("act: %v", msgs(act))
	}
}

// A plugin that defines a hook without its capability gets a warning on
// Late, and nothing turns off.
func TestMuxLateWarnsUndeclaredHook(t *testing.T) {
	t.Parallel()
	m := testMux(t, capPlugin("w", []string{"query"}, "def run_incident(ctx, i):\n    return True\n"))
	var warn bool
	for _, p := range m.Late() {
		if p.Plugin == "w" && p.Warn && !strings.Contains(p.Msg, "turned off") {
			warn = true
		}
	}
	if !warn || m.extensions().RunsIncidents() {
		t.Fatalf("late %+v", m.Late())
	}
}

// TestScriptPluginsCaps (C8): the host gets what CapsOf says.
func TestScriptPluginsCaps(t *testing.T) {
	t.Parallel()
	// legacy: events is granted, a declared query-only plugin gets no on_event
	ev := `EVENTS = ["quadrum"]
def on_event(ctx, event):
    ctx.message("e")
`
	l := testMux(t, starPlugin("l", "scripts/a.star", ev))
	d := testMux(t, capPlugin("d", []string{"query"}, ev))
	if !l.extensions().Subscribed(scripting.Quadrum) {
		t.Fatal("legacy grants events")
	}
	if d.extensions().Subscribed(scripting.Quadrum) {
		t.Fatal("declared query only: no events")
	}
}

// TestStartQueriesBeforeExtensions (C7): start attaches the extension points
// after the queries, so a map_generated handler reads the new map.
func TestStartQueriesBeforeExtensions(t *testing.T) {
	t.Parallel()
	m := testMux(t, capPlugin("p", []string{"events", "commands", "world", "worldgen"}, `
EVENTS = ["map_generated"]
def on_event(ctx, event):
    ctx.message("map %d" % ctx.map.width)
`))
	g := appgame.NewColony(appgame.Config{Seed: 1, Width: 24, Height: 20})
	m.start(g)
	g.Step(1)
	found := false
	for _, msg := range g.World().Messages {
		found = found || strings.Contains(msg.Text, "map 24")
	}
	if !found {
		t.Fatalf("map_generated did not read the map: %+v late %+v", g.World().Messages, m.Late())
	}
}

func TestStartWiresExtensions(t *testing.T) {
	t.Parallel()
	// no star segment: nothing is wired (the game keeps its tick path)
	m := newScriptMux(nil, wasmhost.Options{})
	g := appgame.New(appgame.Config{Seed: 1, Width: 8, Height: 8})
	m.start(g)
	if m.extensions() != nil || m.host() != nil {
		t.Fatal("empty mux wires nothing")
	}
	// a quadrum event reaches the plugin through the game
	m2 := testMux(t, capPlugin("p", []string{"events", "commands"}, `
EVENTS = ["quadrum"]
def on_event(ctx, event):
    ctx.message("quadrum seen")
`))
	g2 := appgame.New(appgame.Config{Seed: 1, Width: 16, Height: 16})
	if err := g2.LoadStartKit(); err != nil {
		t.Fatal(err)
	}
	m2.start(g2)
	// jump (SkipTo) to the last hour before the second quadrum instead of
	// running 16 days of a colony (~25 s under -race), then run across it
	seen := func() bool {
		for _, msg := range g2.World().Messages {
			if strings.Contains(msg.Text, "quadrum seen") {
				return true
			}
		}
		return false
	}
	g2.SkipTo(clock.DaysPerQuadrum-1, clock.HoursPerDay-1)
	g2.Step(clock.TicksPerHour)
	if seen() {
		t.Fatalf("quadrum event before the quadrum: %+v", g2.World().Messages)
	}
	g2.Step(clock.TicksPerHour)
	if !seen() {
		t.Fatalf("quadrum event did not reach the plugin: %+v", g2.World().Messages)
	}
}
