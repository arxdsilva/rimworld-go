// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package wasmrewrite_test

import (
	"bytes"
	"math"
	"testing"

	"rimworld-go/internal/adapters/outbound/wasmhost/wasmrewrite"
	"rimworld-go/internal/adapters/outbound/wasmhost/wasmtest"
)

const (
	nanCanon32 = 0x7FC00000
	nanCanon64 = 0x7FF8000000000000
	// Payload NaNs: sign set, signalling pattern, payload bits.
	nanPay32 = 0xFFA00001
	nanPay64 = 0xFFF4000000000001
	nanPos32 = 0x7FA00001 // positive payload NaN
	nanPos64 = 0x7FF4000000000001
)

type nanOp struct {
	name string
	op   byte
}

var (
	nanBin32 = []nanOp{{"f32.add", wasmtest.OpF32Add}, {"f32.sub", wasmtest.OpF32Sub}, {"f32.mul", wasmtest.OpF32Mul},
		{"f32.div", wasmtest.OpF32Div}, {"f32.min", wasmtest.OpF32Min}, {"f32.max", wasmtest.OpF32Max}}
	nanUn32 = []nanOp{{"f32.sqrt", wasmtest.OpF32Sqrt}, {"f32.ceil", wasmtest.OpF32Ceil}, {"f32.floor", wasmtest.OpF32Floor},
		{"f32.trunc", wasmtest.OpF32Trunc}, {"f32.nearest", wasmtest.OpF32Nearest}}
	nanBin64 = []nanOp{{"f64.add", wasmtest.OpF64Add}, {"f64.sub", wasmtest.OpF64Sub}, {"f64.mul", wasmtest.OpF64Mul},
		{"f64.div", wasmtest.OpF64Div}, {"f64.min", wasmtest.OpF64Min}, {"f64.max", wasmtest.OpF64Max}}
	nanUn64 = []nanOp{{"f64.sqrt", wasmtest.OpF64Sqrt}, {"f64.ceil", wasmtest.OpF64Ceil}, {"f64.floor", wasmtest.OpF64Floor},
		{"f64.trunc", wasmtest.OpF64Trunc}, {"f64.nearest", wasmtest.OpF64Nearest}}
)

// nanModule exports one function per op, named by the op, taking and
// returning raw bits: binary ops (iN, iN) -> iN, unary (iN) -> iN, and
// "f32.demote_f64" (i64) -> i32, "f64.promote_f32" (i32) -> i64.
func nanModule() []byte {
	var b wasmtest.Builder
	bin := func(ops []nanOp, it, ft, reinTo, reinFrom byte) {
		for _, o := range ops {
			f := b.Func([]byte{it, it}, []byte{it}, nil,
				wasmtest.OpLocalGet, 0, reinFrom, wasmtest.OpLocalGet, 1, reinFrom, o.op, reinTo, wasmtest.OpEnd)
			b.Export(o.name, wasmtest.KindFunc, f)
		}
	}
	un := func(ops []nanOp, it, ft, reinTo, reinFrom byte) {
		for _, o := range ops {
			f := b.Func([]byte{it}, []byte{it}, nil, wasmtest.OpLocalGet, 0, reinFrom, o.op, reinTo, wasmtest.OpEnd)
			b.Export(o.name, wasmtest.KindFunc, f)
		}
	}
	bin(nanBin32, wasmtest.I32, wasmtest.F32, wasmtest.OpI32ReinterpretF32, wasmtest.OpF32ReinterpretI32)
	un(nanUn32, wasmtest.I32, wasmtest.F32, wasmtest.OpI32ReinterpretF32, wasmtest.OpF32ReinterpretI32)
	bin(nanBin64, wasmtest.I64, wasmtest.F64, wasmtest.OpI64ReinterpretF64, wasmtest.OpF64ReinterpretI64)
	un(nanUn64, wasmtest.I64, wasmtest.F64, wasmtest.OpI64ReinterpretF64, wasmtest.OpF64ReinterpretI64)
	d := b.Func([]byte{wasmtest.I64}, []byte{wasmtest.I32}, nil,
		wasmtest.OpLocalGet, 0, wasmtest.OpF64ReinterpretI64, wasmtest.OpF32DemoteF64, wasmtest.OpI32ReinterpretF32, wasmtest.OpEnd)
	b.Export("f32.demote_f64", wasmtest.KindFunc, d)
	p := b.Func([]byte{wasmtest.I32}, []byte{wasmtest.I64}, nil,
		wasmtest.OpLocalGet, 0, wasmtest.OpF32ReinterpretI32, wasmtest.OpF64PromoteF32, wasmtest.OpI64ReinterpretF64, wasmtest.OpEnd)
	b.Export("f64.promote_f32", wasmtest.KindFunc, p)
	return b.Bytes()
}

