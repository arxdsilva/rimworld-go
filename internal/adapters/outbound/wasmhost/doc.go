// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

// Package wasmhost runs plugins' WebAssembly modules on wazero behind
// application/scripting.Host, beside starlarkhost (PLG-063).
//
// Spike: not wired. No composition root, game or binary builds a Host;
// only tests do. The scripting port is unchanged; per-file stats, save and
// load, and Close live on the concrete Host.
//
// # Loading
//
// New compiles every plugin's files in load order. For each file the host
// compiles the original bytes with wazero (validity), runs
// wasmrewrite.Prepare (Decode, Check, Reserve, CanonicaliseNaN, Meter,
// Encode), compiles the output and instantiates it with no start
// functions. A file that fails any step is a Problem and never enters the
// host's module list; a failure on the rewritten output is an internal
// error, but the file still only turns off.
//
// # Engine
//
// Features: api.CoreFeaturesV2 &^ api.CoreFeatureSIMD (bulk memory and
// reference types are on together; threads are not a v1.9.0 core
// feature). Runtime config: WithMemoryLimitPages(Options.MaxPages) and
// WithCloseOnContextDone(false). No WASI, no clock, no RNG; one
// context.Background(). Engine is Auto (wazero's default), Compiler or
// Interpreter.
//
// # Limits
//
// DefaultFuel (1,000,000) per call, or per file per tick for on_pawn; a
// Wasm instruction does less than a Starlark step, so this is 5x
// scripting.MaxSteps. DefaultMaxPages 256 (16 MiB). MaxState 64 KiB.
// MaxArity 64 params and 64 results per type. MaxLocals 4096 (params plus
// locals) per function. MaxBody 64 KiB per body. MaxWeight 1<<17: a
// function whose weight w(f) is over it is rejected after metering
// (wasmrewrite.RuleWeight). The call stack is bounded by MaxDepth (1000)
// floor-weight frames in StackBudget (1<<22) units.
//
// # Allow-list (wasmrewrite.Check)
//
// Everything not allowed is rejected with a typed error naming the opcode
// and function index.
//
//   - MVP opcodes, except 0x26 table.set. Also sign-ext 0xC0-0xC4;
//     0xFC 0-7 (sat-trunc); 0xFC 8-11 (memory.init, data.drop,
//     memory.copy, memory.fill); 0xFC 13 (elem.drop); 0xD0-0xD2 (ref.null,
//     ref.is_null, ref.func); 0x25 table.get; 0x1C select t.
//   - Blocktypes: empty, one valtype, or a type index (multi-value).
//   - Rejected: the 0xFD prefix (SIMD), the 0xFE prefix (threads), tail
//     calls 0x12/0x13, exception opcodes, 0xFC 12 and 14-17 (table.init,
//     table.copy, table.grow, table.size, table.fill), unknown opcodes.
//   - Imports: functions only, from module "rimworld" only.
//   - Memory: at most one, not shared, not memory64.
//   - No start section.
//   - No original export name starts with "__rimworld". The rewriter adds
//     "__rimworld_fuel" and "__rimworld_depth" (i64 mutable globals).
//   - Every type (function and block types alike) has at most MaxArity
//     params and MaxArity results ("arity over 64").
//   - A body has at most MaxLocals locals and MaxBody bytes.
//   - Every index immediate is below the count of its space in the
//     original module ("index out of range"): global.get/set, local.*,
//     call, ref.func, call_indirect's type and table, a blocktype's type,
//     memory.init/data.drop's data, elem.drop's elem, table.get's table,
//     and each export's index. A global.set on an immutable global is
//     rejected too.
//
// # Fuel (wasmrewrite.Meter)
//
// A charge at each function entry and after each loop opcode:
//
//	global.get F; i64.const cost; i64.sub; global.set F;
//	global.get F; i64.const 0; i64.lt_s; if; unreachable; end
//
// cost is 1 plus the instructions that belong to that header (each
// instruction belongs to its innermost enclosing loop, or to the
// function): static and deterministic. memory.fill, memory.copy and
// memory.init are charged their length n at run time, through one i32
// scratch local per body:
//
//	local.set s; global.get F; local.get s; i64.extend_i32_u; i64.sub;
//	global.set F; <check>; local.get s; <op>
//
// The fuel global starts at 0; the host sets it before every top-level
// call.
//
// # Call stack
//
// The "__rimworld_depth" i64 global is a weighted depth. Each function
// weighs w(f) = max(params + locals + V(f), StackBudget/MaxDepth), the
// floor being 4194 units. V(f) sums 1 + pushes over every instruction of
// the final body (original plus inserted): call and call_indirect push
// their callee type's result arity, block, loop and if their blocktype's
// result arity, every other op its own result count (0 or 1). The value
// of the i64.const w(f) operand does not change V(f). floor(4194304/4194)
// = 1000 floor-weight frames fit; frame 1001 traps.
//
// On entry depth += w(f), and depth > StackBudget is unreachable. The body
// is wrapped in block (type []->results) ... end followed by
// depth -= w(f); each original return becomes a br to the wrapper. The
// host resets depth to 0 before each top-level call.
//
// Worked bound: at most 1000 live frames, half the interpreter's
// 2000-frame ceiling (one extra frame for a host import call fits). The
// compiler (wazevo, 400 MB stack ceiling) keeps one 8-byte slot per param,
// local and spilled value; allowing at most 4 slots per weight unit plus
// about 512 bytes of fixed frame, and 2 x MaxArity x 8 B = 1 KiB of call
// arguments and multi-value return space per frame (bounded only because
// Check caps every type's arity), live frames use at most
// 4 x 8 B x 4,194,304 + 1000 x (512 B + 1 KiB), about 136 MB. The
// arithmetic is the rationale; the worst-case-frame test is the proof. If
// that test fails, StackBudget is lowered, never the test.
//
// # NaN (wasmrewrite.CanonicaliseNaN)
//
// f32/f64 add, sub, mul, div, sqrt, min, max, ceil, floor, trunc and
// nearest, f32.demote_f64 and f64.promote_f32 are each followed by
//
//	local.set t; local.get t; f.const canon; local.get t; local.get t;
//	f.eq; select
//
// with canon 0x7FC00000 (f32) or 0x7FF8000000000000 (f64) and one scratch
// local per float type per body. fneg, fabs, fcopysign, const, load,
// store and reinterpret keep their bits.
//
// # Guest ABI v1 (little-endian)
//
// Required exports: memory; rw_buf() -> i32; rw_buf_size() -> i32. The
// buffer lies inside memory and is at least 64 bytes; both calls are
// metered once, at load. Optional hook exports, each () -> (): on_hour,
// on_day, on_incident, on_colonist_died, on_pawns, on_pawn. Save and
// load: state_version() -> i32, save() -> i32, load(len i32) -> i32, all
// three or none. A wrong export signature fails the load.
//
// Before each hook call the host writes the event at rw_buf:
//
//	header (32 bytes)
//	  0 u32 abi    4 u32 hook (scripting.Hook)   8 u64 tick
//	 16 u32 nrec  20 u32 len1   24 u32 len2      28 u32 0
//	nrec records of 32 bytes, then len1 bytes of text1, then len2 of text2
//
// on_incident: text1 is the key, text2 the category. on_colonist_died:
// text1 is the name. on_pawns: nrec is the number of due pawns. on_pawn:
// nrec is 1, rewritten per pawn. An event larger than the buffer fails
// that call.
//
// A record (32 bytes):
//
//	0 u64 id   8 i32 x   12 i32 y   16 i32 food   20 i32 rest
//	24 i32 mood   28 u32 flags (bit 0 downed, bit 1 drafted)
//
// Imports, module "rimworld"; pointers and lengths are i32:
//
//	commands  message(p,n)  letter(tp,tn,p,n)  spawn(kp,kn,count)
//	reads     rand(n) -> i32   count(kp,kn) -> i32   day() -> i32
//	          hour() -> i32    colonists() -> i32    wealth() -> i64
//	          season(p,cap) -> i32       full length; writes at most cap bytes
//	          pawns(p,cap) -> i32        cap in records; writes min(total,cap); returns total
//	          pawn(id i64,p) -> i32      1 if it wrote the record, else 0
//	          pawn_name(id i64,p,cap) -> i32   the length, or -1
//
// Each import charges the fuel global 4, plus 1 per record or per 32 bytes
// moved (rounded up); bounds are checked before the charge, so a bad
// pointer reports its own message, not out of fuel; a charge that drives fuel below 0 stops the call at once with
// "out of fuel in <hook>" and the import does nothing. Every guest pointer
// and length is bounds-checked. A bad argument or failed check fails the
// whole call with the import's own message, the same strings as
// starlarkhost (including "more than %d commands in %s"). Host functions
// never panic on guest input except through the one internal stop the
// host recovers.
//
// During save, load and the load-time calls there is no event and no
// world: message, letter and spawn fail the call with "<name>: not available
// outside a hook" (a Problem; the file turns off), as do the world reads.
//
// # Save blob
//
//	"RWMS" | u32 ABIVersion | u32 state_version | u32 len | data
//
// len <= MaxState and len <= rw_buf_size. Load rejects a bad magic, ABI,
// version (it must equal state_version()) or size before it calls the
// guest; a non-zero return from load is an error.
//
// # Rand key
//
// randx.NewKeyed(ev.Seed, ev.Tick, fnv64a(plugin+"\x00"+path),
// uint64(hook), sub), with sub = fnv64a(incidentKey+"\x00"+name); on_pawn
// rekeys with fnv64a(decimal id). rand(n) requires 1 <= n <= 1<<30 and
// returns IntN(n), as starlarkhost does, so a mod ported 1:1 with the same
// plugin ID and path string draws the same numbers.
//
// # Problems
//
// Problem{Plugin, File: path, Line: 0, Msg}. Msg is one of:
// "out of fuel in <hook> (budget N)"; "call stack over budget in <hook>";
// "trap in <hook>: <first line of the wazero error>" (the stack trace is
// cut, so both engines give the same text); a load error such as
// "rejected: <check error>"; or the import's message. Out of fuel and too
// deep are read from the reserved globals after the trap, never from the
// error text.
package wasmhost
