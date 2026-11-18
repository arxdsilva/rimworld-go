// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package starlarkhost

import (
	"reflect"
	"strings"
	"testing"

	"rimworld-go/internal/application/scripting"
	"rimworld-go/internal/domain/sim/command"
)

type fakeWorld struct{ asked int }

func (w *fakeWorld) Day() int       { return 4 }
func (w *fakeWorld) Hour() int      { return 6 }
func (w *fakeWorld) Season() string { return "summer" }
func (w *fakeWorld) Colonists() int { return 3 }
func (w *fakeWorld) Wealth() int    { return 1234 }
func (w *fakeWorld) ItemCount(key string) (int, bool) {
	w.asked++
	if key == "core.wood" {
		return 75, true
	}
	return 0, false
}

func plugin(id string, files ...string) Plugin {
	p := Plugin{ID: id}
	for i := 0; i+1 < len(files); i += 2 {
		p.Files = append(p.Files, File{Path: files[i], Src: []byte(files[i+1])})
	}
	return p
}

func fire(h *Host, hook scripting.Hook) scripting.Result {
	return h.Fire(scripting.Event{Hook: hook, Tick: 2500, Seed: 7}, &fakeWorld{})
}

func TestHooksQueueCommandsInLoadOrder(t *testing.T) {
	t.Parallel()
	h := New([]Plugin{
		plugin("b_first", "scripts/a.star", `
def on_hour(ctx):
    ctx.message("hour %d day %d %s %d colonists %d wealth %d wood" % (ctx.hour, ctx.day, ctx.season, ctx.colonists, ctx.wealth, ctx.count("core.wood")))
def on_day(ctx):
    ctx.letter("Dawn", "tick %d" % ctx.tick)
`),
		plugin("a_second", "scripts/z.star", `
def on_hour(ctx):
    ctx.spawn("core.wood", 5)
def on_incident(ctx, incident):
    ctx.message(incident.key + "/" + incident.category)
def on_colonist_died(ctx, name):
    ctx.message("farewell " + name)
`),
	})
	if p := h.Problems(); len(p) != 0 {
		t.Fatalf("problems: %v", p)
	}
	got := fire(h, scripting.OnHour).Commands
	want := []command.Command{
		{Kind: command.ScriptMessage, Name: "hour 6 day 4 summer 3 colonists 1234 wealth 75 wood"},
		{Kind: command.ScriptSpawn, Name: "core.wood", Count: 5},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("on_hour: got %+v, want %+v", got, want)
	}
	if got := fire(h, scripting.OnDay).Commands; len(got) != 1 || got[0].Mode != command.ScriptLetter || got[0].Name != "Dawn\ntick 2500" {
		t.Fatalf("on_day: %+v", got)
	}
	r := h.Fire(scripting.Event{Hook: scripting.OnIncident, Tick: 9, Incident: scripting.Incident{Key: "core.herd_crossing", Category: "nature"}}, &fakeWorld{})
	if len(r.Commands) != 1 || r.Commands[0].Name != "core.herd_crossing/nature" {
		t.Fatalf("on_incident: %+v", r)
	}
	r = h.Fire(scripting.Event{Hook: scripting.OnColonistDied, Tick: 9, Name: "Ada"}, &fakeWorld{})
	if len(r.Commands) != 1 || r.Commands[0].Name != "farewell Ada" {
		t.Fatalf("on_colonist_died: %+v", r)
	}
}

func TestErrorsDisableOnlyTheFailingScript(t *testing.T) {
	t.Parallel()
	h := New([]Plugin{plugin("p",
		"scripts/bad.star", "def on_hour(ctx):\n    x = 1\n    ctx.message(\"before\")\n    return 1 // 0\n",
		"scripts/good.star", "def on_hour(ctx):\n    ctx.message(\"ok\")\n")})
	r := fire(h, scripting.OnHour)
	if len(r.Commands) != 1 || r.Commands[0].Name != "ok" {
		t.Fatalf("a failing call's commands must be dropped, the others kept: %+v", r.Commands)
	}
	if len(r.Disabled) != 1 || r.Disabled[0].File != "scripts/bad.star" || r.Disabled[0].Line != 4 || r.Disabled[0].Plugin != "p" {
		t.Fatalf("disabled: %+v", r.Disabled)
	}
	r = fire(h, scripting.OnHour)
	if len(r.Commands) != 1 || len(r.Disabled) != 0 {
		t.Fatalf("the bad script must stay off and be reported once: %+v", r)
	}
	if p := h.Problems(); len(p) != 1 || !strings.Contains(p[0].Msg, "division by zero") {
		t.Fatalf("problems: %+v", p)
	}
}

