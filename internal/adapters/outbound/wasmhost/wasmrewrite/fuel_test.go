// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package wasmrewrite_test

import (
	"bytes"
	"fmt"
	"slices"
	"strings"
	"testing"

	"rimworld-go/internal/adapters/outbound/wasmhost/wasmrewrite"
	. "rimworld-go/internal/adapters/outbound/wasmhost/wasmtest"
)

// fuelSLEB appends v as a signed LEB128.
func fuelSLEB(b []byte, v int64) []byte {
	for {
		c := byte(v) & 0x7f
		v >>= 7
		if (v == 0 && c&0x40 == 0) || (v == -1 && c&0x40 != 0) {
			return append(b, c)
		}
		b = append(b, c|0x80)
	}
}

// fuelI32 is i32.const v.
func fuelI32(v int32) []byte { return fuelSLEB([]byte{OpI32Const}, int64(v)) }

func fuelCat(parts ...[]byte) []byte { return slices.Concat(parts...) }

// fuelKnownModule holds functions whose fuel is computed by hand from the
// documented rule: a charge of 1 + the instructions of the function region
// at entry, and of 1 + the instructions of the loop's region each time
// control enters the loop. A loop opcode belongs to the enclosing region;
// the instructions after it, its end included, belong to the loop.
func fuelKnownModule() []byte {
	var b Builder
	i32 := []byte{I32}
	// sum: function region {loop, local.get 1, end} costs 4; the loop
	// region {9 instructions + end} costs 11, once per iteration.
	sum := b.Func(i32, i32, i32,
		OpLoop, Empty,
		OpLocalGet, 1, OpLocalGet, 0, OpI32Add, OpLocalSet, 1,
		OpLocalGet, 0, OpI32Const, 1, OpI32Sub, OpLocalTee, 0, OpBrIf, 0,
		OpEnd, OpLocalGet, 1, OpEnd)
	// nest: function {outer loop, end} costs 3; outer region {i32.const,
	// local.set, inner loop, local.get, i32.const, i32.sub, local.tee,
	// br_if, end} costs 10; inner region {5 + end} costs 7 and runs 3 times
	// per outer iteration: 3 + 31n.
	nest := b.Func(i32, nil, i32,
		OpLoop, Empty,
		OpI32Const, 3, OpLocalSet, 1,
		OpLoop, Empty,
		OpLocalGet, 1, OpI32Const, 1, OpI32Sub, OpLocalTee, 1, OpBrIf, 0,
		OpEnd,
		OpLocalGet, 0, OpI32Const, 1, OpI32Sub, OpLocalTee, 0, OpBrIf, 0,
		OpEnd, OpEnd)
	// pick: {local.get, if, i32.const, else, i32.const, end, end} costs 8
	// on either branch: if and block open no region and the cost is static.
	pick := b.Func(i32, i32, nil,
		OpLocalGet, 0, OpIf, I32, OpI32Const, 1, OpElse, OpI32Const, 2, OpEnd, OpEnd)
	// caller: {i32.const, call, end} costs 4, plus pick's 8.
	caller := b.Func(nil, i32, nil, OpI32Const, 1, OpCall, byte(pick), OpEnd)
	b.Export("sum", KindFunc, sum)
	b.Export("nest", KindFunc, nest)
	b.Export("pick", KindFunc, pick)
	b.Export("caller", KindFunc, caller)
	return b.Bytes()
}

func TestFuelKnownBodyUsesDocumentedFuel(t *testing.T) {
	out := fuelMeter(t, fuelKnownModule())
	cases := []struct {
		export string
		args   []uint64
		want   int64
	}{
		{"sum", []uint64{1}, 4 + 11},
		{"sum", []uint64{10}, 4 + 11*10},
		{"nest", []uint64{1}, 3 + 31},
		{"nest", []uint64{4}, 3 + 31*4},
		{"pick", []uint64{0}, 8},
		{"pick", []uint64{1}, 8},
		{"caller", nil, 4 + 8},
	}
	for _, e := range engines(t) {
		fi := fuelInstantiate(t, e, out)
		for _, c := range cases {
			if got := fuelUsed(t, fi, c.export, c.args...); got != c.want {
				t.Errorf("%s %s%v: used %d fuel, want %d", e.name, c.export, c.args, got, c.want)
			}
		}
		if res := fi.call(t, fuelPlenty, "sum", 10); res.res[0] != 55 || res.depth != 0 {
			t.Errorf("%s: sum(10) = %v depth %d, want 55 depth 0", e.name, res.res, res.depth)
		}
	}
}

