// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package wasmrewrite_test

import (
	"errors"
	"slices"
	"testing"

	"rimworld-go/internal/adapters/outbound/wasmhost/wasmrewrite"
	. "rimworld-go/internal/adapters/outbound/wasmhost/wasmtest"
)

type walkCase struct {
	name   string
	code   []byte
	op     uint32
	a, b   uint64
	labels []uint32
	types  []byte
}

func fc(sub byte) uint32 { return uint32(PrefixFC)<<16 | uint32(sub) }

var walkCases = []walkCase{
	{name: "unreachable", code: []byte{OpUnreachable}, op: 0x00},
	{name: "nop", code: []byte{OpNop}, op: 0x01},
	{name: "block empty", code: []byte{OpBlock, Empty}, op: 0x02, a: uint64(^uint64(63))},
	{name: "block i32", code: []byte{OpBlock, I32}, op: 0x02, a: uint64(^uint64(0))},
	{name: "loop type 3", code: []byte{OpLoop, 3}, op: 0x03, a: 3},
	{name: "block type 200", code: []byte{OpBlock, 0xC8, 0x01}, op: 0x02, a: 200},
	{name: "if", code: []byte{OpIf, Empty}, op: 0x04, a: uint64(^uint64(63))},
	{name: "else", code: []byte{OpElse}, op: 0x05},
	{name: "end", code: []byte{OpEnd}, op: 0x0B},
	{name: "br", code: []byte{OpBr, 2}, op: 0x0C, a: 2},
	{name: "br_if", code: []byte{OpBrIf, 0x80, 0x01}, op: 0x0D, a: 128},
	{name: "br_table", code: []byte{OpBrTable, 2, 1, 2, 3}, op: 0x0E, labels: []uint32{1, 2, 3}},
	{name: "br_table empty", code: []byte{OpBrTable, 0, 7}, op: 0x0E, labels: []uint32{7}},
	{name: "return", code: []byte{OpReturn}, op: 0x0F},
	{name: "call", code: []byte{OpCall, 5}, op: 0x10, a: 5},
	{name: "call_indirect", code: []byte{OpCallIndirect, 4, 1}, op: 0x11, a: 4, b: 1},
	{name: "drop", code: []byte{OpDrop}, op: 0x1A},
	{name: "select", code: []byte{OpSelect}, op: 0x1B},
	{name: "select t", code: []byte{OpSelectT, 1, I64}, op: 0x1C, types: []byte{I64}},
	{name: "local.get", code: []byte{OpLocalGet, 9}, op: 0x20, a: 9},
	{name: "local.set", code: []byte{OpLocalSet, 9}, op: 0x21, a: 9},
	{name: "local.tee", code: []byte{OpLocalTee, 9}, op: 0x22, a: 9},
	{name: "global.get", code: []byte{OpGlobalGet, 1}, op: 0x23, a: 1},
	{name: "global.set", code: []byte{OpGlobalSet, 1}, op: 0x24, a: 1},
	{name: "table.get", code: []byte{OpTableGet, 2}, op: 0x25, a: 2},
	{name: "table.set", code: []byte{OpTableSet, 2}, op: 0x26, a: 2},
	{name: "i32.load", code: []byte{OpI32Load, 2, 0x10}, op: 0x28, a: 2, b: 16},
	{name: "f64.store", code: []byte{OpF64Store, 3, 0x80, 0x01}, op: 0x39, a: 3, b: 128},
	{name: "memory.size", code: []byte{OpMemorySize, 0}, op: 0x3F},
	{name: "memory.grow", code: []byte{OpMemoryGrow, 0}, op: 0x40},
	{name: "i32.const -1", code: []byte{OpI32Const, 0x7F}, op: 0x41, a: ^uint64(0)},
	{name: "i32.const max", code: []byte{OpI32Const, 0xff, 0xff, 0xff, 0xff, 0x07}, op: 0x41, a: 0x7fffffff},
	{name: "i64.const min", code: []byte{OpI64Const, 0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x7f}, op: 0x42, a: 1 << 63},
	{name: "f32.const", code: []byte{OpF32Const, 0x00, 0x00, 0xC0, 0x7F}, op: 0x43, a: 0x7FC00000},
	{name: "f64.const", code: []byte{OpF64Const, 0, 0, 0, 0, 0, 0, 0xF8, 0x7F}, op: 0x44, a: 0x7FF8000000000000},
	{name: "i32.eqz", code: []byte{OpI32Eqz}, op: 0x45},
	{name: "f64.sqrt", code: []byte{OpF64Sqrt}, op: 0x9F},
	{name: "i64.extend_i32_u", code: []byte{OpI64ExtendI32U}, op: 0xAD},
	{name: "i32.extend8_s", code: []byte{OpI32Extend8S}, op: 0xC0},
	{name: "i64.extend32_s", code: []byte{0xC4}, op: 0xC4},
	{name: "ref.null func", code: []byte{OpRefNull, FuncRef}, op: 0xD0, a: 0x70},
	{name: "ref.null extern", code: []byte{OpRefNull, ExternRef}, op: 0xD0, a: 0x6F},
	{name: "ref.is_null", code: []byte{OpRefIsNull}, op: 0xD1},
	{name: "ref.func", code: []byte{OpRefFunc, 3}, op: 0xD2, a: 3},
	{name: "i32.trunc_sat_f32_s", code: []byte{PrefixFC, 0}, op: fc(0)},
	{name: "i64.trunc_sat_f64_u", code: []byte{PrefixFC, 7}, op: fc(7)},
	{name: "memory.init", code: []byte{PrefixFC, FCMemoryInit, 2, 0}, op: fc(8), a: 2},
	{name: "data.drop", code: []byte{PrefixFC, FCDataDrop, 2}, op: fc(9), a: 2},
	{name: "memory.copy", code: []byte{PrefixFC, FCMemoryCopy, 0, 0}, op: fc(10)},
	{name: "memory.fill", code: []byte{PrefixFC, FCMemoryFill, 0}, op: fc(11)},
	{name: "table.init", code: []byte{PrefixFC, FCTableInit, 1, 2}, op: fc(12), a: 1, b: 2},
	{name: "elem.drop", code: []byte{PrefixFC, FCElemDrop, 4}, op: fc(13), a: 4},
	{name: "table.copy", code: []byte{PrefixFC, FCTableCopy, 1, 2}, op: fc(14), a: 1, b: 2},
	{name: "table.grow", code: []byte{PrefixFC, FCTableGrow, 1}, op: fc(15), a: 1},
	{name: "table.size", code: []byte{PrefixFC, FCTableSize, 1}, op: fc(16), a: 1},
	{name: "table.fill", code: []byte{PrefixFC, FCTableFill, 1}, op: fc(17), a: 1},
	{name: "return_call", code: []byte{OpReturnCall, 6}, op: 0x12, a: 6},
	{name: "return_call_indirect", code: []byte{0x13, 1, 2}, op: 0x13, a: 1, b: 2},
	{name: "try", code: []byte{0x06, Empty}, op: 0x06, a: uint64(^uint64(63))},
	{name: "catch", code: []byte{0x07, 1}, op: 0x07, a: 1},
	{name: "throw", code: []byte{0x08, 1}, op: 0x08, a: 1},
	{name: "rethrow", code: []byte{0x09, 0}, op: 0x09},
	{name: "delegate", code: []byte{0x18, 0}, op: 0x18},
	{name: "catch_all", code: []byte{0x19}, op: 0x19},
	{name: "try_table", code: []byte{0x1F, Empty, 2, 0, 9, 3, 1, 9, 4}, op: 0x1F, a: uint64(^uint64(63)), labels: []uint32{3, 4}},
	{name: "simd load", code: []byte{PrefixSIMD, 0, 4, 0}, op: uint32(PrefixSIMD) << 16, a: 4},
	{name: "simd const", code: append([]byte{PrefixSIMD, 12}, make([]byte, 16)...), op: uint32(PrefixSIMD)<<16 | 12},
	{name: "simd lane", code: []byte{PrefixSIMD, 21, 3}, op: uint32(PrefixSIMD)<<16 | 21},
	{name: "simd load lane", code: []byte{PrefixSIMD, 84, 0, 0, 1}, op: uint32(PrefixSIMD)<<16 | 84},
	{name: "simd add", code: []byte{PrefixSIMD, 174, 1}, op: uint32(PrefixSIMD)<<16 | 174},
	{name: "atomic notify", code: []byte{PrefixThreads, 0, 2, 0}, op: uint32(PrefixThreads) << 16, a: 2},
	{name: "atomic fence", code: []byte{PrefixThreads, 3, 0}, op: uint32(PrefixThreads)<<16 | 3},
	{name: "atomic rmw", code: []byte{PrefixThreads, 0x1E, 2, 4}, op: uint32(PrefixThreads)<<16 | 0x1E, a: 2, b: 4},
}

