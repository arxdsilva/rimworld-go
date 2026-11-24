// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package wasmrewrite

import (
	"errors"
	"fmt"
)

var errStopWalk = errors.New("stop")

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
	Desc         []byte // the raw descriptor after the kind byte; unused for a func
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

// Limits is a memory or table size range.
type Limits struct {
	Min, Max uint64
	HasMax   bool
	Shared   bool
	Mem64    bool
}

// Module is a decoded module: the raw sections in order plus decoded
// views of the sections the rewriter reads or changes.
//
// Encode re-encodes the sections that have a view (type, import,
// function, global, export, start, code) from the views, so a rewrite
// edits the views. The table, memory, element, datacount and data
// sections are kept raw in Sections; their decoded facts (Memories and
// the counts) are read-only for Check.
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

	Memories []Limits // defined memories
	// NumTables, NumElems and NumDatas count the entries of the table,
	// element and data sections. HasDataCount and DataCount describe the
	// datacount section.
	NumTables, NumElems, NumDatas uint32
	HasDataCount                  bool
	DataCount                     uint32
	// ElemFuncs are the function indices in function-index element
	// segments; ElemTables the table of each active element segment;
	// DataMems the memory of each active data segment; InitExprs every
	// constant expression of the element and data sections (offsets and
	// element items). Read-only facts for Check.
	ElemFuncs, ElemTables, DataMems []uint32
	InitExprs                       [][]byte
}

// sectionRank orders the non-custom sections as the binary format does:
// datacount (12) sits between element (9) and code (10). Zero means an
// unknown id.
func sectionRank(id byte) int {
	switch {
	case id >= 1 && id <= 9:
		return int(id)
	case id == 12:
		return 10
	case id == 10:
		return 11
	case id == 11:
		return 12
	}
	return 0
}

// Decode parses a binary module: magic, version, sections in id order and
// the views above. It is at least as strict as wazero on anything it drops
// or re-encodes: section sizes are exact, names are UTF-8, custom sections
// are checked before they are dropped, the function and code counts agree,
// and the datacount agrees with the data section.
func Decode(wasm []byte) (*Module, error) {
	if len(wasm) < 8 {
		return nil, errors.New("wasmrewrite: decode: truncated header")
	}
	if string(wasm[:4]) != "\x00asm" {
		return nil, errors.New("wasmrewrite: decode: bad magic")
	}
	if wasm[4] != 1 || wasm[5] != 0 || wasm[6] != 0 || wasm[7] != 0 {
		return nil, errors.New("wasmrewrite: decode: unsupported version")
	}
	m := &Module{}
	c := &cursor{b: wasm, pos: 8}
	lastRank := 0
	for c.rest() > 0 && c.err == nil {
		id := c.byte()
		size := c.u32()
		body := c.take(uint64(size))
		if c.err != nil {
			break
		}
		if id == 0 {
			if err := checkCustom(body); err != nil {
				return nil, fmt.Errorf("wasmrewrite: decode: custom section: %w", err)
			}
			continue
		}
		rank := sectionRank(id)
		switch {
		case rank == 0:
			return nil, fmt.Errorf("wasmrewrite: decode: unknown section id %d", id)
		case rank <= lastRank:
			return nil, fmt.Errorf("wasmrewrite: decode: section %d duplicated or out of order", id)
		}
		lastRank = rank
		m.Sections = append(m.Sections, Section{ID: id, Body: body})
		if err := m.decodeSection(id, body); err != nil {
			return nil, fmt.Errorf("wasmrewrite: decode: section %d: %w", id, err)
		}
	}
	if c.err != nil {
		return nil, fmt.Errorf("wasmrewrite: decode: %w", c.err)
	}
	if len(m.Funcs) != len(m.Bodies) {
		return nil, fmt.Errorf("wasmrewrite: decode: %d functions but %d bodies", len(m.Funcs), len(m.Bodies))
	}
	if m.HasDataCount && m.DataCount != m.NumDatas {
		return nil, fmt.Errorf("wasmrewrite: decode: datacount %d but %d data segments", m.DataCount, m.NumDatas)
	}
	return m, nil
}

// checkCustom validates a custom section's name before it is dropped.
func checkCustom(body []byte) error {
	c := &cursor{b: body}
	c.name()
	return c.err
}

func valtypeOK(t byte) bool {
	switch t {
	case 0x7F, 0x7E, 0x7D, 0x7C, 0x70, 0x6F:
		return true
	}
	return false
}

func (c *cursor) valtype() byte {
	t := c.byte()
	if c.err == nil && !valtypeOK(t) {
		c.fail("bad value type 0x%x", t)
	}
	return t
}

func (c *cursor) reftype() byte {
	t := c.byte()
	if c.err == nil && t != 0x70 && t != 0x6F {
		c.fail("bad reference type 0x%x", t)
	}
	return t
}

