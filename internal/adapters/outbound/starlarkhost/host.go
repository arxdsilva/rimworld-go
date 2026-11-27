// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

// Package starlarkhost runs plugins' Starlark scripts behind
// application/scripting.Host (PLG-060); the hooks, ctx and budgets are
// documented on that package.
//
// Every file of every plugin is compiled and run once in New (its globals
// then frozen, so no state survives between calls); Fire calls each live
// module's hook on a fresh thread with a step limit. A module that fails
// (at its first run or in a hook), runs over its budget or breaks a rule
// is disabled for the rest of the process's run and listed by Problems.
// load() reaches only the same plugin's scripts folder.
package starlarkhost

import (
	"errors"
	"fmt"
	"hash/fnv"
	"path"
	"strings"
	"sync"

	"go.starlark.net/resolve"
	"go.starlark.net/starlark"
	"go.starlark.net/syntax"

	"rimworld-go/internal/application/pluginapi"
	"rimworld-go/internal/application/scripting"
)

var (
	_ scripting.Host       = (*Host)(nil)
	_ scripting.Extensions = (*Host)(nil)
)

// Options are the Starlark dialect scripts are written in: no while
// loops, no recursion, no reassigned globals (a script terminates on its
// own; the step limit is the backstop). contentlint compiles with the
// same options.
func Options() *syntax.FileOptions { return &syntax.FileOptions{Set: true, TopLevelControl: true} }

// Plugin is one loaded plugin's scripts, in load order.
type Plugin struct {
	ID    string
	Files []File // sorted by path
	// Caps are the capabilities its manifest declares (modding.CapsOf),
	// granted only when Declared; the zero Plugin is legacy.
	Caps     pluginapi.Set
	Declared bool
}

// Granted is the capability set p's scripts run with: Caps when
// declared, else the legacy set. The only place the host decides it.
func (p Plugin) Granted() pluginapi.Set {
	if p.Declared {
		return p.Caps
	}
	return pluginapi.Legacy()
}

// File is one script: Path is its slash path inside the plugin
// ("scripts/main.star").
type File struct {
	Path string
	Src  []byte
}

// module is one file after its first run.
type module struct {
	plugin, path string
	caps         pluginapi.Set       // its plugin's Granted set
	globals      starlark.StringDict // without the hooks caps does not grant (gate)
	off          bool
	events       uint32 // the EVENTS kinds it may take, a bit per EventKind
}

// Host runs the scripts. Fire and the extension calls are made by the
// simulation owner only; Problems and Warnings may be read from any
// goroutine.
type Host struct {
	mods    []*module // in load order, then path order
	mu      sync.Mutex
	probs   []scripting.Problem
	warns   []scripting.Problem // New's warnings: hooks and EVENTS ignored
	unsent  []scripting.Problem // first-run failures not yet in a Result
	stepCap uint64
	q       scripting.Queries // the world capability's readers (UseQueries)
	// What the live modules take, recomputed when one is turned off
	// (refresh): the EVENTS kinds, run_incident and validate_command.
	subs                 uint32
	incidents, validates bool
}

// New compiles and runs every plugin's files once. A file that fails is
// a problem and stays off; the rest run.
func New(plugins []Plugin) *Host {
	return newHost(plugins, scripting.MaxSteps)
}

func newHost(plugins []Plugin, steps uint64) *Host {
	h := &Host{stepCap: steps}
	for _, p := range plugins {
		l := &loader{plugin: p, steps: steps, done: map[string]*module{}, errs: map[string]error{}, busy: map[string]bool{}}
		for _, f := range p.Files {
			m, err := l.load(f.Path)
			if err != nil {
				h.unsent = append(h.unsent, h.disable(m, err))
				continue
			}
			h.mods = append(h.mods, m)
		}
	}
	for _, m := range h.mods {
		h.gate(m)
	}
	h.refresh()
	return h
}

// refresh recomputes what the live modules take.
func (h *Host) refresh() {
	h.subs, h.incidents, h.validates = 0, false, false
	for _, m := range h.mods {
		if m.off {
			continue
		}
		h.subs |= m.events
		_, inc := m.globals["run_incident"].(starlark.Callable)
		_, val := m.globals["validate_command"].(starlark.Callable)
		h.incidents, h.validates = h.incidents || inc, h.validates || val
	}
}

// Warnings are New's warnings: the hooks a plugin defines without their
// capability (never called) and the EVENTS entries ignored. They turn
// nothing off.
func (h *Host) Warnings() []scripting.Problem {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]scripting.Problem(nil), h.warns...)
}

