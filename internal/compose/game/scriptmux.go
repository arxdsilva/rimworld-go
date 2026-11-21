// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"slices"
	"strings"
	"sync"

	"rimworld-go/internal/adapters/inbound/ebiten/plugins"
	"rimworld-go/internal/adapters/outbound/starlarkhost"
	"rimworld-go/internal/adapters/outbound/wasmhost"
	appgame "rimworld-go/internal/application/game"
	"rimworld-go/internal/application/game/modstate"
	"rimworld-go/internal/application/modding"
	"rimworld-go/internal/application/scripting"
	"rimworld-go/internal/domain/sim/command"
)

// scriptMux is the run's scripts (PLG-060, PLG-063): one segment per
// loaded plugin runtime, in load order, a plugin's Starlark files before
// its Wasm files, so plugins interleave across runtimes. It is the games'
// scripting.Host and the save's modstate.Source, and feeds the Plugins
// screens (Late, Stats).
//
// Fire, Uses, SaveBlobs, Reset, start and restore are the simulation
// owner's; Late and Stats may be called from any goroutine: they read the
// hosts' locked Problems and the copies kept under mu.
type scriptMux struct {
	segs  []scripting.Host     // load order: per plugin its Starlark host, then its Wasm view
	stars []*starlarkhost.Host // the Starlark segments, for their Problems
	wasm  *wasmhost.Host       // every Wasm file; nil: no plugin has one

	mu    sync.Mutex
	stats []wasmhost.Stat   // the Wasm counters after the last Fire, Reset or restore
	saved []plugins.Problem // one per file whose last save failed
	notes []plugins.Problem // what this game's restore and saves reported, without repeats
}

var (
	_ scripting.Host  = (*scriptMux)(nil)
	_ modstate.Source = (*scriptMux)(nil)
)

// newScriptMux builds the segments of ps (loaded plugins, in load order,
// core excluded) and one Wasm host for every plugin's Wasm files.
func newScriptMux(ps []modding.Plugin, o wasmhost.Options) *scriptMux {
	m := &scriptMux{}
	var wps []wasmhost.Plugin
	for _, p := range ps {
		if len(p.Wasm) == 0 {
			continue
		}
		wp := wasmhost.Plugin{ID: p.Manifest.ID}
		for _, b := range p.Wasm {
			wp.Files = append(wp.Files, wasmhost.File{Path: b.Path, Wasm: b.Data})
		}
		wps = append(wps, wp)
	}
	if len(wps) > 0 {
		m.wasm = wasmhost.New(wps, o)
	}
	for _, p := range ps {
		if len(p.Scripts) > 0 {
			sp := starlarkhost.Plugin{ID: p.Manifest.ID}
			for _, b := range p.Scripts {
				sp.Files = append(sp.Files, starlarkhost.File{Path: b.Path, Src: b.Data})
			}
			h := starlarkhost.New([]starlarkhost.Plugin{sp})
			m.stars, m.segs = append(m.stars, h), append(m.segs, h)
		}
		if len(p.Wasm) > 0 {
			m.segs = append(m.segs, m.wasm.Plugin(p.Manifest.ID))
		}
	}
	m.refresh()
	return m
}

// Fire runs the segments in order and concatenates their results; each
// segment keeps its own budgets.
func (m *scriptMux) Fire(ev scripting.Event, w scripting.World, p scripting.PawnReader) scripting.Result {
	var res scripting.Result
	for _, s := range m.segs {
		r := s.Fire(ev, w, p)
		res.Commands = append(res.Commands, r.Commands...)
		res.Disabled = append(res.Disabled, r.Disabled...)
	}
	m.refresh()
	return res
}

// Uses reports that a segment uses hk. It does not allocate.
func (m *scriptMux) Uses(hk scripting.Hook) bool {
	for _, s := range m.segs {
		if s.Uses(hk) {
			return true
		}
	}
	return false
}

