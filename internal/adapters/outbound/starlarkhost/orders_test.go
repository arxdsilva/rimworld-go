// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package starlarkhost

import (
	"fmt"
	"strings"
	"testing"

	"go.starlark.net/starlark"

	"rimworld-go/internal/application/pluginapi"
	"rimworld-go/internal/application/scripting"
	"rimworld-go/internal/domain/sim/command"
)

var jobsSet = uiSet(pluginapi.Events, pluginapi.Query, pluginapi.Commands, pluginapi.Jobs)

// orderTick is a tick on every pawn's order visit.
const orderTick = scripting.PawnEvery * scripting.OrderEvery * 3

func orderCmds(r scripting.Result) []command.Command {
	var out []command.Command
	for _, c := range r.Commands {
		if c.Kind == command.ScriptOrder {
			out = append(out, c)
		}
	}
	return out
}

// onEvent runs plugin's first module's on_event for ev directly, through
// the same call the host builds; it returns the queued commands and the
// error.
func onEvent(t *testing.T, p Plugin, ev scripting.Event) ([]command.Command, error) {
	t.Helper()
	h := New([]Plugin{p})
	if len(h.mods) != 1 || h.mods[0].off {
		t.Fatalf("module did not load: %+v", h.Problems())
	}
	m := h.mods[0]
	c := h.callFor(m, "on_event", ev, &fakeWorld{}, nil)
	th := &starlark.Thread{Name: "t"}
	_, err := starlark.Call(th, m.globals["on_event"], starlark.Tuple{&ctxValue{c: c}}, nil)
	return c.cmds, err
}

