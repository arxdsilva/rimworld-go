// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package starlarkhost

import (
	"reflect"
	"strings"
	"testing"

	"rimworld-go/internal/application/pluginapi"
	"rimworld-go/internal/application/scripting"
)

// uiSet is a granted set of the named capabilities.
func uiSet(cs ...pluginapi.Capability) pluginapi.Set {
	var s pluginapi.Set
	for _, c := range cs {
		s |= pluginapi.Set(1) << c
	}
	return s
}

// uiPlugin is a plugin that declares caps and has one script.
func uiPlugin(id string, caps pluginapi.Set, src string) Plugin {
	p := plugin(id, "scripts/main.star", src)
	p.Caps, p.Declared = caps, true
	return p
}

func panelsOf(h *Host) ([]scripting.Panel, scripting.Result) {
	return h.Panels(scripting.Event{Tick: 3600, Seed: 7}, &fakeWorld{}, nil)
}

func TestPanelLimits(t *testing.T) {
	t.Parallel()
	lines := func(n int) string { return "[" + strings.Repeat(`"x",`, n) + "]" }
	acts := func(n int) string {
		var b strings.Builder
		b.WriteString("[")
		for i := 0; i < n; i++ {
			b.WriteString(`{"id": "a` + string(rune('a'+i)) + `", "label": "L"},`)
		}
		return b.String() + "]"
	}
	long := strings.Repeat("x", scripting.MaxText+1)
	tests := []struct {
		name, ret, wantErr string
	}{
		{"title and lines", `{"title": "T", "lines": ["a", "b"]}`, ""},
		{"title only", `{"title": "T"}`, ""},
		{"max lines", `{"title": "T", "lines": ` + lines(scripting.MaxPanelLines) + `}`, ""},
		{"too many lines", `{"title": "T", "lines": ` + lines(scripting.MaxPanelLines+1) + `}`, "more than 12"},
		{"max actions", `{"title": "T", "actions": ` + acts(scripting.MaxActions) + `}`, ""},
		{"too many actions", `{"title": "T", "actions": ` + acts(scripting.MaxActions+1) + `}`, "more than 4"},
		{"tuple lines", `{"title": "T", "lines": ("a", "b")}`, ""},
		{"id edge", `{"title": "T", "actions": [{"id": "` + strings.Repeat("a", 32) + `", "label": "L"}]}`, ""},
		{"id too long", `{"title": "T", "actions": [{"id": "` + strings.Repeat("a", 33) + `", "label": "L"}]}`, "want 1-32"},
		{"id upper", `{"title": "T", "actions": [{"id": "Go", "label": "L"}]}`, "want 1-32"},
		{"id dash", `{"title": "T", "actions": [{"id": "a-b", "label": "L"}]}`, "want 1-32"},
		{"id empty", `{"title": "T", "actions": [{"id": "", "label": "L"}]}`, "empty text"},
		{"id twice", `{"title": "T", "actions": [{"id": "go", "label": "A"}, {"id": "go", "label": "B"}]}`, "twice"},
		{"label empty", `{"title": "T", "actions": [{"id": "go", "label": " "}]}`, "empty text"},
		{"label missing", `{"title": "T", "actions": [{"id": "go"}]}`, "missing"},
		{"no title", `{"lines": ["a"]}`, "missing"},
		{"title empty", `{"title": ""}`, "empty text"},
		{"title long", `{"title": "` + long + `"}`, "longer than"},
		{"line long", `{"title": "T", "lines": ["` + long + `"]}`, "longer than"},
		{"line empty", `{"title": "T", "lines": [""]}`, "empty text"},
		{"line not text", `{"title": "T", "lines": [1]}`, "want a string"},
		{"title not text", `{"title": 3}`, "want a string"},
		{"lines not list", `{"title": "T", "lines": "a"}`, "want a list"},
		{"action not dict", `{"title": "T", "actions": ["go"]}`, "want a dict"},
		{"unknown key", `{"title": "T", "extra": 1}`, "unknown key"},
		{"unknown action key", `{"title": "T", "actions": [{"id": "go", "label": "L", "x": 1}]}`, "unknown key"},
		{"non-string key", `{"title": "T", 1: 2}`, "unknown key"},
		{"not a dict", `["T"]`, "want a dict or None"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			h := New([]Plugin{uiPlugin("p", uiSet(pluginapi.UI), "def panel(ctx):\n    return "+tc.ret+"\n")})
			ps, res := panelsOf(h)
			if tc.wantErr == "" {
				if len(res.Disabled) != 0 || len(ps) != 1 || ps[0].Plugin != "p" || ps[0].Title != "T" {
					t.Fatalf("panels %+v, disabled %+v; want one panel of p", ps, res.Disabled)
				}
				return
			}
			if len(ps) != 0 || len(res.Disabled) != 1 || !strings.Contains(res.Disabled[0].Msg, tc.wantErr) {
				t.Fatalf("panels %+v, disabled %+v; want it disabled with %q", ps, res.Disabled, tc.wantErr)
			}
			if h.HasPanels() {
				t.Error("a disabled module still has panels")
			}
		})
	}
}