// warn adds a warning about m at line (0 when unknown).
func (h *Host) warn(m *module, line int, msg string) {
	h.mu.Lock()
	h.warns = append(h.warns, scripting.Problem{Plugin: m.plugin, File: m.path, Line: line, Msg: msg})
	h.mu.Unlock()
}

// firstResult is an extension call's Result to fill: like Fire, the first
// call of any kind also carries the files that failed their first run.
func (h *Host) firstResult() scripting.Result {
	var res scripting.Result
	res.Disabled, h.unsent = h.unsent, nil
	return res
}

// run calls fn(args) on a fresh thread with the step limit; c's command
// budget is the call's.
func (h *Host) run(m *module, fn starlark.Callable, args starlark.Tuple) (starlark.Value, error) {
	th := &starlark.Thread{Name: m.plugin + "/" + m.path, Print: func(*starlark.Thread, string) {}}
	th.SetMaxExecutionSteps(h.stepCap)
	return starlark.Call(th, fn, args, nil)
}

// collect adds c's outcome to res: its commands, or m turned off by err
// (its commands dropped).
func (h *Host) collect(res *scripting.Result, m *module, c *call, err error) {
	if err != nil {
		res.Disabled = append(res.Disabled, h.disable(m, err))
		return
	}
	res.Commands = append(res.Commands, c.cmds...)
}

// UseQueries sets the readers ctx.animals(), ctx.buildings(), ctx.map
// and ctx.world read (a nil reader reads as empty). The composition root
// calls it once, before the first call.
func (h *Host) UseQueries(q scripting.Queries) { h.q = q }

// Problems are the scripts disabled so far, in the order they failed.
func (h *Host) Problems() []scripting.Problem {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]scripting.Problem(nil), h.probs...)
}

// Fire runs every live module's hook for ev. The first call also returns
// the files that failed their first run, so the game reports them. p
// answers ctx.pawns() and ctx.pawn(id) during this call only; it may be
// nil.
//
// on_pawn runs on one thread per file per tick, so the step limit and the
// command budget are shared by all the pawns of the tick.
func (h *Host) Fire(ev scripting.Event, w scripting.World, p scripting.PawnReader) scripting.Result {
	res := h.firstResult()
	if len(ev.Pawns) == 0 && (ev.Hook == scripting.OnPawns || ev.Hook == scripting.OnPawn) {
		return res // nobody is due: no call
	}
	name := ev.Hook.String()
	for _, m := range h.mods {
		fn, ok := m.globals[name].(starlark.Callable)
		if m.off || !ok {
			continue
		}
		c := h.callFor(m, name, ev, w, p)
		th := &starlark.Thread{Name: m.plugin + "/" + m.path, Print: func(*starlark.Thread, string) {}}
		th.SetMaxExecutionSteps(h.stepCap)
		var err error
		if ev.Hook == scripting.OnPawn {
			c.what = "on_pawn's per-tick budget"
			err = c.eachPawn(th, m, fn)
		} else {
			_, err = starlark.Call(th, fn, c.args(ev), nil)
		}
		if err != nil {
			res.Disabled = append(res.Disabled, h.disable(m, err))
			continue
		}
		res.Commands = append(res.Commands, c.cmds...)
	}
	return res
}

// callFor is a call of m's hook (its function name, "on_event" for an
// extension) for ev, carrying what allowed and the attribute groups read:
// the plugin's granted set, the hook and the host's readers. Every hook
// call, Fire's and the extensions', is built here.
func (h *Host) callFor(m *module, hook string, ev scripting.Event, w scripting.World, p scripting.PawnReader) *call {
	c := newCall(m, ev, w, p)
	c.hook, c.caps, c.q = hook, m.caps, h.q
	if hook != ev.Hook.String() {
		// An extension call: ev.Hook does not name it, so ctx.rand is keyed
		// by the hook's name and the event (Fire's keys stay as they were).
		c.rekey(m, key(fmt.Sprintf("%s\x00%s\x00%s\x00%s\x00%s\x00%s\x00%s\x00%v\x00%v",
			hook, ev.Kind, ev.Incident.Key, ev.Name, ev.Plugin, ev.Action, ev.Research, ev.Recipe, ev.Map)))
	}
	return c
}

// eachPawn calls fn(ctx, pawn) for every due colonist on th, so the steps
// and the commands add up; the first error stops the calls.
func (c *call) eachPawn(th *starlark.Thread, m *module, fn starlark.Callable) error {
	ctx := &ctxValue{c: c}
	for _, p := range c.ev.Pawns {
		c.rekey(m, key(fmt.Sprint(p.ID)))
		if _, err := starlark.Call(th, fn, starlark.Tuple{ctx, pawnValue{p}}, nil); err != nil {
			return err
		}
	}
	return nil
}

