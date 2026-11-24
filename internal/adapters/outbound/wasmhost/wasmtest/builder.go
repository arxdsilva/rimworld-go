// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

// Package wasmtest builds small WebAssembly modules from typed Go values
// for the rewriter, host, golden and fuzz tests, so each edge case is a
// few lines of Go instead of a committed blob. It emits through
// wasmrewrite's encoder and imports nothing else of this repo.
package wasmtest

import (
	"bytes"
	"slices"

	"rimworld-go/internal/adapters/outbound/wasmhost/wasmrewrite"
)

// Value types and block types.
const (
	I32       byte = 0x7F
	I64       byte = 0x7E
	F32       byte = 0x7D
	F64       byte = 0x7C
	FuncRef   byte = 0x70
	ExternRef byte = 0x6F
	Empty     byte = 0x40 // the empty blocktype
)

// Export and import kinds.
const (
	KindFunc   byte = 0x00
	KindTable  byte = 0x01
	KindMemory byte = 0x02
	KindGlobal byte = 0x03
)

// One-byte opcodes the tests use.
const (
	OpUnreachable       byte = 0x00
	OpNop               byte = 0x01
	OpBlock             byte = 0x02
	OpLoop              byte = 0x03
	OpIf                byte = 0x04
	OpElse              byte = 0x05
	OpEnd               byte = 0x0B
	OpBr                byte = 0x0C
	OpBrIf              byte = 0x0D
	OpBrTable           byte = 0x0E
	OpReturn            byte = 0x0F
	OpCall              byte = 0x10
	OpCallIndirect      byte = 0x11
	OpReturnCall        byte = 0x12 // rejected (tail call)
	OpDrop              byte = 0x1A
	OpSelect            byte = 0x1B
	OpSelectT           byte = 0x1C
	OpLocalGet          byte = 0x20
	OpLocalSet          byte = 0x21
	OpLocalTee          byte = 0x22
	OpGlobalGet         byte = 0x23
	OpGlobalSet         byte = 0x24
	OpTableGet          byte = 0x25
	OpTableSet          byte = 0x26 // rejected
	OpI32Load           byte = 0x28
	OpI64Load           byte = 0x29
	OpF32Load           byte = 0x2A
	OpF64Load           byte = 0x2B
	OpI32Store          byte = 0x36
	OpI64Store          byte = 0x37
	OpF32Store          byte = 0x38
	OpF64Store          byte = 0x39
	OpMemorySize        byte = 0x3F
	OpMemoryGrow        byte = 0x40
	OpI32Const          byte = 0x41
	OpI64Const          byte = 0x42
	OpF32Const          byte = 0x43
	OpF64Const          byte = 0x44
	OpI32Eqz            byte = 0x45
	OpI32Eq             byte = 0x46
	OpI32LtS            byte = 0x48
	OpI64LtS            byte = 0x53
	OpF32Eq             byte = 0x5B
	OpF64Eq             byte = 0x61
	OpI32Add            byte = 0x6A
	OpI32Sub            byte = 0x6B
	OpI32Mul            byte = 0x6C
	OpI64Add            byte = 0x7C
	OpI64Sub            byte = 0x7D
	OpF32Abs            byte = 0x8B
	OpF32Neg            byte = 0x8C
	OpF32Ceil           byte = 0x8D
	OpF32Floor          byte = 0x8E
	OpF32Trunc          byte = 0x8F
	OpF32Nearest        byte = 0x90
	OpF32Sqrt           byte = 0x91
	OpF32Add            byte = 0x92
	OpF32Sub            byte = 0x93
	OpF32Mul            byte = 0x94
	OpF32Div            byte = 0x95
	OpF32Min            byte = 0x96
	OpF32Max            byte = 0x97
	OpF32Copysign       byte = 0x98
	OpF64Abs            byte = 0x99
	OpF64Neg            byte = 0x9A
	OpF64Ceil           byte = 0x9B
	OpF64Floor          byte = 0x9C
	OpF64Trunc          byte = 0x9D
	OpF64Nearest        byte = 0x9E
	OpF64Sqrt           byte = 0x9F
	OpF64Add            byte = 0xA0
	OpF64Sub            byte = 0xA1
	OpF64Mul            byte = 0xA2
	OpF64Div            byte = 0xA3
	OpF64Min            byte = 0xA4
	OpF64Max            byte = 0xA5
	OpF64Copysign       byte = 0xA6
	OpI64ExtendI32U     byte = 0xAD
	OpF32DemoteF64      byte = 0xB6
	OpF64PromoteF32     byte = 0xBB
	OpI32ReinterpretF32 byte = 0xBC
	OpI64ReinterpretF64 byte = 0xBD
	OpF32ReinterpretI32 byte = 0xBE
	OpF64ReinterpretI64 byte = 0xBF
	OpI32Extend8S       byte = 0xC0
	OpRefNull           byte = 0xD0
	OpRefIsNull         byte = 0xD1
	OpRefFunc           byte = 0xD2
	PrefixFC            byte = 0xFC // sat-trunc and bulk memory/table; sub-op follows as LEB u32
	PrefixSIMD          byte = 0xFD // rejected
	PrefixThreads       byte = 0xFE // rejected
)

