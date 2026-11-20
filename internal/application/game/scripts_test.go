// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"slices"
	"strings"
	"testing"

	"rimworld-go/internal/application/game/maps"
	"rimworld-go/internal/application/game/raid"
	"rimworld-go/internal/application/scripting"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/caravan"
	"rimworld-go/internal/testkit/allocs"
)

// The fake and the game's reader satisfy the port.
var (
	_ scripting.Host       = (*fakeScripts)(nil)
	_ scripting.World      = (*scriptWorld)(nil)
	_ scripting.PawnReader = (*scriptWorld)(nil)
)

// fakeScripts records the events (each with its own copy of the due
// colonists) and answers on_hour with a message, a letter and a spawn;
// on_day turns a script off. It uses only the cadence hooks in uses.
type fakeScripts struct {
	events []scripting.Event
	wood   int
	uses   []scripting.Hook
}

func (f *fakeScripts) Fire(ev scripting.Event, w scripting.World, _ scripting.PawnReader) scripting.Result {
	ev.Pawns = slices.Clone(ev.Pawns) // the host does not keep the game's slice
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

// Uses is true for the hooks in f.uses (none by default).
func (f *fakeScripts) Uses(h scripting.Hook) bool { return slices.Contains(f.uses, h) }

// pawnEvents is the due colonists' ids of f's events of hook h, by tick.
func (f *fakeScripts) pawnEvents(h scripting.Hook) map[uint64][]entity.EntityID {
	out := map[uint64][]entity.EntityID{}
	for _, ev := range f.events {
		if ev.Hook == h {
			for _, p := range ev.Pawns {
				out[ev.Tick] = append(out[ev.Tick], entity.EntityID(p.ID))
			}
		}
	}
	return out
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

// TestScriptHooksListed: the port names its six hooks.
func TestScriptHooksListed(t *testing.T) {
	t.Parallel()
	names := []string{"on_hour", "on_day", "on_incident", "on_colonist_died", "on_pawns", "on_pawn"}
	hs := scripting.Hooks()
	if len(hs) != len(names) {
		t.Fatalf("%d hooks: %v", len(hs), hs)
	}
	for i, h := range hs {
		if h.String() != names[i] {
			t.Fatalf("hook %d is %q, want %q", i, h, names[i])
		}
	}
	if scripting.OnPawns.String() != "on_pawns" || scripting.OnPawn.String() != "on_pawn" || scripting.Hook(0).String() != "" {
		t.Fatal("hook names")
	}
}

// spawnColonists stands n colonists on g's walkable cells, row by row,
// from row y0.
func spawnColonists(t testing.TB, g *Game, n, y0 int) []entity.EntityID {
	t.Helper()
	var ids []entity.EntityID
	for y := y0; y < g.grid.Height() && len(ids) < n; y++ {
		for x := 0; x < g.grid.Width() && len(ids) < n; x++ {
			if g.grid.Walkable(x, y) {
				id, err := g.SpawnNamedPawn(x, y, "")
				if err != nil {
					t.Fatal(err)
				}
				ids = append(ids, id)
			}
		}
	}
	if len(ids) < n {
		t.Fatalf("room for %d of %d colonists", len(ids), n)
	}
	return ids
}

// caravanStash takes id off the map onto a new caravan (Where caravan).
func caravanStash(t testing.TB, g *Game, id entity.EntityID) {
	t.Helper()
	cid := g.entities.Next()
	w := g.worldS()
	w.caravans = append(w.caravans, maps.Caravan{C: caravan.Caravan{ID: uint64(cid)}})
	if !g.stashPawn(id, cid) {
		t.Fatalf("stash %d", id)
	}
}

// setDowned downs id (alive, on the map).
func setDowned(g *Game, id entity.EntityID) {
	v, _ := g.vital.Get(id)
	v.Status = health.Downed
	g.vital.Set(id, v)
}

// TestScriptWorldPawns: the reader lists the living home colonists, id
// ascending, with the HUD's values, and finds nobody else.
func TestScriptWorldPawns(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 3, Width: 32, Height: 32, Scripts: &fakeScripts{}})
	c := spawnColonists(t, g, 6, 2)
	a, b, d, away, dead, held := c[0], c[1], c[2], c[3], c[4], c[5]
	caravanStash(t, g, away)
	g.die(dead, health.CauseInjury)
	g.makePrisoner(held, a)
	raider := spawnRaiderAt(t, g, 20, 20, 0)
	g.raidGroups = append(g.raidGroups, raid.Group{Size: 1}) // its group 0, for the steps
	guest := g.spawnVisitor(0, nil, 22, 20, uint64(g.clock.Now()))
	beast, err := g.SpawnAnimal("deer", 24, 20)
	if err != nil {
		t.Fatal(err)
	}
	setDowned(g, b)
	g.draft[d] = true
	check := func(stage string) {
		w := &scriptWorld{g: g, now: uint64(g.clock.Now())}
		ps := w.Pawns()
		var got []entity.EntityID
		for _, p := range ps {
			got = append(got, entity.EntityID(p.ID))
		}
		if !slices.Equal(got, []entity.EntityID{a, b, d}) {
			t.Fatalf("%s: Pawns %v, want %v", stage, got, []entity.EntityID{a, b, d})
		}
		if again := w.Pawns(); &again[0] != &ps[0] {
			t.Fatalf("%s: Pawns is built once per reader", stage)
		}
		views := map[entity.EntityID]PawnView{}
		for _, v := range g.Snapshot() {
			views[v.ID] = v
		}
		for _, p := range ps {
			v := views[entity.EntityID(p.ID)]
			if p.Name != v.Name || p.X != v.X || p.Y != v.Y || p.Food != v.Hunger || p.Rest != v.Rest || p.Mood != v.Mood ||
				p.Drafted != v.Drafted || p.Downed != (v.Status == health.Downed.String()) {
				t.Fatalf("%s: %+v, view %+v", stage, p, v)
			}
			if one, ok := w.Pawn(p.ID); !ok || one != p {
				t.Fatalf("%s: Pawn(%d) = %+v %v, want %+v", stage, p.ID, one, ok, p)
			}
		}
		for _, id := range []entity.EntityID{0, 99999, away, dead, held, raider, guest, beast} {
			if p, ok := w.Pawn(uint64(id)); ok {
				t.Fatalf("%s: Pawn(%d) found %+v", stage, id, p)
			}
		}
	}
	if g.soc != nil && len(g.soc.moodM) > 0 {
		t.Fatal("fixture: no smoothed mood before the first step")
	}
	check("before a step") // the mood target
	g.Step(3)
	setDowned(g, b)
	// smoothed moods, off the target
	g.socState().moodM[a], g.socState().moodM[d] = 37_400, 81_600
	check("after steps")
	if p, _ := (&scriptWorld{g: g}).Pawn(uint64(a)); p.Mood != 37 {
		t.Fatalf("smoothed mood %d, want 37", p.Mood)
	}
}

