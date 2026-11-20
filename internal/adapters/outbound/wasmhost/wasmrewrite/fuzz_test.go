// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package wasmrewrite_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/tetratelabs/wazero"

	"rimworld-go/internal/adapters/outbound/wasmhost/wasmrewrite"
	. "rimworld-go/internal/adapters/outbound/wasmhost/wasmtest"
)

// fuzzSeeds returns the seed corpus: every committed fixture, builder
// modules covering loops, nested blocks, br_table, bulk operations and
// float operations, the index-out-of-range modules, and truncated and
// bad-LEB variants of all of them.
func fuzzSeeds(tb testing.TB) [][]byte {
	tb.Helper()
	var seeds [][]byte
	files, err := filepath.Glob("../testdata/*.wasm")
	if err != nil || len(files) == 0 {
		tb.Fatalf("fixtures: %v (%d files)", err, len(files))
	}
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			tb.Fatal(err)
		}
		seeds = append(seeds, b)
	}
	seeds = append(seeds, richModule(), hBytes, handModule(hType, hFunc, hMem, hExp, hCode, hCust))
	seeds = append(seeds, builderSeeds()...)
	base := len(seeds)
	for i := 0; i < base; i++ {
		s := seeds[i]
		seeds = append(seeds, s[:len(s)/2], s[:len(s)-1], s[:8])
		if len(s) > 12 {
			// A section size LEB that never terminates.
			seeds = append(seeds, append(append([]byte{}, s[:9]...), 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0x7F))
			flipped := bytes.Clone(s)
			flipped[len(flipped)-2] ^= 0xFF
			seeds = append(seeds, flipped)
		}
	}
	return append(seeds, nil, []byte("\x00asm"), []byte("\x00asm\x01\x00\x00\x00"))
}