func (c *cursor) limits() Limits {
	flag := c.byte()
	var l Limits
	if c.err == nil && flag > 7 {
		c.fail("bad limits flag 0x%x", flag)
		return l
	}
	l.HasMax = flag&1 != 0
	l.Shared = flag&2 != 0
	l.Mem64 = flag&4 != 0
	if l.Mem64 {
		l.Min = c.u64()
		if l.HasMax {
			l.Max = c.u64()
		}
		return l
	}
	l.Min = uint64(c.u32())
	if l.HasMax {
		l.Max = uint64(c.u32())
	}
	return l
}

// constExpr reads a constant initialiser (one const-like instruction and
// its end) and returns its bytes.
func (c *cursor) constExpr() []byte {
	if c.err != nil {
		return nil
	}
	rest := c.b[c.pos:]
	end, n := -1, 0
	err := Walk(rest, func(in Instr) error {
		n++
		if in.Op == opEnd {
			if n == 1 {
				return errors.New("empty constant expression")
			}
			end = in.End
			return errStopWalk
		}
		switch {
		case n == 1 && ((in.Op >= 0x41 && in.Op <= 0x44) || in.Op == 0x23 || in.Op == 0xD0 || in.Op == 0xD2):
			return nil
		}
		return fmt.Errorf("opcode 0x%x is not allowed in a constant expression", in.Op)
	})
	if err != nil && !errors.Is(err, errStopWalk) {
		c.fail("%w", err)
		return nil
	}
	if end < 0 {
		c.fail("unterminated constant expression")
		return nil
	}
	return c.take(uint64(end))
}

func (m *Module) decodeSection(id byte, body []byte) error {
	c := &cursor{b: body}
	switch id {
	case 1:
		m.decodeTypes(c)
	case 2:
		m.decodeImports(c)
	case 3:
		for n := c.count(); n > 0 && c.err == nil; n-- {
			m.Funcs = append(m.Funcs, c.u32())
		}
	case 4:
		m.NumTables = c.count()
		for i := uint32(0); i < m.NumTables && c.err == nil; i++ {
			c.reftype()
			if l := c.limits(); l.Shared || l.Mem64 {
				c.fail("bad table limits")
			}
		}
	case 5:
		for n := c.count(); n > 0 && c.err == nil; n-- {
			m.Memories = append(m.Memories, c.limits())
		}
	case 6:
		for n := c.count(); n > 0 && c.err == nil; n-- {
			g := Global{Type: c.valtype()}
			switch mut := c.byte(); mut {
			case 0:
			case 1:
				g.Mutable = true
			default:
				c.fail("bad global mutability %d", mut)
			}
			g.Init = c.constExpr()
			m.Globals = append(m.Globals, g)
		}
	case 7:
		for n := c.count(); n > 0 && c.err == nil; n-- {
			e := Export{Name: c.name(), Kind: c.byte(), Index: 0}
			if c.err == nil && e.Kind > 3 {
				c.fail("bad export kind %d", e.Kind)
			}
			e.Index = c.u32()
			m.Exports = append(m.Exports, e)
		}
	case 8:
		m.Start, m.HasStart = c.u32(), true
	case 9:
		m.decodeElems(c)
	case 11:
		m.decodeDatas(c)
	case 12:
		m.DataCount, m.HasDataCount = c.u32(), true
	case 10:
		m.decodeCode(c)
	}
	if c.err != nil {
		return c.err
	}
	if c.rest() != 0 {
		return fmt.Errorf("%d trailing bytes", c.rest())
	}
	return nil
}

func (m *Module) decodeTypes(c *cursor) {
	for n := c.count(); n > 0 && c.err == nil; n-- {
		if form := c.byte(); c.err == nil && form != 0x60 {
			c.fail("bad function type form 0x%x", form)
			return
		}
		var t FuncType
		for k := c.count(); k > 0 && c.err == nil; k-- {
			t.Params = append(t.Params, c.valtype())
		}
		for k := c.count(); k > 0 && c.err == nil; k-- {
			t.Results = append(t.Results, c.valtype())
		}
		m.Types = append(m.Types, t)
	}
}

func (m *Module) decodeImports(c *cursor) {
	for n := c.count(); n > 0 && c.err == nil; n-- {
		im := Import{Module: c.name(), Name: c.name(), Kind: c.byte()}
		from := c.pos
		switch im.Kind {
		case 0:
			im.Index = c.u32()
		case 1:
			c.reftype()
			c.limits()
		case 2:
			c.limits()
		case 3:
			c.valtype()
			if mut := c.byte(); c.err == nil && mut > 1 {
				c.fail("bad global mutability %d", mut)
			}
		default:
			c.fail("bad import kind %d", im.Kind)
		}
		if im.Kind != 0 && c.err == nil {
			im.Desc = append([]byte(nil), c.b[from:c.pos]...)
		}
		m.Imports = append(m.Imports, im)
	}
}

