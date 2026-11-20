// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"path/filepath"
	"strings"
	"testing"

	uiadapter "rimworld-go/internal/adapters/inbound/ebiten"
	"rimworld-go/internal/adapters/inbound/ebiten/ports"
	"rimworld-go/internal/adapters/outbound/modloader"
	"rimworld-go/internal/adapters/outbound/starlarkhost"
	appgame "rimworld-go/internal/application/game"
	"rimworld-go/internal/application/scripting"
	"rimworld-go/internal/domain/sim/clock"
)

// a script that draws from ctx.rand and spawns what it drew, every hour
const randSpawner = `
def on_hour(ctx):
    n = 1 + ctx.rand(9)
    ctx.spawn("core.wood", n)
    ctx.message("dropped %d wood at %d:00" % (n, ctx.hour))
`

// runColony plays a start-kit colony for hours with the scripts given
// (a fresh host each run) and returns its state hash and wood stock.
func runColony(t *testing.T, seed uint64, hours int, scripts ...starlarkhost.File) (uint64, int) {
	t.Helper()
	var host scripting.Host
	if len(scripts) > 0 {
		h := starlarkhost.New([]starlarkhost.Plugin{{ID: "dice", Files: scripts}})
		if p := h.Problems(); len(p) > 0 {
			t.Fatalf("problems: %+v", p)
		}
		host = h
	}
	g := appgame.New(appgame.Config{Seed: seed, Width: 32, Height: 32, Scripts: host})
	if err := g.LoadStartKit(); err != nil {
		t.Fatal(err)
	}
	g.Step(hours * clock.TicksPerHour)
	return g.StateHash(), g.StockOf("wood")
}

// Same seed and same plugins give the same game, scripts and ctx.rand
// included; the scripts really ran (more wood than without them).
func TestScriptedColonyIsDeterministic(t *testing.T) {
	t.Parallel()
	script := starlarkhost.File{Path: "scripts/dice.star", Src: []byte(randSpawner)}
	h1, wood1 := runColony(t, 11, 3, script)
	h2, wood2 := runColony(t, 11, 3, script)
	if h1 != h2 || wood1 != wood2 {
		t.Fatalf("two runs differ: hash %x/%x, wood %d/%d", h1, h2, wood1, wood2)
	}
	plain, woodPlain := runColony(t, 11, 3)
	if plain == h1 || wood1 <= woodPlain {
		t.Fatalf("the script did not change the colony: wood %d scripted, %d plain", wood1, woodPlain)
	}
}

func TestScriptHostFromLoadedPlugins(t *testing.T) {
	t.Parallel()
	src := modloader.New(filepath.Join("..", "..", "..", "mods"))
	on := buildContent(src, []string{"example_frontier"}, "0.1.35")
	if scriptHost(on, true) != nil {
		t.Fatal("a capture runs no script")
	}
	if scriptHost(buildContent(src, nil, "0.1.35"), false) != nil {
		t.Fatal("no loaded plugin with scripts: no host")
	}
	h := scriptHost(on, false)
	if h == nil || len(h.Problems()) != 0 {
		t.Fatalf("the example's scripts: %v", h)
	}
	if ps := scriptPlugins(on); len(ps) != 1 || ps[0].ID != "example_frontier" || len(ps[0].Files) == 0 {
		t.Fatalf("plugins: %+v", ps)
	}
	day5 := h.Fire(scripting.Event{Hook: scripting.OnDay, Tick: 5 * clock.TicksPerDay, Seed: 1}, fixedWorld{day: 5}, nil)
	day4 := h.Fire(scripting.Event{Hook: scripting.OnDay, Tick: 4 * clock.TicksPerDay, Seed: 1}, fixedWorld{day: 4}, nil)
	raid := h.Fire(scripting.Event{Hook: scripting.OnIncident, Tick: 9, Incident: scripting.Incident{Key: "core.rival_raid", Category: "threat"}}, fixedWorld{}, nil)
	herd := h.Fire(scripting.Event{Hook: scripting.OnIncident, Tick: 9, Incident: scripting.Incident{Key: "core.herd_crossing", Category: "nature"}}, fixedWorld{}, nil)
	if len(day5.Commands) != 1 || !strings.Contains(day5.Commands[0].Name, "Frontier wisdom") || len(day4.Commands) != 0 ||
		len(raid.Commands) != 1 || len(herd.Commands) != 0 || len(h.Problems()) != 0 {
		t.Fatalf("example: day 5 %+v, day 4 %+v, raid %+v, herd %+v, problems %+v", day5, day4, raid, herd, h.Problems())
	}
}

