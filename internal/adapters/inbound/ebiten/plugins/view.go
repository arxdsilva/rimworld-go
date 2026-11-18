// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package plugins

import (
	"slices"
	"strings"

	"rimworld-go/internal/platform/config"
)

// Core is the game's own plugin id: always loaded, never turned off.
const Core = "core"

// View is the run's plugins as the screens show them: plain values the
// composition root maps from the content catalog once at start.
type View struct {
	// Rows are every installed plugin: the loaded ones in load order, then
	// the rest by id.
	Rows []Row
	// Problems are what loading reported, warnings included, in order.
	Problems []Problem
	// Enabled is the enabled set this run started with (sorted): a saved
	// set that differs from it takes effect at the next start.
	Enabled []string
	// Folders are where plugins are installed from (the empty state names
	// them).
	Folders []string
}

// Row is one installed plugin.
type Row struct {
	ID, Name, Version, Description, URL string
	Authors, Requires, After, Before    []string
	Conflicts                           []string
	Loaded                              bool   // it loads this run
	Position                            int    // its place in the load order, the game being 0 (Loaded only)
	Reason                              string // why it sits there (Loaded only)
}

// Problem is one diagnostic with its provenance.
type Problem struct {
	Plugin, File, Def, Field, Msg string
	Warn                          bool
}

// Where is the problem's provenance: plugin, file, def and field, the
// parts it has.
func (p Problem) Where() string {
	var parts []string
	for _, s := range []string{p.Plugin, p.File, p.Def, p.Field} {
		if s != "" {
			parts = append(parts, s)
		}
	}
	return strings.Join(parts, " › ")
}

// Errors counts the problems that are not warnings: what the main menu's
// notice reports.
func (v View) Errors() int {
	n := 0
	for _, p := range v.Problems {
		if !p.Warn {
			n++
		}
	}
	return n
}

// Counts are a plugin's errors and warnings.
func (v View) Counts(id string) (errs, warns int) {
	for _, p := range v.Problems {
		switch {
		case p.Plugin != id:
		case p.Warn:
			warns++
		default:
			errs++
		}
	}
	return errs, warns
}

// FirstError is a plugin's first error message ("" when none).
func (v View) FirstError(id string) string {
	for _, p := range v.Problems {
		if p.Plugin == id && !p.Warn {
			return p.Msg
		}
	}
	return ""
}

// RowOf is the index of the row for id (-1 when none is installed).
func (v View) RowOf(id string) int {
	for i, r := range v.Rows {
		if r.ID == id {
			return i
		}
	}
	return -1
}

// Toggle is the enabled set with id turned on or off: a fresh, sorted
// slice, so the caller's settings never share its backing array. Ids of
// plugins no longer installed stay.
func Toggle(set []string, id string, on bool) []string {
	out := make([]string, 0, len(set)+1)
	for _, x := range set {
		if x != id {
			out = append(out, x)
		}
	}
	if on {
		out = append(out, id)
	}
	return config.Plugins{Enabled: out}.Normalize().Enabled
}

// Pending reports the saved set differs from the one the run started
// with: the change waits for the next start.
func Pending(saved, started []string) bool {
	return !slices.Equal(config.Plugins{Enabled: saved}.Normalize().Enabled,
		config.Plugins{Enabled: started}.Normalize().Enabled)
}

// State is the row's state as the list says it: loaded, failed to load,
// off, or waiting for the next start.
type State uint8

const (
	StateOff          State = iota // off, and off this run
	StateLoaded                    // on and loaded
	StateFailed                    // on, but it could not load
	StateOnAtRestart               // turned on since the start
	StateOffAtRestart              // turned off since the start, still loaded
)

// StateOf is a row's state given whether the saved settings enable it now
// and whether the run started with it enabled.
func StateOf(r Row, on, started bool) State {
	switch {
	case on && r.Loaded:
		return StateLoaded
	case on && started:
		return StateFailed
	case on:
		return StateOnAtRestart
	case r.Loaded:
		return StateOffAtRestart
	}
	return StateOff
}
