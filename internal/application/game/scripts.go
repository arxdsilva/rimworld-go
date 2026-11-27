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
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/recipe"
	"rimworld-go/internal/domain/incident"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/research"
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
	ext    scripting.Extensions // the run-time extension points (nil: none)
	kinds  map[string]item.Kind // item key -> kind, built on first use
	pawns  idindex.Buckets      // home colonists by cadence bucket (pawnIndex)
	state  modstate.Source      // nil: no Wasm mod state to save
	loaded []modstate.Blob      // the loaded save's kept blobs, until taken
	notes  []modstate.Note      // what checking them dropped, until taken
	panels []scripting.Panel    // the last hourly panels (never saved)
	// ordered: the pawns given a script order on visit orderAt (one each, from any hook or
	// plugin); a visit's orders apply in one tick's batch, so a new visit clears it. Not saved.
	ordered []entity.EntityID
	orderAt uint64
	gen     scripting.MapGen // a new game's home map, until map_generated fires (W 0: none)
}

// UseScripts sets the scripts g runs (nil: none). The composition root
// sets it on a loaded save; a new game takes Config.Scripts. A function,
// not a method: Game is over its method budget.
func UseScripts(g *Game, h scripting.Host) { g.scripts.host = h }

// UseExtensions sets the plugins' run-time extension points g calls (nil:
// none). The composition root sets it beside UseScripts, after the
// host's queries; on a new game the first set fires map_generated, once.
func UseExtensions(g *Game, x scripting.Extensions) {
	g.scripts.ext = x
	if gen := g.scripts.gen; x != nil && gen.W > 0 {
		g.scripts.gen = scripting.MapGen{}
		emitEvent(g, scripting.Event{Kind: scripting.MapGenerated, Tick: uint64(g.clock.Now()), Map: gen})
	}
}

// BuildingCounts is how many buildings of each kind stand on g's home
// map, kind ascending, at most scripting.MaxQueryRows rows: one pass over
// the home map's building index.
func BuildingCounts(g *Game) []scripting.BuildingCount {
	var n [256]int // construction.Kind is a byte
	home := g.maps[0]
	for _, id := range home.bldgIDs {
		if b := home.bldgs.Ref(id); b != nil {
			n[b.Kind]++
		}
	}
	var out []scripting.BuildingCount
	for k, c := range n {
		if c > 0 {
			out = append(out, scripting.BuildingCount{Kind: construction.Kind(k).String(), Count: c})
		}
	}
	slices.SortFunc(out, func(a, b scripting.BuildingCount) int { return strings.Compare(a.Kind, b.Kind) })
	return out[:min(len(out), scripting.MaxQueryRows)]
}

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

// scriptClock runs, at each hour's first tick (phases.go: a colony
// system), in this order: on_day at 00:00; then the quadrum event on a
// quadrum's first day, then the year event on a year's; on_hour; then the
// panels, refreshed.
func scriptClock(g *Game, now uint64) {
	if d := clock.DateAtPolicy(clock.Tick(now), g.clock.Calendar()); d.Hour == 0 {
		fireScripts(g, scripting.Event{Hook: scripting.OnDay, Tick: now})
		if d.Day == 0 {
			emitEvent(g, scripting.Event{Kind: scripting.Quadrum, Tick: now})
			if d.Quadrum == 0 {
				emitEvent(g, scripting.Event{Kind: scripting.Year, Tick: now})
			}
		}
	}
	fireScripts(g, scripting.Event{Hook: scripting.OnHour, Tick: now})
	scriptPanels(g, now)
}

// scriptPanels replaces the panels with the plugins' hourly answer; with
// no panel hook left, none stand.
func scriptPanels(g *Game, now uint64) {
	x := g.scripts.ext
	if x == nil || !x.HasPanels() {
		if g.scripts.panels != nil {
			g.scripts.panels = nil
			g.viewGen++
		}
		return
	}
	var ps []scripting.Panel
	scriptCall(g, scripting.Event{Tick: now}, func(ev scripting.Event, w *scriptWorld) (res scripting.Result) {
		ps, res = x.Panels(ev, w, w)
		return res
	})
	g.scripts.panels = ps
	g.viewGen++ // view cache (indexes.go): WorldView.PluginPanels
}

// pluginPanelViews is the panels as WorldView shows them (nil: none).
func pluginPanelViews(ps []scripting.Panel) []PluginPanelView {
	if len(ps) == 0 {
		return nil
	}
	out := make([]PluginPanelView, len(ps))
	for i, p := range ps {
		v := PluginPanelView{Plugin: p.Plugin, Title: p.Title, Lines: slices.Clone(p.Lines)}
		for _, a := range p.Actions {
			v.Actions = append(v.Actions, PluginActionView{ID: a.ID, Label: a.Label})
		}
		out[i] = v
	}
	return out
}

