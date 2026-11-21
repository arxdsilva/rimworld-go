// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package wasmrewrite

import (
	"errors"
	"fmt"
	"strings"
)

// RejectError is a Check rejection: the rule that was broken, the
// function it was found in (-1 for module level) and the detail.
type RejectError struct {
	Rule   string
	Func   int
	Detail string
}

func (e *RejectError) Error() string {
	s := "wasmrewrite: " + e.Rule
	if e.Func >= 0 {
		s += fmt.Sprintf(" in function %d", e.Func)
	}
	if e.Detail != "" {
		s += ": " + e.Detail
	}
	return s
}

// Rule names carried by RejectError.Rule.
const (
	RuleSIMD        = "simd opcode"
	RuleThreads     = "threads opcode"
	RuleTailCall    = "tail call opcode"
	RuleException   = "exception opcode"
	RuleTable       = "table opcode"
	RuleUnknownOp   = "unknown opcode"
	RuleMalformed   = "malformed body"
	RuleImportMod   = "import module"
	RuleImportKind  = "import kind"
	RuleMemory      = "memory"
	RuleStart       = "start section"
	RuleReserved    = "reserved export"
	RuleLocals      = "too many locals"
	RuleBody        = "body too large"
	RuleArity       = "arity"
	RuleIndex       = "index out of range"
	RuleImmutable   = "immutable global"
	RuleRefGlobal   = "mutable reference global"
	RuleBlockType   = "bad blocktype"
	RuleSelectTypes = "bad select types"
)

// spaces holds the original index-space sizes.
type spaces struct {
	types, funcs, tables, mems, globals, datas, elems uint32
	immutableGlobal                                   []bool
}

func reject(rule string, fn int, format string, args ...any) error {
	return &RejectError{Rule: rule, Func: fn, Detail: fmt.Sprintf(format, args...)}
}

// Check enforces the allow-list on the original module before any
// rewrite: opcodes, imports, memory, start section, reserved export
// names, type arity, body size and locals, and every index immediate
// against the original counts. Each error names the rule it broke.
func Check(m *Module, c Config) error {
	if m.HasStart {
		return reject(RuleStart, -1, "start section is not allowed")
	}
	for i, im := range m.Imports {
		if im.Kind != 0 {
			return reject(RuleImportKind, -1, "import %d (%s.%s) is not a function", i, im.Module, im.Name)
		}
		if im.Module != c.Namespace {
			return reject(RuleImportMod, -1, "import %d module %q is not %q", i, im.Module, c.Namespace)
		}
		if im.Index >= uint32(len(m.Types)) {
			return reject(RuleIndex, -1, "import %d type %d of %d", i, im.Index, len(m.Types))
		}
	}
	if len(m.Memories) > 1 {
		return reject(RuleMemory, -1, "%d memories, at most 1", len(m.Memories))
	}
	for _, l := range m.Memories {
		if l.Shared {
			return reject(RuleMemory, -1, "shared memory")
		}
		if l.Mem64 {
			return reject(RuleMemory, -1, "64-bit memory")
		}
	}
	for i, t := range m.Types {
		if uint64(len(t.Params)) > uint64(c.MaxArity) || uint64(len(t.Results)) > uint64(c.MaxArity) {
			return reject(RuleArity, -1, "arity over %d: type %d has %d params and %d results",
				c.MaxArity, i, len(t.Params), len(t.Results))
		}
	}
	sp := spaces{
		types: uint32(len(m.Types)), funcs: m.NumFuncs(), tables: m.NumTables,
		mems: uint32(len(m.Memories)), globals: m.NumGlobals(), elems: m.NumElems, datas: m.NumDatas,
	}
	if m.HasDataCount {
		sp.datas = m.DataCount
	}
	for _, g := range m.Globals {
		sp.immutableGlobal = append(sp.immutableGlobal, !g.Mutable)
	}
	if err := checkSpaces(m, c, sp); err != nil {
		return err
	}
	base := m.NumFuncs() - uint32(len(m.Funcs))
	for i := range m.Bodies {
		if err := checkBody(m, c, sp, i, int(base)+i); err != nil {
			return err
		}
	}
	return nil
}