// TestPawnIDsGrowAscending: pawnIDs stays ascending through spawn,
// enlist, unstashPawn and load.
func TestPawnIDsGrowAscending(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 3, Width: 32, Height: 32})
	c := spawnColonists(t, g, 4, 2)
	asc := func(stage string, g *Game) {
		if !slices.IsSorted(g.pawnIDs) || len(slices.Compact(slices.Clone(g.pawnIDs))) != len(g.pawnIDs) {
			t.Fatalf("%s: %v", stage, g.pawnIDs)
		}
	}
	asc("spawn", g)
	g.makePrisoner(c[1], c[0])
	spawnColonists(t, g, 2, 10)
	g.enlist(c[1]) // older than the newest: the in-order insert
	asc("enlist", g)
	if !g.stashPawn(c[2], g.entities.Next()) {
		t.Fatal("stash")
	}
	spawnColonists(t, g, 1, 12)
	if !g.unstashPawn(c[2], 5, 20) {
		t.Fatal("unstash")
	}
	asc("unstash", g)
	if !slices.Contains(g.pawnIDs, c[1]) || !slices.Contains(g.pawnIDs, c[2]) {
		t.Fatalf("enlisted or unstashed missing: %v", g.pawnIDs)
	}
	asc("load", saveLoad(t, g))
}

// TestPawnIDsGrowSpawnAppendsInPlace: a new colonist, whose id is the
// newest, joins pawnIDs by append (no copy of the list), and the index
// notes it without an allocation once its bucket has room.
func TestPawnIDsGrowSpawnAppendsInPlace(t *testing.T) {
	g := New(Config{Seed: 3, Width: 16, Height: 16})
	g.pawnIDs = make([]entity.EntityID, 0, 64)
	pawnIndex(g) // built: notePawn adds
	// 5 ids a bucket: room for 3 more
	for i := range entity.EntityID(5 * scripting.PawnEvery) {
		g.scripts.pawns.Add(1_000_000 + i)
	}
	base := &g.pawnIDs[:1][0]
	if n := allocs.Fewest(3, func() { addPawnID(g, g.entities.Next()) }); n != 0 {
		t.Fatalf("addPawnID allocated %v times on the spawn path", n)
	}
	if len(g.pawnIDs) != 6 || &g.pawnIDs[0] != base || !slices.IsSorted(g.pawnIDs) {
		t.Fatalf("pawnIDs copied or out of order: %v", g.pawnIDs)
	}
}
