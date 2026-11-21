// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package wasmhost

import "rimworld-go/internal/application/scripting"

// Reset starts a new game: it closes every instance and instantiates a
// fresh one from the kept CompiledModule. Stub: W1-wasmhost fills it.
func (h *Host) Reset() {}

// Plugin is the view of plugin id's Wasm files, one segment of the
// composition root's mux. Stub: it returns h; W1-wasmhost fills it.
func (h *Host) Plugin(id string) scripting.Host {
	return h
}

// StatsInto is Stats appended to dst[:0], so a caller can reuse its
// buffer. Stub: W1-wasmhost replaces it with an allocation-free copy.
func (h *Host) StatsInto(dst []Stat) []Stat {
	return append(dst[:0], h.Stats()...)
}
