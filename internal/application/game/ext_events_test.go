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
	"rimworld-go/internal/domain/economy/recipe"
	proj "rimworld-go/internal/domain/research"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/world/biome"
)

// TestExtEventsOrder: each event fires once at its point. At an hour's
// first tick the order is on_day, quadrum, year, on_hour, panels; an
// unsubscribed kind is never delivered; research_done, recipe_done and
// map_generated fire where the work completes, once.
func TestExtEventsOrder(t *testing.T) {
	t.Parallel()
	f := &fakeExt{subs: allEvents(), panels: []scripting.Panel{{Plugin: "p", Title: "T"}}}
	g := New(Config{Seed: 3, Width: 24, Height: 24, Scripts: f})
	UseExtensions(g, f)
	at := func(now uint64) []string {
		i := len(f.log)
		scriptClock(g, now)
		return f.logFrom(i)
	}
	for _, c := range []struct {
		now  uint64
		want []string
	}{
		{clock.TicksPerYear, []string{"on_day", "event:quadrum", "event:year", "on_hour", "panels"}},
		{clock.TicksPerYear + clock.TicksPerQuadrum, []string{"on_day", "event:quadrum", "on_hour", "panels"}},
		{clock.TicksPerYear + clock.TicksPerDay, []string{"on_day", "on_hour", "panels"}},
		{clock.TicksPerYear + clock.TicksPerHour, []string{"on_hour", "panels"}},
	} {
		if got := at(c.now); !slices.Equal(got, c.want) {
			t.Fatalf("at %d: %v, want %v", c.now, got, c.want)
		}
	}
	f.subs = map[scripting.EventKind]bool{scripting.Year: true}
	if got := at(2 * clock.TicksPerYear); !slices.Equal(got, []string{"on_day", "event:year", "on_hour", "panels"}) {
		t.Fatalf("only the year subscribed: %v", got)
	}

	f.subs = allEvents()
	f.events = nil
	g.lab.Prog[proj.Electricity] = 0 // a legacy colony starts with it done
	g.lab.Pick(proj.Electricity)
	if !g.lab.Working() {
		t.Fatalf("fixture: electricity not picked (done %v)", g.lab.Prog.Done(proj.Electricity))
	}
	g.lab.DevFinish()
	if len(f.events) != 1 || f.events[0].Kind != scripting.ResearchDone || f.events[0].Research != g.sk.content.Research.Key(proj.Electricity) || f.events[0].Tick != uint64(g.clock.Now()) {
		t.Fatalf("research_done: %+v", f.events)
	}
}

// TestExtRecipeDone: a finished bill on the home map fires recipe_done
// once, with the recipe and product keys, the count, the worker and the
// bench's cell.
func TestExtRecipeDone(t *testing.T) {
	t.Parallel()
	g, ada, bench := clubGame(t)
	f := &fakeExt{subs: map[scripting.EventKind]bool{scripting.RecipeDone: true}}
	UseExtensions(g, f)
	g.SpawnStack(item.Wood, 10, 6, 5)
	done := func() bool { return g.craft.Bills.Of(bench)[0].Done == 1 }
	runBillTicks(g, ada, 6000, done)
	if !done() {
		t.Fatal("no club made")
	}
	b, _ := g.bldgs.Get(bench)
	want := scripting.Craft{Key: g.sk.content.Recipes.Key(recipe.Club), Product: g.sk.content.Items.Key(item.Club), Count: 1, Worker: uint64(ada), X: b.X, Y: b.Y}
	if want.Key == "" || want.Product == "" || len(f.events) != 1 || f.events[0].Kind != scripting.RecipeDone || f.events[0].Recipe != want {
		t.Fatalf("recipe_done %+v, want one of %+v", f.events, want)
	}
}

// TestExtMapGenerated: a new colony's home map fires map_generated once
// when the extensions are set, before its first tick, with the map's
// seed, size and biome; a second set and a loaded save fire nothing.
func TestExtMapGenerated(t *testing.T) {
	t.Parallel()
	g := NewColony(Config{Seed: 9, Width: 40, Height: 30})
	f := &fakeExt{subs: allEvents(), res: scripting.Result{Commands: []command.Command{{Kind: command.ScriptPlace, Name: "core.wood", Count: 5, X: 2, Y: 2}}}}
	start := g.clock.Now()
	UseExtensions(g, f)
	want := scripting.MapGen{Seed: 9, W: 40, H: 30, Biome: biome.DefOf(g.biome).Key}
	if len(f.events) != 1 || f.events[0].Kind != scripting.MapGenerated || f.events[0].Map != want || f.events[0].Tick != uint64(start) || g.clock.Now() != start {
		t.Fatalf("map_generated %+v, want one of %+v at %d", f.events, want, start)
	}
	if g.cmds.Len() != 1 {
		t.Fatalf("its place waits for the first tick: %+v", g.cmds.Items())
	}
	UseExtensions(g, f)
	h := saveLoad(t, g)
	hf := &fakeExt{subs: allEvents()}
	UseExtensions(h, hf)
	if len(f.events) != 1 || len(hf.events) != 0 {
		t.Fatalf("fired again: %d, after load %d", len(f.events), len(hf.events))
	}
	if New(Config{Seed: 9, Width: 20, Height: 20}).scripts.gen.W != 0 {
		t.Fatal("a plain New generates no map to announce")
	}
}