func TestOrderRules(t *testing.T) {
	t.Parallel()
	pawns := []scripting.Pawn{{ID: 3}, {ID: 9}}
	fireOn := func(hook scripting.Hook, caps pluginapi.Set, call string, tick uint64) scripting.Result {
		fn := "def on_pawns(ctx, ps):\n    "
		if hook == scripting.OnPawn {
			fn = "def on_pawn(ctx, p):\n    "
		}
		h := New([]Plugin{uiPlugin("p", caps, fn+call+"\n")})
		return fireWith(h, hook, tick, nil, pawns)
	}

	t.Run("queues on the order visit", func(t *testing.T) {
		t.Parallel()
		res := fireOn(scripting.OnPawns, jobsSet, `ctx.message("r=%s" % ctx.order(9, "haul", 4, 5))`, orderTick)
		cs := orderCmds(res)
		want := command.Command{Kind: command.ScriptOrder, Pawn: 9, Order: command.OrderHaul, X: 4, Y: 5, Tick: orderTick}
		if len(cs) != 1 || cs[0] != want {
			t.Fatalf("orders %+v, want [%+v]", cs, want)
		}
		if got := msgs(res); len(got) != 1 || got[0] != "r=True" {
			t.Errorf("returned %v", got)
		}
	})
	t.Run("on_pawn too", func(t *testing.T) {
		t.Parallel()
		res := fireOn(scripting.OnPawn, jobsSet, `ctx.order(p.id, "mine", 1, 2)`, orderTick)
		cs := orderCmds(res)
		if len(cs) != 2 || cs[0].Pawn != 3 || cs[1].Pawn != 9 || cs[0].Order != command.OrderMine {
			t.Fatalf("orders %+v", cs)
		}
	})
	t.Run("off the order visit is a no-op returning False", func(t *testing.T) {
		t.Parallel()
		res := fireOn(scripting.OnPawns, jobsSet, `ctx.message("r=%s" % ctx.order(9, "haul", 4, 5))`, orderTick+scripting.PawnEvery)
		if len(orderCmds(res)) != 0 || len(res.Disabled) != 0 {
			t.Fatalf("result %+v", res)
		}
		if got := msgs(res); len(got) != 1 || got[0] != "r=False" {
			t.Errorf("returned %v", got)
		}
	})
	t.Run("one order per pawn per visit", func(t *testing.T) {
		t.Parallel()
		res := fireOn(scripting.OnPawns, jobsSet, `ctx.order(9, "haul", 4, 5)
    ctx.message("again=%s" % ctx.order(9, "mine", 4, 5))
    ctx.order(3, "mine", 1, 1)`, orderTick)
		if cs := orderCmds(res); len(cs) != 2 || cs[0].Pawn != 9 || cs[1].Pawn != 3 {
			t.Fatalf("orders %+v", cs)
		}
		if got := msgs(res); got[0] != "again=False" {
			t.Errorf("returned %v", got)
		}
	})

	errs := []struct{ name, call, want string }{
		{"unknown order", `ctx.order(9, "teleport", 1, 1)`, "unknown order"},
		{"empty order", `ctx.order(9, "", 1, 1)`, "unknown order"},
		{"pawn outside the due set", `ctx.order(77, "haul", 1, 1)`, "not due"},
		{"unknown order off visit", `ctx.order(9, "teleport", 1, 1)`, "unknown order"},
		{"pawn outside off visit", `ctx.order(77, "haul", 1, 1)`, "not due"},
		{"bad arity", `ctx.order(9, "haul")`, "order:"},
		{"bad type", `ctx.order("9", "haul", 1, 1)`, "order:"},
	}
	for _, tc := range errs {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			tick := uint64(orderTick)
			if strings.HasSuffix(tc.name, "off visit") {
				tick += scripting.PawnEvery
			}
			res := fireOn(scripting.OnPawns, jobsSet, tc.call, tick)
			if len(res.Disabled) != 1 || !strings.Contains(res.Disabled[0].Msg, tc.want) || len(orderCmds(res)) != 0 {
				t.Errorf("result %+v, want it disabled with %q", res, tc.want)
			}
		})
	}

	t.Run("not in a cadence hook", func(t *testing.T) {
		t.Parallel()
		h := New([]Plugin{uiPlugin("p", jobsSet, "def on_hour(ctx):\n    ctx.order(3, 'haul', 1, 1)\n")})
		res := h.Fire(scripting.Event{Hook: scripting.OnHour, Tick: orderTick, Pawns: pawns}, &fakeWorld{}, nil)
		if len(res.Disabled) != 1 || len(orderCmds(res)) != 0 {
			t.Errorf("on_hour order: %+v", res)
		}
	})
	t.Run("without jobs", func(t *testing.T) {
		t.Parallel()
		res := fireOn(scripting.OnPawns, uiSet(pluginapi.Events, pluginapi.Query), `ctx.order(9, "haul", 1, 1)`, orderTick)
		if len(res.Disabled) != 1 || len(orderCmds(res)) != 0 {
			t.Errorf("order without jobs: %+v", res)
		}
	})
	t.Run("not in dir(ctx) without jobs", func(t *testing.T) {
		t.Parallel()
		c := &call{hook: "on_pawns", caps: uiSet(pluginapi.Query)}
		if v, ok := c.orderAttr("order"); ok || v != nil {
			t.Errorf("orderAttr(order) = %v, %v", v, ok)
		}
		if _, ok := c.orderAttr("message"); ok {
			t.Error("orderAttr serves message")
		}
		if _, ok := c.orderAttr("nope"); ok {
			t.Error("orderAttr serves nope")
		}
	})
}

func TestOrderNamesAreClosed(t *testing.T) {
	t.Parallel()
	for name, o := range orderNames {
		if o == command.OrderNone {
			t.Errorf("order %q maps to OrderNone", name)
		}
	}
}

