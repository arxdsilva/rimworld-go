// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"rimworld-go/internal/adapters/outbound/starlarkhost"
	"rimworld-go/internal/application/scripting"
)

// starExt is the run's Starlark segments as the game's extension points
// (events, behaviour, panels), in load order. The Wasm segment has none.
// A call goes to every segment; the answers are merged: commands and
// disabled files concatenate, the first replaced incident or denied
// command wins. Only the simulation owner calls it.
type starExt struct {
	hosts []*starlarkhost.Host
	after func() // after a call that may have turned a file off (refreshes Late's copies)
}

var _ scripting.Extensions = (*starExt)(nil)

func merge(dst *scripting.Result, r scripting.Result) {
	dst.Commands = append(dst.Commands, r.Commands...)
	dst.Disabled = append(dst.Disabled, r.Disabled...)
}

func (x *starExt) done() {
	if x.after != nil {
		x.after()
	}
}

// Subscribed reports that a segment takes events of kind k. It does not allocate.
func (x *starExt) Subscribed(k scripting.EventKind) bool {
	for _, h := range x.hosts {
		if h.Subscribed(k) {
			return true
		}
	}
	return false
}

// Event hands ev to every segment that takes its kind.
func (x *starExt) Event(ev scripting.Event, w scripting.World, p scripting.PawnReader) scripting.Result {
	var res scripting.Result
	for _, h := range x.hosts {
		if h.Subscribed(ev.Kind) {
			merge(&res, h.Event(ev, w, p))
		}
	}
	x.done()
	return res
}

// RunsIncidents reports that a segment replaces incidents.
func (x *starExt) RunsIncidents() bool {
	for _, h := range x.hosts {
		if h.RunsIncidents() {
			return true
		}
	}
	return false
}

// RunIncident asks each segment in order; the first to replace the
// incident wins.
func (x *starExt) RunIncident(ev scripting.Event, w scripting.World, p scripting.PawnReader) (scripting.Result, bool) {
	var res scripting.Result
	for _, h := range x.hosts {
		if !h.RunsIncidents() {
			continue
		}
		r, ok := h.RunIncident(ev, w, p)
		merge(&res, r)
		if ok {
			x.done()
			return res, true
		}
	}
	x.done()
	return res, false
}

// Validates reports that a segment vets commands.
func (x *starExt) Validates() bool {
	for _, h := range x.hosts {
		if h.Validates() {
			return true
		}
	}
	return false
}

// ValidateCommand asks each segment in order; the first denial wins.
func (x *starExt) ValidateCommand(c scripting.CommandView, ev scripting.Event, w scripting.World, p scripting.PawnReader) (plugin, reason string, res scripting.Result) {
	for _, h := range x.hosts {
		if !h.Validates() {
			continue
		}
		pl, why, r := h.ValidateCommand(c, ev, w, p)
		merge(&res, r)
		if pl != "" || why != "" {
			x.done()
			return pl, why, res
		}
	}
	x.done()
	return "", "", res
}

// HasPanels reports that a segment has panels.
func (x *starExt) HasPanels() bool {
	for _, h := range x.hosts {
		if h.HasPanels() {
			return true
		}
	}
	return false
}

// Panels collects every segment's panels, in load order.
func (x *starExt) Panels(ev scripting.Event, w scripting.World, p scripting.PawnReader) ([]scripting.Panel, scripting.Result) {
	var (
		out []scripting.Panel
		res scripting.Result
	)
	for _, h := range x.hosts {
		if !h.HasPanels() {
			continue
		}
		ps, r := h.Panels(ev, w, p)
		out = append(out, ps...)
		merge(&res, r)
	}
	x.done()
	return out, res
}

// Act hands a panel action to every segment (the host takes only its own
// plugin's).
func (x *starExt) Act(ev scripting.Event, w scripting.World, p scripting.PawnReader) scripting.Result {
	var res scripting.Result
	for _, h := range x.hosts {
		merge(&res, h.Act(ev, w, p))
	}
	x.done()
	return res
}