func checkSpaces(m *Module, c Config, sp spaces) error {
	for i, t := range m.Funcs {
		if t >= sp.types {
			return reject(RuleIndex, -1, "function %d type %d of %d", i, t, sp.types)
		}
	}
	for i, g := range m.Globals {
		if err := checkInit(g.Init, sp); err != nil {
			return reject(RuleIndex, -1, "global %d init: %v", i, err)
		}
		if g.Mutable && !numeric(g.Type) { // the save copy could not carry it
			return reject(RuleRefGlobal, -1, "global %d", i)
		}
	}
	if err := checkSections(m, sp); err != nil {
		return err
	}
	for _, e := range m.Exports {
		if strings.HasPrefix(e.Name, "__rimworld") {
			return reject(RuleReserved, -1, "export %q", e.Name)
		}
		limit := [...]uint32{sp.funcs, sp.tables, sp.mems, sp.globals}[e.Kind&3]
		if e.Index >= limit {
			return reject(RuleIndex, -1, "export %q index %d of %d", e.Name, e.Index, limit)
		}
	}
	return nil
}

// checkSections bounds-checks what the element and data sections hold
// against the original counts: function indices, table and memory
// indices, and every constant expression's indices.
func checkSections(m *Module, sp spaces) error {
	for i, f := range m.ElemFuncs {
		if f >= sp.funcs {
			return reject(RuleIndex, -1, "element function index %d of %d (entry %d)", f, sp.funcs, i)
		}
	}
	for _, t := range m.ElemTables {
		if t >= sp.tables {
			return reject(RuleIndex, -1, "element table %d of %d", t, sp.tables)
		}
	}
	for _, mem := range m.DataMems {
		if mem >= sp.mems {
			return reject(RuleIndex, -1, "data memory %d of %d", mem, sp.mems)
		}
	}
	for i, e := range m.InitExprs {
		if err := checkInit(e, sp); err != nil {
			return reject(RuleIndex, -1, "constant expression %d: %v", i, err)
		}
	}
	return nil
}

// checkInit validates the indices in a global's constant initialiser.
func checkInit(init []byte, sp spaces) error {
	return Walk(init, func(in Instr) error {
		switch in.Op {
		case 0x23: // global.get: only imported globals may appear, and none are allowed
			return fmt.Errorf("global.get %d in a constant expression", in.A)
		case 0xD2:
			if in.A >= uint64(sp.funcs) {
				return fmt.Errorf("ref.func %d of %d", in.A, sp.funcs)
			}
		}
		return nil
	})
}

func classify(op uint32) string {
	switch op >> 16 {
	case 0xFD:
		return RuleSIMD
	case 0xFE:
		return RuleThreads
	case 0xFC:
		switch op & 0xFFFF {
		case 12, 14, 15, 16, 17:
			return RuleTable
		}
		return ""
	}
	switch op {
	case 0x12, 0x13:
		return RuleTailCall
	case 0x06, 0x07, 0x08, 0x09, 0x18, 0x19, 0x1F:
		return RuleException
	case 0x26:
		return RuleTable
	}
	return ""
}