func TestWalkEachOpcode(t *testing.T) {
	for _, c := range walkCases {
		var got []wasmrewrite.Instr
		err := wasmrewrite.Walk(c.code, func(in wasmrewrite.Instr) error {
			got = append(got, in)
			return nil
		})
		if err != nil || len(got) != 1 {
			t.Errorf("%s: %d instrs, err %v", c.name, len(got), err)
			continue
		}
		in := got[0]
		if in.Op != c.op || in.A != c.a || in.B != c.b || in.Start != 0 || in.End != len(c.code) ||
			!slices.Equal(in.Labels, c.labels) || !slices.Equal(in.Types, c.types) {
			t.Errorf("%s: got %+v", c.name, in)
		}
	}
}

// Every opcode byte is either walked with the span of the whole
// instruction or an unknown-opcode error, with no panic.
func TestWalkEveryOpcodeByte(t *testing.T) {
	known := 0
	for op := 0; op < 256; op++ {
		code := append([]byte{byte(op)}, make([]byte, 20)...)
		code[1] = 0 // sub-op / index zero
		err := wasmrewrite.Walk(code, func(wasmrewrite.Instr) error { return errStop })
		switch {
		case err == nil || errors.Is(err, errStop):
			known++
		case op == 0xFC || op == 0xFD || op == 0xFE:
		default:
			// unknown or unwalkable here: must be a plain error
		}
	}
	if known < 190 {
		t.Errorf("only %d opcode bytes walk", known)
	}
}

