// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"bytes"
	"fmt"
	"slices"
	"strings"
	"testing"

	"rimworld-go/internal/adapters/outbound/modloader"
	appgame "rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/recipe"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/pawn/stat"
	"rimworld-go/internal/domain/pawn/stat/statmod"
	"rimworld-go/internal/domain/research"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// The plugin API's end-to-end proof: the fixture plugins of testdata/extapi
// are read by the real loader, built into content, their scripts run by the
// real scriptMux inside a real game, and each AC is read back from what the
// player or the plugin author could observe: problems, Plugins rows, the
// message ticker, WorldView, stocks and the colonist card.

const extVersion = "0.1.35"

// extContent is the fixture root's content with ids enabled.
func extContent(ids ...string) runContent {
	return buildContent(modloader.New("testdata/extapi"), ids, extVersion)
}

// said is one ticker message with the tick it was posted at.
type said struct {
	tick uint64
	text string
}

// extRun is a game with the fixture plugins' scripts attached.
type extRun struct {
	t    *testing.T
	c    runContent
	m    *scriptMux
	g    *appgame.Game
	log  []said
	seen map[said]bool
}

// newExt is a game over c's content, the scripts not yet attached: a
// generated colony (colony) or an empty yard. cfg's size is 40x30.
func newExt(t *testing.T, c runContent, seed uint64, colony bool) *extRun {
	t.Helper()
	cfg := appgame.Config{Seed: seed, Width: 40, Height: 30, Content: c.game()}
	var g *appgame.Game
	if colony {
		g = appgame.NewColony(cfg)
	} else {
		g = appgame.New(cfg)
	}
	m := scriptHost(c, false)
	t.Cleanup(func() { _ = m.Close() })
	return &extRun{t: t, c: c, m: m, g: g, seen: map[said]bool{}}
}

// start attaches the scripts as a new game does (and fires map_generated).
func (e *extRun) start() *extRun {
	e.m.start(e.g)
	return e
}

// collect adds the messages new since the last read to the log.
func (e *extRun) collect() {
	for _, m := range e.g.World().Messages {
		s := said{m.Tick, strings.TrimSpace(m.Text)}
		if !e.seen[s] {
			e.seen[s] = true
			e.log = append(e.log, s)
		}
	}
}

// step runs n ticks, reading the ticker every chunk ticks (the ticker keeps
// only the last few messages).
func (e *extRun) step(n, chunk int) {
	for n > 0 {
		k := min(n, chunk)
		e.g.Step(k)
		e.collect()
		n -= k
	}
}

// texts is every message so far, oldest first.
func (e *extRun) texts() []string {
	var out []string
	for _, s := range e.log {
		out = append(out, s.text)
	}
	return out
}

// index is the log position of the first message containing sub (-1: none).
func (e *extRun) index(sub string) int {
	return slices.IndexFunc(e.log, func(s said) bool { return strings.Contains(s.text, sub) })
}

// count is how many messages contain sub.
func (e *extRun) count(sub string) int {
	n := 0
	for _, s := range e.log {
		if strings.Contains(s.text, sub) {
			n++
		}
	}
	return n
}

// first is the first colonist's id.
func first(t *testing.T, g *appgame.Game) entity.EntityID {
	t.Helper()
	cs := g.Colonists()
	if len(cs) == 0 {
		t.Fatal("no colonist")
	}
	return cs[0].ID
}

// problemsOf are c's problems of plugin id.
func problemsOf(c runContent, id string) (errs, warns []string) {
	for _, p := range c.problems {
		if p.Plugin != id {
			continue
		}
		if p.Warn {
			warns = append(warns, p.String())
		} else {
			errs = append(errs, p.String())
		}
	}
	return errs, warns
}

func hasSub(list []string, sub string) bool {
	return slices.ContainsFunc(list, func(s string) bool { return strings.Contains(s, sub) })
}