// homeReads is what the gameread readers read through the game's
// exported reads, and the script world's own answers.
type homeReads struct {
	w, h       int
	terrain    string
	temp       int
	weather    string
	factions   int
	wildlife   int
	buildings  string
	colonists  int
	homeIsCurr bool
}

func readHome(g *Game, p scripting.PawnReader) homeReads {
	var bs []string
	for _, b := range BuildingCounts(g) {
		bs = append(bs, b.Kind+"="+string(rune('0'+b.Count)))
	}
	return homeReads{w: g.Width(), h: g.Height(), terrain: g.TerrainAt(3, 3), temp: g.TemperatureAt(3, 3),
		weather: g.Weather().Kind.String(), factions: len(g.Factions()), wildlife: len(g.Wildlife()),
		buildings: strings.Join(bs, ","), colonists: len(p.Pawns()), homeIsCurr: g.mapState == g.maps[0]}
}

// TestExtCallsHomeMapWhileViewingAnother: with the player viewing a
// second map, each of Fire, Event, RunIncident, ValidateCommand, Panels
// and Act reads the home map, through the script world and through the
// exported reads gameread uses (Width/Height, TerrainAt, TemperatureAt,
// Weather, Factions, Wildlife, BuildingCounts); each call's commands
// carry the home map id, and a script it disabled posts its message.
func TestExtCallsHomeMapWhileViewingAnother(t *testing.T) {
	t.Parallel()
	book := incidentBookWith(false, "frontier.dark_noon")
	f := &fakeExt{subs: allEvents(), runs: true, validates: true, panels: []scripting.Panel{{Plugin: "p", Title: "T", Actions: []scripting.PanelAction{{ID: "go"}}}}}
	g := New(Config{Seed: 3, Width: 24, Height: 16, Content: Content{Incidents: book}, Scripts: f})
	ada := spawnColonists(t, g, 1, 2)[0]
	buildNow(t, g, construction.Workbench, item.Wood, construction.South, 5, 8)
	home := readHome(g, &scriptWorld{g: g})
	if home.buildings == "" || home.colonists != 1 {
		t.Fatalf("fixture: home reads %+v", home)
	}
	UseExtensions(g, f)
	id, ok := DevOpenMap(g, 5)
	if !ok {
		t.Fatal("no second map")
	}
	g.ViewMap(id)
	if g.Width() == home.w || len(BuildingCounts(g)) == 0 {
		t.Fatalf("fixture: the viewed map reads %d wide (home %d)", g.Width(), home.w)
	}
	other := g.mapState
	seen := map[string]homeReads{}
	f.probe = func(call string, _ scripting.World, p scripting.PawnReader) {
		seen[call] = readHome(g, p)
		f.res = scripting.Result{ // each call's own notice (equal messages merge)
			Commands: []command.Command{{Kind: command.ScriptMessage, Name: "hello " + call}},
			Disabled: []scripting.Problem{{Plugin: "p", File: "scripts/x.star", Msg: "boom " + call}},
		}
	}
	before := g.cmds.Len()
	scriptClock(g, clock.TicksPerYear) // Fire, Event, Panels
	if !g.FireIncident("frontier.dark_noon") {
		t.Fatal("the plugin incident did not fire")
	}
	g.applyOne(command.Command{Kind: command.DraftPawn, Pawn: uint64(ada), Enabled: true}) // as the viewed map's pass would
	g.applyOne(command.Command{Kind: command.ScriptAction, Name: "p\ngo"})
	for _, call := range []string{"on_day", "event:quadrum", "event:year", "on_hour", "panels", "run_incident", "validate", "act"} {
		r, ok := seen[call]
		if !ok || r != home {
			t.Fatalf("%s read %+v (seen %v), want the home map's %+v", call, r, ok, home)
		}
	}
	if g.mapState != other {
		t.Fatal("the viewed map was not restored")
	}
	added := g.cmds.Items()[before:]
	if len(added) < 8 {
		t.Fatalf("%d commands queued", len(added))
	}
	for _, c := range added {
		if c.Kind == command.ScriptMessage && (!c.MapSet || c.Map != g.maps[0].id) {
			t.Fatalf("a script command for map %d (set %v), want home %d", c.Map, c.MapSet, g.maps[0].id)
		}
	}
	g.ViewMap(MapID(g.maps[0].id))
	n := 0
	for _, m := range g.messageViews() {
		if strings.Contains(m.Text, "scripts/x.star") && strings.Contains(m.Text, "boom") {
			n++
		}
	}
	if n < 8 {
		t.Fatalf("%d disabled-script notices, want one per call", n)
	}
}
