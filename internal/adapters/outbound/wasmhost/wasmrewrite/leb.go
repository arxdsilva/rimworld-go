// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package wasmrewrite

import (
	"errors"
	"fmt"
	"unicode/utf8"
)

// LEB128 decode and encode (u32, u64, s32, s33, s64) and the sticky-error
// cursor that Decode and Walk read bytes with.
//
// Decoding accepts overlong encodings (wasm-ld pads to five bytes) as long
// as they stay within the type's maximum byte count and the unused bits of
// the last byte are zero (unsigned) or a sign extension (signed), the same
// rule the spec and wazero apply. Truncated or over-long input is an error.

var (
	errLEBTruncated = errors.New("leb128 truncated")
	errLEBTooLong   = errors.New("leb128 too long or unused bits set")
)

// readLEB decodes one LEB128 of the given width. A signed result is sign
// extended to 64 bits. It returns the value and the bytes consumed.
func readLEB(b []byte, bits uint, signed bool) (uint64, int, error) {
	maxBytes := int((bits + 6) / 7)
	var result uint64
	var shift uint
	for i := 0; i < len(b); i++ {
		if i >= maxBytes {
			return 0, 0, errLEBTooLong
		}
		c := b[i]
		result |= uint64(c&0x7f) << shift
		shift += 7
		if c&0x80 != 0 {
			continue
		}
		if i == maxBytes-1 {
			used := bits - 7*uint(i) // payload bits that count in the last byte
			if used < 7 {
				extra := (c & 0x7f) >> used
				var want byte
				if signed && (c>>(used-1))&1 == 1 {
					want = 0x7f >> used
				}
				if extra != want {
					return 0, 0, errLEBTooLong
				}
			}
		}
		if signed && c&0x40 != 0 && shift < 64 {
			result |= ^uint64(0) << shift
		}
		return result, i + 1, nil
	}
	if len(b) >= maxBytes {
		return 0, 0, errLEBTooLong
	}
	return 0, 0, errLEBTruncated
}

func readU32(b []byte) (uint32, int, error) {
	v, n, err := readLEB(b, 32, false)
	return uint32(v), n, err
}

func readU64(b []byte) (uint64, int, error) { return readLEB(b, 64, false) }

func readS32(b []byte) (int32, int, error) {
	v, n, err := readLEB(b, 32, true)
	return int32(v), n, err
}

func readS33(b []byte) (int64, int, error) {
	v, n, err := readLEB(b, 33, true)
	return int64(v), n, err
}

func readS64(b []byte) (int64, int, error) {
	v, n, err := readLEB(b, 64, true)
	return int64(v), n, err
}

// appendU appends v as the shortest unsigned LEB128.
func appendU(b []byte, v uint64) []byte {
	for v >= 0x80 {
		b = append(b, byte(v)|0x80)
		v >>= 7
	}
	return append(b, byte(v))
}

func appendU32(b []byte, v uint32) []byte { return appendU(b, uint64(v)) }

// appendS appends v as the shortest signed LEB128 (s32, s33 and s64
// values all fit in an int64).
func appendS(b []byte, v int64) []byte {
	for {
		c := byte(v) & 0x7f
		v >>= 7
		if (v == 0 && c&0x40 == 0) || (v == -1 && c&0x40 != 0) {
			return append(b, c)
		}
		b = append(b, c|0x80)
	}
}

// cursor reads from a byte slice. The first error sticks: after it every
// read returns zero values, so callers check err once at the end.
type cursor struct {
	b   []byte
	pos int
	err error
}

func (c *cursor) fail(format string, args ...any) {
	if c.err == nil {
		c.err = fmt.Errorf(format, args...)
	}
}

func (c *cursor) rest() int { return len(c.b) - c.pos }

func (c *cursor) lebErr(err error) {
	c.fail("at byte %d: %w", c.pos, err)
}

func (c *cursor) u32() uint32 {
	if c.err != nil {
		return 0
	}
	v, n, err := readU32(c.b[c.pos:])
	if err != nil {
		c.lebErr(err)
		return 0
	}
	c.pos += n
	return v
}

func (c *cursor) u64() uint64 {
	if c.err != nil {
		return 0
	}
	v, n, err := readU64(c.b[c.pos:])
	if err != nil {
		c.lebErr(err)
		return 0
	}
	c.pos += n
	return v
}

func (c *cursor) s32() int32 {
	if c.err != nil {
		return 0
	}
	v, n, err := readS32(c.b[c.pos:])
	if err != nil {
		c.lebErr(err)
		return 0
	}
	c.pos += n
	return v
}

func (c *cursor) s33() int64 {
	if c.err != nil {
		return 0
	}
	v, n, err := readS33(c.b[c.pos:])
	if err != nil {
		c.lebErr(err)
		return 0
	}
	c.pos += n
	return v
}

func (c *cursor) s64() int64 {
	if c.err != nil {
		return 0
	}
	v, n, err := readS64(c.b[c.pos:])
	if err != nil {
		c.lebErr(err)
		return 0
	}
	c.pos += n
	return v
}

func (c *cursor) byte() byte {
	if c.err != nil {
		return 0
	}
	if c.pos >= len(c.b) {
		c.fail("at byte %d: unexpected end", c.pos)
		return 0
	}
	v := c.b[c.pos]
	c.pos++
	return v
}

// take returns the next n bytes (aliasing the input).
func (c *cursor) take(n uint64) []byte {
	if c.err != nil {
		return nil
	}
	if n > uint64(c.rest()) {
		c.fail("at byte %d: %d bytes wanted, %d left", c.pos, n, c.rest())
		return nil
	}
	v := c.b[c.pos : c.pos+int(n)]
	c.pos += int(n)
	return v
}

// count reads a vector length and rejects one that cannot fit in the rest
// of the input (every element is at least one byte).
func (c *cursor) count() uint32 {
	n := c.u32()
	if c.err == nil && uint64(n) > uint64(c.rest()) {
		c.fail("at byte %d: vector of %d elements exceeds the %d bytes left", c.pos, n, c.rest())
		return 0
	}
	return n
}

// name reads a length-prefixed UTF-8 name.
func (c *cursor) name() string {
	n := c.u32()
	b := c.take(uint64(n))
	if c.err == nil && !utf8.Valid(b) {
		c.fail("name is not valid UTF-8")
	}
	return string(b)
}
