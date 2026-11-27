// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"slices"
	"strings"
	"testing"

	"rimworld-go/internal/application/scripting"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/world/terrain"
)

// postedText counts g's messages containing s.
func postedText(g *Game, s string) int {
	n := 0
	for _, m := range g.messageViews() {
		if strings.Contains(m.Text, s) {
			n++
		}
	}
	return n
}

// TestExtCommandVeto: a player command of the closed set that a plugin
// vetoes is not applied and posts "Plugin <id>: <reason>"; without a
// reason it applies; commands outside the set, and script commands, are
// never asked about.
func TestExtCommandVeto(t *testing.T) {
	t.Parallel()
	f := &fakeExt{validates: true, veto: "not today"}
	g := New(Config{Seed: 3, Width: 24, Height: 24})
	ada := spawnColonists(t, g, 1, 2)[0]
	UseExtensions(g, f)
	g.Enqueue(command.Command{Kind: command.DraftPawn, Pawn: uint64(ada), Enabled: true})
	g.Step(1)
	if g.draft[ada] || postedText(g, "Plugin vetter: not today") != 1 {
		t.Fatalf("vetoed draft applied (%v) or no notice: %+v", g.draft[ada], g.messageViews())
	}
	if len(f.views) != 1 || f.views[0] != (scripting.CommandView{Kind: "draft", Pawn: uint64(ada)}) {
		t.Fatalf("views %+v", f.views)
	}
	g.Enqueue(command.Command{Kind: command.DesignateBuild, Build: uint8(construction.Wall), X: 9, Y: 9})
	g.Step(1)
	if len(g.frameIDs) != 0 || f.views[1] != (scripting.CommandView{Kind: "build", X: 9, Y: 9, Name: construction.Wall.String()}) {
		t.Fatalf("vetoed build: frames %d, view %+v", len(g.frameIDs), f.views[1])
	}
	f.veto = ""
	g.Enqueue(command.Command{Kind: command.DraftPawn, Pawn: uint64(ada), Enabled: true})
	g.Step(1)
	if !g.draft[ada] || len(f.views) != 3 {
		t.Fatalf("an allowed draft: drafted %v, asked %d times", g.draft[ada], len(f.views))
	}
	f.veto = "never"
	g.Enqueue(command.Command{Kind: command.SetWorkPriority, Pawn: uint64(ada)})
	g.Enqueue(command.Command{Kind: command.ScriptMessage, Name: "from a script", MapSet: true, Map: g.maps[0].id})
	g.Enqueue(command.Command{Kind: command.ScriptPlace, Name: "core.wood", Count: 1, X: 3, Y: 3, MapSet: true, Map: g.maps[0].id})
	g.Step(1)
	if len(f.views) != 3 || postedText(g, "from a script") != 1 {
		t.Fatalf("asked about a command outside the set (%d views) or a script's message dropped", len(f.views))
	}
}

// TestExtRunIncident: a plugin incident a plugin handles skips its like's
// behaviour and still counts as fired (on_incident hears it); unhandled,
// the like runs; a core incident never asks.
func TestExtRunIncident(t *testing.T) {
	t.Parallel()
	for _, replace := range []bool{true, false} {
		book := incidentBookWith(false, "frontier.dark_noon")
		f := &fakeExt{runs: true, replace: replace}
		g := New(Config{Seed: 3, Width: 20, Height: 20, Content: Content{Incidents: book}, Scripts: f})
		UseExtensions(g, f)
		if !g.FireIncident("frontier.dark_noon") {
			t.Fatalf("replace %v: not fired", replace)
		}
		if ran := g.incidents.EclipseUntil != 0; ran == replace {
			t.Fatalf("replace %v: the like ran %v", replace, ran)
		}
		want := scripting.Incident{Key: "frontier.dark_noon", Category: "nature"}
		if len(f.incidents) != 1 || f.incidents[0].Incident != want || !slices.Contains(f.log, "on_incident") {
			t.Fatalf("replace %v: asked %+v, log %v", replace, f.incidents, f.log)
		}
	}
	f := &fakeExt{runs: true, replace: true}
	g := New(Config{Seed: 3, Width: 20, Height: 20, Scripts: f})
	UseExtensions(g, f)
	g.FireIncident("eclipse")
	if len(f.incidents) != 0 || g.incidents.EclipseUntil == 0 {
		t.Fatalf("a core incident asked %d times, ran %v", len(f.incidents), g.incidents.EclipseUntil != 0)
	}
}

