// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package wasmrewrite

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

// Walk decodes code (a body's instructions) in order and calls fn for each
// instruction; it stops at the first error from fn or from decoding. An
// unknown opcode is an error. It is the only decoder of instruction
// streams.
func Walk(code []byte, fn func(Instr) error) error {
	return errNotYet // W0 stub
}
