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
// PLG-063 adds the Wasm case (wasm_test.go): the same workload through the
// real wasmhost adapter on both wazero engines (wasm-compiler is the shipping
// one, wasm-interp the portable fallback), fuel metering on at DefaultFuel.
// The guests are the Rust fixtures wasmhost/testdata/bench_{batched,percall,
// noop}.wasm (17680, 1569 and 229 bytes; batched reserves a 16 KiB event
// buffer). Records are 32 bytes per colonist, read at the same fields
// (food, mood) the Starlark and Lua scripts read; batched copies all N
// records in one pawns() import, percall one pawn() import per id. The noop
// case is a full host Fire, like Starlark's. BenchmarkCompile builds a host
// per iteration (Starlark parse and compile; Wasm decode, check, rewrite,
// compile, instantiate, and Close).
//
// Every PawnReads case (all runtimes, noop included) makes one untimed warm
// call before ResetTimer, so first-call costs are excluded symmetrically.
//
// Measured 2026-11-20 on an Apple M2 Max, idle, no -race, median ns/op of
// -count=10 (go test -run '^$' -bench 'PawnReads|Compile' -benchtime 300x
// -count=10 ./internal/adapters/outbound/scriptbench/):
//
//	case               starlark  wasm-compiler  wasm-interp  lua-lazy  lua-eager
//	batched n=50          6.5k        1.0k          11.2k      14.4k      51.9k
//	batched n=500          57k        5.7k            85k       130k       612k
//	percall n=50          10.9k       3.5k          13.9k      16.2k      57.8k
//	percall n=500          107k        30k            105k       159k       557k
//	empty hook             357         252 (wasm)       -       166 (lua)    -
//	compile (per host)     9.1k        786k            175k        -          -
//
// Fuel per call against DefaultFuel 1,000,000: batched n=500 15,304 (1.5%),
// percall n=500 18,301 (1.8%) (TestWasmFuelHeadroom).
//
// The game's cadence bucket is about N/60 colonists per call (assumption
// A8), so the 50 and 500 sizes measure the reads, not the cadence.
//
// gopher-lua has no deterministic step budget (only context cancellation),
// so it is benchmark only: evidence for the runtime criteria, never a
// shipped runtime. TestGopherLuaOnlyInBenchTests guards that.
package scriptbench