func TestPlaceRules(t *testing.T) {
	t.Parallel()
	place := "def on_event(ctx):\n    ctx.place('core.wood', 7, 3, 4)\n"
	recipe := scripting.Event{Kind: scripting.RecipeDone, Tick: 5}
	mapgen := scripting.Event{Kind: scripting.MapGenerated, Tick: 0}

	t.Run("recipe_done with recipes", func(t *testing.T) {
		t.Parallel()
		cs, err := onEvent(t, uiPlugin("p", uiSet(pluginapi.Events, pluginapi.Recipes), place), recipe)
		want := command.Command{Kind: command.ScriptPlace, Name: "core.wood", Count: 7, X: 3, Y: 4}
		if err != nil || len(cs) != 1 || cs[0] != want {
			t.Fatalf("cmds %+v err %v, want [%+v]", cs, err, want)
		}
	})
	t.Run("map_generated with worldgen", func(t *testing.T) {
		t.Parallel()
		cs, err := onEvent(t, uiPlugin("p", uiSet(pluginapi.Events, pluginapi.Worldgen), place), mapgen)
		if err != nil || len(cs) != 1 || cs[0].Kind != command.ScriptPlace {
			t.Fatalf("cmds %+v err %v", cs, err)
		}
	})
	denied := []struct {
		name string
		caps pluginapi.Set
		ev   scripting.Event
	}{
		{"recipe_done without recipes", uiSet(pluginapi.Events, pluginapi.Worldgen), recipe},
		{"map_generated without worldgen", uiSet(pluginapi.Events, pluginapi.Recipes), mapgen},
		{"quadrum", uiSet(pluginapi.Events, pluginapi.Recipes, pluginapi.Worldgen), scripting.Event{Kind: scripting.Quadrum}},
		{"research_done", uiSet(pluginapi.Events, pluginapi.Recipes, pluginapi.Worldgen), scripting.Event{Kind: scripting.ResearchDone}},
		{"no kind", uiSet(pluginapi.Events, pluginapi.Recipes, pluginapi.Worldgen), scripting.Event{}},
	}
	for _, tc := range denied {
		t.Run("denied "+tc.name, func(t *testing.T) {
			t.Parallel()
			cs, err := onEvent(t, uiPlugin("p", tc.caps, place), tc.ev)
			if err == nil || len(cs) != 0 {
				t.Errorf("cmds %+v err %v, want an error", cs, err)
			}
		})
	}
	t.Run("not in another hook", func(t *testing.T) {
		t.Parallel()
		h := New([]Plugin{uiPlugin("p", uiSet(pluginapi.Events, pluginapi.Recipes, pluginapi.Worldgen), "def on_hour(ctx):\n    ctx.place('core.wood', 1, 1, 1)\n")})
		res := h.Fire(scripting.Event{Hook: scripting.OnHour, Kind: scripting.RecipeDone}, &fakeWorld{}, nil)
		if len(res.Disabled) != 1 || len(res.Commands) != 0 {
			t.Errorf("on_hour place: %+v", res)
		}
	})

	args := []struct{ name, call, want string }{
		{"count zero", "ctx.place('core.wood', 0, 1, 1)", "count must be 1 to 500"},
		{"count over", "ctx.place('core.wood', 501, 1, 1)", "count must be 1 to 500"},
		{"unknown key", "ctx.place('core.nothing', 1, 1, 1)", "unknown item"},
		{"arity", "ctx.place('core.wood', 1)", "place:"},
	}
	caps := uiSet(pluginapi.Events, pluginapi.Recipes)
	for _, tc := range args {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			cs, err := onEvent(t, uiPlugin("p", caps, "def on_event(ctx):\n    "+tc.call+"\n"), recipe)
			if err == nil || !strings.Contains(err.Error(), tc.want) || len(cs) != 0 {
				t.Errorf("cmds %+v err %v, want %q", cs, err, tc.want)
			}
		})
	}
	t.Run("count edges", func(t *testing.T) {
		t.Parallel()
		cs, err := onEvent(t, uiPlugin("p", caps, "def on_event(ctx):\n    ctx.place('core.wood', 1, 0, 0)\n    ctx.place('core.wood', 500, -1, 2)\n"), recipe)
		if err != nil || len(cs) != 2 || cs[1].Count != 500 || cs[1].X != -1 {
			t.Errorf("cmds %+v err %v", cs, err)
		}
	})
}

