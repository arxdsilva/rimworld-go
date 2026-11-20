// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package wasmrewrite

import (
	"encoding/binary"
	"fmt"
)

const (
	opLocalGet = 0x20
	opLocalSet = 0x21
	opF32Const = 0x43
	opF64Const = 0x44
	opF32Eq    = 0x5B
	opF64Eq    = 0x61
	opSelect   = 0x1B

	valF32 = 0x7D
	valF64 = 0x7C

	canonNaN32 uint32 = 0x7FC00000
	canonNaN64 uint64 = 0x7FF8000000000000
)

// nanKind reports the float type a covered op produces, or 0 when the op is
// not covered. Covered: f32/f64 add, sub, mul, div, sqrt, min, max, ceil,
// floor, trunc, nearest, f32.demote_f64 and f64.promote_f32. Not covered
// (they keep their bits): abs, neg, copysign, const, load, store, reinterpret.
func nanKind(op uint32) byte {
	switch {
	case op >= 0x8D && op <= 0x97, op == 0xB6:
		return valF32
	case op >= 0x9B && op <= 0xA5, op == 0xBB:
		return valF64
	}
	return 0
}

// CanonicaliseNaN follows every f32/f64 add, sub, mul, div, sqrt, min,
// max, ceil, floor, trunc, nearest, f32.demote_f64 and f64.promote_f32
// with a select that replaces any NaN by the canonical one (0x7FC00000,
// 0x7FF8000000000000), using one scratch local per float type per body.
// The inserted sequence is
//
//	local.set t; local.get t; f.const canon; local.get t; local.get t; f.eq; select
//
// which leaves the stack as the op left it, so it is valid in unreachable
// code too. Scratch locals are added only to bodies that need them.
func CanonicaliseNaN(m *Module) error {
	for fn := range m.Bodies {
		code, err := nanBody(m, uint32(fn))
		if err != nil {
			return fmt.Errorf("wasmrewrite: nan: function %d: %w", uint32(len(m.Imports))+uint32(fn), err)
		}
		m.Bodies[fn].Code = code
	}
	return nil
}

func nanBody(m *Module, fn uint32) ([]byte, error) {
	src := m.Bodies[fn].Code
	var out []byte
	last := 0
	var t32, t64 uint32
	has32, has64 := false, false
	err := Walk(src, func(in Instr) error {
		k := nanKind(in.Op)
		if k == 0 {
			return nil
		}
		out = append(out, src[last:in.End]...)
		last = in.End
		var t uint32
		if k == valF32 {
			if !has32 {
				t32, has32 = m.AddLocal(fn, valF32), true
			}
			t = t32
		} else {
			if !has64 {
				t64, has64 = m.AddLocal(fn, valF64), true
			}
			t = t64
		}
		out = appendCanon(out, k, t)
		return nil
	})
	if err != nil {
		return nil, err
	}
	if last == 0 {
		return src, nil
	}
	return append(out, src[last:]...), nil
}

func appendCanon(b []byte, k byte, t uint32) []byte {
	b = append(b, opLocalSet)
	b = appendU32(b, t)
	b = append(b, opLocalGet)
	b = appendU32(b, t)
	if k == valF32 {
		b = append(b, opF32Const)
		b = binary.LittleEndian.AppendUint32(b, canonNaN32)
	} else {
		b = append(b, opF64Const)
		b = binary.LittleEndian.AppendUint64(b, canonNaN64)
	}
	b = append(b, opLocalGet)
	b = appendU32(b, t)
	b = append(b, opLocalGet)
	b = appendU32(b, t)
	if k == valF32 {
		b = append(b, opF32Eq)
	} else {
		b = append(b, opF64Eq)
	}
	return append(b, opSelect)
}
