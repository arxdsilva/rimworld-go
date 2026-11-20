// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package wasmhost

// Stat is one loaded file's counters.
type Stat struct {
	Plugin, File string
	Fuel         uint64 // fuel spent, all calls
	Calls        uint64 // top-level calls made
	Pages        uint32 // memory size now, in 64 KiB pages
	Off          bool   // disabled
}

// Stats lists every loaded file's counters, in load order.
func (h *Host) Stats() []Stat {
	return nil // W0 stub
}