func checkBody(m *Module, c Config, sp spaces, i, fn int) error {
	body := m.Bodies[i]
	params := uint64(len(m.Types[m.Funcs[i]].Params))
	locals := params
	for _, r := range body.Locals {
		locals += uint64(r.Count)
	}
	if locals > uint64(c.MaxLocals) {
		return reject(RuleLocals, fn, "%d params and locals, at most %d", locals, c.MaxLocals)
	}
	if uint64(len(body.Code)) > uint64(c.MaxBody) {
		return reject(RuleBody, fn, "%d bytes, at most %d", len(body.Code), c.MaxBody)
	}
	depth, ended := 0, false
	idx := func(in Instr, what string, v uint64, limit uint32) error {
		if v >= uint64(limit) {
			return reject(RuleIndex, fn, "%s %d of %d at byte %d", what, v, limit, in.Start)
		}
		return nil
	}
	err := Walk(body.Code, func(in Instr) error {
		if ended {
			return reject(RuleMalformed, fn, "instruction after the final end at byte %d", in.Start)
		}
		if rule := classify(in.Op); rule != "" {
			return reject(rule, fn, "opcode 0x%x at byte %d", in.Op, in.Start)
		}
		switch in.Op {
		case opBlock, opLoop, opIf:
			depth++
			return checkBlockType(in, fn, sp)
		case opEnd:
			if depth == 0 {
				ended = true
			} else {
				depth--
			}
		case 0x0C, 0x0D:
			return labelOK(in, fn, in.A, depth)
		case 0x0E:
			for _, l := range in.Labels {
				if err := labelOK(in, fn, uint64(l), depth); err != nil {
					return err
				}
			}
		case 0x10, 0xD2:
			return idx(in, "function", in.A, sp.funcs)
		case 0x11:
			if err := idx(in, "type", in.A, sp.types); err != nil {
				return err
			}
			return idx(in, "table", in.B, sp.tables)
		case 0x1C:
			if len(in.Types) != 1 {
				return reject(RuleSelectTypes, fn, "select with %d types at byte %d", len(in.Types), in.Start)
			}
		case 0x20, 0x21, 0x22:
			return idx(in, "local", in.A, uint32(min(locals, 1<<32-1)))
		case 0x23:
			return idx(in, "global", in.A, sp.globals)
		case 0x24:
			if err := idx(in, "global", in.A, sp.globals); err != nil {
				return err
			}
			if in.A < uint64(len(sp.immutableGlobal)) && sp.immutableGlobal[in.A] {
				return reject(RuleImmutable, fn, "global.set %d at byte %d", in.A, in.Start)
			}
		case 0x25:
			return idx(in, "table", in.A, sp.tables)
		case 0x3F, 0x40:
			return idx(in, "memory", in.A, sp.mems)
		case 0xFC<<16 | 8:
			if err := idx(in, "data", in.A, sp.datas); err != nil {
				return err
			}
			return idx(in, "memory", in.B, sp.mems)
		case 0xFC<<16 | 9:
			return idx(in, "data", in.A, sp.datas)
		case 0xFC<<16 | 10:
			if err := idx(in, "memory", in.A, sp.mems); err != nil {
				return err
			}
			return idx(in, "memory", in.B, sp.mems)
		case 0xFC<<16 | 11:
			return idx(in, "memory", in.A, sp.mems)
		case 0xFC<<16 | 13:
			return idx(in, "elem", in.A, sp.elems)
		default:
			if in.Op >= 0x28 && in.Op <= 0x3E {
				return idx(in, "memory", 0, sp.mems)
			}
		}
		return nil
	})
	if err != nil {
		var re *RejectError
		if errors.As(err, &re) {
			return err
		}
		if errors.Is(err, errUnknownOpcode) {
			return reject(RuleUnknownOp, fn, "%v", err)
		}
		return reject(RuleMalformed, fn, "%v", err)
	}
	if !ended {
		return reject(RuleMalformed, fn, "missing final end")
	}
	return nil
}

func labelOK(in Instr, fn int, l uint64, depth int) error {
	if l > uint64(depth) {
		return reject(RuleIndex, fn, "label %d with %d open blocks at byte %d", l, depth, in.Start)
	}
	return nil
}

func checkBlockType(in Instr, fn int, sp spaces) error {
	bt := int64(in.A)
	switch {
	case bt >= 0:
		if bt >= int64(sp.types) {
			return reject(RuleIndex, fn, "blocktype type %d of %d at byte %d", bt, sp.types, in.Start)
		}
	case bt == -64, bt >= -4 && bt <= -1, bt == -16, bt == -17:
	default:
		return reject(RuleBlockType, fn, "blocktype %d at byte %d", bt, in.Start)
	}
	return nil
}