func TestWithScriptsWiresStartsLoadsAndProblems(t *testing.T) {
	t.Parallel()
	if cfg := withScripts(uiadapter.AppConfig{}, nil); cfg.Plugins.Late != nil {
		t.Fatal("no host: nothing wired")
	}
	h := starlarkhost.New([]starlarkhost.Plugin{{ID: "p", Files: []starlarkhost.File{{Path: "scripts/x.star", Src: []byte("x = 1 // 0\n")}}}})
	g := appgame.New(appgame.Config{Seed: 1, Width: 8, Height: 8})
	cfg := withScripts(uiadapter.AppConfig{Saves: uiadapter.SaveHooks{Load: func(string) (uiadapter.Simulation, ports.SaveMeta, error) {
		return g, ports.SaveMeta{}, nil
	}}}, h)
	if _, _, err := cfg.Saves.Load("slot"); err != nil {
		t.Fatal(err)
	}
	g.Step(1) // a game that has the host fires on_hour on its first tick and reports the broken script
	found := false
	for _, m := range g.World().Messages {
		found = found || strings.Contains(m.Text, "scripts/x.star:1")
	}
	if !found {
		t.Fatalf("a loaded game must run the scripts: %+v", g.World().Messages)
	}
	v := cfg.Plugins.Live()
	if len(v.Problems) != 1 || v.Problems[0].Plugin != "p" || v.Problems[0].File != "scripts/x.star" || v.Problems[0].Field != "line 1" {
		t.Fatalf("Plugins problems: %+v", v.Problems)
	}
}

type fixedWorld struct{ day int }

func (w fixedWorld) Day() int                         { return w.day }
func (w fixedWorld) Hour() int                        { return 0 }
func (w fixedWorld) Season() string                   { return "spring" }
func (w fixedWorld) Colonists() int                   { return 3 }
func (w fixedWorld) Wealth() int                      { return 100 }
func (w fixedWorld) ItemCount(key string) (int, bool) { return 0, key == "core.wood" }

type stubBuilder struct{ g *appgame.Game }

func (b stubBuilder) Step(int) (string, float32, bool)   { return "", 1, true }
func (b stubBuilder) Sim() (uiadapter.Simulation, error) { return b.g, nil }

// A wizard colony (the path players take) gets the host too.
func TestWithScriptsGivesWizardColoniesTheHost(t *testing.T) {
	t.Parallel()
	h := starlarkhost.New([]starlarkhost.Plugin{{ID: "p", Files: []starlarkhost.File{{Path: "scripts/x.star",
		Src: []byte("def on_hour(ctx):\n    ctx.message(\"hello from p\")\n")}}}})
	g := appgame.New(appgame.Config{Seed: 1, Width: 8, Height: 8})
	cfg := withScripts(uiadapter.AppConfig{NewSimBuilder: func(uiadapter.NewGame) (uiadapter.SimBuilder, error) {
		return stubBuilder{g}, nil
	}}, h)
	b, err := cfg.NewSimBuilder(uiadapter.NewGame{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := b.Sim(); err != nil {
		t.Fatal(err)
	}
	g.Step(2) // on_hour at the first tick, its message applied on the next
	found := false
	for _, m := range g.World().Messages {
		found = found || m.Text == "hello from p"
	}
	if !found {
		t.Fatalf("the built colony must run the scripts: %+v", g.World().Messages)
	}
}