// SaveBlobs saves every live Wasm file with state, in load order then
// path. A failed save leaves the file out, records one Problem for it
// (replaced on a repeat, cleared by a later success) and never touches
// the game.
func (m *scriptMux) SaveBlobs() []modstate.Blob {
	if m.wasm == nil {
		return nil
	}
	var out []modstate.Blob
	for _, s := range m.wasm.Stats() {
		if !s.State || s.Off {
			continue
		}
		data, err := m.wasm.Save(s.Plugin, s.File)
		if err != nil {
			m.noteSave(s.Plugin, s.File, "mod state not saved: "+err.Error())
			continue
		}
		m.noteSave(s.Plugin, s.File, "")
		out = append(out, modstate.Blob{Plugin: s.Plugin, File: s.File, Data: data})
	}
	return out
}

// noteSave sets (msg != "") or clears the save Problem of one file.
func (m *scriptMux) noteSave(plugin, file, msg string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i, p := range m.saved {
		if p.Plugin == plugin && p.File == file {
			if msg == "" {
				m.saved = append(m.saved[:i], m.saved[i+1:]...)
			} else {
				m.saved[i].Msg = msg
			}
			return
		}
	}
	if msg != "" {
		m.saved = append(m.saved, plugins.Problem{Plugin: plugin, File: file, Msg: msg, Warn: true})
	}
}

// Reset starts a new game: fresh Wasm instances, no restore notes, the
// save Problems of files that still keep state, and fresh counters.
// Starlark keeps no state, and a Starlark file turned off stays off for
// the run, as before.
func (m *scriptMux) Reset() {
	if m.wasm != nil {
		m.wasm.Reset()
	}
	m.refresh()
	m.mu.Lock()
	defer m.mu.Unlock()
	m.notes = nil
	kept := m.saved[:0]
	for _, p := range m.saved {
		if m.keepsStateLocked(p.Plugin, p.File) {
			kept = append(kept, p)
		}
	}
	m.saved = kept
}

// start hands g a fresh run of the scripts: Reset, then the host and the
// mod state source (literal nils when there is none, so a run with no
// scripted plugin keeps today's tick path).
func (m *scriptMux) start(g *appgame.Game) {
	m.Reset()
	m.attach(g)
}

// attach sets the mux on g as its scripts and its mod state source.
func (m *scriptMux) attach(g *appgame.Game) {
	appgame.UseScripts(g, m.host())
	appgame.UseModState(g, m.source())
}

// host is m as g's scripts; nil with no segment.
func (m *scriptMux) host() scripting.Host {
	if len(m.segs) == 0 {
		return nil
	}
	return m
}

// source is m as g's mod state source; nil with no Wasm host.
func (m *scriptMux) source() modstate.Source {
	if m.wasm == nil {
		return nil
	}
	return m
}

// restore hands the Wasm files the state the loaded save g held. A blob
// for a file that is not loaded with state is dropped; a rejected one
// leaves its file on with fresh state (a trap turns it off through the
// host's own Problem). Every note is a Warn Problem on Late, and one
// in-game message names them.
func (m *scriptMux) restore(g *appgame.Game) {
	blobs, checked := appgame.TakeLoadedModState(g)
	var notes []plugins.Problem
	note := func(plugin, file, msg string) {
		notes = append(notes, plugins.Problem{Plugin: plugin, File: file, Msg: msg, Warn: true})
	}
	for _, n := range checked {
		note(n.Plugin, n.File, n.Msg)
	}
	for _, b := range blobs {
		switch {
		case m.wasm == nil:
			note(b.Plugin, b.File, "mod state dropped: plugin or file not loaded")
		case !m.keepsState(b.Plugin, b.File):
			note(b.Plugin, b.File, "mod state dropped: file no longer loaded")
		default:
			if err := m.wasm.Load(b.Plugin, b.File, b.Data); err != nil {
				note(b.Plugin, b.File, "mod state not restored: "+err.Error())
			}
		}
	}
	m.refresh()
	if fresh := m.addNotes(notes); len(fresh) > 0 {
		g.Enqueue(command.Command{Kind: command.ScriptMessage, Name: notice("Plugin mod state on load: ", fresh)})
	}
}

