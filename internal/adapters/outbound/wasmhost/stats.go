// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package wasmhost

// Stat is one loaded file's counters.
type Stat struct {
	Plugin, File string
	Fuel         uint64 // fuel spent by hook, save and load calls (not the load-time setup)
	Calls        uint64 // guest calls made: hook (one per on_pawn pawn), save and load
	Pages        uint32 // memory size now, in 64 KiB pages
	Off          bool   // disabled
	State        bool   // exports state_version, save and load (set by W1-wasmhost)
}

// Stats lists every loaded file's counters, in load order. A file that
// failed to load is a Problem, not a Stat.
func (h *Host) Stats() []Stat {
	out := make([]Stat, 0, len(h.mods))
	for _, m := range h.mods {
		out = append(out, Stat{
			Plugin: m.plugin, File: m.path,
			Fuel: m.spent, Calls: m.calls,
			Pages: m.mem.Size() / 65536, Off: m.off,
		})
	}
	return out
}
