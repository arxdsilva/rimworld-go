// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package wasmrewrite_test

import (
	"bytes"
	"errors"
	"slices"
	"strings"
	"testing"

	"rimworld-go/internal/adapters/outbound/wasmhost/wasmrewrite"
	. "rimworld-go/internal/adapters/outbound/wasmhost/wasmtest"
)

func oneFunc(code ...byte) []byte {
	var b Builder
	b.Func(nil, nil, nil, code...)
	return b.Bytes()
}

func rejectRule(t *testing.T, name string, wasm []byte, rule string) {
	t.Helper()
	_, err := wasmrewrite.Prepare(wasm, testConfig())
	var re *wasmrewrite.RejectError
	if !errors.As(err, &re) {
		t.Errorf("%s: want RejectError, got %v", name, err)
		return
	}
	if re.Rule != rule || !strings.Contains(err.Error(), rule) {
		t.Errorf("%s: rule %q (%v), want %q", name, re.Rule, err, rule)
	}
}

// C4: each rejection names its rule.
func TestCheckRejects(t *testing.T) {
	big := make([]byte, 0, 70000)
	for len(big) < 65537 {
		big = append(big, OpNop)
	}
	big = append(big, OpEnd)
	manyParams := func(n, r int) []byte {
		var b Builder
		b.Func(bytes.Repeat([]byte{I64}, n), bytes.Repeat([]byte{I64}, r), nil, OpUnreachable, OpEnd)
		return b.Bytes()
	}
	withMem := func(f func(*Builder)) []byte {
		var b Builder
		f(&b)
		return b.Bytes()
	}
	cases := []struct {
		name string
		wasm []byte
		rule string
	}{
		{"simd", oneFunc(PrefixSIMD, 12, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, OpEnd), wasmrewrite.RuleSIMD},
		{"threads", oneFunc(PrefixThreads, 3, 0, OpEnd), wasmrewrite.RuleThreads},
		{"tail call", oneFunc(OpReturnCall, 0, OpEnd), wasmrewrite.RuleTailCall},
		{"exception throw", oneFunc(0x08, 0, OpEnd), wasmrewrite.RuleException},
		{"exception try", oneFunc(0x06, Empty, OpEnd, OpEnd), wasmrewrite.RuleException},
		{"table.set", oneFunc(OpTableSet, 0, OpEnd), wasmrewrite.RuleTable},
		{"table.grow", oneFunc(PrefixFC, FCTableGrow, 0, OpEnd), wasmrewrite.RuleTable},
		{"table.fill", oneFunc(PrefixFC, FCTableFill, 0, OpEnd), wasmrewrite.RuleTable},
		{"table.copy", oneFunc(PrefixFC, FCTableCopy, 0, 0, OpEnd), wasmrewrite.RuleTable},
		{"table.init", oneFunc(PrefixFC, FCTableInit, 0, 0, OpEnd), wasmrewrite.RuleTable},
		{"table.size", oneFunc(PrefixFC, FCTableSize, 0, OpEnd), wasmrewrite.RuleTable},
		{"unknown opcode", oneFunc(0x14, OpEnd), wasmrewrite.RuleUnknownOp},
		{"truncated immediate", oneFunc(OpI32Const, 0x80, OpEnd)[:0], ""},
		{"import module", withMem(func(b *Builder) { b.Import("env", "f", nil, nil) }), wasmrewrite.RuleImportMod},
		{"memory import", withMem(func(b *Builder) { b.ImportRaw("rimworld", "m", KindMemory, 0, 1) }), wasmrewrite.RuleImportKind},
		{"global import", withMem(func(b *Builder) { b.ImportRaw("rimworld", "g", KindGlobal, I32, 0) }), wasmrewrite.RuleImportKind},
		{"table import", withMem(func(b *Builder) { b.ImportRaw("rimworld", "t", KindTable, FuncRef, 0, 1) }), wasmrewrite.RuleImportKind},
		{"shared memory", withMem(func(b *Builder) { b.MemoryFlags(3, 1, 2) }), wasmrewrite.RuleMemory},
		{"memory64", withMem(func(b *Builder) { b.MemoryFlags(4, 1, 0) }), wasmrewrite.RuleMemory},
		{"start", withMem(func(b *Builder) { b.Start(b.Func(nil, nil, nil, OpEnd)) }), wasmrewrite.RuleStart},
		{"reserved export", withMem(func(b *Builder) { b.Export("__rimworld_x", KindFunc, b.Func(nil, nil, nil, OpEnd)) }), wasmrewrite.RuleReserved},
		{"too many locals", withMem(func(b *Builder) { b.Func(nil, nil, bytes.Repeat([]byte{I32}, 4097), OpEnd) }), wasmrewrite.RuleLocals},
		{"oversize body", oneFunc(big...), wasmrewrite.RuleBody},
		{"65 params", manyParams(65, 0), wasmrewrite.RuleArity},
		{"65 results", manyParams(0, 65), wasmrewrite.RuleArity},
		{"after final end", oneFunc(OpEnd, OpNop, OpEnd), wasmrewrite.RuleMalformed},
		{"missing final end", oneFunc(OpBlock, Empty, OpEnd), wasmrewrite.RuleMalformed},
		{"label out of range", oneFunc(OpBr, 1, OpEnd), wasmrewrite.RuleIndex},
		{"br_table label", oneFunc(OpBlock, Empty, OpBrTable, 1, 0, 2, OpEnd, OpEnd), wasmrewrite.RuleIndex},
		{"bad blocktype", oneFunc(OpBlock, 0x7B, OpEnd, OpEnd), wasmrewrite.RuleBlockType},
		{"select 2 types", oneFunc(OpSelectT, 2, I32, I32, OpEnd), wasmrewrite.RuleSelectTypes},
		{"call range", oneFunc(OpCall, 5, OpEnd), wasmrewrite.RuleIndex},
		{"ref.func range", oneFunc(OpRefFunc, 5, OpEnd), wasmrewrite.RuleIndex},
		{"call_indirect type", withMem(func(b *Builder) { b.Table(1); b.Func(nil, nil, nil, OpCallIndirect, 9, 0, OpEnd) }), wasmrewrite.RuleIndex},
		{"call_indirect table", oneFunc(OpCallIndirect, 0, 0, OpEnd), wasmrewrite.RuleIndex},
		{"table.get range", oneFunc(OpTableGet, 0, OpEnd), wasmrewrite.RuleIndex},
		{"load without memory", oneFunc(OpI32Load, 2, 0, OpEnd), wasmrewrite.RuleIndex},
		{"memory.size range", oneFunc(OpMemorySize, 0, OpEnd), wasmrewrite.RuleIndex},
		{"memory.init data", withMem(func(b *Builder) { b.Memory(1, 0); b.Func(nil, nil, nil, PrefixFC, FCMemoryInit, 0, 0, OpEnd) }), wasmrewrite.RuleIndex},
		{"data.drop", withMem(func(b *Builder) { b.Memory(1, 0); b.Func(nil, nil, nil, PrefixFC, FCDataDrop, 0, OpEnd) }), wasmrewrite.RuleIndex},
		{"memory.copy", oneFunc(PrefixFC, FCMemoryCopy, 0, 0, OpEnd), wasmrewrite.RuleIndex},
		{"memory.fill", oneFunc(PrefixFC, FCMemoryFill, 0, OpEnd), wasmrewrite.RuleIndex},
		{"elem.drop", oneFunc(PrefixFC, FCElemDrop, 0, OpEnd), wasmrewrite.RuleIndex},
		{"local range", oneFunc(OpLocalGet, 0, OpEnd), wasmrewrite.RuleIndex},
		{"global.set immutable", withMem(func(b *Builder) {
			b.Global(I32, false, OpI32Const, 0, OpEnd)
			b.Func(nil, nil, nil, OpI32Const, 0, OpGlobalSet, 0, OpEnd)
		}), wasmrewrite.RuleImmutable},
		{"global init global.get", withMem(func(b *Builder) { b.Global(I32, false, OpGlobalGet, 0, OpEnd) }), wasmrewrite.RuleIndex},
		{"global init ref.func", withMem(func(b *Builder) { b.Global(FuncRef, false, OpRefFunc, 3, OpEnd) }), wasmrewrite.RuleIndex},
		{"export func range", withMem(func(b *Builder) { b.Export("x", KindFunc, 4) }), wasmrewrite.RuleIndex},
		{"export memory range", withMem(func(b *Builder) { b.Export("x", KindMemory, 0) }), wasmrewrite.RuleIndex},
	}
	for _, c := range cases {
		if c.rule == "" {
			continue
		}
		rejectRule(t, c.name, c.wasm, c.rule)
	}
	// A truncated immediate is malformed.
	rejectRule(t, "truncated immediate", oneFunc(OpI32Const, 0x80, 0x80, 0x80, 0x80, 0x80, OpEnd), wasmrewrite.RuleMalformed)
}

