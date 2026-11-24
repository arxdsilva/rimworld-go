// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package wasmrewrite

import (
	"errors"
	"fmt"
)

// DefaultMaxWeight is Config.MaxWeight when it is zero.
const DefaultMaxWeight = 1 << 17

// RuleWeight is Meter's rejection of a function whose weight w(f) is over
// Config.MaxWeight.
const RuleWeight = "function weight over budget"

// Opcodes the metering sequences use.
const (
	fuelOpUnreachable    = 0x00
	fuelOpBr             = 0x0C
	fuelOpReturn         = 0x0F
	fuelOpCall           = 0x10
	fuelOpCallIndirect   = 0x11
	fuelOpLocalGet       = 0x20
	fuelOpLocalSet       = 0x21
	fuelOpGlobalGet      = 0x23
	fuelOpGlobalSet      = 0x24
	fuelOpI64Const       = 0x42
	fuelOpI64LtS         = 0x53
	fuelOpI64GtU         = 0x56
	fuelOpI64Add         = 0x7C
	fuelOpI64Sub         = 0x7D
	fuelOpI64ExtendI32U  = 0xAD
	fuelBlockEmpty       = 0x40
	fuelValI32           = 0x7F
	fuelEmptyBlockS33    = -64
	fuelMemoryInit       = 0xFC<<16 | 8
	fuelMemoryCopy       = 0xFC<<16 | 10
	fuelMemoryFill       = 0xFC<<16 | 11
	fuelPrefixFC         = 0xFC
	fuelPrefixFCSubLimit = 7 // 0xFC 0-7 (sat-trunc) push one value
)

// Meter inserts the fuel charges (function entry, after each loop opcode,
// bulk memory ops by length) and the weighted call-depth charge around
// every body. Reserve must have run; imported functions are not touched.
//
// The static cost of a region is 1 + the instructions of the body Meter
// receives (after CanonicaliseNaN, before any metering) that belong to it.
// Each instruction belongs to its innermost enclosing loop, or to the
// function: a loop opcode belongs to the enclosing region, the
// instructions after it up to and including its end belong to the loop.
// else, end and unreachable code count like any other instruction. The
// function region is charged once at entry, a loop region each time
// control enters the loop (first entry and every branch back).
//
// A body becomes:
//
//	depth += w; if depth >u StackBudget { unreachable }
//	fuel charge of the function region
//	block (type [] -> results)
//	  the original body, each return turned into a br to this block,
//	  each loop followed by its fuel charge, each memory.fill,
//	  memory.copy and memory.init preceded by its length charge
//	end
//	depth -= w
//
// w = max(params + locals + V, StackBudget/MaxDepth), V summing 1 + pushes
// over every instruction of that final body (see Config and wasmhost's
// doc).
// A function whose w is over Config.MaxWeight (DefaultMaxWeight when 0)
// is rejected with a *RejectError of RuleWeight.
func Meter(m *Module, c Config) error {
	if c.MaxDepth == 0 || c.StackBudget == 0 {
		return errors.New("wasmrewrite: meter: MaxDepth and StackBudget must be positive")
	}
	if m.FuelGlobal == m.DepthGlobal {
		return errors.New("wasmrewrite: meter: the reserved globals are missing (run Reserve first)")
	}
	ftypes := make([]uint32, 0, m.NumFuncs())
	for _, im := range m.Imports {
		if im.Kind == 0 {
			ftypes = append(ftypes, im.Index)
		}
	}
	ftypes = append(ftypes, m.Funcs...)
	imported := uint32(len(ftypes) - len(m.Funcs)) // #nosec G115 -- counts decoded as u32 from <= MaxWasmBytes input (wasmhost/config.go:34)
	mt := &meter{Module: m}
	for i := range m.Bodies {
		if err := mt.meterBody(uint32(i), imported+uint32(i), ftypes, c); err != nil {
			if _, ok := err.(*RejectError); ok {
				return err
			}
			return fmt.Errorf("wasmrewrite: meter: function %d: %w", imported+uint32(i), err)
		}
	}
	return nil
}

// meter holds the metering steps, so they stay off Module's method set.
type meter struct {
	*Module
}

