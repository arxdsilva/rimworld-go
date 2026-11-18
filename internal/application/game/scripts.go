// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"fmt"
	"strings"

	"rimworld-go/internal/application/scripting"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/incident"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// scriptRun is the game's side of the plugins' scripts (PLG-060,
// application/scripting): the host the run's plugins run in (nil: none)
// and the run's item keys. Nothing here is saved: scripts keep no state.
type scriptRun struct {
	host  scripting.Host
	kinds map[string]item.Kind // item key -> kind, built on first use
}

// UseScripts sets the scripts g runs (nil: none). The composition root
// sets it on a loaded save; a new game takes Config.Scripts. A function,
// not a method: Game is over its method budget.
func UseScripts(g *Game, h scripting.Host) { g.scripts.host = h }

// scriptsOn gates the scripts' clock system (phases.go).
func scriptsOn(g *Game) bool { return g.scripts.host != nil }

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
	res := g.scripts.host.Fire(ev, &scriptWorld{g: g, now: ev.Tick})
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

// scriptWorld is what a script reads (scripting.World), on the home map;
// the item counts are tallied once per call, so a script asking often
// costs one pass over the stacks.
type scriptWorld struct {
	g      *Game
	now    uint64
	counts map[item.Kind]int
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
