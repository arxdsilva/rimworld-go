// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"fmt"
	"slices"
	"strings"

	"rimworld-go/internal/application/game/cadence"
	"rimworld-go/internal/application/game/idindex"
	"rimworld-go/internal/application/game/modstate"
	"rimworld-go/internal/application/scripting"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/incident"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// scriptRun is the game's side of the plugins' scripts (PLG-060,
// application/scripting): the host the run's plugins run in (nil: none),
// the run's item keys, the cadence hooks' colonist index and the Wasm mods'
// state (PLG-063): its source for a real save, and the loaded save's until
// compose takes it. Starlark keeps no state; the index rebuilds on use.
type scriptRun struct {
	host   scripting.Host
	kinds  map[string]item.Kind // item key -> kind, built on first use
	pawns  idindex.Buckets      // home colonists by cadence bucket (pawnIndex)
	state  modstate.Source      // nil: no Wasm mod state to save
	loaded []modstate.Blob      // the loaded save's kept blobs, until taken
	notes  []modstate.Note      // what checking them dropped, until taken
}

// UseScripts sets the scripts g runs (nil: none). The composition root
// sets it on a loaded save; a new game takes Config.Scripts. A function,
// not a method: Game is over its method budget.
func UseScripts(g *Game, h scripting.Host) { g.scripts.host = h }

// UseModState sets where a real save reads the Wasm mods' state (nil:
// none). StateHash never asks it.
func UseModState(g *Game, s modstate.Source) { g.scripts.state = s }

// TakeLoadedModState hands over the mod state the loaded save held and the
// notes checking it, and every save since, left; once: a second call gives
// nothing, so a blob the mods dropped is never written again.
func TakeLoadedModState(g *Game) ([]modstate.Blob, []modstate.Note) {
	bs, ns := g.scripts.loaded, g.scripts.notes
	g.scripts.loaded, g.scripts.notes = nil, nil
	return bs, ns
}

// scriptsOn gates the scripts' clock system (phases.go).
func scriptsOn(g *Game) bool { return g.scripts.host != nil }

// pawnScriptsOn gates the cadence hooks' system (phases.go): a live
// script defines on_pawns or on_pawn.
func pawnScriptsOn(g *Game) bool {
	h := g.scripts.host
	return h != nil && (h.Uses(scripting.OnPawns) || h.Uses(scripting.OnPawn))
}

// scriptPawns runs on_pawns, then on_pawn, for the colonists due at now
// (each once every scripting.PawnEvery ticks), when anyone is due.
func scriptPawns(g *Game, now uint64) {
	if !pawnScriptsOn(g) {
		return
	}
	due, _ := duePawns(g, now)
	for _, h := range [2]scripting.Hook{scripting.OnPawns, scripting.OnPawn} {
		if len(due) > 0 && g.scripts.host.Uses(h) {
			fireScripts(g, scripting.Event{Hook: h, Tick: now, Pawns: due})
		}
	}
}

// duePawns is the home colonists due at now, id ascending, read from the
// index's bucket and verified (homeColonist); visited is the bucket's size.
func duePawns(g *Game, now uint64) (due []scripting.Pawn, visited int) {
	cur := g.mapState
	g.mapState = g.maps[0]
	defer func() { g.mapState = cur }()
	ids := pawnIndex(g).Due(now, cadence.OffPawnScripts)
	due = make([]scripting.Pawn, 0, len(ids))
	for _, id := range ids {
		if homeColonist(g, id) {
			due = append(due, scriptPawn(g, id))
		}
	}
	return due, len(ids)
}

// pawnIndex is the cadence hooks' colonist index, (re)built from the home
// map's pawnIDs when it was built from another map or not at all. It only
// grows (notePawn): a visit verifies each id, so a departure needs no
// bookkeeping.
func pawnIndex(g *Game) *idindex.Buckets {
	b, home := &g.scripts.pawns, g.maps[0]
	if !b.From(home) {
		b.Reset(scripting.PawnEvery, home)
		for _, id := range home.pawnIDs {
			b.Add(id)
		}
	}
	return b
}

