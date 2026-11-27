// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package starlarkhost

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"go.starlark.net/starlark"

	"rimworld-go/internal/application/pluginapi"
	"rimworld-go/internal/application/scripting"
)

// attrRule is what one ctx attribute needs to be called.
type attrRule struct {
	name string
	// cap is the capability it needs; unused when byEvent is set.
	cap pluginapi.Capability
	// writes marks an attribute that queues a command: denied in the
	// readOnlyHooks.
	writes bool
	// hooks are the only hooks it may be called in; nil means any.
	hooks []string
	// byEvent, when set, is the capability it needs per on_event kind;
	// in any other kind it is denied.
	byEvent map[scripting.EventKind]pluginapi.Capability
}

// hookCaps is the capability each hook function needs: a module whose
// plugin lacks it never has that hook called (W1-host-core checks it).
var hookCaps = map[string]pluginapi.Capability{
	"on_hour":          pluginapi.Events,
	"on_day":           pluginapi.Events,
	"on_incident":      pluginapi.Events,
	"on_colonist_died": pluginapi.Events,
	"on_pawns":         pluginapi.Events,
	"on_pawn":          pluginapi.Events,
	"on_event":         pluginapi.Events,
	"register_defs":    pluginapi.Defs,
	"validate_defs":    pluginapi.Validate,
	"validate_command": pluginapi.Validate,
	"panel":            pluginapi.UI,
	"on_action":        pluginapi.UI,
	"run_incident":     pluginapi.Incidents,
}

// attrCaps is every ctx attribute's rule; a name not here is denied.
var attrCaps = []attrRule{
	{name: "tick", cap: pluginapi.Query},
	{name: "day", cap: pluginapi.Query},
	{name: "hour", cap: pluginapi.Query},
	{name: "season", cap: pluginapi.Query},
	{name: "colonists", cap: pluginapi.Query},
	{name: "wealth", cap: pluginapi.Query},
	{name: "count", cap: pluginapi.Query},
	{name: "rand", cap: pluginapi.Query},
	{name: "pawns", cap: pluginapi.Query},
	{name: "pawn", cap: pluginapi.Query},
	{name: "message", cap: pluginapi.Commands, writes: true},
	{name: "letter", cap: pluginapi.Commands, writes: true},
	{name: "spawn", cap: pluginapi.Commands, writes: true},
	{name: "animals", cap: pluginapi.World},
	{name: "buildings", cap: pluginapi.World},
	{name: "map", cap: pluginapi.World},
	{name: "world", cap: pluginapi.World},
	{name: "order", cap: pluginapi.Jobs, writes: true, hooks: []string{"on_pawns", "on_pawn"}},
	{name: "place", writes: true, hooks: []string{"on_event"}, byEvent: map[scripting.EventKind]pluginapi.Capability{
		scripting.RecipeDone: pluginapi.Recipes, scripting.MapGenerated: pluginapi.Worldgen,
	}},
}

// readOnlyHooks are the hooks whose ctx may not queue commands.
var readOnlyHooks = []string{"panel", "validate_command", "validate_defs", "register_defs"}

// ruleOf is name's rule; ok is false for an unmapped name.
func ruleOf(name string) (attrRule, bool) {
	for _, r := range attrCaps {
		if r.name == name {
			return r, true
		}
	}
	return attrRule{}, false
}

// allowed is the one check before any ctx attribute is served: an
// unmapped name is no such attribute; a mapped one is denied, in this
// order, as a write in a read-only hook, outside its hooks, in an event
// kind it has no capability for, or without its capability. A denial is
// an error that turns the file off.
func (c *call) allowed(name string) error {
	r, ok := ruleOf(name)
	if !ok {
		return starlark.NoSuchAttrError(fmt.Sprintf("ctx has no .%s field or method", name))
	}
	switch {
	case r.writes && slices.Contains(readOnlyHooks, c.hook):
		return fmt.Errorf("ctx.%s is read-only in %s", name, c.hook)
	case len(r.hooks) > 0 && !slices.Contains(r.hooks, c.hook):
		return fmt.Errorf("ctx.%s is only for %s", name, strings.Join(r.hooks, " and "))
	}
	need := r.cap
	if r.byEvent != nil {
		var ok bool
		if need, ok = r.byEvent[c.ev.Kind]; !ok {
			return fmt.Errorf("ctx.%s is not available in %s for %s", name, c.hook, c.ev.Kind)
		}
	}
	if !c.caps.Has(need) {
		return fmt.Errorf("ctx.%s needs capability %s", name, need)
	}
	return nil
}

// gate takes out of m the hooks its plugin may not have, each with a
// warning, so no caller can call one; then reads its EVENTS. Hooks go in
// name order, so the warnings do too.
func (h *Host) gate(m *module) {
	for _, name := range slices.Sorted(maps.Keys(hookCaps)) {
		fn, ok := m.globals[name].(starlark.Callable)
		if !ok || m.caps.Has(hookCaps[name]) {
			continue
		}
		delete(m.globals, name)
		h.warn(m, lineOf(fn), fmt.Sprintf("defines %s but plugin.json does not declare %s; it is never called", name, hookCaps[name]))
	}
	h.subscribe(m)
}

// lineOf is the line fn is defined on, 0 for a builtin.
func lineOf(fn starlark.Callable) int {
	if f, ok := fn.(*starlark.Function); ok {
		return int(f.Position().Line)
	}
	return 0
}
