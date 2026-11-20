// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package wasmrewrite_test

import (
	"bytes"
	"slices"
	"testing"

	"rimworld-go/internal/adapters/outbound/wasmhost/wasmrewrite"
	. "rimworld-go/internal/adapters/outbound/wasmhost/wasmtest"
)

// richModule exercises memory, a table, globals, data, a loop and an
// indirect call, and needs no imports so wazero can run it.
func richModule() []byte {
	var b Builder
	b.Memory(1, 2)
	b.Table(4)
	b.Global(I32, true, OpI32Const, 5, OpEnd)
	ii := []byte{I32, I32}
	add := b.Func(ii, []byte{I32}, nil, OpLocalGet, 0, OpLocalGet, 1, OpI32Add, OpEnd)
	sub := b.Func(ii, []byte{I32}, nil, OpLocalGet, 0, OpLocalGet, 1, OpI32Sub, OpEnd)
	calc := b.Func(nil, []byte{I32}, nil,
		OpI32Const, 0, OpI32Const, 7, OpI32Store, 2, 0,
		OpI32Const, 0, OpI32Load, 2, 0,
		OpGlobalGet, 0, OpI32Add, OpEnd)
	sum := b.Func([]byte{I32}, []byte{I32}, []byte{I32},
		OpLoop, Empty,
		OpLocalGet, 1, OpLocalGet, 0, OpI32Add, OpLocalSet, 1,
		OpLocalGet, 0, OpI32Const, 1, OpI32Sub, OpLocalTee, 0, OpBrIf, 0,
		OpEnd, OpLocalGet, 1, OpEnd)
	tAdd := byte(b.Type(ii, []byte{I32}))
	ind := b.Func([]byte{I32}, []byte{I32}, nil,
		OpI32Const, 3, OpI32Const, 4, OpLocalGet, 0, OpCallIndirect, tAdd, 0, OpEnd)
	b.Elem(add, sub)
	b.Data(0, []byte("hi"))
	b.Export("add", KindFunc, add)
	b.Export("calc", KindFunc, calc)
	b.Export("sum", KindFunc, sum)
	b.Export("ind", KindFunc, ind)
	b.Export("memory", KindMemory, 0)
	return b.Bytes()
}

type call struct {
	export string
	args   []uint64
}

var richCalls = []call{
	{"add", []uint64{3, 4}}, {"calc", nil}, {"sum", []uint64{10}}, {"ind", []uint64{0}}, {"ind", []uint64{1}},
}

func assertSameResults(t *testing.T, a, b []byte, calls []call) {
	t.Helper()
	for _, e := range engines(t) {
		if err := compileErr(e, b); err != nil {
			t.Fatalf("%s: output does not compile: %v", e.name, err)
		}
		for _, c := range calls {
			want := run(t, e, a, c.export, c.args...)
			got := run(t, e, b, c.export, c.args...)
			if !slices.Equal(want, got) {
				t.Errorf("%s %s%v: got %v want %v", e.name, c.export, c.args, got, want)
			}
		}
	}
}

func roundTrip(t *testing.T, in []byte) []byte {
	t.Helper()
	m, err := wasmrewrite.Decode(in)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	return m.Encode()
}

func TestRoundTripBuilderModule(t *testing.T) {
	in := richModule()
	out := roundTrip(t, in)
	assertSameResults(t, in, out, richCalls)
	if want := []uint64{12}; !slices.Equal(run(t, engines(t)[0], out, "calc"), want) {
		t.Error("calc != 12")
	}
	if !bytes.Equal(out, roundTrip(t, out)) {
		t.Error("encode is not a fixed point")
	}
}

// handModule is written byte by byte, with a five-byte (overlong) type
// count and a memory section that must be copied raw.
func handModule(extra ...[]byte) []byte {
	m := []byte("\x00asm\x01\x00\x00\x00")
	for _, s := range extra {
		m = append(m, s...)
	}
	return m
}

func sec(id byte, body ...byte) []byte {
	return append([]byte{id, byte(len(body))}, body...)
}

var (
	hType  = sec(1, 0x81, 0x80, 0x80, 0x80, 0x00, 0x60, 0x00, 0x01, 0x7F)
	hFunc  = sec(3, 0x01, 0x00)
	hMem   = sec(5, 0x01, 0x00, 0x81, 0x80, 0x80, 0x80, 0x00)
	hExp   = sec(7, 0x01, 0x01, 'f', 0x00, 0x00)
	hCode  = sec(10, 0x01, 0x04, 0x00, 0x41, 0x2A, 0x0B)
	hCust  = sec(0, 0x01, 'x', 0xAA)
	hBytes = handModule(hType, hFunc, hMem, hExp, hCode)
)

