// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package starlarkhost

import (
	"fmt"

	"go.starlark.net/starlark"

	"rimworld-go/internal/application/pluginapi"
	"rimworld-go/internal/application/scripting"
)

// uiHook is m's function name when m is live, its plugin has ui and it
// defines name.
func uiHook(m *module, name string) (starlark.Callable, bool) {
	if m.off || !m.caps.Has(pluginapi.UI) {
		return nil, false
	}
	fn, ok := m.globals[name].(starlark.Callable)
	return fn, ok
}

// HasPanels reports that a live module defines panel and its plugin has ui.
func (h *Host) HasPanels() bool {
	for _, m := range h.mods {
		if _, ok := uiHook(m, "panel"); ok {
			return true
		}
	}
	return false
}

// Panels runs every ui module's panel(ctx), plugins in load order, at most
// one panel per module. A module whose panel fails or breaks a bound is
// disabled; the rest still answer.
func (h *Host) Panels(ev scripting.Event, w scripting.World, p scripting.PawnReader) ([]scripting.Panel, scripting.Result) {
	res := h.firstResult()
	var out []scripting.Panel
	for _, m := range h.mods {
		fn, ok := uiHook(m, "panel")
		if !ok {
			continue
		}
		c := h.callFor(m, "panel", ev, w, p)
		v, err := h.callUI(c, fn, starlark.Tuple{&ctxValue{c: c}})
		var panel scripting.Panel
		if err == nil && v != starlark.None {
			panel, err = parsePanel(m.plugin, v)
		}
		if err != nil {
			res.Disabled = append(res.Disabled, h.disable(m, err))
			continue
		}
		if v != starlark.None {
			out = append(out, panel)
		}
	}
	return out, res
}

// Act runs on_action(ctx, ev.Action) of the ui modules of ev.Plugin, and
// of no other plugin. Its commands join the result; a module that fails is
// disabled.
func (h *Host) Act(ev scripting.Event, w scripting.World, p scripting.PawnReader) scripting.Result {
	res := h.firstResult()
	for _, m := range h.mods {
		if m.plugin != ev.Plugin {
			continue
		}
		fn, ok := uiHook(m, "on_action")
		if !ok {
			continue
		}
		c := h.callFor(m, "on_action", ev, w, p)
		if _, err := h.callUI(c, fn, starlark.Tuple{&ctxValue{c: c}, starlark.String(ev.Action)}); err != nil {
			res.Disabled = append(res.Disabled, h.disable(m, err))
			continue
		}
		res.Commands = append(res.Commands, c.cmds...)
	}
	return res
}

// callUI calls fn on a fresh thread with the step limit.
func (h *Host) callUI(c *call, fn starlark.Callable, args starlark.Tuple) (starlark.Value, error) {
	th := &starlark.Thread{Name: c.hook, Print: func(*starlark.Thread, string) {}}
	th.SetMaxExecutionSteps(h.stepCap)
	return starlark.Call(th, fn, args, nil)
}

// parsePanel reads {title, lines, actions} within the bounds: title and
// lines are checked text, at most MaxPanelLines lines and MaxActions
// actions, each {id, label} with a unique id of 1-32 of a-z, 0-9 and _.
func parsePanel(plugin string, v starlark.Value) (scripting.Panel, error) {
	d, ok := v.(*starlark.Dict)
	if !ok {
		return scripting.Panel{}, fmt.Errorf("panel: want a dict or None, got %s", v.Type())
	}
	f, err := dictFields(d, "panel", "title", "lines", "actions")
	if err != nil {
		return scripting.Panel{}, err
	}
	pn := scripting.Panel{Plugin: plugin}
	if pn.Title, err = textOf(f["title"], "panel title", true); err != nil {
		return scripting.Panel{}, err
	}
	if pn.Lines, err = panelLines(f["lines"]); err != nil {
		return scripting.Panel{}, err
	}
	if pn.Actions, err = panelActions(f["actions"]); err != nil {
		return scripting.Panel{}, err
	}
	return pn, nil
}

// dictFields maps d's string keys, which must all be in allowed.
func dictFields(d *starlark.Dict, what string, allowed ...string) (map[string]starlark.Value, error) {
	out := map[string]starlark.Value{}
	for _, it := range d.Items() {
		k, ok := it[0].(starlark.String)
		if !ok || !isOneOf(string(k), allowed) {
			return nil, fmt.Errorf("%s: unknown key %s", what, it[0])
		}
		out[string(k)] = it[1]
	}
	return out, nil
}

func isOneOf(s string, set []string) bool {
	for _, x := range set {
		if x == s {
			return true
		}
	}
	return false
}

// textOf is v as a checked text; an absent v is an error when required,
// else empty.
func textOf(v starlark.Value, what string, required bool) (string, error) {
	if v == nil {
		if required {
			return "", fmt.Errorf("%s: missing", what)
		}
		return "", nil
	}
	s, ok := starlark.AsString(v)
	if !ok {
		return "", fmt.Errorf("%s: want a string, got %s", what, v.Type())
	}
	return s, checkText(what, s)
}

// seqOf is v's elements when it is a list or tuple; none when absent.
func seqOf(v starlark.Value, what string, limit int) ([]starlark.Value, error) {
	if v == nil {
		return nil, nil
	}
	var xs []starlark.Value
	switch s := v.(type) {
	case *starlark.List:
		for i := 0; i < s.Len(); i++ {
			xs = append(xs, s.Index(i))
		}
	case starlark.Tuple:
		xs = s
	default:
		return nil, fmt.Errorf("%s: want a list, got %s", what, v.Type())
	}
	if len(xs) > limit {
		return nil, fmt.Errorf("%s: more than %d entries", what, limit)
	}
	return xs, nil
}

func panelLines(v starlark.Value) ([]string, error) {
	xs, err := seqOf(v, "panel lines", scripting.MaxPanelLines)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, x := range xs {
		s, err := textOf(x, "panel line", true)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, nil
}

func panelActions(v starlark.Value) ([]scripting.PanelAction, error) {
	xs, err := seqOf(v, "panel actions", scripting.MaxActions)
	if err != nil {
		return nil, err
	}
	var out []scripting.PanelAction
	seen := map[string]bool{}
	for _, x := range xs {
		d, ok := x.(*starlark.Dict)
		if !ok {
			return nil, fmt.Errorf("panel action: want a dict, got %s", x.Type())
		}
		f, err := dictFields(d, "panel action", "id", "label")
		if err != nil {
			return nil, err
		}
		id, err := textOf(f["id"], "panel action id", true)
		if err != nil {
			return nil, err
		}
		if !validActionID(id) {
			return nil, fmt.Errorf("panel action id %q: want 1-32 of a-z, 0-9 and _", id)
		}
		if seen[id] {
			return nil, fmt.Errorf("panel action id %q: twice on one panel", id)
		}
		seen[id] = true
		label, err := textOf(f["label"], "panel action label", true)
		if err != nil {
			return nil, err
		}
		out = append(out, scripting.PanelAction{ID: id, Label: label})
	}
	return out, nil
}

// validActionID matches ^[a-z0-9_]{1,32}$.
func validActionID(id string) bool {
	if len(id) < 1 || len(id) > 32 {
		return false
	}
	for _, r := range id {
		if (r < 'a' || r > 'z') && (r < '0' || r > '9') && r != '_' {
			return false
		}
	}
	return true
}