// saveNotes takes what g's saves dropped from the mod state (the blobs
// modstate.Check refused, W1-game C9) onto Late only: no save may change
// the run, so it posts no in-game message (lead ruling save-notes). The
// composition root calls it after every save of g.
func (m *scriptMux) saveNotes(g *appgame.Game) {
	_, checked := appgame.TakeLoadedModState(g)
	var notes []plugins.Problem
	for _, n := range checked {
		notes = append(notes, plugins.Problem{Plugin: n.Plugin, File: n.File, Msg: n.Msg, Warn: true})
	}
	m.addNotes(notes)
}

// addNotes adds the notes this game has not shown yet to Late (so
// repeated autosaves do not repeat them) and returns them.
func (m *scriptMux) addNotes(notes []plugins.Problem) []plugins.Problem {
	m.mu.Lock()
	defer m.mu.Unlock()
	var fresh []plugins.Problem
	for _, n := range notes {
		if !slices.Contains(m.notes, n) && !slices.Contains(fresh, n) {
			fresh = append(fresh, n)
		}
	}
	m.notes = append(m.notes, fresh...)
	return fresh
}

// keepsState reports a loaded Wasm file with save and load exports.
func (m *scriptMux) keepsState(plugin, file string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.keepsStateLocked(plugin, file)
}

func (m *scriptMux) keepsStateLocked(plugin, file string) bool {
	for _, s := range m.stats {
		if s.Plugin == plugin && s.File == file {
			return s.State
		}
	}
	return false
}

// notice is the one in-game message naming the files notes are about,
// after head, at most scripting.MaxText bytes.
func notice(head string, notes []plugins.Problem) string {
	const tail = ". See Plugins > Problems."
	seen := map[string]bool{}
	var files []string
	for _, n := range notes {
		f := n.Plugin + "/" + n.File
		if !seen[f] {
			seen[f] = true
			files = append(files, f)
		}
	}
	list := strings.Join(files, ", ")
	if room := scripting.MaxText - len(head) - len(tail); len(list) > room {
		list = strings.ToValidUTF8(list[:room-len(" and more")], "") + " and more"
	}
	return head + list + tail
}

// refresh copies the Wasm counters for Stats.
func (m *scriptMux) refresh() {
	if m.wasm == nil {
		return
	}
	m.mu.Lock()
	m.stats = m.wasm.StatsInto(m.stats)
	m.mu.Unlock()
}

// Late are the problems found while playing: the Starlark files turned
// off, the Wasm files turned off (load failures included), the failed
// saves and the last restore's notes, without repeats.
func (m *scriptMux) Late() []plugins.Problem {
	var out []plugins.Problem
	for _, h := range m.stars {
		for _, p := range h.Problems() {
			out = append(out, plugins.Problem{Plugin: p.Plugin, File: p.File, Field: lineOf(p.Line),
				Msg: "script turned off for this run: " + p.Msg})
		}
	}
	if m.wasm != nil {
		for _, p := range m.wasm.Problems() {
			out = append(out, plugins.Problem{Plugin: p.Plugin, File: p.File, Field: lineOf(p.Line),
				Msg: "module turned off: " + p.Msg})
		}
	}
	m.mu.Lock()
	out = append(append(out, m.saved...), m.notes...)
	m.mu.Unlock()
	seen := make(map[plugins.Problem]bool, len(out))
	uniq := out[:0]
	for _, p := range out {
		if !seen[p] {
			seen[p] = true
			uniq = append(uniq, p)
		}
	}
	return uniq
}

// Stats are a copy of the Wasm files' counters, for the plugin detail.
func (m *scriptMux) Stats() []plugins.ModStat {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]plugins.ModStat, len(m.stats))
	for i, s := range m.stats {
		out[i] = plugins.ModStat{Plugin: s.Plugin, File: s.File, Fuel: s.Fuel, Calls: s.Calls, Pages: s.Pages, Off: s.Off}
	}
	return out
}

// Close releases the Wasm runtime (tests; the game keeps it for the run).
func (m *scriptMux) Close() error {
	if m.wasm == nil {
		return nil
	}
	return m.wasm.Close()
}