func TestPanelsValuesAndOrder(t *testing.T) {
	t.Parallel()
	ui := uiSet(pluginapi.UI, pluginapi.Query)
	h := New([]Plugin{
		uiPlugin("a", ui, `def panel(ctx):
    return {"title": "A", "lines": ["day %d" % ctx.day], "actions": [{"id": "go", "label": "Go"}, {"id": "stop_2", "label": "Stop"}]}
`),
		uiPlugin("quiet", ui, "def panel(ctx):\n    return None\n"),
		uiPlugin("noui", uiSet(pluginapi.Query), `def panel(ctx):
    return {"title": "no"}
`),
		uiPlugin("z", ui, `def panel(ctx):
    return {"title": "Z"}
`),
	})
	if !h.HasPanels() {
		t.Fatal("HasPanels = false")
	}
	ps, res := panelsOf(h)
	want := []scripting.Panel{
		{Plugin: "a", Title: "A", Lines: []string{"day 4"}, Actions: []scripting.PanelAction{{ID: "go", Label: "Go"}, {ID: "stop_2", Label: "Stop"}}},
		{Plugin: "z", Title: "Z"},
	}
	if !reflect.DeepEqual(ps, want) || len(res.Disabled) != 0 {
		t.Errorf("panels %+v disabled %+v, want %+v", ps, res.Disabled, want)
	}
}

func TestPanelFailureKeepsTheRest(t *testing.T) {
	t.Parallel()
	ui := uiSet(pluginapi.UI)
	h := New([]Plugin{
		uiPlugin("bad", ui, "def panel(ctx):\n    fail('boom')\n"),
		uiPlugin("good", ui, "def panel(ctx):\n    return {'title': 'G'}\n"),
	})
	ps, res := panelsOf(h)
	if len(ps) != 1 || ps[0].Plugin != "good" || len(res.Disabled) != 1 || res.Disabled[0].Plugin != "bad" {
		t.Errorf("panels %+v disabled %+v", ps, res.Disabled)
	}
}

func TestPanelsAndActReportFirstRunFailures(t *testing.T) {
	t.Parallel()
	mk := func() *Host {
		return New([]Plugin{
			uiPlugin("broken", uiSet(pluginapi.UI), "def panel(ctx):\n"),
			uiPlugin("good", uiSet(pluginapi.UI), "def panel(ctx):\n    return None\n"),
		})
	}
	h := mk()
	_, res := panelsOf(h)
	if len(res.Disabled) != 1 || res.Disabled[0].Plugin != "broken" {
		t.Fatalf("Panels first: %+v", res.Disabled)
	}
	if _, again := panelsOf(h); len(again.Disabled) != 0 {
		t.Errorf("reported twice: %+v", again.Disabled)
	}
	h = mk()
	res = h.Act(scripting.Event{Plugin: "good", Action: "go"}, &fakeWorld{}, nil)
	if len(res.Disabled) != 1 || res.Disabled[0].Plugin != "broken" {
		t.Errorf("Act first: %+v", res.Disabled)
	}
}

func TestPanelIsReadOnly(t *testing.T) {
	t.Parallel()
	// panel's ctx has no commands: a message queued from it queues nothing
	// that reaches the result, whichever way allowed treats the call.
	h := New([]Plugin{uiPlugin("p", uiSet(pluginapi.UI, pluginapi.Commands), `def panel(ctx):
    ctx.message("hi")
    return {"title": "T"}
`)})
	_, res := panelsOf(h)
	if len(res.Commands) != 0 {
		t.Errorf("panel queued %+v", res.Commands)
	}
}

func TestActOwnPlugin(t *testing.T) {
	t.Parallel()
	src := func(tag string) string {
		return `def on_action(ctx, action):
    ctx.message("` + tag + `:" + action)
`
	}
	ui := uiSet(pluginapi.UI, pluginapi.Commands)
	h := New([]Plugin{
		uiPlugin("a", ui, src("a")),
		uiPlugin("b", ui, src("b")),
		uiPlugin("noui", uiSet(pluginapi.Commands), src("noui")),
	})
	act := func(plugin string) scripting.Result {
		return h.Act(scripting.Event{Tick: 1, Plugin: plugin, Action: "go"}, &fakeWorld{}, nil)
	}
	if got := msgs(act("a")); !reflect.DeepEqual(got, []string{"a:go"}) {
		t.Errorf("Act(a) ran %v", got)
	}
	if got := msgs(act("b")); !reflect.DeepEqual(got, []string{"b:go"}) {
		t.Errorf("Act(b) ran %v", got)
	}
	if got := msgs(act("noui")); len(got) != 0 {
		t.Errorf("Act(noui) ran %v without ui", got)
	}
	if res := act("nobody"); len(res.Commands) != 0 || len(res.Disabled) != 0 {
		t.Errorf("Act(nobody) = %+v", res)
	}
}

func TestActFailureDisables(t *testing.T) {
	t.Parallel()
	h := New([]Plugin{uiPlugin("p", uiSet(pluginapi.UI), "def on_action(ctx, action):\n    fail('boom')\n")})
	res := h.Act(scripting.Event{Plugin: "p", Action: "go"}, &fakeWorld{}, nil)
	if len(res.Disabled) != 1 || len(res.Commands) != 0 {
		t.Fatalf("Act = %+v", res)
	}
	if again := h.Act(scripting.Event{Plugin: "p", Action: "go"}, &fakeWorld{}, nil); len(again.Disabled) != 0 {
		t.Errorf("a disabled module ran again: %+v", again)
	}
}

func TestActCommandsAreBudgeted(t *testing.T) {
	t.Parallel()
	h := New([]Plugin{uiPlugin("p", uiSet(pluginapi.UI, pluginapi.Commands), `def on_action(ctx, action):
    for i in range(17):
        ctx.message("m")
`)})
	res := h.Act(scripting.Event{Plugin: "p", Action: "go"}, &fakeWorld{}, nil)
	if len(res.Disabled) != 1 || !strings.Contains(res.Disabled[0].Msg, "more than 16 commands") {
		t.Errorf("Act = %+v", res)
	}
}