// scriptClock runs on_day at 00:00 and on_hour every hour (phases.go: a
// colony system at each hour's first tick).
func scriptClock(g *Game, now uint64) {
	if clock.DateAt(clock.Tick(now)).Hour == 0 {
		fireScripts(g, scripting.Event{Hook: scripting.OnDay, Tick: now})
	}
	fireScripts(g, scripting.Event{Hook: scripting.OnHour, Tick: now})
}

// scriptFired runs on_incident for incident id once fireIncident made it
// happen (*fired).
func scriptFired(g *Game, id incident.ID, fired *bool) {
	if *fired {
		scriptIncident(g, id)
	}
}

// scriptIncident runs on_incident for incident id, which just fired.
func scriptIncident(g *Game, id incident.ID) {
	if g.scripts.host == nil {
		return
	}
	b := g.sk.content.Incidents
	cat := "nature"
	if b.DefOf(id).Category == incident.Threat {
		cat = "threat"
	}
	fireScripts(g, scripting.Event{Hook: scripting.OnIncident, Tick: uint64(g.clock.Now()),
		Incident: scripting.Incident{Key: b.Key(id), Category: cat}})
}

// scriptDied runs on_colonist_died for id, who just died (dieBy): only
// the colony's own, not raiders, prisoners or visitors.
func scriptDied(g *Game, id entity.EntityID) {
	if g.scripts.host == nil || !g.colonist(id) || g.prison.Is(id) || g.visitor(id) {
		return
	}
	fireScripts(g, scripting.Event{Hook: scripting.OnColonistDied, Tick: uint64(g.clock.Now()), Name: g.name(id)})
}

// fireScripts runs ev's hooks against the home map and queues their
// commands through Enqueue, for the home map, like the player's. A script
// the host disabled is reported with a message.
func fireScripts(g *Game, ev scripting.Event) {
	cur := g.mapState
	g.mapState = g.maps[0]
	defer func() { g.mapState = cur }()
	ev.Seed = g.streams.seed
	w := &scriptWorld{g: g, now: ev.Tick}
	res := g.scripts.host.Fire(ev, w, w)
	for _, c := range res.Commands {
		c.Map, c.MapSet = g.mapState.id, true
		g.Enqueue(c)
	}
	for _, p := range res.Disabled {
		where := p.File
		if p.Line > 0 {
			where = fmt.Sprintf("%s:%d", p.File, p.Line)
		}
		x, y := g.home()
		g.post(MessageView{Tick: ev.Tick, X: x, Y: y,
			Text: fmt.Sprintf("Plugin %s: script %s stopped and is off for this run (%s). See Plugins > Problems.", p.Plugin, where, p.Msg)})
	}
}

// applyScript applies a script's command (tick.go) on the home map.
func applyScript(g *Game, c command.Command) {
	switch c.Kind {
	case command.ScriptMessage:
		x, y := g.home()
		m := MessageView{Tick: uint64(g.clock.Now()), X: x, Y: y, Text: c.Name}
		if c.Mode == command.ScriptLetter {
			title, text, _ := strings.Cut(c.Name, "\n")
			m.Letter, m.Title, m.Text, m.Kind = true, title, text, "info"
		}
		if m.Text != "" || m.Title != "" {
			g.post(m)
		}
	case command.ScriptSpawn:
		k, ok := scriptKind(g, c.Name)
		if !ok || c.Count <= 0 {
			return
		}
		x, y := g.home()
		g.dropStack(k, min(c.Count, scripting.MaxSpawn), x, y)
	}
}

// scriptKind is the item kind of key: the run's item book's.
func scriptKind(g *Game, key string) (item.Kind, bool) {
	if g.scripts.kinds == nil {
		keys := g.sk.content.Items.Keys()
		g.scripts.kinds = make(map[string]item.Kind, len(keys))
		for i, k := range keys {
			g.scripts.kinds[k] = item.Kind(i)
		}
	}
	k, ok := g.scripts.kinds[key]
	return k, ok
}