func TestFirstRunFailuresAreReportedOnTheFirstFire(t *testing.T) {
	t.Parallel()
	h := New([]Plugin{plugin("p",
		"scripts/syntax.star", "def on_hour(ctx)\n    pass\n",
		"scripts/undefined.star", "def on_hour(ctx):\n    nope()\n",
		"scripts/top.star", "x = [1]\nx.append(1 // 0)\n")})
	probs := h.Problems()
	if len(probs) != 3 {
		t.Fatalf("want 3 problems, got %+v", probs)
	}
	for _, p := range probs {
		if p.Line == 0 || p.File == "" {
			t.Errorf("problem without its place: %+v", p)
		}
	}
	if r := fire(h, scripting.OnHour); len(r.Disabled) != 3 || len(r.Commands) != 0 {
		t.Fatalf("first fire: %+v", r)
	}
	if r := fire(h, scripting.OnHour); len(r.Disabled) != 0 {
		t.Fatalf("reported twice: %+v", r)
	}
}

func TestBudgets(t *testing.T) {
	t.Parallel()
	for name, src := range map[string]string{
		"steps":       "def on_hour(ctx):\n    for i in range(100000000):\n        pass\n",
		"commands":    "def on_hour(ctx):\n    for i in range(17):\n        ctx.message(\"x\")\n",
		"long text":   "def on_hour(ctx):\n    ctx.message(\"x\" * 501)\n",
		"empty text":  "def on_hour(ctx):\n    ctx.message(\"  \")\n",
		"spawn count": "def on_hour(ctx):\n    ctx.spawn(\"core.wood\", 501)\n",
		"spawn key":   "def on_hour(ctx):\n    ctx.spawn(\"core.unobtainium\")\n",
		"count key":   "def on_hour(ctx):\n    ctx.count(\"nope\")\n",
		"rand n":      "def on_hour(ctx):\n    ctx.rand(0)\n",
		"title line":  "def on_hour(ctx):\n    ctx.letter(\"a\\nb\", \"text\")\n",
		"frozen":      "seen = []\ndef on_hour(ctx):\n    seen.append(1)\n",
		"recursion":   "def f(n):\n    return f(n)\ndef on_hour(ctx):\n    f(1)\n",
	} {
		h := New([]Plugin{plugin("p", "scripts/s.star", src)})
		r := fire(h, scripting.OnHour)
		if len(r.Disabled) != 1 || len(r.Commands) != 0 {
			t.Errorf("%s: want the script turned off with no commands, got %+v", name, r)
		}
	}
	h := New([]Plugin{plugin("p", "scripts/s.star", "def on_hour(ctx):\n    for i in range(16):\n        ctx.spawn(\"core.wood\", 500)\n")})
	if r := fire(h, scripting.OnHour); len(r.Commands) != 16 || len(r.Disabled) != 0 {
		t.Fatalf("at the limits a call is fine: %d commands, %+v", len(r.Commands), r.Disabled)
	}
}

func TestFirstRunStepLimit(t *testing.T) {
	t.Parallel()
	h := newHost([]Plugin{plugin("p", "scripts/s.star", "x = [i for i in range(1000)]\n")}, 100)
	if p := h.Problems(); len(p) != 1 || !strings.Contains(p[0].Msg, "too many steps") {
		t.Fatalf("problems: %+v", p)
	}
}