func nanRewrite(t *testing.T, wasm []byte) []byte {
	t.Helper()
	m, err := wasmrewrite.Decode(wasm)
	if err != nil {
		t.Fatal(err)
	}
	if err := wasmrewrite.CanonicaliseNaN(m); err != nil {
		t.Fatal(err)
	}
	return m.Encode()
}

type nanCase struct {
	fn   string
	args []uint64
	desc string
}

func f32b(f float32) uint64 { return uint64(math.Float32bits(f)) }
func f64b(f float64) uint64 { return math.Float64bits(f) }

// nanCases are inputs that make every covered op produce a NaN.
func nanCases() []nanCase {
	var cs []nanCase
	inf32, ninf32 := f32b(float32(math.Inf(1))), f32b(float32(math.Inf(-1)))
	inf64, ninf64 := f64b(math.Inf(1)), f64b(math.Inf(-1))
	for _, o := range nanBin32 {
		cs = append(cs,
			nanCase{o.name, []uint64{nanPay32, f32b(1)}, "payload NaN lhs"},
			nanCase{o.name, []uint64{f32b(1), nanPay32}, "payload NaN rhs"},
			nanCase{o.name, []uint64{nanPos32, nanPay32}, "two payload NaNs"})
	}
	cs = append(cs,
		nanCase{"f32.div", []uint64{0, 0}, "0/0"},
		nanCase{"f32.sub", []uint64{inf32, inf32}, "inf-inf"},
		nanCase{"f32.add", []uint64{inf32, ninf32}, "inf+-inf"},
		nanCase{"f32.mul", []uint64{0, inf32}, "0*inf"})
	for _, o := range nanUn32 {
		cs = append(cs, nanCase{o.name, []uint64{nanPay32}, "negative payload NaN"},
			nanCase{o.name, []uint64{nanPos32}, "positive payload NaN"})
	}
	cs = append(cs, nanCase{"f32.sqrt", []uint64{f32b(-1)}, "sqrt(-1)"})
	for _, o := range nanBin64 {
		cs = append(cs,
			nanCase{o.name, []uint64{nanPay64, f64b(1)}, "payload NaN lhs"},
			nanCase{o.name, []uint64{f64b(1), nanPay64}, "payload NaN rhs"},
			nanCase{o.name, []uint64{nanPos64, nanPay64}, "two payload NaNs"})
	}
	cs = append(cs,
		nanCase{"f64.div", []uint64{0, 0}, "0/0"},
		nanCase{"f64.sub", []uint64{inf64, inf64}, "inf-inf"},
		nanCase{"f64.add", []uint64{inf64, ninf64}, "inf+-inf"},
		nanCase{"f64.mul", []uint64{0, inf64}, "0*inf"})
	for _, o := range nanUn64 {
		cs = append(cs, nanCase{o.name, []uint64{nanPay64}, "negative payload NaN"},
			nanCase{o.name, []uint64{nanPos64}, "positive payload NaN"})
	}
	cs = append(cs, nanCase{"f64.sqrt", []uint64{f64b(-1)}, "sqrt(-1)"},
		nanCase{"f32.demote_f64", []uint64{nanPay64}, "demote payload NaN"},
		nanCase{"f32.demote_f64", []uint64{nanPos64}, "demote positive payload NaN"},
		nanCase{"f64.promote_f32", []uint64{nanPay32}, "promote payload NaN"},
		nanCase{"f64.promote_f32", []uint64{nanPos32}, "promote positive payload NaN"})
	return cs
}

func nanCanonOf(fn string) uint64 {
	if fn == "f32.demote_f64" || fn[:3] == "f32" {
		return nanCanon32
	}
	return nanCanon64
}

func nanMask(fn string, v uint64) uint64 {
	if nanCanonOf(fn) == nanCanon32 {
		return v & 0xFFFFFFFF
	}
	return v
}