// TestExtPlace: ScriptPlace drops the stack (at most MaxSpawn) on an
// in-bounds standable cell of the home map, and nothing off the map, on a
// rock wall or for an unknown key.
func TestExtPlace(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 3, Width: 16, Height: 16})
	g.setTerrain(9, 9, terrain.Rock)
	place := func(name string, n, x, y int) int {
		before := g.stockOf(item.Wood)
		g.Enqueue(command.Command{Kind: command.ScriptPlace, Name: name, Count: n, X: x, Y: y, MapSet: true, Map: g.maps[0].id})
		g.Step(1)
		return g.stockOf(item.Wood) - before
	}
	if got := place("core.wood", 12, 4, 4); got != 12 || g.itemAt(4, 4) == 0 {
		t.Fatalf("placed %d (item at 4,4: %d), want 12 there", got, g.itemAt(4, 4))
	}
	for _, c := range []struct {
		name    string
		n, x, y int
		why     string
	}{
		{"core.wood", 5, -1, 4, "off the map"},
		{"core.wood", 5, 16, 4, "past the edge"},
		{"core.wood", 5, 9, 9, "a rock wall"},
		{"core.nothing", 5, 5, 5, "an unknown key"},
		{"core.wood", 0, 5, 5, "no count"},
	} {
		if got := place(c.name, c.n, c.x, c.y); got != 0 {
			t.Fatalf("%s: placed %d", c.why, got)
		}
	}
	if got := place("core.wood", scripting.MaxSpawn+50, 2, 10); got != scripting.MaxSpawn {
		t.Fatalf("placed %d, want the cap %d", got, scripting.MaxSpawn)
	}
}

// TestExtAction: the hourly panels reach WorldView; a ScriptAction runs
// Act only for an action its plugin's current panel offers (its commands
// queued for the home map); unknown and stale actions are ignored.
func TestExtAction(t *testing.T) {
	t.Parallel()
	f := &fakeExt{panels: []scripting.Panel{{Plugin: "p", Title: "Stores", Lines: []string{"wood 3"}, Actions: []scripting.PanelAction{{ID: "sell", Label: "Sell"}}}},
		res: scripting.Result{Commands: []command.Command{{Kind: command.ScriptMessage, Name: "sold"}}}}
	g := New(Config{Seed: 3, Width: 16, Height: 16, Scripts: f})
	g.Enqueue(command.Command{Kind: command.ScriptAction, Name: "p\nsell"})
	g.Step(1)
	if len(f.acts) != 0 {
		t.Fatal("an action ran with no extensions set")
	}
	UseExtensions(g, f)
	g.Enqueue(command.Command{Kind: command.ScriptAction, Name: "p\nsell"})
	g.Step(1)
	if len(f.acts) != 0 {
		t.Fatal("an action ran before any panel offered it")
	}
	scriptClock(g, uint64(g.clock.Now()))
	want := []PluginPanelView{{Plugin: "p", Title: "Stores", Lines: []string{"wood 3"}, Actions: []PluginActionView{{ID: "sell", Label: "Sell"}}}}
	if got := g.World().PluginPanels; len(got) != 1 || got[0].Title != want[0].Title || !slices.Equal(got[0].Lines, want[0].Lines) || !slices.Equal(got[0].Actions, want[0].Actions) {
		t.Fatalf("panels %+v", got)
	}
	for _, name := range []string{"p\nbuy", "q\nsell", "psell"} {
		g.Enqueue(command.Command{Kind: command.ScriptAction, Name: name})
	}
	g.Step(1)
	if len(f.acts) != 0 {
		t.Fatalf("unknown actions ran: %+v", f.acts)
	}
	g.Enqueue(command.Command{Kind: command.ScriptAction, Name: "p\nsell"})
	g.Step(2)
	if len(f.acts) != 1 || f.acts[0].Plugin != "p" || f.acts[0].Action != "sell" || postedText(g, "sold") == 0 {
		t.Fatalf("acts %+v, sold %d", f.acts, postedText(g, "sold"))
	}
	f.panels = []scripting.Panel{{Plugin: "p", Title: "Stores"}} // the button went
	scriptClock(g, uint64(g.clock.Now()))
	g.Enqueue(command.Command{Kind: command.ScriptAction, Name: "p\nsell"})
	g.Step(1)
	if len(f.acts) != 1 {
		t.Fatal("a stale action ran")
	}
	f.panels = nil // no panel hook left: none stand
	scriptClock(g, uint64(g.clock.Now()))
	if len(g.World().PluginPanels) != 0 {
		t.Fatal("panels stood with no panel hook")
	}
}
