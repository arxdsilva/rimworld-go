// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package wasmrewrite

// Section is one raw section as read, in module order.
type Section struct {
	ID   byte
	Body []byte // the section's contents, without its id and size
}

// FuncType is one entry of the type section; params and results are
// valtype bytes (0x7F i32, 0x7E i64, 0x7D f32, 0x7C f64, 0x70 funcref,
// 0x6F externref).
type FuncType struct {
	Params, Results []byte
}

// Import is one entry of the import section.
type Import struct {
	Module, Name string
	Kind         byte   // 0 func, 1 table, 2 memory, 3 global
	Index        uint32 // a func import's type index
	Desc         []byte // the raw import descriptor after the kind byte
}

// Global is one defined global.
type Global struct {
	Type    byte // valtype
	Mutable bool
	Init    []byte // the constant init expression, its final end included
}

// Export is one entry of the export section.
type Export struct {
	Name  string
	Kind  byte // 0 func, 1 table, 2 memory, 3 global
	Index uint32
}

// LocalRun is one run of same-typed locals in a body's local declarations.
type LocalRun struct {
	Count uint32
	Type  byte // valtype
}

// Body is one entry of the code section.
type Body struct {
	Locals []LocalRun
	Code   []byte // the instructions, the final end included
}

// Module is a decoded module: the raw sections in order plus decoded
// views of the sections the rewriter reads or changes.
type Module struct {
	Sections []Section
	Types    []FuncType
	Imports  []Import
	Funcs    []uint32 // the type index of each defined function
	Globals  []Global // defined globals; their index space follows imported globals
	Exports  []Export
	Start    uint32 // valid when HasStart
	HasStart bool
	Bodies   []Body // one per entry of Funcs
	// FuelGlobal and DepthGlobal are the reserved globals' indices in the
	// global index space, set by Reserve.
	FuelGlobal, DepthGlobal uint32
}

// Decode parses a binary module: magic, version, sections in id order and
// the views above. It is at least as strict as wazero on anything it drops
// or re-encodes.
func Decode(wasm []byte) (*Module, error) {
	return nil, errNotYet // W0 stub
}

// AddType appends the function type params -> results and returns its
// type index. Existing indices do not shift.
func (m *Module) AddType(params, results []byte) uint32 {
	return 0 // W0 stub
}

// AddGlobal appends a defined global and returns its index in the global
// index space (imported globals first).
func (m *Module) AddGlobal(g Global) uint32 {
	return 0 // W0 stub
}

// AddExport appends an export.
func (m *Module) AddExport(e Export) {
	// W0 stub
}

// AddLocal appends one local of type valtype at the end of the locals of
// Bodies[fn] (fn counts defined functions only, not imports) and returns
// its local index (params first).
func (m *Module) AddLocal(fn uint32, valtype byte) uint32 {
	return 0 // W0 stub
}
