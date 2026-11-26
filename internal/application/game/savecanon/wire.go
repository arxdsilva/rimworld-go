// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package savecanon

import (
	"errors"
	"math/bits"
)

var errShort = errors.New("savecanon: truncated gob stream")

// reader walks gob wire primitives (encoding/gob doc, "Encoding Details").
type reader struct {
	b []byte
	i int
}

func (r *reader) done() bool { return r.i >= len(r.b) }

func (r *reader) take(n uint64) ([]byte, error) {
	if n > uint64(len(r.b)-r.i) {
		return nil, errShort
	}
	out := r.b[r.i : r.i+int(n)]
	r.i += int(n)
	return out, nil
}

func (r *reader) uint() (uint64, error) {
	if r.i >= len(r.b) {
		return 0, errShort
	}
	first := r.b[r.i]
	r.i++
	if first < 0x80 {
		return uint64(first), nil
	}
	n := 256 - uint64(first)
	if n > 8 {
		return 0, errors.New("savecanon: bad gob uint")
	}
	raw, err := r.take(n)
	if err != nil {
		return 0, err
	}
	var v uint64
	for _, c := range raw {
		v = v<<8 | uint64(c)
	}
	return v, nil
}

func (r *reader) int() (int64, error) {
	u, err := r.uint()
	if err != nil {
		return 0, err
	}
	if u&1 != 0 {
		return ^int64(u >> 1), nil
	}
	return int64(u >> 1), nil
}

// mark/slice return the bytes consumed since a mark, for verbatim copies.
func (r *reader) mark() int          { return r.i }
func (r *reader) since(m int) []byte { return r.b[m:r.i] }

func appendUint(dst []byte, x uint64) []byte {
	if x < 0x80 {
		return append(dst, byte(x))
	}
	n := 8 - bits.LeadingZeros64(x)/8
	dst = append(dst, byte(256-n))
	for s := (n - 1) * 8; s >= 0; s -= 8 {
		dst = append(dst, byte(x>>uint(s)))
	}
	return dst
}

func appendInt(dst []byte, i int64) []byte {
	if i < 0 {
		return appendUint(dst, uint64(^i<<1)|1)
	}
	return appendUint(dst, uint64(i<<1))
}