func (m *Module) decodeCode(c *cursor) {
	for n := c.count(); n > 0 && c.err == nil; n-- {
		size := c.u32()
		raw := c.take(uint64(size))
		if c.err != nil {
			return
		}
		bc := &cursor{b: raw}
		var b Body
		var total uint64
		for k := bc.count(); k > 0 && bc.err == nil; k-- {
			run := LocalRun{Count: bc.u32(), Type: bc.valtype()}
			total += uint64(run.Count)
			if total > 1<<32-1 {
				bc.fail("more than 2^32-1 locals")
			}
			b.Locals = append(b.Locals, run)
		}
		if bc.err == nil && (bc.rest() == 0 || raw[len(raw)-1] != opEnd) {
			bc.fail("body does not end with end")
		}
		if bc.err != nil {
			c.fail("body %d: %w", len(m.Bodies), bc.err)
			return
		}
		b.Code = raw[bc.pos:]
		m.Bodies = append(m.Bodies, b)
	}
}

func (m *Module) importCount(kind byte) uint32 {
	var n uint32
	for _, im := range m.Imports {
		if im.Kind == kind {
			n++
		}
	}
	return n
}

// NumFuncs is the size of the function index space.
func (m *Module) NumFuncs() uint32 { return m.importCount(0) + uint32(len(m.Funcs)) } // #nosec G115 -- counts decoded as u32 from <= MaxWasmBytes input (wasmhost/config.go:34)

// NumGlobals is the size of the global index space.
func (m *Module) NumGlobals() uint32 { return m.importCount(3) + uint32(len(m.Globals)) } // #nosec G115 -- counts decoded as u32 from <= MaxWasmBytes input (wasmhost/config.go:34)

// AddType appends the function type params -> results and returns its
// type index. Existing indices do not shift.
func (m *Module) AddType(params, results []byte) uint32 {
	m.Types = append(m.Types, FuncType{
		Params:  append([]byte(nil), params...),
		Results: append([]byte(nil), results...),
	})
	return uint32(len(m.Types) - 1) // #nosec G115 -- counts decoded as u32 from <= MaxWasmBytes input (wasmhost/config.go:34)
}

// AddGlobal appends a defined global and returns its index in the global
// index space (imported globals first).
func (m *Module) AddGlobal(g Global) uint32 {
	idx := m.NumGlobals()
	m.Globals = append(m.Globals, g)
	return idx
}

// AddExport appends an export.
func (m *Module) AddExport(e Export) {
	m.Exports = append(m.Exports, e)
}

// AddLocal appends one local of type valtype at the end of the locals of
// Bodies[fn] (fn counts defined functions only, not imports) and returns
// its local index (params first).
func (m *Module) AddLocal(fn uint32, valtype byte) uint32 {
	b := &m.Bodies[fn]
	idx := uint32(len(m.Types[m.Funcs[fn]].Params)) // #nosec G115 -- counts decoded as u32 from <= MaxWasmBytes input (wasmhost/config.go:34)
	for _, r := range b.Locals {
		idx += r.Count
	}
	if n := len(b.Locals); n > 0 && b.Locals[n-1].Type == valtype {
		b.Locals[n-1].Count++
	} else {
		b.Locals = append(b.Locals, LocalRun{Count: 1, Type: valtype})
	}
	return idx
}

// decodeElems parses the element section (all eight segment forms) for
// the indices it holds; the section itself stays raw.
func (m *Module) decodeElems(c *cursor) {
	m.NumElems = c.count()
	for i := uint32(0); i < m.NumElems && c.err == nil; i++ {
		flag := c.u32()
		if c.err == nil && flag > 7 {
			c.fail("bad element segment flag %d", flag)
			return
		}
		if flag&1 == 0 { // active
			table := uint32(0)
			if flag&2 != 0 {
				table = c.u32()
			}
			m.ElemTables = append(m.ElemTables, table)
			m.InitExprs = append(m.InitExprs, c.constExpr())
		}
		if flag&3 != 0 {
			if flag&4 != 0 {
				c.reftype()
			} else if kind := c.byte(); c.err == nil && kind != 0 {
				c.fail("bad element kind %d", kind)
			}
		}
		for n := c.count(); n > 0 && c.err == nil; n-- {
			if flag&4 != 0 {
				m.InitExprs = append(m.InitExprs, c.constExpr())
			} else {
				m.ElemFuncs = append(m.ElemFuncs, c.u32())
			}
		}
	}
}

// decodeDatas parses the data section for memory indices and offsets.
func (m *Module) decodeDatas(c *cursor) {
	m.NumDatas = c.count()
	for i := uint32(0); i < m.NumDatas && c.err == nil; i++ {
		flag := c.u32()
		if c.err == nil && flag > 2 {
			c.fail("bad data segment flag %d", flag)
			return
		}
		if flag != 1 { // active
			mem := uint32(0)
			if flag == 2 {
				mem = c.u32()
			}
			m.DataMems = append(m.DataMems, mem)
			m.InitExprs = append(m.InitExprs, c.constExpr())
		}
		c.take(uint64(c.u32()))
	}
}