func TestNaNCanonicalisesEveryCoveredOp(t *testing.T) { // C1
	wasm := nanRewrite(t, nanModule())
	for _, e := range engines(t) {
		for _, c := range nanCases() {
			got := nanMask(c.fn, run(t, e, wasm, c.fn, c.args...)[0])
			if want := nanCanonOf(c.fn); got != want {
				t.Errorf("%s %s (%s): bits %#x, want %#x", e.name, c.fn, c.desc, got, want)
			}
		}
	}
}

func TestNaNWithoutPassSomeCaseIsNotCanonical(t *testing.T) { // C2
	orig := nanModule()
	for _, e := range engines(t) {
		bad := 0
		for _, c := range nanCases() {
			if nanMask(c.fn, run(t, e, orig, c.fn, c.args...)[0]) != nanCanonOf(c.fn) {
				bad++
			}
		}
		if bad == 0 {
			t.Errorf("%s: every case is canonical without the pass; the test could not catch a missing insertion", e.name)
		}
	}
}

func TestNaNOrdinaryValuesUnchanged(t *testing.T) { // C3
	orig, wasm := nanModule(), nanRewrite(t, nanModule())
	type row struct {
		fn   string
		args []uint64
		want uint64
	}
	rows := []row{
		{"f32.add", []uint64{f32b(1.5), f32b(2.25)}, f32b(3.75)},
		{"f32.sub", []uint64{f32b(1), f32b(3)}, f32b(-2)},
		{"f32.mul", []uint64{f32b(-2), f32b(4)}, f32b(-8)},
		{"f32.div", []uint64{f32b(1), f32b(4)}, f32b(0.25)},
		{"f32.div", []uint64{f32b(1), 0}, f32b(float32(math.Inf(1)))},
		{"f32.min", []uint64{f32b(1), f32b(2)}, f32b(1)},
		{"f32.max", []uint64{f32b(1), f32b(2)}, f32b(2)},
		{"f32.sqrt", []uint64{f32b(9)}, f32b(3)},
		{"f32.ceil", []uint64{f32b(1.2)}, f32b(2)},
		{"f32.floor", []uint64{f32b(-1.2)}, f32b(-2)},
		{"f32.trunc", []uint64{f32b(-1.7)}, f32b(-1)},
		{"f32.nearest", []uint64{f32b(2.5)}, f32b(2)},
		{"f64.add", []uint64{f64b(1.5), f64b(2.25)}, f64b(3.75)},
		{"f64.sub", []uint64{f64b(1), f64b(3)}, f64b(-2)},
		{"f64.mul", []uint64{f64b(-2), f64b(4)}, f64b(-8)},
		{"f64.div", []uint64{f64b(1), f64b(4)}, f64b(0.25)},
		{"f64.min", []uint64{f64b(1), f64b(2)}, f64b(1)},
		{"f64.max", []uint64{f64b(1), f64b(2)}, f64b(2)},
		{"f64.sqrt", []uint64{f64b(9)}, f64b(3)},
		{"f64.ceil", []uint64{f64b(1.2)}, f64b(2)},
		{"f64.floor", []uint64{f64b(-1.2)}, f64b(-2)},
		{"f64.trunc", []uint64{f64b(-1.7)}, f64b(-1)},
		{"f64.nearest", []uint64{f64b(3.5)}, f64b(4)},
		{"f32.demote_f64", []uint64{f64b(1.5)}, f32b(1.5)},
		{"f64.promote_f32", []uint64{f32b(1.5)}, f64b(1.5)},
	}
	for _, e := range engines(t) {
		for _, r := range rows {
			got := run(t, e, wasm, r.fn, r.args...)[0]
			before := run(t, e, orig, r.fn, r.args...)[0]
			if nanMask(r.fn, got) != r.want || got != before {
				t.Errorf("%s %s%v: got %#x (original %#x), want %#x", e.name, r.fn, r.args, got, before, r.want)
			}
		}
	}
}

