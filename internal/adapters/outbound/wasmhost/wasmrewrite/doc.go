// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

// Package wasmrewrite decodes, checks and rewrites a WebAssembly module
// before wasmhost compiles it (PLG-063 spike: not wired into the game).
//
// It is pure Go over bytes and does not import wazero. Prepare runs the
// pipeline Decode, Check, Reserve, CanonicaliseNaN, Meter, Encode:
//
//   - Check rejects everything outside the allow-list (wasmhost's doc
//     lists it) before any rewrite, including every out-of-range index,
//     so Prepare is safe on its own on invalid input.
//   - Reserve appends the two reserved i64 mutable globals (fuel and
//     depth) and exports them under Config.FuelExport and
//     Config.DepthExport.
//   - CanonicaliseNaN follows every float op that can make a NaN with a
//     select that turns any NaN into the canonical one.
//   - Meter charges fuel at each function entry and after each loop
//     opcode, charges bulk memory ops their length, and wraps each body
//     in the weighted call-depth charge.
//
// Rewrites only append: globals at the end of the global index space,
// exports, scratch locals at the end of a body's locals and one type at
// the end of the type section. Existing indices never shift. Custom
// sections are dropped after they are checked.
//
// Walk is the only decoder of instruction streams; Check, the NaN pass
// and Meter all use it.
package wasmrewrite
