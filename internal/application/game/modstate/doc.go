// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

// Package modstate is the shape of the Wasm mods' saved state (PLG-063):
// one opaque, versioned blob per Wasm file, the notes a load leaves, and
// the port the save path reads them through.
//
// A mod state blob is opaque outside wasmhost. Only the save path calls
// Source; the state hash never does. The package imports nothing from
// the game root (a game sub-package consumes the root, never imports it).
package modstate