func TestFuelInfiniteLoopStopsAtTheSamePointOnBothEngines(t *testing.T) {
	var b Builder
	// Function {loop, end} costs 3; loop {br 0, end} costs 3 per turn.
	spin := b.Func(nil, nil, nil, OpLoop, Empty, OpBr, 0, OpEnd, OpEnd)
	b.Export("spin", KindFunc, spin)
	out := fuelMeter(t, b.Bytes())
	// 1,000,000 - 3 at entry, then -3 per turn: the first negative value
	// is 999,997 - 3*333,333 = -2.
	const budget, want = 1_000_000, -2
	for _, e := range engines(t) {
		res := fuelInstantiate(t, e, out).call(t, budget, "spin")
		if res.err == nil || !strings.Contains(res.err.Error(), "unreachable") {
			t.Fatalf("%s: spin: err %v, want an unreachable trap", e.name, res.err)
		}
		if res.fuel != want {
			t.Errorf("%s: fuel at trap %d, want %d", e.name, res.fuel, want)
		}
	}
}

func TestFuelEntryChargeTrapsWhenTheBudgetIsShort(t *testing.T) {
	out := fuelMeter(t, fuelKnownModule())
	for _, e := range engines(t) {
		fi := fuelInstantiate(t, e, out)
		if res := fi.call(t, 7, "pick", 0); res.err == nil || res.fuel != -1 {
			t.Errorf("%s: pick with 7 fuel: err %v fuel %d, want a trap at -1", e.name, res.err, res.fuel)
		}
		if res := fi.call(t, 8, "pick", 0); res.err != nil || res.fuel != 0 {
			t.Errorf("%s: pick with 8 fuel: err %v fuel %d, want success at 0", e.name, res.err, res.fuel)
		}
	}
}

const fuelDst = 1 << 20

// fuelBulkModule writes n bytes at fuelDst with memory.fill (0xAB),
// memory.copy (from the active 0xEE segment at 0) or memory.init (from a
// passive 0xCD segment, then data.drop).
func fuelBulkModule() []byte {
	var b Builder
	b.Memory(32, 32)
	b.Data(0, bytes.Repeat([]byte{0xEE}, fuelDst))
	b.DataPassive(bytes.Repeat([]byte{0xCD}, fuelDst))
	i32 := []byte{I32}
	dst := fuelI32(fuelDst)
	fill := b.Func(i32, nil, nil, fuelCat(dst, fuelI32(0xAB),
		[]byte{OpLocalGet, 0, PrefixFC, FCMemoryFill, 0, OpEnd})...)
	cp := b.Func(i32, nil, nil, fuelCat(dst, fuelI32(0),
		[]byte{OpLocalGet, 0, PrefixFC, FCMemoryCopy, 0, 0, OpEnd})...)
	ini := b.Func(i32, nil, nil, fuelCat(dst, fuelI32(0),
		[]byte{OpLocalGet, 0, PrefixFC, FCMemoryInit, 1, 0, PrefixFC, FCDataDrop, 1, OpEnd})...)
	b.Export("fill", KindFunc, fill)
	b.Export("copy", KindFunc, cp)
	b.Export("init", KindFunc, ini)
	b.Export("memory", KindMemory, 0)
	return b.Bytes()
}

func TestFuelBulkOpsChargeTheirLength(t *testing.T) {
	out := fuelMeter(t, fuelBulkModule())
	ops := []struct {
		export string
		static int64 // the function region's cost, by hand
		byte   byte  // what the op writes
	}{
		{"fill", 6, 0xAB}, // i32.const, i32.const, local.get, memory.fill, end
		{"copy", 6, 0xEE}, // i32.const, i32.const, local.get, memory.copy, end
		{"init", 7, 0xCD}, // i32.const, i32.const, local.get, memory.init, data.drop, end
	}
	for _, e := range engines(t) {
		for _, op := range ops {
			if got := fuelUsed(t, fuelInstantiate(t, e, out), op.export, 0); got != op.static {
				t.Errorf("%s %s(0): used %d, want %d", e.name, op.export, got, op.static)
			}
			for _, n := range []int64{1, 4096, 1 << 20} {
				fuelBulkCase(t, e, out, op.export, op.static, op.byte, n)
			}
		}
	}
}

