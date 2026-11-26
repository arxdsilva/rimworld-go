// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package command

// ConfigEdit identifies a successful configuration command on a native
// owner. Revision zero means no recorded edit, not permission to edit.
// Row is a bench-local bill identity; Index is its index after the edit
// (before removal for RemoveBill). Neither field is used for pen/trough
// edits. This is source provenance, never an assertion about the actor.
type ConfigEdit struct {
	Revision, Tick, Row uint64
	Kind                Kind
	Index               int
}

// ConfigMutation resolves an actual applied command to its native map,
// owner and edit. It is a transient result, not an authenticated journal.
type ConfigMutation struct {
	Map     uint32
	Owner   uint64
	Edit    ConfigEdit
	Command Command
}
