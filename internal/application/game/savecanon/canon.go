// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

// Package savecanon makes a gob stream canonical: one value, one byte string.
//
// encoding/gob ranges every map in Go's random order and numbers the types it
// defines in the order the process first met them, so the same state encodes
// to different bytes from run to run (and from process to process). Both are
// free choices of the encoder, and any decoder reads either, so Canonical
// rewrites a finished stream without changing what it decodes to: map entries
// in ascending order of their encoded key bytes, user types numbered in order
// of first use from each value's type, definitions sent just before the first
// value that needs them. Interface values (gob.Register) are not supported and
// make Canonical fail; the game save has none.
package savecanon

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"io"
	"sort"
)

// Captured owns a locally encoded Gob stream, with no references to the
// original values. It may cross goroutines, but its buffers must not be reused
// while another goroutine is canonicalizing it.
type Captured struct {
	raw []byte
}

// Capture serializes values before the caller resumes mutating them. reuse
// must be exclusively owned; on success its ownership passes to the capture.
func Capture(reuse []byte, values ...any) (Captured, error) {
	raw := bytes.NewBuffer(reuse[:0])
	enc := gob.NewEncoder(raw)
	for _, v := range values {
		if err := enc.Encode(v); err != nil {
			return Captured{}, fmt.Errorf("save gob: %w", err)
		}
	}
	return Captured{raw: raw.Bytes()}, nil
}

func (c Captured) Size() int { return len(c.raw) }

// Recycle returns the input buffer. Only call after all uses of c are over.
func (c Captured) Recycle() []byte { return c.raw[:0] }

// CanonicalBytes canonicalizes the captured stream into an independently
// owned output buffer. reuse must not alias c's input buffer.
func (c Captured) CanonicalBytes(reuse []byte) ([]byte, error) {
	return canonicalizeInto(c.raw, reuse[:0])
}

// Encode gob-encodes values in order, as one gob.Encoder would write them to
// a stream, and writes the canonical form of that stream to w.
func Encode(w io.Writer, values ...any) error {
	c, err := Capture(nil, values...)
	if err != nil {
		return err
	}
	var reuse []byte
	if b, ok := w.(*bytes.Buffer); ok {
		b.Grow(c.Size())
		reuse = b.AvailableBuffer()
	}
	canonical, err := c.CanonicalBytes(reuse)
	if err != nil {
		return err
	}
	n, err := w.Write(canonical)
	if err == nil && n != len(canonical) {
		return io.ErrShortWrite
	}
	return err
}

// canonicalize rewrites a locally encoded gob stream.
func canonicalize(raw []byte) ([]byte, error) {
	return canonicalizeInto(raw, nil)
}

func canonicalizeInto(raw, out []byte) ([]byte, error) {
	var c canon
	r := &reader{b: raw}
	if cap(out) < len(raw) {
		out = make([]byte, 0, len(raw))
	}
	for !r.done() {
		n, err := r.uint()
		if err != nil {
			return nil, err
		}
		body, err := r.take(n)
		if err != nil {
			return nil, err
		}
		if out, err = c.message(&reader{b: body}, out); err != nil {
			return nil, err
		}
	}
	return out, nil
}

type canon struct {
	defs    map[int]*typeDef
	renamed map[int]int // stream id -> canonical id
	sent    map[int]bool
	next    int
}

func (c *canon) message(r *reader, out []byte) ([]byte, error) {
	id, err := r.int()
	if err != nil {
		return nil, err
	}
	if id < 0 {
		return out, c.define(r, int(-id))
	}
	if c.next == 0 {
		c.next = firstUser
	}
	if err := c.number(int(id)); err != nil {
		return nil, err
	}
	out = c.sendDefs(out)
	// Local Gob emits minimal wire lengths. Sorting map entries keeps
	// the value body length; only the type ID's prefix may change.
	tag := appendInt(nil, int64(c.rename(int(id))))
	bodyLen := len(r.b) - r.i
	out = appendUint(out, uint64(len(tag)+bodyLen))
	out = append(out, tag...)
	end := len(out) + bodyLen
	v := values{defs: c.defs}
	out, err = v.message(r, int(id), out)
	if err != nil {
		return nil, err
	}
	if !r.done() || len(out) != end {
		return nil, fmt.Errorf("savecanon: invalid value message length")
	}
	return out, nil
}

func (c *canon) define(r *reader, id int) error {
	d, err := parseTypeDef(r)
	if err != nil {
		return err
	}
	if !r.done() {
		return fmt.Errorf("savecanon: trailing bytes in the definition of type %d", id)
	}
	if c.defs == nil {
		c.defs = map[int]*typeDef{}
	}
	d.id = id
	c.defs[id] = d
	return nil
}

// number gives the user types reachable from id their canonical ids, depth
// first in field order.
func (c *canon) number(id int) error {
	if id < firstUser {
		return nil
	}
	if _, ok := c.renamed[id]; ok {
		return nil
	}
	d := c.defs[id]
	if d == nil {
		return fmt.Errorf("savecanon: type %d is used before its definition", id)
	}
	if c.renamed == nil {
		c.renamed = map[int]int{}
	}
	c.renamed[id] = c.next
	c.next++
	for _, ref := range d.refs() {
		if err := c.number(ref); err != nil {
			return err
		}
	}
	return nil
}

func (c *canon) rename(id int) int {
	if id < firstUser {
		return id
	}
	return c.renamed[id]
}

// sendDefs appends the definitions numbered but not yet sent, in id order.
func (c *canon) sendDefs(out []byte) []byte {
	var todo []int
	for old := range c.renamed {
		if !c.sent[old] {
			todo = append(todo, old)
		}
	}
	sort.Slice(todo, func(i, j int) bool { return c.renamed[todo[i]] < c.renamed[todo[j]] })
	if c.sent == nil {
		c.sent = map[int]bool{}
	}
	for _, old := range todo {
		c.sent[old] = true
		out = frame(out, c.defs[old].encode(c.renamed[old], c.rename))
	}
	return out
}

// frame appends one message: its length, then the parts.
func frame(out []byte, parts ...[]byte) []byte {
	n := 0
	for _, p := range parts {
		n += len(p)
	}
	out = appendUint(out, uint64(n))
	for _, p := range parts {
		out = append(out, p...)
	}
	return out
}