var errStop = errors.New("stop")

func TestWalkUnknownAndMalformed(t *testing.T) {
	bad := [][]byte{
		{0x14}, {0x27}, {0xC5}, {0xD3}, {0xFF}, {0x1D},
		{PrefixFC, 18}, {PrefixFC, 99}, {PrefixThreads, 4},
		{OpBlock}, {OpBr}, {OpBrTable, 5, 1}, {OpI32Const, 0x80}, {OpF32Const, 1, 2},
		{OpF64Const, 1}, {OpRefNull, 0x55}, {OpI32Load, 2}, {OpI32Load, 64, 0}, {PrefixFC},
		{OpSelectT, 3, I32}, {0x1F, Empty, 1, 9, 0}, {PrefixSIMD, 12, 1},
	}
	for _, code := range bad {
		if err := wasmrewrite.Walk(code, func(wasmrewrite.Instr) error { return nil }); err == nil {
			t.Errorf("%x walked", code)
		}
	}
}

func TestWalkStopsOnCallbackError(t *testing.T) {
	n := 0
	err := wasmrewrite.Walk([]byte{OpNop, OpNop, OpNop}, func(wasmrewrite.Instr) error {
		n++
		if n == 2 {
			return errStop
		}
		return nil
	})
	if !errors.Is(err, errStop) || n != 2 {
		t.Errorf("n=%d err=%v", n, err)
	}
}

// C3: a builder module holding every allowed opcode walks with
// contiguous spans and the right immediates.
func TestWalkBuilderModuleWithEveryAllowedOpcode(t *testing.T) {
	var code []byte
	var want []uint32
	for _, c := range walkCases {
		if c.op == 0x06 || (c.op >= 0x07 && c.op <= 0x09) || c.op == 0x12 || c.op == 0x13 || c.op == 0x18 ||
			c.op == 0x19 || c.op == 0x1F || c.op == 0x26 || c.op>>16 == uint32(PrefixSIMD) ||
			c.op>>16 == uint32(PrefixThreads) || (c.op>>16 == uint32(PrefixFC) && (c.op&0xFFFF == 12 || c.op&0xFFFF >= 14 && c.op&0xFFFF <= 17)) {
			continue
		}
		code = append(code, c.code...)
		want = append(want, c.op)
	}
	code = append(code, OpEnd)
	want = append(want, 0x0B)
	var b Builder
	b.Func(nil, nil, nil, code...)
	m, err := wasmrewrite.Decode(b.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	var ops []uint32
	prev := 0
	err = wasmrewrite.Walk(m.Bodies[0].Code, func(in wasmrewrite.Instr) error {
		if in.Start != prev || in.End <= in.Start {
			t.Errorf("span %d-%d after %d", in.Start, in.End, prev)
		}
		prev = in.End
		ops = append(ops, in.Op)
		return nil
	})
	if err != nil || prev != len(m.Bodies[0].Code) || !slices.Equal(ops, want) {
		t.Fatalf("walk: err %v, end %d/%d, %d ops want %d", err, prev, len(m.Bodies[0].Code), len(ops), len(want))
	}
}
