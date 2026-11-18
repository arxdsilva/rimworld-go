// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"strings"
	"testing"

	"rimworld-go/internal/application/scripting"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/command"
)

// fakeScripts records the events and answers on_hour with a message, a
// letter and a spawn; on_day turns a script off.
type fakeScripts struct {
	events []scripting.Event
	wood   int
}

func (f *fakeScripts) Fire(ev scripting.Event, w scripting.World) scripting.Result {
	f.events = append(f.events, ev)
	f.wood, _ = w.ItemCount("core.wood")
	switch ev.Hook {
	case scripting.OnHour:
		return scripting.Result{Commands: []command.Command{
			{Kind: command.ScriptMessage, Name: "the wind turns"},
			{Kind: command.ScriptMessage, Mode: command.ScriptLetter, Name: "Omen\nA hawk circles."},
			{Kind: command.ScriptSpawn, Name: "core.wood", Count: 7},
			{Kind: command.ScriptSpawn, Name: "core.nothing", Count: 7},
		}}
	case scripting.OnDay:
		return scripting.Result{Disabled: []scripting.Problem{{Plugin: "p", File: "scripts/x.star", Line: 3, Msg: "boom"}}}
	}
	return scripting.Result{}
}

func TestScriptCommandsLandThroughEnqueue(t *testing.T) {
	t.Parallel()
	f := &fakeScripts{}
	g := New(Config{Seed: 3, Width: 24, Height: 24, Scripts: f})
	start := uint64(g.Tick()) // a new game starts at an hour's first tick
	g.Step(1)
	if len(f.events) != 1 || f.events[0].Hook != scripting.OnHour || f.events[0].Tick != start || f.events[0].Seed != 3 {
		t.Fatalf("events at the first tick: %+v", f.events)
	}
	if len(g.pending) != 4 {
		t.Fatalf("the commands wait in the queue for the next tick: %+v", g.pending)
	}
	for _, c := range g.pending {
		if !c.MapSet || c.Map != g.maps[0].id {
			t.Fatalf("a script command acts on the home map, whatever is viewed: %+v", c)
		}
	}
	scriptClock(g, clock.TicksPerDay) // 00:00: on_day, then on_hour
	if len(f.events) != 3 || f.events[1].Hook != scripting.OnDay || f.events[2].Hook != scripting.OnHour {
		t.Fatalf("at midnight: %+v", f.events)
	}
	g.pending = g.pending[:4]
	before := g.stockOf(item.Wood)
	g.Step(1)
	if got := g.stockOf(item.Wood) - before; got != 7 {
		t.Fatalf("spawned %d wood, want 7", got)
	}
	var note, letter, off bool
	for _, m := range g.messageViews() {
		note = note || m.Text == "the wind turns"
		letter = letter || m.Letter && m.Title == "Omen" && m.Text == "A hawk circles."
		off = off || strings.Contains(m.Text, "scripts/x.star:3") && strings.Contains(m.Text, "boom")
	}
	if !note || !letter || !off {
		t.Fatalf("message %v, letter %v, turned-off notice %v: %+v", note, letter, off, g.messageViews())
	}
	g.Step(clock.TicksPerHour - 1)
	if n := len(f.events); n != 4 || f.events[3].Hook != scripting.OnHour || f.events[3].Tick != start+clock.TicksPerHour {
		t.Fatalf("next hour: %+v", f.events)
	}
	if f.wood != g.stockOf(item.Wood) {
		t.Fatalf("ctx count %d, stock %d", f.wood, g.stockOf(item.Wood))
	}
}

func TestScriptsHearIncidentsAndDeaths(t *testing.T) {
	t.Parallel()
	f := &fakeScripts{}
	g := New(Config{Seed: 3, Width: 32, Height: 32, Scripts: f})
	if err := g.LoadStartKit(); err != nil {
		t.Fatal(err)
	}
	f.events = nil
	if !g.FireIncident("herd_crossing") {
		t.Skip("no herd could cross this map")
	}
	if len(f.events) != 1 || f.events[0].Hook != scripting.OnIncident || f.events[0].Incident != (scripting.Incident{Key: "core.herd_crossing", Category: "nature"}) {
		t.Fatalf("incident: %+v", f.events)
	}
	id := g.pawnIDs[0]
	name := g.name(id)
	g.die(id, health.CauseInjury)
	if len(f.events) != 2 || f.events[1].Hook != scripting.OnColonistDied || f.events[1].Name != name {
		t.Fatalf("death: %+v", f.events)
	}
}

func TestNoScriptsNoClock(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 3, Width: 16, Height: 16})
	if scriptsOn(g) {
		t.Fatal("a game without scripts runs no script clock")
	}
	g.Step(2)
	if len(g.pending) != 0 {
		t.Fatalf("%+v", g.pending)
	}
}