// fuelBulkCase checks one op at length n: n more fuel than at n = 0; a
// budget one short traps at fuel -1 before any byte is written; the exact
// budget completes at fuel 0 with the bytes written.
func fuelBulkCase(t *testing.T, e testEngine, out []byte, export string, static int64, want byte, n int64) {
	t.Helper()
	if got := fuelUsed(t, fuelInstantiate(t, e, out), export, uint64(n)); got != static+n {
		t.Errorf("%s %s(%d): used %d, want %d (n more than at 0)", e.name, export, n, got, static+n)
	}
	fi := fuelInstantiate(t, e, out)
	res := fi.call(t, static+n-1, export, uint64(n))
	if res.err == nil || res.fuel != -1 {
		t.Errorf("%s %s(%d) one short: err %v fuel %d, want a trap at -1", e.name, export, n, res.err, res.fuel)
	}
	mem, _ := fi.mod.Memory().Read(fuelDst, uint32(n))
	if !bytes.Equal(mem, make([]byte, n)) {
		t.Errorf("%s %s(%d): memory written before the out-of-fuel trap", e.name, export, n)
	}
	fi = fuelInstantiate(t, e, out)
	res = fi.call(t, static+n, export, uint64(n))
	if res.err != nil || res.fuel != 0 {
		t.Errorf("%s %s(%d) exact budget: err %v fuel %d, want success at 0", e.name, export, n, res.err, res.fuel)
	}
	mem, _ = fi.mod.Memory().Read(fuelDst, uint32(n))
	if !bytes.Equal(mem, bytes.Repeat([]byte{want}, int(n))) {
		t.Errorf("%s %s(%d): memory not written with the exact budget", e.name, export, n)
	}
}

// The length is charged unsigned: 0xFFFFFFFF costs 4,294,967,295, never -1.
func TestFuelBulkLengthIsUnsigned(t *testing.T) {
	out := fuelMeter(t, fuelBulkModule())
	const budget = int64(1) << 20
	for _, e := range engines(t) {
		fi := fuelInstantiate(t, e, out)
		res := fi.call(t, budget, "fill", 0xFFFFFFFF)
		if want := budget - 6 - 0xFFFFFFFF; res.err == nil || res.fuel != want {
			t.Errorf("%s: fill(0xFFFFFFFF): err %v fuel %d, want a trap at %d", e.name, res.err, res.fuel, want)
		}
		mem, _ := fi.mod.Memory().Read(fuelDst, fuelDst)
		if !bytes.Equal(mem, make([]byte, fuelDst)) {
			t.Errorf("%s: memory written before the out-of-fuel trap", e.name)
		}
	}
}

func TestMeterAddsOneScratchLocalOnlyToBodiesWithBulkOps(t *testing.T) {
	var b Builder
	b.Memory(1, 1)
	i32 := []byte{I32}
	b.Func(i32, nil, nil, OpI32Const, 0, OpI32Const, 0, OpLocalGet, 0, PrefixFC, FCMemoryFill, 0,
		OpI32Const, 0, OpI32Const, 0, OpLocalGet, 0, PrefixFC, FCMemoryCopy, 0, 0, OpEnd)
	b.Func(i32, nil, []byte{I64}, OpEnd)
	m := fuelMetered(t, b.Bytes())
	if want := []wasmrewrite.LocalRun{{Count: 1, Type: I32}}; !slices.Equal(m.Bodies[0].Locals, want) {
		t.Errorf("bulk body locals %v, want %v", m.Bodies[0].Locals, want)
	}
	if want := []wasmrewrite.LocalRun{{Count: 1, Type: I64}}; !slices.Equal(m.Bodies[1].Locals, want) {
		t.Errorf("plain body locals %v, want %v", m.Bodies[1].Locals, want)
	}
}