func TestExtE2E(t *testing.T) {
	t.Run("AC1_API_version", func(t *testing.T) {
		t.Parallel()
		c := extContent("extapi", "extapi_newer", "extapi_legacy")
		ids := c.catalog.IDs()
		if !slices.Contains(ids, "extapi") || !slices.Contains(ids, "extapi_legacy") || slices.Contains(ids, "extapi_newer") {
			t.Fatalf("loaded %v: the plugin that needs API 2 must be refused, the others load", ids)
		}
		errs, _ := problemsOf(c, "extapi_newer")
		if len(errs) == 0 {
			t.Fatalf("no error problem for the API 2 plugin: %v", c.problems)
		}
		for _, p := range c.problems {
			if p.Plugin == "extapi" && p.File == "plugin.json" {
				t.Fatalf("an api 1 plugin with a known capability set has a manifest problem: %v", p)
			}
		}
		_, warns := problemsOf(c, "extapi_legacy")
		if !hasSub(warns, "declares no plugin API version; treated as 1") {
			t.Fatalf("legacy plugin warnings: %v", warns)
		}
		rows := map[string]int{}
		loaded := map[string]bool{}
		for _, r := range c.view().Rows {
			rows[r.ID], loaded[r.ID] = r.API, r.Loaded
		}
		if rows["extapi"] != 1 || rows["extapi_legacy"] != 1 || rows["extapi_newer"] != 2 || loaded["extapi_newer"] || !loaded["extapi"] {
			t.Fatalf("Plugins rows: api %v loaded %v", rows, loaded)
		}
	})

	t.Run("AC2_capabilities", func(t *testing.T) {
		t.Parallel()
		c := extContent("extapi", "extapi_badcap", "extapi_legacy", "extapi_nocap")
		if slices.Contains(c.catalog.IDs(), "extapi_badcap") {
			t.Fatal("a plugin that declares an unknown capability loaded")
		}
		if errs, _ := problemsOf(c, "extapi_badcap"); !hasSub(errs, "teleport") {
			t.Fatalf("the unknown capability is not named: %v", errs)
		}
		caps := map[string][]string{}
		legacy := map[string]bool{}
		for _, r := range c.view().Rows {
			caps[r.ID], legacy[r.ID] = r.Caps, r.LegacyCaps
		}
		if !legacy["extapi_legacy"] || legacy["extapi"] || !slices.Equal(caps["extapi_legacy"], []string{"query", "commands", "events"}) {
			t.Fatalf("legacy default: caps %v legacy %v", caps["extapi_legacy"], legacy)
		}
		for _, want := range []string{"world", "defs", "validate", "ui", "incidents", "stats", "health", "recipes", "worldgen"} {
			if !slices.Contains(caps["extapi"], want) {
				t.Fatalf("extapi's Plugins row lacks %q: %v", want, caps["extapi"])
			}
		}
		// a def that needs an undeclared capability is refused at load
		if errs, _ := problemsOf(c, "extapi_nocap"); !hasSub(errs, "stats") {
			t.Fatalf("an undeclared stats capability is no error: %v", errs)
		}
		for _, d := range c.catalog.StatMods.Defs {
			if strings.Contains(d.Key, "undeclared") {
				t.Fatalf("a stat_mod from a plugin without stats is in the book: %+v", d)
			}
		}
		// undeclared hooks and calls at run time: warned and never called, or the file turned off
		e := newExt(t, c, 3, false).start()
		e.step(3*clock.TicksPerHour, 600)
		if e.count("nocap event delivered") != 0 || e.count("nocap animals") != 0 || e.count("nocap panel") != 0 {
			t.Fatalf("an undeclared hook or call ran: %v", e.texts())
		}
		warned, off := map[string]bool{}, map[string]bool{}
		for _, p := range e.m.Late() {
			if p.Plugin == "extapi_nocap" {
				if p.Warn {
					warned[p.File] = true
				} else {
					off[p.File] = true
				}
			}
		}
		if !warned["scripts/a_event.star"] || !warned["scripts/c_ui.star"] || !off["scripts/b_world.star"] || len(off) != 1 {
			t.Fatalf("undeclared event kind and hook want warnings, the undeclared ctx.animals an error: warned %v, turned off %v: %+v", warned, off, e.m.Late())
		}
	})

	t.Run("AC3_world_queries", func(t *testing.T) {
		t.Parallel()
		e := newExt(t, extContent("extapi"), 5, true).start()
		g := e.g
		built := false
		for x := 8; x < 32 && !built; x++ {
			for y := 8; y < 24 && !built; y++ {
				built = g.BuildNow("workbench", "wood", "south", x, y)
			}
		}
		if !built {
			t.Fatal("no bench")
		}
		g.Enqueue(command.Command{Kind: command.SetResearch, Build: uint8(research.Electricity)})
		g.Enqueue(command.Command{Kind: command.DevFinishResearch})
		e.step(2, 1) // research_done is the event the fixture reads the world on
		bs := appgame.BuildingCounts(g)
		an, total := 0, 0
		for _, w := range g.Wildlife() {
			an++
			total += w.Count
		}
		want := fmt.Sprintf("q %dx%d %s %s %d an=%d/%d bld=%d fac=%d sites=%d", g.Width(), g.Height(), g.Weather().Kind.String(),
			g.TerrainAt(3, 3), g.TemperatureAt(3, 3), an, total, len(bs), len(g.Factions()), len(g.WorldSites()))
		if len(bs) == 0 || e.index(want) < 0 {
			t.Fatalf("the script read\n%v\nwant a line %q with a building row", e.texts(), want)
		}
	})

	t.Run("AC4_register_defs", func(t *testing.T) {
		t.Parallel()
		c := extContent("extapi")
		if _, ok := c.catalog.Incidents.ByName("extapi.dark_noon"); !ok {
			t.Fatalf("a script-registered incident is not in the catalog: %v", c.problems)
		}
		if !slices.ContainsFunc(c.catalog.StatMods.Defs, func(d statmod.Def) bool { return d.Key == "extapi.hasty" }) {
			t.Fatalf("a script-registered stat_mod is not in the book")
		}
		g := newExt(t, c, 3, false).g
		var found bool
		for _, p := range g.PluginIncidents() {
			found = found || p.Key == "extapi.dark_noon" && p.Like == "core.eclipse" || p.Key == "extapi.dark_noon" && p.Like != ""
		}
		if !found {
			t.Fatalf("the game's plugin incidents: %+v", g.PluginIncidents())
		}
		if src := c.game().SourceOf("incident", "extapi.dark_noon"); src.AddedBy != "extapi" {
			t.Fatalf("the def's source: %+v", src)
		}
	})

	t.Run("AC5_events", func(t *testing.T) {
		t.Parallel()
		e := newExt(t, extContent("extapi", "extapi_legacy"), 5, true).start()
		e.step(1, 1)
		if e.index("ev map_generated 5 40x30") < 0 || e.count("legacy saw") != 0 {
			t.Fatalf("map_generated, and no event a plugin did not subscribe to: %v", e.texts())
		}
		e.g.Enqueue(command.Command{Kind: command.SetResearch, Build: uint8(research.Electricity)})
		e.g.Enqueue(command.Command{Kind: command.DevFinishResearch})
		e.step(2, 1) // the handler's messages post on the tick after
		rd, lg := e.index("ev research_done"), e.index("legacy saw research_done")
		if rd < 0 || !strings.Contains(e.log[rd].text, "electricity") || lg < 0 || lg < rd {
			t.Fatalf("research_done (plugins in load order): %v", e.texts())
		}
		if e.count("ev map_generated") != 1 || e.count("ev research_done") != 1 {
			t.Fatalf("an event repeated: %v", e.texts())
		}
	})

	// the quadrum and year events come once a calendar quadrum and year in:
	// a year of an empty colony (no pawns, to keep it cheap)
	t.Run("AC5_quadrum_year", func(t *testing.T) {
		t.Parallel()
		e := newExt(t, extContent("extapi"), 5, false).start()
		e.step(4*15*clock.TicksPerDay+2, clock.TicksPerDay)
		if e.count("ev quadrum") != 4 || e.count("ev year") != 1 {
			t.Fatalf("one year in: %d quadrums, %d years: %v", e.count("ev quadrum"), e.count("ev year"), e.texts())
		}
		if e.index("ev year") < e.index("ev quadrum") || e.log[e.index("ev year")-1].text != "ev quadrum" || e.log[e.index("ev year")].tick != e.log[e.index("ev year")-1].tick {
			t.Fatalf("the quadrum event comes before the year event of the same tick: %v", e.texts())
		}
	})

	t.Run("AC6_validate_defs", func(t *testing.T) {
		t.Parallel()
		c := extContent("extapi")
		if _, ok := c.catalog.Incidents.ByName("extapi.bad_noon"); ok {
			t.Fatal("a plugin def with a validate_defs error is in the catalog")
		}
		if _, ok := c.catalog.Incidents.ByName("extapi.dark_noon"); !ok {
			t.Fatal("a valid plugin def was dropped")
		}
		var badErr, coreWarn bool
		for _, p := range c.problems {
			badErr = badErr || p.Def == "incident/extapi.bad_noon" && !p.Warn
			coreWarn = coreWarn || p.Def == "incident/core.eclipse" && p.Warn && strings.Contains(p.Msg, "eclipses are disliked")
		}
		if !badErr || !coreWarn {
			t.Fatalf("bad_noon error %v, core.eclipse warning %v: %v", badErr, coreWarn, c.problems)
		}
		if _, ok := c.catalog.Incidents.ByName("core.eclipse"); !ok {
			t.Fatal("a core def was dropped by a plugin's validator")
		}
	})

	t.Run("AC6_validate_command", func(t *testing.T) {
		t.Parallel()
		drafted := func(g *appgame.Game, id entity.EntityID) bool {
			for _, p := range g.Snapshot() {
				if p.ID == id {
					return p.Drafted
				}
			}
			t.Fatalf("no pawn %d", id)
			return false
		}
		plain := newExt(t, extContent(), 5, true).start()
		id := first(t, plain.g)
		plain.g.Enqueue(command.Command{Kind: command.DraftPawn, Pawn: uint64(id), Enabled: true})
		plain.step(1, 1)
		if !drafted(plain.g, id) {
			t.Fatal("control: a draft with no plugin must apply")
		}
		e := newExt(t, extContent("extapi"), 5, true).start()
		id = first(t, e.g)
		e.g.Enqueue(command.Command{Kind: command.DraftPawn, Pawn: uint64(id), Enabled: true})
		e.step(1, 1)
		if drafted(e.g, id) || e.index("Plugin extapi: no drafting in this colony") < 0 {
			t.Fatalf("vetoed draft: drafted %v, messages %v", drafted(e.g, id), e.texts())
		}
		// outside the closed set the validator is never asked
		e.g.Enqueue(command.Command{Kind: command.SetResearch, Build: uint8(research.Electricity)})
		e.step(1, 1)
		if e.count("Plugin extapi:") != 1 {
			t.Fatalf("a command outside the closed set was vetoed: %v", e.texts())
		}
	})

	t.Run("AC7_panels_and_actions", func(t *testing.T) {
		t.Parallel()
		e := newExt(t, extContent("extapi"), 5, true).start()
		e.step(1, 1)
		ps := e.g.World().PluginPanels
		if len(ps) != 1 || ps[0].Plugin != "extapi" || ps[0].Title != "Extapi panel" || len(ps[0].Lines) != 2 ||
			ps[0].Lines[1] != "colonists 3" || len(ps[0].Actions) != 1 || ps[0].Actions[0].ID != "ping" || ps[0].Actions[0].Label != "Ping" {
			t.Fatalf("panels: %+v", ps)
		}
		e.g.Enqueue(command.Command{Kind: command.ScriptAction, Name: "extapi\nnope"})
		e.g.Enqueue(command.Command{Kind: command.ScriptAction, Name: "other\nping"})
		e.step(1, 1)
		if e.count("acted") != 0 {
			t.Fatalf("a stale or foreign action ran: %v", e.texts())
		}
		e.g.Enqueue(command.Command{Kind: command.ScriptAction, Name: "extapi\nping"})
		e.step(2, 1)
		if e.index("acted ping") < 0 {
			t.Fatalf("on_action did not run: %v", e.texts())
		}
		if len(newExt(t, extContent(), 5, true).start().g.World().PluginPanels) != 0 {
			t.Fatal("panels with no plugin")
		}
	})

	t.Run("AC8_run_incident", func(t *testing.T) {
		t.Parallel()
		fire := func(key string) *extRun {
			e := newExt(t, extContent("extapi"), 5, true).start()
			e.step(1, 1)
			e.g.Step(12 * clock.TicksPerHour) // the sky event needs daylight
			if !e.g.FireIncident(key) {
				t.Fatalf("%s did not fire", key)
			}
			e.step(1, 1)
			return e
		}
		replaced := fire("extapi.dark_noon")
		if replaced.index("replaced extapi.dark_noon") < 0 || replaced.g.World().SkyEvent != "" {
			t.Fatalf("a replaced incident: sky %q, messages %v", replaced.g.World().SkyEvent, replaced.texts())
		}
		kept := fire("extapi.dim_noon")
		if kept.count("replaced") != 0 || kept.g.World().SkyEvent == "" {
			t.Fatalf("an unreplaced incident runs its like: sky %q, messages %v", kept.g.World().SkyEvent, kept.texts())
		}
	})

	t.Run("AC9_script_orders", func(t *testing.T) {
		t.Parallel()
		e := newExt(t, extContent("extapi_jobs"), 1, false)
		g := e.g
		ada, err := g.SpawnNamedPawn(1, 1, "Ada")
		if err != nil {
			t.Fatal(err)
		}
		g.Enqueue(command.Command{Kind: command.DesignateStockpile, X: 16, Y: 1, X2: 22, Y2: 10})
		g.Step(1)
		wood, err := g.SpawnStack(item.Wood, 20, 4, 6)
		if err != nil {
			t.Fatal(err)
		}
		g.Enqueue(command.Command{Kind: command.SetForbidden, Target: uint64(wood), Enabled: true}) // only an order hauls it
		g.Step(1)
		at := func() int {
			for _, it := range g.World().Items {
				if it.ID == wood {
					return it.X
				}
			}
			return -1
		}
		if at() != 4 {
			t.Fatalf("fixture: the stack is at x %d", at())
		}
		e.start()
		for range 24 {
			g.SetNeeds(ada, needs.Full, needs.Full)
			e.step(100, 100)
		}
		if x := at(); x < 16 || x > 22 {
			t.Fatalf("the scripted haul order did not move the stack to the stockpile (x %d): %v", x, e.texts())
		}
		var ticks []uint64
		for _, s := range e.log {
			if strings.HasPrefix(s.text, "ordered haul") {
				ticks = append(ticks, s.tick)
			}
		}
		if len(ticks) == 0 {
			t.Fatalf("no order was accepted: %v", e.texts())
		}
		for i := 1; i < len(ticks); i++ {
			if ticks[i]-ticks[i-1] < 600 {
				t.Fatalf("orders accepted %d ticks apart, want at least 600: %v", ticks[i]-ticks[i-1], ticks)
			}
		}
	})

	t.Run("AC10_stat_mods", func(t *testing.T) {
		t.Parallel()
		base := newExt(t, extContent(), 5, true)
		mod := newExt(t, extContent("extapi"), 5, true)
		id := first(t, base.g)
		if id != first(t, mod.g) {
			t.Fatal("fixture: the same seed makes different colonists")
		}
		card := func(g *appgame.Game) stat.Sheet {
			c, ok := g.Dossier(id)
			if !ok {
				t.Fatal("no card")
			}
			return c.Stats
		}
		b, m := card(base.g), card(mod.g)
		if m[stat.WorkSpeed] != b[stat.WorkSpeed]/2 {
			t.Fatalf("work_speed x0.5 (registered stat_mod): base %d, mod %d", b[stat.WorkSpeed], m[stat.WorkSpeed])
		}
		if m[stat.MoveSpeed] >= b[stat.MoveSpeed] {
			t.Fatalf("move_speed x0.8 (JSON stat_mod): base %d, mod %d", b[stat.MoveSpeed], m[stat.MoveSpeed])
		}
		// the same sheet after a save and a load
		var buf bytes.Buffer
		if err := mod.g.Save(&buf, "ext"); err != nil {
			t.Fatal(err)
		}
		h, _, err := appgame.LoadContent(&buf, mod.c.game())
		if err != nil {
			t.Fatal(err)
		}
		if got := card(h); got != m {
			t.Fatalf("stat sheet changed over save and load: %v then %v", m, got)
		}
		mod.g.Step(600)
		h.Step(600)
		if card(h) != card(mod.g) {
			t.Fatalf("a loaded game and a continued one differ: %v %v", card(h), card(mod.g))
		}
	})

	t.Run("AC11_health_condition", func(t *testing.T) {
		t.Parallel()
		c := extContent("extapi", "extapi_nocap")
		var hurt bool
		for _, d := range c.catalog.StatMods.Defs {
			hurt = hurt || d.Key == "extapi.hurt" && d.When.Injured
		}
		if !hurt {
			t.Fatalf("the injured stat_mod is not in the book: %v", c.problems)
		}
		// no injury, no effect: a healthy colonist's mood sheet is the base game's
		base, mod := newExt(t, extContent(), 5, true), newExt(t, extContent("extapi"), 5, true)
		id := first(t, base.g)
		b, _ := base.g.Dossier(id)
		m, _ := mod.g.Dossier(id)
		if m.Stats[stat.MoodOffset] != b.Stats[stat.MoodOffset] {
			t.Fatalf("an injured-only mod applies to a healthy colonist: %d vs %d", m.Stats[stat.MoodOffset], b.Stats[stat.MoodOffset])
		}
		// the same plugin without the health capability cannot condition on health
		weak := extContent("extapi_nocap")
		if errs, _ := problemsOf(weak, "extapi_nocap"); len(errs) == 0 {
			t.Fatal("fixture: nocap has no problem")
		}
	})

	t.Run("AC12_recipe_done", func(t *testing.T) {
		t.Parallel()
		craft := func(ids ...string) (*extRun, int) {
			e := newExt(t, extContent(ids...), 1, false)
			g := e.g
			ada, err := g.SpawnNamedPawn(1, 1, "Ada")
			if err != nil {
				t.Fatal(err)
			}
			if !g.BuildNow("workbench", "wood", "south", 5, 2) {
				t.Fatal("no bench")
			}
			g.Enqueue(command.Command{Kind: command.AddBill, X: 5, Y: 2, Recipe: uint8(recipe.Club), Mode: uint8(recipe.DoCount), Count: 1})
			g.Step(1)
			if _, err := g.SpawnStack(item.Wood, 10, 6, 5); err != nil {
				t.Fatal(err)
			}
			e.start()
			for i := 0; i < 8000 && g.StockOf("club") == 0; i++ {
				if i%100 == 0 {
					g.SetNeeds(ada, needs.Full, needs.Full)
				}
				g.Step(1)
				if i%20 == 0 {
					e.collect()
				}
			}
			g.Step(5)
			e.collect()
			if g.StockOf("club") != 1 {
				t.Fatalf("no club made (ids %v)", ids)
			}
			return e, g.StockOf("wood")
		}
		_, plainWood := craft()
		e, extWood := craft("extapi")
		if i := e.index("ev recipe_done"); i < 0 || !strings.Contains(e.log[i].text, "club") {
			t.Fatalf("recipe_done: %v", e.texts())
		}
		if extWood-plainWood != 3 {
			t.Fatalf("the handler's bonus: wood %d with the plugin, %d without, want +3", extWood, plainWood)
		}
	})

	t.Run("AC13_map_generated", func(t *testing.T) {
		t.Parallel()
		e := newExt(t, extContent("extapi"), 5, true)
		before := e.g.StockOf("wood")
		e.start()
		e.step(1, 1)
		if i := e.index("ev map_generated 5 40x30"); i < 0 || !strings.HasSuffix(e.log[i].text, e.g.World().BiomeKey) {
			t.Fatalf("map_generated: biome %q: %v", e.g.World().BiomeKey, e.texts())
		}
		if got := e.g.StockOf("wood") - before; got < 5 {
			t.Fatalf("the handler placed %d wood, want at least 5 (five stacks of 1 to 3)", got)
		}
		// once: more hours and a second start fire nothing more
		e.m.start(e.g)
		e.step(2*clock.TicksPerHour, 600)
		if e.count("ev map_generated") != 1 {
			t.Fatalf("map_generated fired %d times", e.count("ev map_generated"))
		}
		// a loaded save is not a new game
		var buf bytes.Buffer
		if err := e.g.Save(&buf, "ext"); err != nil {
			t.Fatal(err)
		}
		h, _, err := appgame.LoadContent(&buf, e.c.game())
		if err != nil {
			t.Fatal(err)
		}
		l := &extRun{t: t, c: e.c, m: e.m, g: h, seen: map[said]bool{}}
		l.collect() // the saved ticker still holds the first game's notices
		l.log = nil
		l.start().step(1, 1)
		if l.count("ev map_generated") != 0 {
			t.Fatalf("a loaded game fired map_generated: %v", l.texts())
		}
	})

	t.Run("Deterministic", func(t *testing.T) {
		t.Parallel()
		run := func(seed uint64, ids ...string) (uint64, []string) {
			e := newExt(t, extContent(ids...), seed, true).start()
			e.step(2*clock.TicksPerHour, 600)
			return e.g.StateHash(), e.texts()
		}
		h1, t1 := run(7, "extapi", "extapi_legacy")
		h2, t2 := run(7, "extapi", "extapi_legacy")
		if h1 != h2 || !slices.Equal(t1, t2) {
			t.Fatalf("same seed, same plugins: hash %x vs %x, messages %v vs %v", h1, h2, t1, t2)
		}
		if h3, _ := run(7); h3 == h1 {
			t.Fatal("the plugins changed nothing")
		}
		if h4, _ := run(8, "extapi", "extapi_legacy"); h4 == h1 {
			t.Fatal("a different seed gave the same state")
		}
	})

	t.Run("LegacyPlugin", func(t *testing.T) {
		t.Parallel()
		c := extContent("extapi_legacy")
		if !slices.Contains(c.catalog.IDs(), "extapi_legacy") {
			t.Fatalf("a legacy plugin must still load: %v", c.problems)
		}
		if errs, _ := problemsOf(c, "extapi_legacy"); len(errs) != 0 {
			t.Fatalf("a legacy plugin has errors: %v", errs)
		}
		e := newExt(t, c, 5, true).start()
		e.step(clock.TicksPerDay+2, clock.TicksPerHour)
		e.g.Enqueue(command.Command{Kind: command.SetResearch, Build: uint8(research.Electricity)})
		e.g.Enqueue(command.Command{Kind: command.DevFinishResearch})
		e.step(2, 1) // the handler's messages post on the tick after
		if e.count("legacy day") != 1 || e.index("legacy saw research_done") < 0 {
			t.Fatalf("a legacy plugin's on_day and its events subscription: %v (late %+v)", e.texts(), e.m.Late())
		}
		for _, p := range e.m.Late() {
			if !p.Warn {
				t.Fatalf("a legacy plugin was turned off: %+v", p)
			}
		}
	})
}
