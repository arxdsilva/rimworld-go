// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package wasmhost

import "rimworld-go/internal/application/scripting"

// Reset starts a new game: it closes every instance and instantiates a
// fresh one from the kept CompiledModule under the same name, so every file
// is on again with zero counters and its initial memory and globals. It
// compiles nothing. The load-time Problems stay; the previous game's are
// dropped, and a load failure is still sent in one Result per host. A file
// whose instance cannot be made again turns off with a Problem.
func (h *Host) Reset() {
	h.cur = nil
	h.mu.Lock()
	h.probs = clip(h.probs, h.loadProbs)
	h.mu.Unlock()
	h.unsent = clip(h.unsent, h.loadUnsent)
	for _, m := range h.mods {
		if m.inst != nil {
			_ = m.inst.Close(h.ctx)
			m.inst, m.mem = nil, nil
		}
		m.off, m.calls, m.spent = false, 0, 0
		if msg := h.instantiate(m); msg != "" {
			h.unsent = append(h.unsent, h.disable(m, msg))
		}
	}
}

// clip is s[:n] with no room to append over the dropped entries; nil when
// n is 0.
func clip(s []scripting.Problem, n int) []scripting.Problem {
	if n == 0 {
		return nil
	}
	return s[:n:n]
}

// Plugin is the view of plugin id's Wasm files, one segment of the
// composition root's mux: its Fire runs Host.Fire over those files only and
// its Uses covers them only, without allocating. The host's unsent load
// failures go out once, in the first Result of any view or of the host. An
// unknown id gives a view with no files. A view stays valid across Reset.
func (h *Host) Plugin(id string) scripting.Host {
	lo := 0
	for lo < len(h.mods) && h.mods[lo].plugin != id {
		lo++
	}
	hi := lo
	for hi < len(h.mods) && h.mods[hi].plugin == id {
		hi++
	}
	return &view{h: h, mods: h.mods[lo:hi:hi]}
}

// view is one plugin's files: contiguous in h.mods, since New appends
// each plugin's files together.
type view struct {
	h    *Host
	mods []*module
}

func (v *view) Fire(ev scripting.Event, w scripting.World, p scripting.PawnReader) scripting.Result {
	return v.h.fire(v.mods, ev, w, p)
}

func (v *view) Uses(hk scripting.Hook) bool { return uses(v.mods, hk) }
