// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package wasmrewrite

import (
	"encoding/binary"
	"errors"
	"fmt"
)

// Instr is one decoded instruction.
//
// Op is the opcode byte for a one-byte opcode, and prefix<<16 | sub for a
// prefixed one (0xFC, 0xFD, 0xFE): memory.fill is 0xFC<<16 | 11.
// A and B are the first and second immediates in binary order: an index,
// a memarg's align and offset, call_indirect's type and table, a constant
// (i32/i64 sign-extended to 64 bits, f32/f64 as their bits) or a
// blocktype (its s33 value as int64 bits: -64 empty, negative a valtype,
// otherwise a type index). Labels are br_table's targets with the default
// last; Types are select t's valtypes. Start and End are the byte span of
// the whole instruction in the code it was walked from.
type Instr struct {
	Op         uint32
	A, B       uint64
	Labels     []uint32
	Types      []byte
	Start, End int
}

// Opcodes the package itself needs by name.
const (
	opBlock = 0x02
	opLoop  = 0x03
	opIf    = 0x04
	opElse  = 0x05
	opEnd   = 0x0B
)

var errUnknownOpcode = errors.New("unknown opcode")

// Walk decodes code (a body's instructions) in order and calls fn for each
// instruction; it stops at the first error from fn or from decoding. An
// unknown opcode is an error. It is the only decoder of instruction
// streams.
//
// Every opcode of the allow-list and every opcode Check rejects by name
// (tail calls, exceptions, table ops, threads, SIMD) is decoded with its
// immediates, so a rejection can name the opcode. SIMD and threads
// immediates are decoded by shape only; Check rejects them at once.
func Walk(code []byte, fn func(Instr) error) error {
	for pos := 0; pos < len(code); {
		in, err := decodeInstr(code, pos)
		if err != nil {
			return err
		}
		if err := fn(in); err != nil {
			return err
		}
		pos = in.End
	}
	return nil
}

func decodeInstr(code []byte, start int) (Instr, error) {
	c := &cursor{b: code, pos: start + 1}
	op := code[start]
	in := Instr{Op: uint32(op), Start: start}
	switch {
	case op == opBlock || op == opLoop || op == opIf || op == 0x06:
		in.A = uint64(c.s33()) // #nosec G115 -- signed immediate kept as its u64 bit pattern
	case op == 0x0C || op == 0x0D || op == 0x10 || op == 0x12 || op == 0x07 ||
		op == 0x08 || op == 0x09 || op == 0x18 || (op >= 0x20 && op <= 0x26) || op == 0xD2:
		in.A = uint64(c.u32())
	case op == 0x0E:
		n := c.count()
		for i := uint32(0); i <= n && c.err == nil; i++ {
			in.Labels = append(in.Labels, c.u32())
		}
	case op == 0x11 || op == 0x13:
		in.A = uint64(c.u32())
		in.B = uint64(c.u32())
	case op == 0x1C:
		n := c.count()
		in.Types = append([]byte(nil), c.take(uint64(n))...)
	case op == 0x1F:
		in.A = uint64(c.s33()) // #nosec G115 -- signed immediate kept as its u64 bit pattern
		n := c.count()
		for i := uint32(0); i < n && c.err == nil; i++ {
			if kind := c.byte(); kind < 2 {
				c.u32() // tag
			} else if kind > 3 {
				c.fail("bad catch clause kind %d", kind)
			}
			in.Labels = append(in.Labels, c.u32())
		}
	case op >= 0x28 && op <= 0x3E:
		memarg(c, &in)
	case op == 0x3F || op == 0x40:
		in.A = uint64(c.u32())
	case op == 0x41:
		in.A = uint64(int64(c.s32())) // #nosec G115 -- signed immediate kept as its u64 bit pattern
	case op == 0x42:
		in.A = uint64(c.s64()) // #nosec G115 -- signed immediate kept as its u64 bit pattern
	case op == 0x43:
		if b := c.take(4); b != nil {
			in.A = uint64(binary.LittleEndian.Uint32(b))
		}
	case op == 0x44:
		if b := c.take(8); b != nil {
			in.A = binary.LittleEndian.Uint64(b)
		}
	case op == 0xD0:
		in.A = uint64(c.byte())
		if c.err == nil && in.A != 0x70 && in.A != 0x6F {
			c.fail("bad reference type 0x%x", in.A)
		}
	case op == 0xFC:
		prefixFC(c, &in)
	case op == 0xFD:
		prefixSIMD(c, &in)
	case op == 0xFE:
		prefixThreads(c, &in)
	case noImmediate(op):
	default:
		return in, fmt.Errorf("%w 0x%02x at byte %d", errUnknownOpcode, op, start)
	}
	if c.err != nil {
		if errors.Is(c.err, errUnknownOpcode) {
			return in, c.err
		}
		return in, fmt.Errorf("malformed instruction at byte %d: %w", start, c.err)
	}
	in.End = c.pos
	return in, nil
}

func noImmediate(op byte) bool {
	switch {
	case op <= 0x01, op == opElse, op == opEnd, op == 0x0F, op == 0x19,
		op == 0x1A, op == 0x1B, op == 0xD1:
		return true
	}
	return op >= 0x45 && op <= 0xC4
}

func memarg(c *cursor, in *Instr) {
	align := c.u32()
	if c.err == nil && align >= 64 {
		c.fail("memarg alignment %d (multi-memory is not supported)", align)
	}
	in.A = uint64(align)
	in.B = uint64(c.u32())
}

func prefixFC(c *cursor, in *Instr) {
	sub := c.u32()
	in.Op = 0xFC<<16 | sub
	switch {
	case sub <= 7:
	case sub == 8 || sub == 10 || sub == 12 || sub == 14:
		in.A = uint64(c.u32())
		in.B = uint64(c.u32())
	case sub == 9 || sub == 11 || sub == 13 || (sub >= 15 && sub <= 17):
		in.A = uint64(c.u32())
	default:
		c.fail("%w 0xFC %d", errUnknownOpcode, sub)
	}
}

func prefixSIMD(c *cursor, in *Instr) {
	sub := c.u32()
	in.Op = 0xFD<<16 | sub
	switch {
	case sub <= 11 || sub == 92 || sub == 93:
		memarg(c, in)
	case sub == 12 || sub == 13:
		c.take(16)
	case sub >= 21 && sub <= 34:
		c.byte()
	case sub >= 84 && sub <= 91:
		memarg(c, in)
		c.byte()
	}
}

func prefixThreads(c *cursor, in *Instr) {
	sub := c.u32()
	in.Op = 0xFE<<16 | sub
	switch {
	case sub == 3:
		c.byte()
	case sub <= 2 || (sub >= 0x10 && sub <= 0x4E):
		memarg(c, in)
	default:
		c.fail("%w 0xFE %d", errUnknownOpcode, sub)
	}
}