func builderSeeds() [][]byte {
	var out [][]byte
	add := func(f func(b *Builder)) {
		var b Builder
		f(&b)
		out = append(out, b.Bytes())
	}
	add(func(b *Builder) { // loop with nested blocks and br_table
		b.Func([]byte{I32}, []byte{I32}, []byte{I32},
			OpBlock, Empty, OpBlock, Empty,
			OpLoop, Empty, OpLocalGet, 0, OpBrTable, 2, 0, 1, 2, OpEnd,
			OpEnd, OpEnd,
			OpLocalGet, 1, OpEnd)
	})
	add(func(b *Builder) { // if/else, br_if, select, return
		b.Func([]byte{I32}, []byte{I32}, nil,
			OpLocalGet, 0, OpIf, I32, OpI32Const, 1, OpElse, OpI32Const, 2, OpEnd,
			OpLocalGet, 0, OpBrIf, 0, OpDrop, OpReturn, OpEnd)
	})
	add(func(b *Builder) { // bulk memory and table operations
		b.Memory(1, 1)
		b.Table(4)
		f := b.Func(nil, nil, nil, OpEnd)
		b.Elem(f)
		b.DataPassive([]byte{1, 2, 3})
		b.Func(nil, nil, nil,
			OpI32Const, 0, OpI32Const, 0, OpI32Const, 3, PrefixFC, FCMemoryInit, 0, 0,
			OpI32Const, 8, OpI32Const, 0, OpI32Const, 3, PrefixFC, FCMemoryCopy, 0, 0,
			OpI32Const, 0, OpI32Const, 7, OpI32Const, 4, PrefixFC, FCMemoryFill, 0,
			PrefixFC, FCDataDrop, 0,
			OpI32Const, 0, OpI32Const, 0, OpI32Const, 1, PrefixFC, FCTableCopy, 0, 0,
			OpEnd)
	})
	add(func(b *Builder) { // float operations
		b.Func([]byte{F32, F64}, []byte{F64}, nil,
			OpLocalGet, 0, OpF32Sqrt, OpF64PromoteF32, OpLocalGet, 1, OpF64Div,
			OpLocalGet, 1, OpF64Min, OpLocalGet, 1, OpF64Copysign,
			OpLocalGet, 0, OpLocalGet, 0, OpF32Add, OpF32Nearest, OpF64PromoteF32, OpF64Add,
			OpF64Const, 0, 0, 0, 0, 0, 0, 0xF8, 0x7F, OpF64Mul, OpEnd)
	})
	add(func(b *Builder) { // call_indirect, imports, globals, start
		b.Import("rimworld", "tick", []byte{I32}, []byte{I32})
		b.Memory(1, 2)
		b.Table(2)
		b.Global(I64, true, OpI64Const, 0, OpEnd)
		b.Func(nil, nil, nil, OpEnd)
		b.Elem(1)
	})
	// C8: indices out of range.
	i32z := []byte{OpI32Const, 0, OpEnd}
	add(func(b *Builder) { b.Memory(1, 0); b.Table(2); b.Func(nil, nil, nil, OpEnd); b.Elem(5) })
	add(func(b *Builder) { b.Memory(1, 0); b.Table(2); b.ElemRaw(2, 3, OpI32Const, 0, OpEnd, 0, 0) })
	add(func(b *Builder) { b.Memory(1, 0); b.DataRaw(0, OpGlobalGet, 0, OpEnd, 1, 9) })
	add(func(b *Builder) { b.Memory(1, 0); b.DataRaw(append([]byte{2, 3}, append(i32z, 1, 9)...)...) })
	add(func(b *Builder) { b.Func(nil, nil, nil, OpCall, 9, OpEnd) })
	add(func(b *Builder) { b.Func(nil, nil, nil, OpLocalGet, 9, OpDrop, OpEnd) })
	add(func(b *Builder) { b.Export("x", KindFunc, 3) })
	// Indices equal to the original count: the slot a rewrite appends
	// (the fuel global, a helper function) must not make them valid.
	add(func(b *Builder) { b.Func(nil, []byte{I32}, nil, OpGlobalGet, 0, OpEnd) })
	add(func(b *Builder) { b.Func(nil, nil, nil, OpI64Const, 0, OpGlobalSet, 0, OpEnd) })
	add(func(b *Builder) {
		b.Global(I64, true, OpI64Const, 0, OpEnd)
		b.Func(nil, nil, nil, OpI64Const, 0, OpGlobalSet, 1, OpEnd)
	})
	add(func(b *Builder) { b.Global(I64, true, OpI64Const, 0, OpEnd); b.Export("g", KindGlobal, 1) })
	add(func(b *Builder) { b.Export("g", KindGlobal, 0) })
	add(func(b *Builder) { b.Func(nil, nil, nil, OpEnd); b.Export("f", KindFunc, 1) })
	add(func(b *Builder) { b.Func(nil, nil, nil, OpCall, 1, OpEnd) })
	add(func(b *Builder) { b.Func(nil, nil, nil, OpEnd); b.Table(1); b.Elem(1) })
	add(func(b *Builder) { b.Func(nil, nil, nil, OpEnd); b.Elem(0) })
	add(func(b *Builder) { b.Table(1); b.Func(nil, nil, nil, OpI32Const, 0, OpTableGet, 1, OpDrop, OpEnd) })
	add(func(b *Builder) { b.Func(nil, nil, nil, OpI32Const, 0, OpTableGet, 0, OpDrop, OpEnd) })
	add(func(b *Builder) { b.Func(nil, nil, nil, PrefixFC, FCTableSize, 0, OpDrop, OpEnd) })
	add(func(b *Builder) { b.Func(nil, nil, nil, OpI32Const, 0, OpCallIndirect, 0, 0, OpEnd) })
	add(func(b *Builder) { b.Func(nil, nil, nil, OpRefFunc, 1, OpDrop, OpEnd) })
	return out
}

func TestFuzzSeedsAreNonTrivial(t *testing.T) {
	if n := len(fuzzSeeds(t)); n < 40 {
		t.Fatalf("only %d seeds", n)
	}
}