func (m *meter) meterBody(fn, funcIndex uint32, ftypes []uint32, c Config) error {
	if m.Funcs[fn] >= uint32(len(m.Types)) { // #nosec G115 -- counts decoded as u32 from <= MaxWasmBytes input (wasmhost/config.go:34)
		return errors.New("type index out of range")
	}
	var ins []Instr
	if err := Walk(m.Bodies[fn].Code, func(in Instr) error {
		ins = append(ins, in)
		return nil
	}); err != nil {
		return err
	}
	costs := fuelRegionCosts(ins)
	var scratch uint32
	for _, in := range ins {
		if fuelBulk(in.Op) {
			scratch = m.AddLocal(fn, fuelValI32)
			break
		}
	}
	body := &m.Bodies[fn]
	inner := m.fuelInner(body.Code, ins, costs, scratch)
	wrapper := m.fuelWrapper(m.Types[m.Funcs[fn]].Results)
	assemble := func(w uint64) []byte {
		out := make([]byte, 0, len(inner)+64)
		out = m.fuelDepthCharge(out, w, c.StackBudget)
		out = m.fuelCharge(out, costs[0])
		out = append(out, wrapper...)
		out = append(out, inner...)
		out = m.fuelDepthRelease(out, w)
		return append(out, opEnd)
	}
	v, err := m.fuelValues(assemble(0), ftypes)
	if err != nil {
		return err
	}
	slots := uint64(len(m.Types[m.Funcs[fn]].Params))
	for _, r := range body.Locals {
		slots += uint64(r.Count)
	}
	w := max(slots+v, c.StackBudget/uint64(c.MaxDepth))
	limit := c.MaxWeight
	if limit == 0 {
		limit = DefaultMaxWeight
	}
	if w > limit {
		return reject(RuleWeight, int(funcIndex), "weight %d, at most %d", w, limit)
	}
	body.Code = assemble(w)
	return nil
}

func fuelBulk(op uint32) bool {
	return op == fuelMemoryInit || op == fuelMemoryCopy || op == fuelMemoryFill
}

// fuelRegionCosts returns the static cost of the function region at index
// 0 and of the k-th loop (in code order) at index k.
func fuelRegionCosts(ins []Instr) []uint64 {
	costs := []uint64{1}
	var open []int // the region enclosing each open block
	cur := 0
	for _, in := range ins {
		costs[cur]++
		switch in.Op {
		case opBlock, opIf:
			open = append(open, cur)
		case opLoop:
			open = append(open, cur)
			costs = append(costs, 1)
			cur = len(costs) - 1
		case opEnd:
			if n := len(open); n > 0 {
				cur = open[n-1]
				open = open[:n-1]
			}
		}
	}
	return costs
}

// fuelInner copies the body with each return turned into a br to the
// wrapper, a fuel charge after each loop opcode and a length charge
// before each bulk op. Its final end closes the wrapper.
func (m *meter) fuelInner(code []byte, ins []Instr, costs []uint64, scratch uint32) []byte {
	out := make([]byte, 0, len(code)+len(code)/4+64)
	depth, loops := 0, 0
	for _, in := range ins {
		switch {
		case in.Op == fuelOpReturn:
			out = append(out, fuelOpBr)
			out = appendU32(out, uint32(depth))
			continue
		case fuelBulk(in.Op):
			out = append(out, fuelOpLocalSet)
			out = appendU32(out, scratch)
			out = m.fuelGlobal(out, fuelOpGlobalGet, m.FuelGlobal)
			out = append(out, fuelOpLocalGet)
			out = appendU32(out, scratch)
			out = append(out, fuelOpI64ExtendI32U, fuelOpI64Sub)
			out = m.fuelGlobal(out, fuelOpGlobalSet, m.FuelGlobal)
			out = m.fuelCheck(out)
			out = append(out, fuelOpLocalGet)
			out = appendU32(out, scratch)
		}
		out = append(out, code[in.Start:in.End]...)
		switch in.Op {
		case opBlock, opIf:
			depth++
		case opLoop:
			depth++
			loops++
			out = m.fuelCharge(out, costs[loops])
		case opEnd:
			depth--
		}
	}
	return out
}

func (m *meter) fuelGlobal(out []byte, op byte, idx uint32) []byte {
	return appendU32(append(out, op), idx)
}

func fuelConst(out []byte, v uint64) []byte {
	return appendS(append(out, fuelOpI64Const), int64(v)) // #nosec G115 -- signed immediate kept as its u64 bit pattern
}

// fuelCharge is: fuel -= cost; if fuel < 0 { unreachable }.
func (m *meter) fuelCharge(out []byte, cost uint64) []byte {
	out = m.fuelGlobal(out, fuelOpGlobalGet, m.FuelGlobal)
	out = fuelConst(out, cost)
	out = append(out, fuelOpI64Sub)
	out = m.fuelGlobal(out, fuelOpGlobalSet, m.FuelGlobal)
	return m.fuelCheck(out)
}