// Uses reports that a live module defines hook h.
func (h *Host) Uses(hook scripting.Hook) bool {
	name := hook.String()
	for _, m := range h.mods {
		if m.off {
			continue
		}
		if _, ok := m.globals[name].(starlark.Callable); ok {
			return true
		}
	}
	return false
}

// disable turns m off with err as its problem.
func (h *Host) disable(m *module, err error) scripting.Problem {
	m.off = true
	p := problemOf(m, err)
	h.mu.Lock()
	h.probs = append(h.probs, p)
	h.mu.Unlock()
	h.refresh()
	return p
}

// problemOf places err in m's plugin: the innermost position inside one
// of its files, else m's own file.
func problemOf(m *module, err error) scripting.Problem {
	p := scripting.Problem{Plugin: m.plugin, File: m.path, Msg: err.Error()}
	var ee *starlark.EvalError
	var se syntax.Error
	var re resolve.ErrorList
	switch {
	case errors.As(err, &ee):
		p.Msg = ee.Msg
		for i := 0; i < len(ee.CallStack); i++ {
			if pos := ee.CallStack.At(i).Pos; pos.Line > 0 && strings.HasPrefix(pos.Filename(), "scripts/") {
				p.File, p.Line = pos.Filename(), int(pos.Line)
				break
			}
		}
	case errors.As(err, &se):
		p.File, p.Line, p.Msg = se.Pos.Filename(), int(se.Pos.Line), se.Msg
	case errors.As(err, &re) && len(re) > 0:
		p.File, p.Line, p.Msg = re[0].Pos.Filename(), int(re[0].Pos.Line), re[0].Msg
	}
	return p
}

// loader runs one plugin's files, each once, resolving load() inside the
// plugin's scripts folder.
type loader struct {
	plugin Plugin
	steps  uint64
	done   map[string]*module
	errs   map[string]error // a file's first-run failure
	busy   map[string]bool
}

// load runs file p once; later calls return the same module and error.
func (l *loader) load(p string) (*module, error) {
	if m, ok := l.done[p]; ok {
		return m, l.errs[p]
	}
	m := &module{plugin: l.plugin.ID, path: p, caps: l.plugin.Granted()}
	l.done[p] = m
	src, ok := l.src(p)
	if !ok {
		l.errs[p] = fmt.Errorf("no such script %q", p)
		return m, l.errs[p]
	}
	th := &starlark.Thread{Name: l.plugin.ID + "/" + p, Print: func(*starlark.Thread, string) {}, Load: l.loadFrom}
	th.SetMaxExecutionSteps(l.steps)
	l.busy[p] = true
	g, err := starlark.ExecFileOptions(Options(), th, p, src, nil)
	delete(l.busy, p)
	m.globals = g
	if err != nil {
		l.errs[p] = err
	}
	return m, err
}

// loadFrom is Thread.Load: module is a path relative to the scripts
// folder ("lib/util.star"); nothing outside it, nothing of another
// plugin, no cycle.
func (l *loader) loadFrom(_ *starlark.Thread, module string) (starlark.StringDict, error) {
	if module == "" || strings.HasPrefix(module, "/") || strings.Contains(module, "\\") || !strings.HasSuffix(module, ".star") {
		return nil, fmt.Errorf("load %q: only a .star file of this plugin's scripts folder can be loaded", module)
	}
	for _, part := range strings.Split(module, "/") {
		if part == ".." || part == "." || part == "" {
			return nil, fmt.Errorf("load %q: only a .star file of this plugin's scripts folder can be loaded", module)
		}
	}
	p := path.Join("scripts", module)
	if l.busy[p] {
		return nil, fmt.Errorf("load %q: a load cycle", module)
	}
	if _, ok := l.src(p); !ok {
		return nil, fmt.Errorf("load %q: no such script in this plugin", module)
	}
	m, err := l.load(p)
	if err != nil {
		return nil, fmt.Errorf("load %q: %w", module, err)
	}
	return m.globals, nil
}

func (l *loader) src(p string) ([]byte, bool) {
	for _, f := range l.plugin.Files {
		if f.Path == p {
			return f.Src, true
		}
	}
	return nil, false
}

// key hashes s for the rand stream's key.
func key(s string) uint64 {
	f := fnv.New64a()
	_, _ = f.Write([]byte(s))
	return f.Sum64()
}