func TestNaNLeavesSignOpsAlone(t *testing.T) { // C4
	var b wasmtest.Builder
	un := func(op byte) uint32 {
		return b.Func([]byte{wasmtest.I32}, []byte{wasmtest.I32}, nil, wasmtest.OpLocalGet, 0,
			wasmtest.OpF32ReinterpretI32, op, wasmtest.OpI32ReinterpretF32, wasmtest.OpEnd)
	}
	b.Export("neg", wasmtest.KindFunc, un(wasmtest.OpF32Neg))
	b.Export("abs", wasmtest.KindFunc, un(wasmtest.OpF32Abs))
	b.Export("copysign", wasmtest.KindFunc, b.Func([]byte{wasmtest.I32, wasmtest.I32}, []byte{wasmtest.I32}, nil,
		wasmtest.OpLocalGet, 0, wasmtest.OpF32ReinterpretI32, wasmtest.OpLocalGet, 1, wasmtest.OpF32ReinterpretI32,
		wasmtest.OpF32Copysign, wasmtest.OpI32ReinterpretF32, wasmtest.OpEnd))
	b.Export("neg64", wasmtest.KindFunc, b.Func([]byte{wasmtest.I64}, []byte{wasmtest.I64}, nil, wasmtest.OpLocalGet, 0,
		wasmtest.OpF64ReinterpretI64, wasmtest.OpF64Neg, wasmtest.OpI64ReinterpretF64, wasmtest.OpEnd))
	b.Export("abs64", wasmtest.KindFunc, b.Func([]byte{wasmtest.I64}, []byte{wasmtest.I64}, nil, wasmtest.OpLocalGet, 0,
		wasmtest.OpF64ReinterpretI64, wasmtest.OpF64Abs, wasmtest.OpI64ReinterpretF64, wasmtest.OpEnd))
	orig := b.Bytes()
	m, err := wasmrewrite.Decode(orig)
	if err != nil {
		t.Fatal(err)
	}
	if err := wasmrewrite.CanonicaliseNaN(m); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(m.Encode(), orig) {
		t.Error("a module with only neg/abs/copysign changed")
	}
	wasm := m.Encode()
	for _, e := range engines(t) {
		for _, c := range []struct {
			fn   string
			args []uint64
			want uint64
		}{
			{"neg", []uint64{nanPay32}, nanPos32},
			{"neg", []uint64{nanPos32}, nanPay32},
			{"abs", []uint64{nanPay32}, nanPos32},
			{"copysign", []uint64{nanPos32, nanPay32}, nanPay32},
			{"copysign", []uint64{nanPay32, nanPos32}, nanPos32},
			{"neg64", []uint64{nanPay64}, nanPos64},
			{"abs64", []uint64{nanPay64}, nanPos64},
		} {
			if got := run(t, e, wasm, c.fn, c.args...)[0]; got != c.want {
				t.Errorf("%s %s: bits %#x, want %#x", e.name, c.fn, got, c.want)
			}
		}
	}
}

// nanBitsModule exports "f32" and "f64": the payload NaN built by
// reinterpret, then `x + 0`.
func nanSLEB(v int64) []byte {
	var out []byte
	for {
		c := byte(v & 0x7F)
		v >>= 7
		if (v == 0 && c&0x40 == 0) || (v == -1 && c&0x40 != 0) {
			return append(out, c)
		}
		out = append(out, c|0x80)
	}
}

var (
	nanPay32v uint32 = nanPay32
	nanPay64v uint64 = nanPay64
)

func nanCat(parts ...[]byte) []byte { return bytes.Join(parts, nil) }

func nanBitsModule() []byte {
	var b wasmtest.Builder
	b.Export("f32", wasmtest.KindFunc, b.Func(nil, []byte{wasmtest.I32}, nil, nanCat(
		[]byte{wasmtest.OpI32Const}, nanSLEB(int64(int32(nanPay32v))),
		[]byte{wasmtest.OpF32ReinterpretI32, wasmtest.OpF32Const, 0, 0, 0, 0, wasmtest.OpF32Add,
			wasmtest.OpI32ReinterpretF32, wasmtest.OpEnd})...))
	b.Export("f64", wasmtest.KindFunc, b.Func(nil, []byte{wasmtest.I64}, nil, nanCat(
		[]byte{wasmtest.OpI64Const}, nanSLEB(int64(nanPay64v)),
		[]byte{wasmtest.OpF64ReinterpretI64, wasmtest.OpF64Const, 0, 0, 0, 0, 0, 0, 0, 0, wasmtest.OpF64Add,
			wasmtest.OpI64ReinterpretF64, wasmtest.OpEnd})...))
	return b.Bytes()
}