func TestCheckRejectsMutatedModules(t *testing.T) {
	cases := map[string]func(*wasmrewrite.Module){
		"two memories":  func(m *wasmrewrite.Module) { m.Memories = append(m.Memories, m.Memories[0]) },
		"func type":     func(m *wasmrewrite.Module) { m.Funcs[0] = 99 },
		"import type":   func(m *wasmrewrite.Module) { m.Imports[0].Index = 99 },
		"export table":  func(m *wasmrewrite.Module) { m.AddExport(wasmrewrite.Export{Name: "t", Kind: KindTable, Index: 9}) },
		"export global": func(m *wasmrewrite.Module) { m.AddExport(wasmrewrite.Export{Name: "g", Kind: KindGlobal, Index: 9}) },
	}
	for name, mut := range cases {
		var b Builder
		b.Import("rimworld", "f", nil, nil)
		b.Memory(1, 0)
		b.Func(nil, nil, nil, OpEnd)
		m, err := wasmrewrite.Decode(b.Bytes())
		if err != nil {
			t.Fatal(err)
		}
		mut(m)
		if err := wasmrewrite.Check(m, testConfig()); err == nil {
			t.Errorf("%s accepted", name)
		}
	}
}

func TestRejectErrorText(t *testing.T) {
	e := &wasmrewrite.RejectError{Rule: "r", Func: -1}
	if e.Error() != "wasmrewrite: r" {
		t.Error(e.Error())
	}
	e = &wasmrewrite.RejectError{Rule: "r", Func: 2, Detail: "d"}
	if e.Error() != "wasmrewrite: r in function 2: d" {
		t.Error(e.Error())
	}
}