// 0xFC sub-ops the tests use.
const (
	FCMemoryInit byte = 8
	FCDataDrop   byte = 9
	FCMemoryCopy byte = 10
	FCMemoryFill byte = 11
	FCTableInit  byte = 12 // rejected
	FCElemDrop   byte = 13
	FCTableCopy  byte = 14 // rejected
	FCTableGrow  byte = 15 // rejected
	FCTableSize  byte = 16 // rejected
	FCTableFill  byte = 17 // rejected
)

// Builder assembles a module. Add every import before the first Func, so
// the function indices it returns stay valid. The zero value is ready.
// It does not validate: tests build invalid modules on purpose.
type Builder struct {
	m        wasmrewrite.Module
	memory   []byte   // raw memory section body, empty when none
	table    []byte   // raw table section body
	elems    [][]byte // raw element segments
	datas    [][]byte // raw data segments
	dataCnt  bool
	nImports uint32 // function imports
}

// Type adds (or reuses) the function type params -> results and returns
// its type index.
func (b *Builder) Type(params, results []byte) uint32 {
	for i, t := range b.m.Types {
		if bytes.Equal(t.Params, params) && bytes.Equal(t.Results, results) {
			return uint32(i)
		}
	}
	return b.m.AddType(params, results)
}

// Func adds a function of type params -> results with the given locals
// (one valtype byte per local) and body code, the final OpEnd included,
// and returns its function index (imports first).
func (b *Builder) Func(params, results []byte, locals []byte, code ...byte) uint32 {
	var runs []wasmrewrite.LocalRun
	for _, t := range locals {
		if n := len(runs); n > 0 && runs[n-1].Type == t {
			runs[n-1].Count++
		} else {
			runs = append(runs, wasmrewrite.LocalRun{Count: 1, Type: t})
		}
	}
	b.m.Funcs = append(b.m.Funcs, b.Type(params, results))
	b.m.Bodies = append(b.m.Bodies, wasmrewrite.Body{Locals: runs, Code: slices.Clone(code)})
	return b.nImports + uint32(len(b.m.Funcs)-1) // #nosec G115 -- test fixture builder: values from test code
}

// Import adds a function import module.name of type params -> results
// and returns its function index.
func (b *Builder) Import(module, name string, params, results []byte) uint32 {
	b.m.Imports = append(b.m.Imports, wasmrewrite.Import{
		Module: module, Name: name, Kind: KindFunc, Index: b.Type(params, results),
	})
	b.nImports++
	return b.nImports - 1
}

// ImportRaw adds a non-function import (kind and raw descriptor bytes),
// for tests that must see it rejected.
func (b *Builder) ImportRaw(module, name string, kind byte, desc ...byte) {
	b.m.Imports = append(b.m.Imports, wasmrewrite.Import{
		Module: module, Name: name, Kind: kind, Desc: slices.Clone(desc),
	})
}

// Export exports the item of the given kind and index as name.
func (b *Builder) Export(name string, kind byte, index uint32) {
	b.m.AddExport(wasmrewrite.Export{Name: name, Kind: kind, Index: index})
}

// Start sets the start function.
func (b *Builder) Start(fn uint32) {
	b.m.Start, b.m.HasStart = fn, true
}

func limitsBytes(flags byte, min, max uint64) []byte {
	out := []byte{flags}
	out = leb(out, min)
	if flags&1 != 0 {
		out = leb(out, max)
	}
	return out
}

func leb(b []byte, v uint64) []byte {
	for v >= 0x80 {
		b = append(b, byte(v)|0x80)
		v >>= 7
	}
	return append(b, byte(v))
}

// Memory adds the module's memory with minPages and maxPages; maxPages 0
// means no maximum.
func (b *Builder) Memory(minPages, maxPages uint32) {
	if maxPages == 0 {
		b.MemoryFlags(0, uint64(minPages), 0)
		return
	}
	b.MemoryFlags(1, uint64(minPages), uint64(maxPages))
}

