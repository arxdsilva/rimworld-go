// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

// Package wasmtest builds small WebAssembly modules from typed Go values
// for the rewriter, host, golden and fuzz tests, so each edge case is a
// few lines of Go instead of a committed blob. It emits through
// wasmrewrite's encoder and imports nothing else of this repo.
package wasmtest

import "rimworld-go/internal/adapters/outbound/wasmhost/wasmrewrite"

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
type Builder struct {
	m wasmrewrite.Module
}

// Func adds a function of type params -> results with the given locals
// (one valtype byte per local) and body code, the final OpEnd included,
// and returns its function index (imports first).
func (b *Builder) Func(params, results []byte, locals []byte, code ...byte) uint32 {
	return 0 // W0 stub
}

// Import adds a function import module.name of type params -> results
// and returns its function index.
func (b *Builder) Import(module, name string, params, results []byte) uint32 {
	return 0 // W0 stub
}

// Export exports the item of the given kind and index as name.
func (b *Builder) Export(name string, kind byte, index uint32) {
	// W0 stub
}

// Memory adds the module's memory with minPages and maxPages; maxPages 0
// means no maximum.
func (b *Builder) Memory(minPages, maxPages uint32) {
	// W0 stub
}

// Global adds a defined global with its constant init expression (the
// final OpEnd included) and returns its global index.
func (b *Builder) Global(valtype byte, mutable bool, init ...byte) uint32 {
	return 0 // W0 stub
}

// Data adds an active data segment for memory 0 at the i32 offset.
func (b *Builder) Data(offset uint32, data []byte) {
	// W0 stub
}

// Bytes encodes the module.
func (b *Builder) Bytes() []byte {
	return b.m.Encode()
}