// fuelControlModule covers return at open-block depths 0, 1, 2 and 4,
// inside a loop and an if/else, br to the function label (br and br_if),
// br_table into blocks and to the function label, and multi-value results.
func fuelControlModule() []byte {
	var b Builder
	i32 := []byte{I32}
	nested := b.Func(i32, i32, nil,
		OpBlock, Empty,
		OpBlock, Empty,
		OpLocalGet, 0, OpI32Eqz, OpBrIf, 0,
		OpLocalGet, 0, OpI32Const, 5, OpI32Eq, OpBrIf, 0,
		OpBlock, Empty,
		OpLocalGet, 0, OpI32Const, 1, OpI32Eq, OpIf, Empty, OpI32Const, 100, OpReturn, OpEnd,
		OpEnd,
		OpI32Const, 7, OpBr, 2,
		OpEnd,
		OpLocalGet, 0, OpI32Const, 5, OpI32Eq, OpBrIf, 0,
		OpI32Const, 42, OpReturn,
		OpEnd,
		OpI32Const, 9, OpEnd)
	retLoop := b.Func(i32, i32, nil,
		OpLoop, Empty,
		OpLocalGet, 0, OpI32Const, 1, OpI32Add, OpLocalTee, 0, OpI32Const, 10, 0x4A, // i32.gt_s
		OpIf, Empty, OpLocalGet, 0, OpReturn, OpEnd,
		OpBr, 0,
		OpEnd, OpI32Const, 0, OpEnd)
	ifElse := b.Func(i32, i32, nil,
		OpLocalGet, 0, OpIf, I32, OpI32Const, 1, OpReturn, OpElse, OpI32Const, 2, OpEnd, OpEnd)
	ret0 := b.Func(i32, i32, nil, OpLocalGet, 0, OpReturn, OpEnd)
	brFn := b.Func(i32, i32, nil,
		OpI32Const, 77, OpLocalGet, 0, OpBrIf, 0, OpDrop, OpI32Const, 88, OpEnd)
	table := b.Func(i32, i32, nil,
		OpBlock, Empty, OpBlock, Empty, OpBlock, Empty,
		OpLocalGet, 0, OpBrTable, 2, 0, 1, 2,
		OpEnd, OpI32Const, 10, OpReturn,
		OpEnd, OpI32Const, 20, OpReturn,
		OpEnd, OpI32Const, 30, OpEnd)
	tableFn := b.Func(i32, i32, nil,
		OpBlock, I32, OpI32Const, 5, OpLocalGet, 0, OpBrTable, 2, 0, 1, 0, OpEnd,
		OpI32Const, 10, OpI32Add, OpEnd)
	tmv := byte(b.Type(nil, []byte{I32, I64}))
	mv := b.Func(i32, []byte{I32, I64}, nil,
		OpLocalGet, 0, OpI32Eqz, OpIf, Empty, OpI32Const, 1, OpI64Const, 2, OpReturn, OpEnd,
		OpBlock, tmv, OpLocalGet, 0, OpI64Const, 5, OpEnd, OpEnd)
	mv2 := b.Func(i32, []byte{I64, I64}, nil,
		OpLocalGet, 0, OpI32Eqz, OpIf, Empty, OpI64Const, 3, OpI64Const, 4, OpReturn, OpEnd,
		OpI64Const, 6, OpI64Const, 7, OpEnd)
	for name, fn := range map[string]uint32{
		"nested": nested, "retloop": retLoop, "ifelse": ifElse, "ret0": ret0, "brfn": brFn,
		"table": table, "tablefn": tableFn, "mv": mv, "mv2": mv2,
	} {
		b.Export(name, KindFunc, fn)
	}
	return b.Bytes()
}