// fuelCheck is: if fuel < 0 { unreachable }.
func (m *meter) fuelCheck(out []byte) []byte {
	out = m.fuelGlobal(out, fuelOpGlobalGet, m.FuelGlobal)
	out = fuelConst(out, 0)
	return append(out, fuelOpI64LtS, opIf, fuelBlockEmpty, fuelOpUnreachable, opEnd)
}

// fuelDepthCharge is: depth += w; if depth >u budget { unreachable }.
func (m *meter) fuelDepthCharge(out []byte, w, budget uint64) []byte {
	out = m.fuelGlobal(out, fuelOpGlobalGet, m.DepthGlobal)
	out = fuelConst(out, w)
	out = append(out, fuelOpI64Add)
	out = m.fuelGlobal(out, fuelOpGlobalSet, m.DepthGlobal)
	out = m.fuelGlobal(out, fuelOpGlobalGet, m.DepthGlobal)
	out = fuelConst(out, budget)
	return append(out, fuelOpI64GtU, opIf, fuelBlockEmpty, fuelOpUnreachable, opEnd)
}

// fuelDepthRelease is: depth -= w.
func (m *meter) fuelDepthRelease(out []byte, w uint64) []byte {
	out = m.fuelGlobal(out, fuelOpGlobalGet, m.DepthGlobal)
	out = fuelConst(out, w)
	out = append(out, fuelOpI64Sub)
	return m.fuelGlobal(out, fuelOpGlobalSet, m.DepthGlobal)
}

// fuelWrapper is the block opcode and blocktype of the wrapper: empty, a
// single valtype, or a [] -> results type, reused when one exists and
// appended otherwise.
func (m *meter) fuelWrapper(results []byte) []byte {
	switch len(results) {
	case 0:
		return []byte{opBlock, fuelBlockEmpty}
	case 1:
		return []byte{opBlock, results[0]}
	}
	idx := -1
	for i, t := range m.Types {
		if len(t.Params) == 0 && string(t.Results) == string(results) {
			idx = i
			break
		}
	}
	if idx < 0 {
		idx = int(m.AddType(nil, results))
	}
	return appendS([]byte{opBlock}, int64(idx))
}

// fuelValues is V: the sum over every instruction of code of 1 + the
// values it pushes.
func (m *meter) fuelValues(code []byte, ftypes []uint32) (uint64, error) {
	var v uint64
	err := Walk(code, func(in Instr) error {
		p, err := m.fuelPushes(in, ftypes)
		v += 1 + p
		return err
	})
	return v, err
}

// fuelPushes is how many values in pushes: a call or call_indirect its
// callee type's result arity, a block, loop or if its blocktype's result
// arity, every other op its own result count (0 or 1).
func (m *meter) fuelPushes(in Instr, ftypes []uint32) (uint64, error) {
	results := func(t uint64) (uint64, error) {
		if t >= uint64(len(m.Types)) {
			return 0, fmt.Errorf("type %d out of range at byte %d", t, in.Start)
		}
		return uint64(len(m.Types[t].Results)), nil
	}
	op := in.Op
	switch {
	case op == opBlock || op == opLoop || op == opIf:
		switch bt := int64(in.A); { // #nosec G115 -- signed immediate kept as its u64 bit pattern
		case bt == fuelEmptyBlockS33:
			return 0, nil
		case bt < 0:
			return 1, nil
		}
		return results(in.A)
	case op == fuelOpCall:
		if in.A >= uint64(len(ftypes)) {
			return 0, fmt.Errorf("function %d out of range at byte %d", in.A, in.Start)
		}
		return results(uint64(ftypes[in.A]))
	case op == fuelOpCallIndirect:
		return results(in.A)
	case op == 0x1B || op == 0x1C || op == fuelOpLocalGet || op == 0x22 || op == fuelOpGlobalGet || op == 0x25,
		op >= 0x28 && op <= 0x35, // loads
		op >= 0x3F && op <= 0xC4, // memory.size, memory.grow, consts, numeric ops
		op >= 0xD0 && op <= 0xD2, // ref.null, ref.is_null, ref.func
		op>>16 == fuelPrefixFC && op&0xFFFF <= fuelPrefixFCSubLimit:
		return 1, nil
	}
	return 0, nil
}