func TestRoundTripHandWritten(t *testing.T) {
	out := roundTrip(t, hBytes)
	if len(out) >= len(hBytes) {
		t.Errorf("overlong LEB was not normalised: %d >= %d", len(out), len(hBytes))
	}
	if !bytes.Contains(out, hMem[2:]) {
		t.Error("memory section was not copied raw")
	}
	for _, e := range engines(t) {
		if got := run(t, e, out, "f"); got[0] != 42 {
			t.Errorf("%s: f() = %v", e.name, got)
		}
	}
}

func TestCustomSectionsDroppedAfterCheck(t *testing.T) {
	withCustom := handModule(hCust, hType, hFunc, hExp, hCode, hCust)
	plain := handModule(hType, hFunc, hExp, hCode)
	if !bytes.Equal(roundTrip(t, withCustom), roundTrip(t, plain)) {
		t.Error("custom sections changed the output")
	}
}

func TestDecodeErrors(t *testing.T) {
	cases := map[string][]byte{
		"empty":             nil,
		"short header":      []byte("\x00asm\x01"),
		"bad magic":         append([]byte("\x01asm\x01\x00\x00\x00"), hType...),
		"bad version":       []byte("\x00asm\x02\x00\x00\x00"),
		"duplicate type":    handModule(hType, hType),
		"out of order":      handModule(hFunc, hType),
		"unknown id":        handModule(sec(13, 0)),
		"size past end":     handModule([]byte{1, 50, 0}),
		"truncated size":    handModule([]byte{1, 0x80}),
		"custom name long":  handModule(sec(0, 0x05, 'a')),
		"custom name utf8":  handModule(sec(0, 0x01, 0xff)),
		"func without code": handModule(hType, hFunc),
		"datacount only":    handModule(hType, sec(12, 0x01)),
		"bad valtype":       handModule(sec(1, 0x01, 0x60, 0x01, 0x55, 0x00)),
		"v128 valtype":      handModule(sec(1, 0x01, 0x60, 0x01, 0x7B, 0x00)),
		"bad type form":     handModule(sec(1, 0x01, 0x61, 0x00, 0x00)),
		"bad export kind":   handModule(sec(7, 0x01, 0x01, 'f', 0x09, 0x00)),
		"export name utf8":  handModule(sec(7, 0x01, 0x01, 0xff, 0x00, 0x00)),
		"trailing bytes":    handModule(sec(3, 0x01, 0x00, 0x00)),
		"body no end":       handModule(hType, hFunc, sec(10, 0x01, 0x03, 0x00, 0x41, 0x2A)),
		"empty body":        handModule(hType, hFunc, sec(10, 0x01, 0x01, 0x00)),
		"locals overflow":   handModule(hType, hFunc, sec(10, 0x01, 0x0F, 0x02, 0xff, 0xff, 0xff, 0xff, 0x0f, 0x7F, 0xff, 0xff, 0xff, 0xff, 0x0f, 0x7F, 0x0B)),
		"bad global mut":    handModule(sec(6, 0x01, 0x7F, 0x05, 0x41, 0x00, 0x0B)),
		"bad global init":   handModule(sec(6, 0x01, 0x7F, 0x00, 0x6A, 0x0B)),
		"empty global init": handModule(sec(6, 0x01, 0x7F, 0x00, 0x0B)),
		"two-op init":       handModule(sec(6, 0x01, 0x7F, 0x00, 0x41, 0x00, 0x41, 0x00, 0x0B)),
		"unterminated init": handModule(sec(6, 0x01, 0x7F, 0x00, 0x41, 0x00)),
		"bad limits flag":   handModule(sec(5, 0x01, 0x09, 0x00)),
		"bad table limits":  handModule(sec(4, 0x01, 0x70, 0x02, 0x00)),
		"bad table type":    handModule(sec(4, 0x01, 0x7F, 0x00, 0x00)),
		"bad import kind":   handModule(sec(2, 0x01, 0x01, 'a', 0x01, 'b', 0x07)),
		"import global mut": handModule(sec(2, 0x01, 0x01, 'a', 0x01, 'b', 0x03, 0x7F, 0x02)),
		"huge vector":       handModule(sec(3, 0x7f)),
	}
	for name, in := range cases {
		if _, err := wasmrewrite.Decode(in); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

func TestDecodeViews(t *testing.T) {
	var b Builder
	b.Import("rimworld", "f", []byte{I32}, nil)
	b.ImportRaw("m", "g", KindGlobal, I32, 0)
	b.ImportRaw("m", "t", KindTable, FuncRef, 0, 1)
	b.ImportRaw("m", "mem", KindMemory, 1, 1, 2)
	b.Memory(1, 3)
	b.Table(2)
	b.Elem()
	b.ElemPassive()
	b.Data(4, []byte{1})
	b.DataPassive([]byte{2})
	b.Start(0)
	b.Func(nil, nil, []byte{I32, I32, I64}, OpEnd)
	m, err := wasmrewrite.Decode(b.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Memories) != 1 || m.Memories[0].Min != 1 || m.Memories[0].Max != 3 || !m.Memories[0].HasMax {
		t.Errorf("memories %+v", m.Memories)
	}
	if m.NumTables != 1 || m.NumElems != 2 || m.NumDatas != 2 || !m.HasDataCount || m.DataCount != 2 || !m.HasStart {
		t.Errorf("counts %+v", m)
	}
	if m.NumFuncs() != 2 || m.NumGlobals() != 1 {
		t.Errorf("spaces %d %d", m.NumFuncs(), m.NumGlobals())
	}
	if len(m.Bodies[0].Locals) != 2 {
		t.Errorf("locals %+v", m.Bodies[0].Locals)
	}
	out, err := wasmrewrite.Decode(m.Encode())
	if err != nil || len(out.Imports) != 4 || string(out.Imports[3].Desc) != "\x01\x01\x02" {
		t.Errorf("import round trip: %v %+v", err, out)
	}
}

func TestAddHelpersAppendWithoutShifting(t *testing.T) {
	m, err := wasmrewrite.Decode(richModule())
	if err != nil {
		t.Fatal(err)
	}
	nTypes, nGlobals, nExports := len(m.Types), m.NumGlobals(), len(m.Exports)
	if got := m.AddType([]byte{I64}, []byte{I64}); got != uint32(nTypes) {
		t.Errorf("AddType = %d", got)
	}
	if got := m.AddGlobal(wasmrewrite.Global{Type: I64, Mutable: true, Init: []byte{OpI64Const, 0, OpEnd}}); got != nGlobals {
		t.Errorf("AddGlobal = %d", got)
	}
	m.AddExport(wasmrewrite.Export{Name: "g", Kind: KindGlobal, Index: nGlobals})
	if len(m.Exports) != nExports+1 {
		t.Error("AddExport")
	}
	// sum has 1 param and one i32 local; an i32 local merges, an f64 does not.
	sum := uint32(3)
	if got := m.AddLocal(sum, I32); got != 2 {
		t.Errorf("AddLocal i32 = %d", got)
	}
	if got := m.AddLocal(sum, F64); got != 3 {
		t.Errorf("AddLocal f64 = %d", got)
	}
	if got := m.AddLocal(0, I32); got != 2 { // add has two params, no locals
		t.Errorf("AddLocal on empty = %d", got)
	}
	out := m.Encode()
	for _, e := range engines(t) {
		if err := compileErr(e, out); err != nil {
			t.Fatalf("%s: %v", e.name, err)
		}
		if got := run(t, e, out, "sum", 10); got[0] != 55 {
			t.Errorf("sum after rewrite = %v", got)
		}
	}
}

func TestEncodeEmptyModule(t *testing.T) {
	out := (&wasmrewrite.Module{}).Encode()
	if string(out) != "\x00asm\x01\x00\x00\x00" {
		t.Errorf("%x", out)
	}
	if _, err := wasmrewrite.Decode(out); err != nil {
		t.Error(err)
	}
}

// C5: no panic on any truncation of a valid module.
func TestNoPanicOnTruncation(t *testing.T) {
	in := richModule()
	for n := 0; n <= len(in); n++ {
		prefix := in[:n]
		m, err := wasmrewrite.Decode(prefix)
		if n == len(in) && err != nil {
			t.Fatalf("full module rejected: %v", err)
		}
		if n < 8 && err == nil {
			t.Fatalf("prefix %d decoded", n)
		}
		_ = m
		_, _ = wasmrewrite.Prepare(prefix, testConfig())
	}
	// Every prefix of every body through Walk.
	m, _ := wasmrewrite.Decode(in)
	for _, b := range m.Bodies {
		for n := 0; n <= len(b.Code); n++ {
			_ = wasmrewrite.Walk(b.Code[:n], func(wasmrewrite.Instr) error { return nil })
		}
	}
}