func allowedModule() []byte {
	var b Builder
	b.Memory(1, 0)
	b.Table(2)
	b.DataPassive([]byte("abcd"))
	f0 := b.Func(nil, nil, nil, OpEnd)
	b.ElemPassive(f0)
	b.ElemDeclare(f0)
	pair := byte(b.Type(nil, []byte{I32, I32}))
	g := b.Func(nil, []byte{I32}, nil,
		OpI32Const, 0, OpI32Const, 0, OpI32Const, 4, PrefixFC, FCMemoryInit, 0, 0,
		PrefixFC, FCDataDrop, 0,
		OpI32Const, 8, OpI32Const, 0x41, OpI32Const, 4, PrefixFC, FCMemoryFill, 0,
		OpI32Const, 16, OpI32Const, 8, OpI32Const, 4, PrefixFC, FCMemoryCopy, 0, 0,
		PrefixFC, FCElemDrop, 0,
		OpRefFunc, byte(f0), OpRefIsNull, OpDrop,
		OpRefNull, FuncRef, OpDrop,
		OpI32Const, 0, OpTableGet, 0, OpRefIsNull, OpDrop,
		OpI32Const, 0x80, 0x01, OpI32Extend8S, OpDrop,
		OpF32Const, 0, 0, 0xC0, 0x3F, PrefixFC, 0, OpDrop,
		OpI32Const, 1, OpI32Const, 2, OpI32Const, 1, OpSelectT, 1, I32, OpDrop,
		OpI32Const, 16, OpI32Load, 2, 0,
		OpBlock, pair, OpI32Const, 1, OpI32Const, 2, OpEnd, OpI32Add, OpI32Add,
		OpEnd)
	b.Export("g", KindFunc, g)
	return b.Bytes()
}