var fuelControlCalls = []call{
	{"nested", []uint64{0}}, {"nested", []uint64{1}}, {"nested", []uint64{2}}, {"nested", []uint64{5}},
	{"retloop", []uint64{0}}, {"retloop", []uint64{20}},
	{"ifelse", []uint64{0}}, {"ifelse", []uint64{1}},
	{"ret0", []uint64{33}},
	{"brfn", []uint64{0}}, {"brfn", []uint64{1}},
	{"table", []uint64{0}}, {"table", []uint64{1}}, {"table", []uint64{2}}, {"table", []uint64{9}},
	{"tablefn", []uint64{0}}, {"tablefn", []uint64{1}}, {"tablefn", []uint64{2}},
	{"mv", []uint64{0}}, {"mv", []uint64{3}},
	{"mv2", []uint64{0}}, {"mv2", []uint64{1}},
}

// fuelSameResults runs every call on the original and the metered module
// on each engine: equal results, and depth back at 0 after each call.
func fuelSameResults(t *testing.T, orig, metered []byte, calls []call) {
	t.Helper()
	for _, e := range engines(t) {
		fi := fuelInstantiate(t, e, metered)
		for _, c := range calls {
			want := run(t, e, orig, c.export, c.args...)
			got := fi.call(t, fuelPlenty, c.export, c.args...)
			if got.err != nil || !slices.Equal(got.res, want) || got.depth != 0 {
				t.Errorf("%s %s%v: got %v err %v depth %d, want %v depth 0",
					e.name, c.export, c.args, got.res, got.err, got.depth, want)
			}
		}
	}
}

func TestMeterKeepsControlFlowResults(t *testing.T) {
	orig := fuelControlModule()
	fuelSameResults(t, orig, fuelMeter(t, orig), fuelControlCalls)
}

func TestMeterKeepsRichModuleResults(t *testing.T) {
	orig := richModule()
	fuelSameResults(t, orig, fuelMeter(t, orig), richCalls)
}

func TestMeterAppendsAWrapperTypeOnlyWhenNoneMatches(t *testing.T) {
	orig, err := wasmrewrite.Decode(fuelControlModule())
	if err != nil {
		t.Fatal(err)
	}
	m := fuelMetered(t, fuelControlModule())
	// mv's [] -> [i32 i64] exists; mv2 needs [] -> [i64 i64].
	if len(m.Types) != len(orig.Types)+1 {
		t.Fatalf("types %d -> %d, want one appended", len(orig.Types), len(m.Types))
	}
	last := m.Types[len(m.Types)-1]
	if len(last.Params) != 0 || !bytes.Equal(last.Results, []byte{I64, I64}) {
		t.Errorf("appended type %+v, want [] -> [i64 i64]", last)
	}
}

// fuelImportModule calls a rimworld import and an existing global from
// two functions, one through call_indirect.
func fuelImportModule() []byte {
	var b Builder
	rnd := b.Import("rimworld", "rand", []byte{I32}, []byte{I32})
	b.Table(1)
	g := b.Global(I32, true, OpI32Const, 3, OpEnd)
	a := b.Func(nil, []byte{I32}, nil, OpGlobalGet, byte(g), OpCall, byte(rnd), OpEnd)
	tA := byte(b.Type(nil, []byte{I32}))
	c := b.Func(nil, []byte{I32}, nil, OpI32Const, 0, OpCallIndirect, tA, 0, OpGlobalSet, byte(g), OpGlobalGet, byte(g), OpEnd)
	b.Elem(a)
	b.Export("a", KindFunc, a)
	b.Export("c", KindFunc, c)
	return b.Bytes()
}

