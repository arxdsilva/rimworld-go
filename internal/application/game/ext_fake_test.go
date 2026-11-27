// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"slices"

	"rimworld-go/internal/application/scripting"
	"rimworld-go/internal/domain/sim/command"
)

// The fake is a whole plugin host: the hooks and every extension point.
var (
	_ scripting.Host       = (*fakeExt)(nil)
	_ scripting.Extensions = (*fakeExt)(nil)
)

// fakeExt logs every call in order ("on_day", "event:quadrum", "panels",
// ...), answers each from its fields, and hands the readers of each call
// to probe. res is every call's result; orders, when set, is the cadence
// hooks' commands instead.
type fakeExt struct {
	log       []string
	events    []scripting.Event // Event calls
	incidents []scripting.Event // RunIncident calls
	acts      []scripting.Event // Act calls
	views     []scripting.CommandView
	subs      map[scripting.EventKind]bool
	uses      []scripting.Hook
	res       scripting.Result
	orders    func(ev scripting.Event) []command.Command
	runs      bool   // RunsIncidents
	replace   bool   // RunIncident's answer
	validates bool   // Validates
	veto      string // ValidateCommand's reason
	panels    []scripting.Panel
	probe     func(call string, w scripting.World, p scripting.PawnReader)
}

// allEvents subscribes to every event.
func allEvents() map[scripting.EventKind]bool {
	m := map[scripting.EventKind]bool{}
	for _, k := range scripting.EventKinds() {
		m[k] = true
	}
	return m
}

func (f *fakeExt) note(call string, w scripting.World, p scripting.PawnReader) {
	f.log = append(f.log, call)
	if f.probe != nil {
		f.probe(call, w, p)
	}
}

func (f *fakeExt) Fire(ev scripting.Event, w scripting.World, p scripting.PawnReader) scripting.Result {
	f.note(ev.Hook.String(), w, p)
	if f.orders != nil && (ev.Hook == scripting.OnPawns || ev.Hook == scripting.OnPawn) {
		return scripting.Result{Commands: f.orders(ev)}
	}
	return f.res
}

func (f *fakeExt) Uses(h scripting.Hook) bool            { return slices.Contains(f.uses, h) }
func (f *fakeExt) Subscribed(k scripting.EventKind) bool { return f.subs[k] }

func (f *fakeExt) Event(ev scripting.Event, w scripting.World, p scripting.PawnReader) scripting.Result {
	f.events = append(f.events, ev)
	f.note("event:"+ev.Kind.String(), w, p)
	return f.res
}

func (f *fakeExt) RunsIncidents() bool { return f.runs }

func (f *fakeExt) RunIncident(ev scripting.Event, w scripting.World, p scripting.PawnReader) (scripting.Result, bool) {
	f.incidents = append(f.incidents, ev)
	f.note("run_incident", w, p)
	return f.res, f.replace
}

func (f *fakeExt) Validates() bool { return f.validates }

func (f *fakeExt) ValidateCommand(c scripting.CommandView, _ scripting.Event, w scripting.World, p scripting.PawnReader) (string, string, scripting.Result) {
	f.views = append(f.views, c)
	f.note("validate", w, p)
	return "vetter", f.veto, f.res
}

func (f *fakeExt) HasPanels() bool { return f.panels != nil }

func (f *fakeExt) Panels(_ scripting.Event, w scripting.World, p scripting.PawnReader) ([]scripting.Panel, scripting.Result) {
	f.note("panels", w, p)
	return f.panels, f.res
}

func (f *fakeExt) Act(ev scripting.Event, w scripting.World, p scripting.PawnReader) scripting.Result {
	f.acts = append(f.acts, ev)
	f.note("act", w, p)
	return f.res
}

// logFrom is f's log from index i on.
func (f *fakeExt) logFrom(i int) []string { return slices.Clone(f.log[i:]) }