func TestOrderPlaceBudget(t *testing.T) {
	t.Parallel()
	t.Run("orders count", func(t *testing.T) {
		t.Parallel()
		src := fmt.Sprintf("def on_pawns(ctx, ps):\n    for i in range(%d):\n        ctx.message('m')\n    ctx.order(3, 'haul', 1, 1)\n", scripting.MaxCommands)
		h := New([]Plugin{uiPlugin("p", jobsSet, src)})
		res := fireWith(h, scripting.OnPawns, orderTick, nil, []scripting.Pawn{{ID: 3}})
		if len(res.Disabled) != 1 || !strings.Contains(res.Disabled[0].Msg, "more than 16 commands") {
			t.Errorf("result %+v", res)
		}
	})
	t.Run("off-visit orders are free", func(t *testing.T) {
		t.Parallel()
		src := fmt.Sprintf("def on_pawns(ctx, ps):\n    for i in range(%d):\n        ctx.message('m')\n    ctx.order(3, 'haul', 1, 1)\n", scripting.MaxCommands)
		h := New([]Plugin{uiPlugin("p", jobsSet, src)})
		res := fireWith(h, scripting.OnPawns, orderTick+scripting.PawnEvery, nil, []scripting.Pawn{{ID: 3}})
		if len(res.Disabled) != 0 || len(res.Commands) != scripting.MaxCommands {
			t.Errorf("result %+v", res)
		}
	})
	t.Run("places count", func(t *testing.T) {
		t.Parallel()
		caps := uiSet(pluginapi.Events, pluginapi.Recipes)
		src := "def on_event(ctx):\n    for i in range(17):\n        ctx.place('core.wood', 1, 1, 1)\n"
		cs, err := onEvent(t, uiPlugin("p", caps, src), scripting.Event{Kind: scripting.RecipeDone})
		if err == nil || !strings.Contains(err.Error(), "more than 16 commands") || len(cs) != scripting.MaxCommands {
			t.Errorf("cmds %d err %v", len(cs), err)
		}
	})
}

// A script that orders at every visit for 6000 ticks gets one order per
// OrderEvery visits (600 ticks), whatever it orders and however it treats
// the answer.
func TestOrderCooldown(t *testing.T) {
	t.Parallel()
	const ticks = 6000
	run := func(src string) int {
		h := New([]Plugin{uiPlugin("p", jobsSet, src)})
		n := 0
		for tick := uint64(0); tick < ticks; tick += scripting.PawnEvery {
			res := fireWith(h, scripting.OnPawn, tick, nil, []scripting.Pawn{{ID: 3}})
			if len(res.Disabled) != 0 {
				t.Fatalf("tick %d disabled %+v", tick, res.Disabled)
			}
			n += len(orderCmds(res))
		}
		return n
	}
	const want = ticks / (scripting.PawnEvery * scripting.OrderEvery)
	t.Run("alternating haul and mine", func(t *testing.T) {
		t.Parallel()
		got := run(`def on_pawn(ctx, p):
    if (ctx.tick // 60) % 2 == 0:
        ctx.order(p.id, "haul", 1, 1)
    else:
        ctx.order(p.id, "mine", 1, 1)
`)
		if got > 10 || got != want {
			t.Errorf("queued %d orders, want at most 10 (%d)", got, want)
		}
	})
	t.Run("retried at every visit, twice per visit", func(t *testing.T) {
		t.Parallel()
		got := run(`def on_pawn(ctx, p):
    ctx.order(p.id, "haul", 99, 99)
    ctx.order(p.id, "haul", 99, 99)
    ctx.order(p.id, "mine", 99, 99)
`)
		if got > 10 {
			t.Errorf("queued %d orders, want at most 10", got)
		}
	})
}