// addPawnID adds a colonist that arrives on the home map to g.pawnIDs, in
// id order, and notes it for the scripts' cadence (PLG-060): the one path
// by which a colonist joins pawnIDs.
// A new colonist's id is the newest, so it appends in place.
func addPawnID(g *Game, id entity.EntityID) {
	if n := len(g.pawnIDs); n == 0 || id > g.pawnIDs[n-1] {
		g.pawnIDs = append(g.pawnIDs, id)
	} else {
		g.pawnIDs = withID(g.pawnIDs, id) // prisoners.go
	}
	notePawn(g, id)
}

// notePawn puts colonist id in the scripts' cadence index when it was
// built from the current map (the home map); before a build it does
// nothing, as the build reads pawnIDs.
func notePawn(g *Game, id entity.EntityID) {
	if g.scripts.pawns.From(g.mapState) {
		g.scripts.pawns.Add(id)
	}
}

// homeColonist reports id is a living colonist of the current map: in
// pawnIDs (ascending) and here (colonistHere). The cadence hooks and
// ctx.pawn decide by it alone.
func homeColonist(g *Game, id entity.EntityID) bool {
	_, in := slices.BinarySearch(g.pawnIDs, id)
	return in && colonistHere(g, id)
}

// scriptPawn is colonist id as a script reads it: the HUD's bars, mood
// (the smoothed one; its target only when there is none yet) and flags.
func scriptPawn(g *Game, id entity.EntityID) scripting.Pawn {
	pos, _ := g.positions.Get(id)
	name, _ := g.names.Get(id)
	n, _ := g.need.Get(id)
	v, _ := g.vital.Get(id)
	md, smoothed := 0, false
	if g.soc != nil {
		_, smoothed = g.soc.moodM[id]
	}
	if smoothed {
		md = g.smoothedMood(id, 0)
	} else {
		md = g.thoughtLevel(id)
	}
	return scripting.Pawn{ID: uint64(id), Name: name, X: pos.X, Y: pos.Y, Food: needs.Percent(n.Hunger), Rest: needs.Percent(n.Rest),
		Mood: md, Downed: v.Status == health.Downed, Drafted: g.draft[id]}
}

// scriptWorld is what a script reads (scripting.World), on the home map;
// the item counts are tallied once per call, so a script asking often
// costs one pass over the stacks.
type scriptWorld struct {
	g      *Game
	now    uint64
	counts map[item.Kind]int
	pawns  []scripting.Pawn // built on first use
	listed bool
}

func (w *scriptWorld) Day() int       { return w.g.relDay(w.now) }
func (w *scriptWorld) Hour() int      { return clock.DateAt(clock.Tick(w.now)).Hour }
func (w *scriptWorld) Season() string { return strings.ToLower(w.g.season(w.now)) }
func (w *scriptWorld) Colonists() int { return len(w.g.colonistsHomeAndAway()) }
func (w *scriptWorld) Wealth() int    { return w.g.Wealth() }

func (w *scriptWorld) ItemCount(key string) (int, bool) {
	k, ok := scriptKind(w.g, key)
	if !ok {
		return 0, false
	}
	if w.counts == nil {
		w.counts = map[item.Kind]int{}
		for _, id := range w.g.itemIDs {
			if st := w.g.stacks.Ref(id); st != nil {
				w.counts[st.Kind] += st.Qty
			}
		}
	}
	return w.counts[k], true
}

// Pawns is every living home colonist, id ascending (scripting.PawnReader),
// built once per reader.
func (w *scriptWorld) Pawns() []scripting.Pawn {
	if !w.listed {
		for _, id := range w.g.homeColonists() {
			w.pawns = append(w.pawns, scriptPawn(w.g, id))
		}
		w.listed = true
	}
	return w.pawns
}

// Pawn is home colonist id (scripting.PawnReader).
func (w *scriptWorld) Pawn(id uint64) (scripting.Pawn, bool) {
	if !homeColonist(w.g, entity.EntityID(id)) {
		return scripting.Pawn{}, false
	}
	return scriptPawn(w.g, entity.EntityID(id)), true
}