func TestAllowedFeaturesPassAndRun(t *testing.T) {
	in := allowedModule()
	out, err := wasmrewrite.Prepare(in, testConfig())
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range engines(t) {
		want := run(t, e, in, "g")
		got := run(t, e, out, "g")
		if !slices.Equal(want, got) || want[0] != 0xC1C1C1C4 {
			t.Errorf("%s: %v vs %v", e.name, want, got)
		}
	}
}

func TestPrepareRichModule(t *testing.T) {
	in := richModule()
	out, err := wasmrewrite.Prepare(in, testConfig())
	if err != nil {
		t.Fatal(err)
	}
	assertSameResults(t, in, out, richCalls)
}

// C8 (B1): indices that a rewrite could turn valid are rejected.
func TestPrepareRejectsFutureIndices(t *testing.T) {
	build := func(f func(b *Builder)) []byte {
		var b Builder
		f(&b)
		return b.Bytes()
	}
	cases := map[string][]byte{
		"loop global.set N": build(func(b *Builder) {
			b.Global(I64, true, OpI64Const, 0, OpEnd)
			b.Func(nil, nil, nil, OpLoop, Empty, OpI64Const, 0, OpGlobalSet, 1, OpEnd, OpEnd)
		}),
		"global.get N+1": build(func(b *Builder) {
			b.Global(I64, true, OpI64Const, 0, OpEnd)
			b.Func(nil, nil, nil, OpGlobalGet, 2, OpDrop, OpEnd)
		}),
		"local past locals": build(func(b *Builder) {
			b.Func([]byte{I32}, nil, nil, OpLocalGet, 1, OpDrop, OpEnd)
		}),
		"blocktype = type count": build(func(b *Builder) {
			b.Func(nil, nil, nil, OpBlock, 1, OpEnd, OpEnd)
		}),
		"export global N": build(func(b *Builder) {
			b.Global(I64, true, OpI64Const, 0, OpEnd)
			b.Export("g", KindGlobal, 1)
		}),
	}
	for name, wasm := range cases {
		rejectRule(t, name, wasm, wasmrewrite.RuleIndex)
		if _, err := wasmrewrite.Prepare(wasm, testConfig()); err == nil || !strings.Contains(err.Error(), "index out of range") {
			t.Errorf("%s: %v", name, err)
		}
		for _, e := range engines(t) {
			if compileErr(e, wasm) == nil {
				t.Errorf("%s: wazero (%s) accepts the original", name, e.name)
			}
		}
	}
}

// C9 (r2 B1): types at the arity cap pass, one over is rejected.
func TestArityCap(t *testing.T) {
	build := func(params, results int) []byte {
		var b Builder
		p, r := bytes.Repeat([]byte{I64}, params), bytes.Repeat([]byte{I64}, results)
		var code []byte
		for i := 0; i < params; i++ {
			code = append(code, OpLocalGet, byte(i))
		}
		blockType := byte(b.Type(p, r))
		code = append(code, OpBlock, blockType, OpEnd, OpEnd)
		f := b.Func(p, r, nil, code...)
		b.Export("f", KindFunc, f)
		return b.Bytes()
	}
	at := build(64, 64)
	out, err := wasmrewrite.Prepare(at, testConfig())
	if err != nil {
		t.Fatalf("cap module: %v", err)
	}
	args := make([]uint64, 64)
	want := make([]uint64, 64)
	for i := range args {
		args[i] = uint64(i)*1000003 + 1
		want[i] = args[i]
	}
	var results [][]uint64
	for _, e := range engines(t) {
		for _, w := range [][]byte{at, out} {
			got := run(t, e, w, "f", args...)
			if !slices.Equal(got, want) {
				t.Errorf("%s: wrong results", e.name)
			}
			results = append(results, got)
		}
	}
	for _, r := range results[1:] {
		if !slices.Equal(r, results[0]) {
			t.Error("engines disagree")
		}
	}
	for _, c := range []struct{ p, r int }{{65, 64}, {64, 65}} {
		var b Builder
		b.Func(bytes.Repeat([]byte{I64}, c.p), bytes.Repeat([]byte{I64}, c.r), nil, OpUnreachable, OpEnd)
		_, err := wasmrewrite.Prepare(b.Bytes(), testConfig())
		if err == nil || !strings.Contains(err.Error(), "arity over 64") {
			t.Errorf("%d/%d: %v", c.p, c.r, err)
		}
	}
	// A type used only as a block type is capped too.
	var b Builder
	b.Type(bytes.Repeat([]byte{I64}, 65), nil)
	b.Func(nil, nil, nil, OpEnd)
	if _, err := wasmrewrite.Prepare(b.Bytes(), testConfig()); err == nil || !strings.Contains(err.Error(), "arity over 64") {
		t.Errorf("block-only type: %v", err)
	}
}