// emitEvent hands ev to the plugins subscribed to its kind (none: no
// call at all).
func emitEvent(g *Game, ev scripting.Event) {
	x := g.scripts.ext
	if x == nil || !x.Subscribed(ev.Kind) {
		return
	}
	scriptCall(g, ev, func(ev scripting.Event, w *scriptWorld) scripting.Result { return x.Event(ev, w, w) })
}

// scriptVetoKind is the kind name a script reads for the player commands
// validate_command sees; "" for any other.
func scriptVetoKind(k command.Kind) string {
	switch k {
	case command.DesignateBuild:
		return "build"
	case command.AddBill:
		return "add_bill"
	case command.SetResearch:
		return "research"
	case command.Prioritize:
		return "prioritize"
	case command.DraftPawn:
		return "draft"
	}
	return ""
}

// scriptVetoed asks the validating plugins about player command c
// (applyOne, first); a reason drops c and posts "Plugin <id>: <reason>".
// Script commands are not in the set, so they are never asked about.
func scriptVetoed(g *Game, c command.Command) bool {
	x := g.scripts.ext
	if x == nil || !x.Validates() {
		return false
	}
	kind := scriptVetoKind(c.Kind)
	if kind == "" {
		return false
	}
	v := scripting.CommandView{Kind: kind, Pawn: c.Pawn, X: c.X, Y: c.Y}
	switch c.Kind {
	case command.DesignateBuild:
		v.Name = construction.Kind(c.Build).String()
	case command.AddBill:
		v.Name = g.sk.content.Recipes.Key(recipe.ID(c.Recipe))
	case command.SetResearch:
		v.Name = g.sk.content.Research.Key(research.Project(c.Build))
	}
	var plugin, reason string
	scriptCall(g, scripting.Event{Tick: uint64(g.clock.Now())}, func(ev scripting.Event, w *scriptWorld) (res scripting.Result) {
		plugin, reason, res = x.ValidateCommand(v, ev, w, w)
		return res
	})
	if reason == "" {
		return false
	}
	hx, hy := g.home()
	g.post(MessageView{Tick: uint64(g.clock.Now()), X: hx, Y: hy, Text: fmt.Sprintf("Plugin %s: %s", plugin, reason)})
	return true
}

// scriptReplaced runs plugin incident id's run_incident (fireIncident,
// before its like's behaviour); true when the plugin took its place.
func scriptReplaced(g *Game, id incident.ID, now uint64) bool {
	x, b := g.scripts.ext, g.sk.content.Incidents
	if x == nil || !x.RunsIncidents() || b.Like(id) == id {
		return false
	}
	var replaced bool
	scriptCall(g, scripting.Event{Tick: now, Incident: scriptIncidentOf(b, id)}, func(ev scripting.Event, w *scriptWorld) (res scripting.Result) {
		res, replaced = x.RunIncident(ev, w, w)
		return res
	})
	return replaced
}

// scriptIncidentOf is incident id as a script sees it.
func scriptIncidentOf(b incident.Book, id incident.ID) scripting.Incident {
	cat := "nature"
	if b.DefOf(id).Category == incident.Threat {
		cat = "threat"
	}
	return scripting.Incident{Key: b.Key(id), Category: cat}
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
	fireScripts(g, scripting.Event{Hook: scripting.OnIncident, Tick: uint64(g.clock.Now()),
		Incident: scriptIncidentOf(g.sk.content.Incidents, id)})
}

// scriptDied runs on_colonist_died for id, who just died (dieBy): only
// the colony's own, not raiders, prisoners or visitors.
func scriptDied(g *Game, id entity.EntityID) {
	if g.scripts.host == nil || !g.colonist(id) || g.prison.Is(id) || g.visitor(id) {
		return
	}
	fireScripts(g, scripting.Event{Hook: scripting.OnColonistDied, Tick: uint64(g.clock.Now()), Name: g.name(id)})
}

// fireScripts runs ev's hooks (scriptCall).
func fireScripts(g *Game, ev scripting.Event) {
	scriptCall(g, ev, func(ev scripting.Event, w *scriptWorld) scripting.Result { return g.scripts.host.Fire(ev, w, w) })
}

