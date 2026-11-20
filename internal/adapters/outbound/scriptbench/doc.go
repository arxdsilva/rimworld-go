// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

// Package scriptbench is a benchmark only (PLG-060): it measures the
// scripting port's Starlark host against a gopher-lua (Lua 5.1) runtime on
// the same per-colonist work. It holds no code; its tests import
// gopher-lua, so the module is a test-only dependency that never reaches
// the game binary. Like game/scalegolden, it runs in its own test binary.
//
// The workload is one on_hour call that reads every colonist's food and
// mood, counts food < 30, sums mood and queues one message "%d %d". It runs
// batched (one ctx.pawns()) and per call (ctx.pawn(id), id in 1..N), in the
// real starlarkhost adapter and in a sandboxed gopher-lua state (base, table
// and string only), at N = 50 and 500, plus an empty hook.
//
// Records are compared like for like: Starlark's pawn is a lazy value
// resolved on field access, and lua-lazy is userdata with one shared
// __index metatable built outside the timer. lua-eager (a 9-field table per
// pawn) is kept to show what record shape costs. The empty-hook cases are
// not symmetric: Starlark's is a full host Fire (result, rand key, step
// budget), Lua's a bare protected call.
//
// The game's cadence bucket is about N/60 colonists per call (assumption
// A8), so the 50 and 500 sizes measure the reads, not the cadence.
//
// gopher-lua has no deterministic step budget (only context cancellation),
// so it is benchmark only: evidence for the runtime criteria, never a
// shipped runtime. TestGopherLuaOnlyInBenchTests guards that.
package scriptbench