// C10: the element, data and table indices are bounded by the original
// module's counts, so Prepare alone rejects them.
func TestCheckBoundsElemDataSections(t *testing.T) {
	build := func(f func(b *Builder)) []byte {
		var b Builder
		b.Memory(1, 0)
		b.Table(2)
		f(&b)
		return b.Bytes()
	}
	i32z := []byte{OpI32Const, 0, OpEnd}
	cases := map[string][]byte{
		"elem function index": build(func(b *Builder) { b.Func(nil, nil, nil, OpEnd); b.Elem(5) }),
		"elem passive func":   build(func(b *Builder) { b.Func(nil, nil, nil, OpEnd); b.ElemPassive(0, 1) }),
		"elem table index": build(func(b *Builder) {
			b.ElemRaw(append([]byte{2, 3}, append(i32z, 0x00, 0)...)...)
		}),
		"elem offset global.get": build(func(b *Builder) {
			b.ElemRaw(0, OpGlobalGet, 0, OpEnd, 0)
		}),
		"elem item ref.func": build(func(b *Builder) {
			b.ElemRaw(5, FuncRef, 1, OpRefFunc, 7, OpEnd)
		}),
		"data offset global.get": build(func(b *Builder) {
			b.DataRaw(0, OpGlobalGet, 0, OpEnd, 1, 9)
		}),
		"data memory index": build(func(b *Builder) {
			b.DataRaw(append([]byte{2, 3}, append(i32z, 1, 9)...)...)
		}),
	}
	for name, wasm := range cases {
		rejectRule(t, name, wasm, wasmrewrite.RuleIndex)
		m, err := wasmrewrite.Decode(wasm)
		if err != nil {
			t.Errorf("%s: decode: %v", name, err)
			continue
		}
		if err := wasmrewrite.Check(m, testConfig()); err == nil {
			t.Errorf("%s: Check accepted", name)
		}
	}
	// Valid forms pass: every element flag and data flag.
	ok := build(func(b *Builder) {
		f := b.Func(nil, nil, nil, OpEnd)
		b.Elem(f)
		b.ElemPassive(f)
		b.ElemDeclare(f)
		b.ElemRaw(append([]byte{2, 0}, append(i32z, 0x00, 1, byte(f))...)...)
		b.ElemRaw(4, OpI32Const, 0, OpEnd, 1, OpRefFunc, byte(f), OpEnd)
		b.ElemRaw(5, FuncRef, 1, OpRefNull, FuncRef, OpEnd)
		b.ElemRaw(6, 0, OpI32Const, 0, OpEnd, FuncRef, 0)
		b.ElemRaw(7, FuncRef, 0)
		b.Data(0, []byte{1})
		b.DataPassive([]byte{2})
		b.DataRaw(append([]byte{2, 0}, append(i32z, 1, 9)...)...)
	})
	if _, err := wasmrewrite.Prepare(ok, testConfig()); err != nil {
		t.Errorf("valid sections rejected: %v", err)
	}
	for name, bad := range map[string][]byte{
		"elem flag":    build(func(b *Builder) { b.ElemRaw(9) }),
		"elem kind":    build(func(b *Builder) { b.ElemRaw(1, 5, 0) }),
		"elem reftype": build(func(b *Builder) { b.ElemRaw(5, 0x7F, 0) }),
		"data flag":    build(func(b *Builder) { b.DataRaw(3, 0) }),
		"data short":   build(func(b *Builder) { b.DataRaw(1, 5, 1) }),
	} {
		if _, err := wasmrewrite.Decode(bad); err == nil {
			t.Errorf("%s: decoded", name)
		}
	}
}
