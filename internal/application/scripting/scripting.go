// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

// Package scripting is the port plugins' scripts run behind (PLG-060): the
// simulation owner calls a Host at fixed points of the tick and gets typed
// commands back. The runtime (Starlark) lives in an outbound adapter.
package scripting

import "rimworld-go/internal/domain/sim/command"

// Host runs the enabled plugins' scripts. The simulation owner calls it,
// never concurrently, and the same calls give the same commands.
// CONTRACT STUB (wave 0): the scripting agent designs the methods.
type Host interface {
	// Fire runs every hook for ev and returns the commands they submit.
	Fire(ev Event) []command.Command
}

// Event is one hook point.
type Event struct {
	Name string
	Tick uint64
}