// MemoryFlags adds the memory with raw limits flags (bit 0 max, bit 1
// shared, bit 2 64-bit), for tests that must see it rejected.
func (b *Builder) MemoryFlags(flags byte, min, max uint64) {
	b.memory = append([]byte{1}, limitsBytes(flags, min, max)...)
}

// Table adds a funcref table of minSize entries (no maximum).
func (b *Builder) Table(minSize uint32) {
	b.table = append([]byte{1, FuncRef}, limitsBytes(0, uint64(minSize), 0)...)
}

// Elem adds an active element segment at offset 0 of table 0 holding the
// given functions.
func (b *Builder) Elem(funcs ...uint32) {
	seg := []byte{0, OpI32Const, 0, OpEnd}
	b.elems = append(b.elems, funcVec(seg, funcs))
}

// ElemRaw adds an element segment from raw bytes, for tests that need a
// malformed or out-of-range one.
func (b *Builder) ElemRaw(seg ...byte) {
	b.elems = append(b.elems, slices.Clone(seg))
}

// DataRaw adds a data segment from raw bytes (flag, optional memory
// index, offset expression, length, bytes).
func (b *Builder) DataRaw(seg ...byte) {
	b.datas = append(b.datas, slices.Clone(seg))
}

// ElemPassive adds a passive element segment (for elem.drop).
func (b *Builder) ElemPassive(funcs ...uint32) {
	b.elems = append(b.elems, funcVec([]byte{1, 0x00}, funcs))
}

// ElemDeclare adds a declarative element segment, which declares the
// functions so ref.func may name them.
func (b *Builder) ElemDeclare(funcs ...uint32) {
	b.elems = append(b.elems, funcVec([]byte{3, 0x00}, funcs))
}

func funcVec(seg []byte, funcs []uint32) []byte {
	seg = leb(seg, uint64(len(funcs)))
	for _, f := range funcs {
		seg = leb(seg, uint64(f))
	}
	return seg
}

// Global adds a defined global with its constant init expression (the
// final OpEnd included) and returns its global index.
func (b *Builder) Global(valtype byte, mutable bool, init ...byte) uint32 {
	return b.m.AddGlobal(wasmrewrite.Global{Type: valtype, Mutable: mutable, Init: slices.Clone(init)})
}

// Data adds an active data segment for memory 0 at the i32 offset.
func (b *Builder) Data(offset uint32, data []byte) {
	seg := []byte{0, OpI32Const}
	// i32.const takes a signed LEB: emit offset as unsigned 5-byte safe form.
	seg = appendS32(seg, int32(offset)) // #nosec G115 -- test fixture builder: values from test code
	seg = append(seg, OpEnd)
	seg = leb(seg, uint64(len(data)))
	b.datas = append(b.datas, append(seg, data...))
}

// DataPassive adds a passive data segment (for memory.init) and writes
// the datacount section.
func (b *Builder) DataPassive(data []byte) {
	seg := leb([]byte{1}, uint64(len(data)))
	b.datas = append(b.datas, append(seg, data...))
	b.dataCnt = true
}

func appendS32(b []byte, v int32) []byte {
	x := int64(v)
	for {
		c := byte(x & 0x7f)
		x >>= 7
		if (x == 0 && c&0x40 == 0) || (x == -1 && c&0x40 != 0) {
			return append(b, c)
		}
		b = append(b, c|0x80)
	}
}

func vecSection(id byte, items [][]byte) wasmrewrite.Section {
	body := leb(nil, uint64(len(items)))
	for _, it := range items {
		body = append(body, it...)
	}
	return wasmrewrite.Section{ID: id, Body: body}
}

// Bytes encodes the module.
func (b *Builder) Bytes() []byte {
	m := b.m
	m.Sections = nil
	add := func(s wasmrewrite.Section) { m.Sections = append(m.Sections, s) }
	if b.table != nil {
		add(wasmrewrite.Section{ID: 4, Body: b.table})
	}
	if b.memory != nil {
		add(wasmrewrite.Section{ID: 5, Body: b.memory})
	}
	if len(b.elems) > 0 {
		add(vecSection(9, b.elems))
	}
	if b.dataCnt {
		add(wasmrewrite.Section{ID: 12, Body: leb(nil, uint64(len(b.datas)))})
	}
	if len(b.datas) > 0 {
		add(vecSection(11, b.datas))
	}
	return m.Encode()
}