// fuzzCheck is the FuzzPrepare property for one input, on one runtime.
func fuzzCheck(t *testing.T, r wazero.Runtime, in []byte) {
	t.Helper()
	ctx := context.Background()
	// wazero v1.9.0's validator panics on some malformed bodies (see
	// testdata/fuzz/FuzzPrepare/wazero_validator_panic). A panic on the
	// input counts as invalid; a panic on Prepare's output fails the test.
	valid := func(b []byte, isOutput bool) (ok bool) {
		defer func() {
			if p := recover(); p != nil {
				if isOutput {
					t.Fatalf("wazero panicked on Prepare output: %v", p)
				}
				ok = false
			}
		}()
		cm, err := r.CompileModule(ctx, b)
		if err != nil {
			return false
		}
		_ = cm.Close(ctx)
		return true
	}
	out, err := wasmrewrite.Prepare(in, testConfig()) // a panic fails the test
	if err != nil {
		if out != nil {
			t.Fatalf("error with non-nil output")
		}
		return
	}
	out2, err2 := wasmrewrite.Prepare(in, testConfig())
	if err2 != nil || !bytes.Equal(out, out2) {
		t.Fatalf("Prepare is not deterministic: %v", err2)
	}
	// Prepare drops custom sections by design, and wazero v1.9.0 rejects
	// some spec-valid ones (a name with no payload after it). Judge the
	// input without them, so the only validity change the property
	// tolerates is that documented one.
	m, err := wasmrewrite.Decode(in)
	if err != nil {
		t.Fatalf("Prepare decoded what Decode rejects: %v", err)
	}
	switch inValid, outValid := valid(m.Encode(), false), valid(out, true); {
	case inValid && !outValid:
		t.Fatalf("rewrite broke a valid module (in %d bytes, out %d bytes)", len(in), len(out))
	case !inValid && outValid:
		t.Fatalf("rewrite made an invalid module valid (in %d bytes, out %d bytes)", len(in), len(out))
	}
}

// FuzzPrepare: Prepare never panics, is deterministic, and never changes
// a module's validity under wazero: valid in gives valid out, and invalid
// in is either rejected or stays invalid. No input class is skipped.
func FuzzPrepare(f *testing.F) {
	for _, s := range fuzzSeeds(f) {
		f.Add(s)
	}
	r := newRuntime(testEngine{"interpreter", wazero.NewRuntimeConfigInterpreter})
	f.Cleanup(func() { _ = r.Close(context.Background()) })
	f.Fuzz(func(t *testing.T, in []byte) { fuzzCheck(t, r, in) })
}

// The property is vacuous on seeds Prepare rejects, so the corpus must
// hold many accepted modules, and many rejected ones.
func TestFuzzSeedsReachTheRewrite(t *testing.T) {
	accepted, rejected := 0, 0
	for _, s := range fuzzSeeds(t) {
		if _, err := wasmrewrite.Prepare(s, testConfig()); err == nil {
			accepted++
		} else {
			rejected++
		}
	}
	if accepted < 12 || rejected < 12 {
		t.Fatalf("accepted %d, rejected %d", accepted, rejected)
	}
	t.Logf("accepted %d, rejected %d", accepted, rejected)
}

// Every out-of-range-index seed is invalid, so Prepare must reject it: the
// slot a rewrite appends never makes an index valid. This pins the seeds in
// plain go test, without a mutation run.
func TestFuzzIndexSeedsAreRejected(t *testing.T) {
	seeds := builderSeeds()
	for i, s := range seeds[5:] {
		if _, err := wasmrewrite.Prepare(s, testConfig()); err == nil {
			t.Errorf("index seed %d accepted", i)
		}
	}
}

// wazero v1.9.0 rejects a custom section that is only a name (no payload),
// which the spec allows. Prepare drops custom sections, so it turns this
// input into a valid empty module. FuzzPrepare judges inputs without custom
// sections for that reason; this test keeps the quirk visible and makes the
// fuzz corpus entry 36e2bb676441b66b mean something.
func TestFuzzWazeroRejectsNameOnlyCustomSection(t *testing.T) {
	in := []byte("\x00asm\x01\x00\x00\x00\x00\n\t000000000")
	e := testEngine{"interpreter", wazero.NewRuntimeConfigInterpreter}
	if compileErr(e, in) == nil {
		t.Skip("wazero accepts name-only custom sections now; drop the strip in fuzzCheck")
	}
	out, err := wasmrewrite.Prepare(in, testConfig())
	if err != nil || compileErr(e, out) != nil {
		t.Fatalf("Prepare: %v, output compile: %v", err, compileErr(e, out))
	}
}