func TestNaNPayloadBuiltByReinterpret(t *testing.T) { // C5
	orig, wasm := nanBitsModule(), nanRewrite(t, nanBitsModule())
	for _, e := range engines(t) {
		if got := run(t, e, orig, "f32")[0]; got == nanCanon32 {
			t.Errorf("%s: f32 before the pass is already canonical (%#x)", e.name, got)
		}
		if got := run(t, e, orig, "f64")[0]; got == nanCanon64 {
			t.Errorf("%s: f64 before the pass is already canonical (%#x)", e.name, got)
		}
		if got := run(t, e, wasm, "f32")[0]; got != nanCanon32 {
			t.Errorf("%s: f32 after the pass %#x, want %#x", e.name, got, nanCanon32)
		}
		if got := run(t, e, wasm, "f64")[0]; got != nanCanon64 {
			t.Errorf("%s: f64 after the pass %#x, want %#x", e.name, got, uint64(nanCanon64))
		}
	}
}

func TestNaNScratchLocalsOnePerTypeOnlyWhenNeeded(t *testing.T) {
	var b wasmtest.Builder
	b.Func(nil, nil, []byte{wasmtest.I32}, wasmtest.OpEnd) // no float ops
	b.Func([]byte{wasmtest.F32}, []byte{wasmtest.F32}, nil,
		wasmtest.OpLocalGet, 0, wasmtest.OpLocalGet, 0, wasmtest.OpF32Add, wasmtest.OpLocalGet, 0, wasmtest.OpF32Mul, wasmtest.OpEnd)
	b.Func([]byte{wasmtest.F64}, []byte{wasmtest.F32}, nil,
		wasmtest.OpLocalGet, 0, wasmtest.OpF32DemoteF64, wasmtest.OpEnd)
	m, err := wasmrewrite.Decode(b.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	before0 := m.Bodies[0].Code
	if err := wasmrewrite.CanonicaliseNaN(m); err != nil {
		t.Fatal(err)
	}
	count := func(i int) (n uint32) {
		for _, r := range m.Bodies[i].Locals {
			n += r.Count
		}
		return n
	}
	if !bytes.Equal(m.Bodies[0].Code, before0) || count(0) != 1 {
		t.Errorf("float-free body changed: locals %d", count(0))
	}
	if count(1) != 1 { // two covered f32 ops share one f32 local
		t.Errorf("body 1 locals %d, want 1", count(1))
	}
	if count(2) != 1 {
		t.Errorf("body 2 locals %d, want 1", count(2))
	}
	for _, e := range engines(t) {
		if err := compileErr(e, m.Encode()); err != nil {
			t.Errorf("%s: rewritten module does not compile: %v", e.name, err)
		}
	}
}

func TestNaNValidInUnreachableCodeAndWithBranches(t *testing.T) {
	var b wasmtest.Builder
	b.Func(nil, []byte{wasmtest.F32}, nil, wasmtest.OpUnreachable, wasmtest.OpF32Add, wasmtest.OpEnd)
	b.Func(nil, []byte{wasmtest.F64}, nil, wasmtest.OpUnreachable, wasmtest.OpF64Sqrt, wasmtest.OpEnd)
	b.Func(nil, []byte{wasmtest.F32}, nil, // block with br after a covered op, then promote/demote chain
		wasmtest.OpBlock, wasmtest.F32, wasmtest.OpF32Const, 0, 0, 0x80, 0x3F, wasmtest.OpF32Const, 0, 0, 0, 0,
		wasmtest.OpF32Div, wasmtest.OpBr, 0, wasmtest.OpEnd, wasmtest.OpF64PromoteF32, wasmtest.OpF32DemoteF64, wasmtest.OpEnd)
	wasm := nanRewrite(t, b.Bytes())
	for _, e := range engines(t) {
		if err := compileErr(e, wasm); err != nil {
			t.Errorf("%s: %v", e.name, err)
		}
	}
}

func TestNaNRejectsMalformedBody(t *testing.T) {
	var b wasmtest.Builder
	b.Func(nil, nil, nil, 0xFF, wasmtest.OpEnd)
	m, err := wasmrewrite.Decode(b.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if err := wasmrewrite.CanonicaliseNaN(m); err == nil {
		t.Error("an undecodable body was accepted")
	}
}

// Prepare runs the NaN pass: the full pipeline's output is canonical.
func TestCanonicalNaNThroughPrepare(t *testing.T) {
	out, err := wasmrewrite.Prepare(nanBitsModule(), testConfig())
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	for _, e := range engines(t) {
		if got := run(t, e, out, "f32")[0]; got != nanCanon32 {
			t.Errorf("%s: Prepare output f32 %#x", e.name, got)
		}
	}
}
