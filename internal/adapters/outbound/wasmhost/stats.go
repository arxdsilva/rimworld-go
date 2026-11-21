// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package wasmhost

// Stat is one loaded file's counters.
type Stat struct {
	Plugin, File string
	Fuel         uint64 // fuel spent by hook and load calls (not the load-time setup, not save: it runs on a copy)
	Calls        uint64 // guest calls made: hook (one per on_pawn pawn) and load
	Pages        uint32 // memory size now, in 64 KiB pages
	Off          bool   // disabled
	State        bool   // exports state_version, save and load
}

// Stats lists every loaded file's counters, in load order. A file that
// failed to load is a Problem, not a Stat.
func (h *Host) Stats() []Stat {
	return h.StatsInto(make([]Stat, 0, len(h.mods)))
}

// StatsInto is Stats appended to dst[:0], so a caller can reuse its
// buffer: it does not allocate when cap(dst) covers every loaded file.
func (h *Host) StatsInto(dst []Stat) []Stat {
	dst = dst[:0]
	for _, m := range h.mods {
		dst = append(dst, Stat{
			Plugin: m.plugin, File: m.path,
			Fuel: m.spent, Calls: m.calls,
			Pages: pages(m), Off: m.off, State: m.save != nil,
		})
	}
	return dst
}

// pages is m's memory size in 64 KiB pages; 0 with no instance.
func pages(m *module) uint32 {
	if m.mem == nil {
		return 0
	}
	return m.mem.Size() / 65536
}
