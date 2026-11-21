// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package modstate

// Limits on the mod state a save holds.
const (
	MaxData  = 64<<10 + 16 // bytes of one blob's data: a module's 64 KiB state plus the host's header
	MaxBlobs = 256         // blobs in one save
	MaxName  = 256         // bytes of a blob's plugin id or file path
)

// Blob is one Wasm file's versioned save data, opaque outside wasmhost.
type Blob struct {
	Plugin, File string
	Data         []byte
}

// Note is one thing loading the mod state reported about a file (a blob
// dropped, a file with no blob), shown to the player as a script message.
type Note struct {
	Plugin, File, Msg string
}

// Source is the run's Wasm mod state for a real save. Only the save path
// calls it; StateHash never does. The composition root's mux implements
// it over wasmhost (W2-compose).
type Source interface {
	SaveBlobs() []Blob
}

// Check keeps the blobs a save may hold and notes the rest. Stub: it
// returns bs unchanged; W1-game fills it (limits, order, duplicates).
func Check(bs []Blob) (kept []Blob, notes []Note) {
	return bs, nil
}