func TestMeterKeepsIndicesAndCompiles(t *testing.T) {
	for name, wasm := range map[string][]byte{"rich": richModule(), "import": fuelImportModule(), "control": fuelControlModule()} {
		orig, err := wasmrewrite.Decode(wasm)
		if err != nil {
			t.Fatal(err)
		}
		out := fuelMeter(t, wasm)
		m, err := wasmrewrite.Decode(out)
		if err != nil {
			t.Fatalf("%s: decode output: %v", name, err)
		}
		if !slices.EqualFunc(m.Imports, orig.Imports, func(a, b wasmrewrite.Import) bool {
			return a.Module == b.Module && a.Name == b.Name && a.Kind == b.Kind && a.Index == b.Index
		}) || !slices.Equal(m.Funcs, orig.Funcs) || len(m.Bodies) != len(orig.Bodies) {
			t.Errorf("%s: imports or functions changed", name)
		}
		if len(m.Globals) != len(orig.Globals)+2 || !slices.EqualFunc(m.Globals[:len(orig.Globals)], orig.Globals, fuelSameGlobal) {
			t.Errorf("%s: existing globals changed or more than the two reserved added", name)
		}
		if !slices.Equal(m.Exports[:len(orig.Exports)], orig.Exports) {
			t.Errorf("%s: existing exports changed", name)
		}
		for i := range orig.Types {
			if !bytes.Equal(m.Types[i].Params, orig.Types[i].Params) || !bytes.Equal(m.Types[i].Results, orig.Types[i].Results) {
				t.Errorf("%s: type %d changed", name, i)
			}
		}
		for i := range orig.Bodies {
			reserved := []uint64{uint64(orig.NumGlobals()), uint64(orig.NumGlobals()) + 1}
			if got, want := fuelRefs(t, m.Bodies[i].Code, reserved), fuelRefs(t, orig.Bodies[i].Code, nil); !slices.Equal(got, want) {
				t.Errorf("%s: body %d calls/globals %v, want %v plus fuel and depth", name, i, got, want)
			}
		}
		for _, e := range engines(t) {
			if err := compileErr(e, out); err != nil {
				t.Errorf("%s %s: output does not compile: %v", name, e.name, err)
			}
		}
	}
}

func fuelSameGlobal(a, b wasmrewrite.Global) bool {
	return a.Type == b.Type && a.Mutable == b.Mutable && bytes.Equal(a.Init, b.Init)
}

// fuelRefs lists the distinct call targets and the global indices other
// than the reserved ones that code refers to, sorted.
func fuelRefs(t *testing.T, code []byte, reserved []uint64) []string {
	t.Helper()
	var refs []string
	err := wasmrewrite.Walk(code, func(in wasmrewrite.Instr) error {
		switch {
		case in.Op == uint32(OpCall) || in.Op == uint32(OpCallIndirect):
			refs = append(refs, fmt.Sprintf("op 0x%x %d", in.Op, in.A))
		case (in.Op == uint32(OpGlobalGet) || in.Op == uint32(OpGlobalSet)) && !slices.Contains(reserved, in.A):
			refs = append(refs, fmt.Sprintf("global %d", in.A))
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	slices.Sort(refs)
	return slices.Compact(refs)
}

// A return, a loop and a bulk op in unreachable code still validate.
func TestMeterOutputValidatesInUnreachableCode(t *testing.T) {
	var b Builder
	b.Memory(1, 1)
	b.Func([]byte{I32}, []byte{I32}, nil,
		OpLocalGet, 0, OpReturn,
		OpI32Const, 0, OpI32Const, 0, OpLocalGet, 0, PrefixFC, FCMemoryFill, 0,
		OpLoop, Empty, OpBr, 0, OpEnd,
		OpReturn, OpUnreachable, OpEnd)
	out := fuelMeter(t, b.Bytes())
	for _, e := range engines(t) {
		if err := compileErr(e, out); err != nil {
			t.Errorf("%s: %v", e.name, err)
		}
	}
}

func TestMeterErrors(t *testing.T) {
	m, err := wasmrewrite.Decode(fuelKnownModule())
	if err != nil {
		t.Fatal(err)
	}
	if err := wasmrewrite.Meter(m, testConfig()); err == nil {
		t.Error("Meter before Reserve: no error")
	}
	if err := wasmrewrite.Reserve(m, testConfig()); err != nil {
		t.Fatal(err)
	}
	for _, c := range []wasmrewrite.Config{{StackBudget: 1 << 22}, {MaxDepth: 1000}} {
		if err := wasmrewrite.Meter(m, c); err == nil {
			t.Errorf("Meter with %+v: no error", c)
		}
	}
}

func TestFuelPrepareMetersThePipeline(t *testing.T) {
	out, err := wasmrewrite.Prepare(fuelKnownModule(), testConfig())
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range engines(t) {
		if got := fuelUsed(t, fuelInstantiate(t, e, out), "sum", 10); got != 114 {
			t.Errorf("%s: Prepare output used %d fuel for sum(10), want 114", e.name, got)
		}
	}
}