// scriptCall is the one way into the plugins, on the simulation owner:
// call runs against the home map (whatever is viewed) with ev carrying the
// game's seed; its commands are queued through Enqueue for the home map,
// like the player's, and a script it disabled is reported with a message.
// A ScriptOrder is kept only from a cadence hook, stamped with that
// visit's tick (applyScript checks the order visit against it); a script
// cannot press a panel button (ScriptAction is dropped).
func scriptCall(g *Game, ev scripting.Event, call func(ev scripting.Event, w *scriptWorld) scripting.Result) scripting.Result {
	cur := g.mapState
	g.mapState = g.maps[0]
	defer func() { g.mapState = cur }()
	ev.Seed = g.streams.seed
	w := &scriptWorld{g: g, now: ev.Tick}
	res := call(ev, w)
	cadence := ev.Hook == scripting.OnPawns || ev.Hook == scripting.OnPawn
	for _, c := range res.Commands {
		if c.Kind == command.ScriptAction || c.Kind == command.ScriptOrder && !cadence {
			continue
		}
		if c.Kind == command.ScriptOrder {
			c.Target = ev.Tick
		}
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
	return res
}

// applyScript applies a script's command (tick.go) on the home map, and
// a panel's ScriptAction on whichever map the player sent it from.
func applyScript(g *Game, c command.Command) {
	switch c.Kind {
	case command.ScriptOrder:
		scriptOrder(g, c)
	case command.ScriptPlace:
		k, ok := scriptKind(g, c.Name)
		if ok && c.Count > 0 && g.grid.InBounds(c.X, c.Y) && cellStandable(g, c.X, c.Y) {
			g.dropStack(k, min(c.Count, scripting.MaxSpawn), c.X, c.Y)
		}
	case command.ScriptAction:
		plugin, action, _ := strings.Cut(c.Name, "\n")
		if x := g.scripts.ext; x != nil && panelHas(g.scripts.panels, plugin, action) {
			ev := scripting.Event{Tick: uint64(g.clock.Now()), Plugin: plugin, Action: action}
			scriptCall(g, ev, func(ev scripting.Event, w *scriptWorld) scripting.Result { return x.Act(ev, w, w) })
		}
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

// panelHas reports that plugin's last panel offers action id.
func panelHas(ps []scripting.Panel, plugin, id string) bool {
	for _, p := range ps {
		if p.Plugin == plugin && slices.ContainsFunc(p.Actions, func(a scripting.PanelAction) bool { return a.ID == id }) {
			return true
		}
	}
	return false
}

// scriptOrderDue reports that ScriptOrder c may act: it was queued on
// its pawn's order visit (the stamped tick is a cadence visit of the pawn
// and one in scripting.OrderEvery of them, and the first order of that
// visit, so a pawn takes at most one script order per
// OrderEvery*PawnEvery ticks), the pawn is a home colonist not in a
// mental break, and the order is in the closed set.
func scriptOrderDue(g *Game, c command.Command) bool {
	id, at := entity.EntityID(c.Pawn), c.Target
	if at/scripting.PawnEvery%scripting.OrderEvery != 0 || !cadence.Due(at, scripting.PawnEvery, cadence.OffPawnScripts, id) {
		return false
	}
	if at == g.scripts.orderAt && slices.Contains(g.scripts.ordered, id) {
		return false
	}
	switch c.Order {
	case command.OrderHaul, command.OrderCut, command.OrderMine, command.OrderHarvest, command.OrderChopWood, command.OrderSleep, command.OrderEat:
	default:
		return false
	}
	_, breaking := g.breaks[id]
	return !breaking && homeColonist(g, id) && g.alive(id)
}

// scriptOrder gives a due pawn its script order, checked like a
// player's prioritize (forcedJob) with a refusal silent; a job the pawn
// already runs (same kind and target) is left alone, so re-sending it
// never restarts it.
func scriptOrder(g *Game, c command.Command) {
	if !scriptOrderDue(g, c) {
		return
	}
	id := entity.EntityID(c.Pawn)
	if s := &g.scripts; c.Target != s.orderAt {
		s.ordered, s.orderAt = s.ordered[:0], c.Target
	}
	g.scripts.ordered = append(g.scripts.ordered, id)
	rec, why := g.forcedJob(id, c.Order, c.X, c.Y, 0)
	if why != "" {
		return
	}
	if j, ok := g.jobs.Get(id); ok && j.Kind == rec.Kind && j.State != job.Fail && j.State != job.Complete &&
		j.Target == rec.Target && (rec.Target != 0 || j.DestX == rec.DestX && j.DestY == rec.DestY) {
		return
	}
	g.prioritize(id, c.Order, c.X, c.Y, 0)
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
	md := g.moodOf(id)
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
