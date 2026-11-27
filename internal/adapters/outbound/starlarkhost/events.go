// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package starlarkhost

import (
	"fmt"

	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"

	"rimworld-go/internal/application/pluginapi"
	"rimworld-go/internal/application/scripting"
)

// kindCaps is the capability an event kind needs beyond on_event's
// events; a kind not here needs events only.
var kindCaps = map[scripting.EventKind]pluginapi.Capability{
	scripting.RecipeDone:   pluginapi.Recipes,
	scripting.MapGenerated: pluginapi.Worldgen,
}

// maxEventWarns is the EVENTS entries one file is warned about one by
// one; the rest are counted in one more warning.
const maxEventWarns = 4

// kindBit is k's bit in an event mask; 0 for a kind outside it.
func kindBit(k scripting.EventKind) uint32 {
	if k >= 32 {
		return 0
	}
	return 1 << k
}

// Subscribed reports that a live module lists k in EVENTS and its plugin
// may take it: one precomputed mask, no allocation.
func (h *Host) Subscribed(k scripting.EventKind) bool { return k != 0 && h.subs&kindBit(k) != 0 }

// Event runs every subscribed live module's on_event(ctx, event) for
// ev.Kind, plugins in load order and files in path order. event is a
// frozen struct: kind, then the kind's own fields.
func (h *Host) Event(ev scripting.Event, w scripting.World, p scripting.PawnReader) scripting.Result {
	res := h.firstResult()
	if !h.Subscribed(ev.Kind) {
		return res
	}
	arg := eventValue(ev)
	for _, m := range h.mods {
		fn, ok := m.globals["on_event"].(starlark.Callable)
		if m.off || !ok || m.events&kindBit(ev.Kind) == 0 {
			continue
		}
		c := h.callFor(m, "on_event", ev, w, p)
		_, err := h.run(m, fn, starlark.Tuple{&ctxValue{c: c}, arg})
		h.collect(&res, m, c, err)
	}
	return res
}

// subscribe reads m's EVENTS once: a list or tuple of event names. A
// name that is no event, a kind the plugin may not take or a value of
// another type is a warning and is ignored.
func (h *Host) subscribe(m *module) {
	v, ok := m.globals["EVENTS"]
	if !ok {
		return
	}
	if _, ok := m.globals["on_event"].(starlark.Callable); !ok {
		if !m.caps.Has(hookCaps["on_event"]) {
			return // gate took on_event out, with its warning
		}
		h.warn(m, 0, "EVENTS is set but on_event is not defined; no event is delivered")
		return
	}
	seq, ok := v.(starlark.Indexable)
	if _, str := v.(starlark.String); !ok || str {
		h.warn(m, 0, fmt.Sprintf("EVENTS is %s, not a list of event names; ignored", v.Type()))
		return
	}
	warned := 0
	for i := range seq.Len() {
		msg := ""
		name, ok := starlark.AsString(seq.Index(i))
		k, known := scripting.ParseEventKind(name)
		need, gated := kindCaps[k]
		switch {
		case !ok:
			msg = fmt.Sprintf("EVENTS[%d] is %s, not an event name; ignored", i, seq.Index(i).Type())
		case !known:
			msg = fmt.Sprintf("EVENTS lists %q, which is no event; ignored", name)
		case gated && !m.caps.Has(need):
			msg = fmt.Sprintf("EVENTS lists %s but plugin.json does not declare %s; it is never delivered", k, need)
		default:
			m.events |= kindBit(k)
			continue
		}
		if warned++; warned <= maxEventWarns {
			h.warn(m, 0, msg)
		}
	}
	if warned > maxEventWarns {
		h.warn(m, 0, fmt.Sprintf("EVENTS has %d more entries ignored", warned-maxEventWarns))
	}
}

// eventValue is on_event's event argument.
func eventValue(ev scripting.Event) starlark.Value {
	d := starlark.StringDict{"kind": starlark.String(ev.Kind.String())}
	switch ev.Kind {
	case scripting.ResearchDone:
		d["research"] = starlark.String(ev.Research)
	case scripting.RecipeDone:
		r := ev.Recipe
		d["key"], d["product"], d["count"] = starlark.String(r.Key), starlark.String(r.Product), starlark.MakeInt(r.Count)
		d["worker"], d["x"], d["y"] = starlark.MakeUint64(r.Worker), starlark.MakeInt(r.X), starlark.MakeInt(r.Y)
	case scripting.MapGenerated:
		g := ev.Map
		d["seed"], d["width"], d["height"] = starlark.MakeUint64(g.Seed), starlark.MakeInt(g.W), starlark.MakeInt(g.H)
		d["biome"] = starlark.String(g.Biome)
	}
	return frozenStruct("event", d)
}

// frozenStruct is a struct named name with d's fields, frozen.
func frozenStruct(name string, d starlark.StringDict) starlark.Value {
	s := starlarkstruct.FromStringDict(starlark.String(name), d)
	s.Freeze()
	return s
}