func TestRandIsDeterministic(t *testing.T) {
	t.Parallel()
	src := "def on_hour(ctx):\n    ctx.message(\"%d %d %d\" % (ctx.rand(1000000), ctx.rand(1000000), ctx.rand(6)))\n"
	draw := func(id string, seed, tick uint64, hook scripting.Hook) string {
		h := New([]Plugin{plugin(id, "scripts/s.star", strings.ReplaceAll(src, "on_hour", hook.String()))})
		r := h.Fire(scripting.Event{Hook: hook, Tick: tick, Seed: seed}, &fakeWorld{})
		if len(r.Commands) != 1 {
			t.Fatalf("%+v", r)
		}
		return r.Commands[0].Name
	}
	a := draw("p", 1, 2500, scripting.OnHour)
	if b := draw("p", 1, 2500, scripting.OnHour); a != b {
		t.Fatalf("same key, different draws: %q %q", a, b)
	}
	for name, other := range map[string]string{
		"seed":   draw("p", 2, 2500, scripting.OnHour),
		"tick":   draw("p", 1, 5000, scripting.OnHour),
		"plugin": draw("q", 1, 2500, scripting.OnHour),
		"hook":   draw("p", 1, 2500, scripting.OnDay),
	} {
		if other == a {
			t.Errorf("a different %s drew the same numbers %q", name, a)
		}
	}
	h := New([]Plugin{plugin("p", "scripts/s.star", src)})
	first := fire(h, scripting.OnHour).Commands[0].Name
	if again := fire(h, scripting.OnHour).Commands[0].Name; again != first {
		t.Fatalf("a call's draws must not depend on earlier calls: %q %q", first, again)
	}
}

func TestLoadStaysInsideThePlugin(t *testing.T) {
	t.Parallel()
	h := New([]Plugin{
		plugin("p",
			"scripts/lib/util.star", "def greet(n):\n    return \"hi \" + n\n",
			"scripts/main.star", "load(\"lib/util.star\", \"greet\")\ndef on_colonist_died(ctx, name):\n    ctx.message(greet(name))\n"),
		plugin("q", "scripts/secret.star", "word = \"x\"\n"),
	})
	r := h.Fire(scripting.Event{Hook: scripting.OnColonistDied, Name: "Bo"}, &fakeWorld{})
	if len(r.Commands) != 1 || r.Commands[0].Name != "hi Bo" || len(r.Disabled) != 0 {
		t.Fatalf("load inside the plugin: %+v", r)
	}
	for _, mod := range []string{"../../q/scripts/secret.star", "/etc/passwd", "secret.star", "lib/../main.star", "lib/util.txt", "main.star"} {
		h := New([]Plugin{
			plugin("p", "scripts/main.star", "load(\""+mod+"\", \"word\")\n"),
			plugin("q", "scripts/secret.star", "word = \"x\"\n"),
		})
		if p := h.Problems(); len(p) != 1 || p[0].Plugin != "p" {
			t.Errorf("load(%q) must fail: %+v", mod, p)
		}
	}
}

func TestPrintIsSilentAndNoIO(t *testing.T) {
	t.Parallel()
	for _, src := range []string{
		"def on_hour(ctx):\n    print(\"hello\")\n",
	} {
		h := New([]Plugin{plugin("p", "scripts/s.star", src)})
		if r := fire(h, scripting.OnHour); len(r.Disabled) != 0 {
			t.Fatalf("%+v", r)
		}
	}
	for _, src := range []string{"x = time.now()\n", "x = open(\"f\")\n", "x = getenv(\"HOME\")\n"} {
		if p := New([]Plugin{plugin("p", "scripts/s.star", src)}).Problems(); len(p) != 1 {
			t.Errorf("%q must not resolve: %+v", src, p)
		}
	}
}

func TestItemCountsAreAskedPerCall(t *testing.T) {
	t.Parallel()
	h := New([]Plugin{plugin("p", "scripts/s.star", "def on_hour(ctx):\n    n = ctx.count(\"core.wood\")\n")})
	w := &fakeWorld{}
	h.Fire(scripting.Event{Hook: scripting.OnHour}, w)
	if w.asked != 1 {
		t.Fatalf("asked %d", w.asked)
	}
}
