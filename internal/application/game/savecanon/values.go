// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package savecanon

import (
	"bytes"
	"fmt"
	"slices"
)

// values rewrites gob values with their map entries in ascending order of the
// encoded key; the bytes of everything else are kept.
type values struct {
	defs map[int]*typeDef
}

// message rewrites one value message body (after its type id).
func (v *values) message(r *reader, id int, out []byte) ([]byte, error) {
	d := v.defs[id]
	if d != nil && d.kind == kStruct {
		return v.value(r, id, out)
	}
	zero, err := r.uint() // a non-struct top-level value follows a zero field delta
	if err != nil || zero != 0 {
		return nil, fmt.Errorf("savecanon: singleton value without its zero delta: %v", err)
	}
	return v.value(r, id, append(out, 0))
}

func (v *values) value(r *reader, id int, out []byte) ([]byte, error) {
	if id < firstUser {
		return v.basic(r, id, out)
	}
	d := v.defs[id]
	if d == nil {
		return nil, fmt.Errorf("savecanon: value of undefined type %d", id)
	}
	switch d.kind {
	case kStruct:
		return v.structure(r, d, out)
	case kArray, kSlice:
		return v.sequence(r, d, out)
	case kMap:
		return v.mapping(r, d, out)
	}
	return v.bytes(r, out)
}

func (v *values) basic(r *reader, id int, out []byte) ([]byte, error) {
	switch id {
	case idBool, idInt, idUint, idFloat:
		return v.token(r, out)
	case idComplex:
		out, err := v.token(r, out)
		if err != nil {
			return nil, err
		}
		return v.token(r, out)
	case idBytes, idString:
		return v.bytes(r, out)
	}
	return nil, fmt.Errorf("savecanon: unsupported predefined type %d (interface values are not canonicalised)", id)
}

// token copies one uint-shaped value (ints, floats and bools all are).
func (v *values) token(r *reader, out []byte) ([]byte, error) {
	m := r.mark()
	if _, err := r.uint(); err != nil {
		return nil, err
	}
	return append(out, r.since(m)...), nil
}

// bytes copies a length-prefixed byte string.
func (v *values) bytes(r *reader, out []byte) ([]byte, error) {
	n, err := r.uint()
	if err != nil {
		return nil, err
	}
	raw, err := r.take(n)
	if err != nil {
		return nil, err
	}
	return append(appendUint(out, n), raw...), nil
}

func (v *values) structure(r *reader, d *typeDef, out []byte) ([]byte, error) {
	num := -1
	for {
		delta, err := r.uint()
		if err != nil {
			return nil, err
		}
		out = appendUint(out, delta)
		if delta == 0 {
			return out, nil
		}
		if delta > uint64(len(d.fields)-num-1) {
			return nil, fmt.Errorf("savecanon: field delta %d of %s out of range", delta, d.name)
		}
		num += int(delta)
		if out, err = v.value(r, d.fields[num].id, out); err != nil {
			return nil, err
		}
	}
}

func (v *values) sequence(r *reader, d *typeDef, out []byte) ([]byte, error) {
	n, err := r.uint()
	if err != nil {
		return nil, err
	}
	out = appendUint(out, n)
	switch d.elem {
	case idBool, idInt, idUint, idFloat:
		return v.tokens(r, n, out)
	}
	for ; n > 0; n-- {
		if out, err = v.value(r, d.elem, out); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// entry stores offsets into one shared map arena rather than allocating
// a separate buffer for each key and value. Offsets survive arena growth.
type entry struct{ start, keyEnd, end int }

func (v *values) mapping(r *reader, d *typeDef, out []byte) ([]byte, error) {
	n, err := r.uint()
	if err != nil {
		return nil, err
	}
	out = appendUint(out, n)
	// Bound speculative allocation by the bytes actually available. Empty
	// maps need no temporary storage.
	if n == 0 {
		return out, nil
	}
	entries := make([]entry, 0, min(n, 1<<16))
	arena := make([]byte, 0, min(len(r.b)-r.i, int(min(n, 1<<16))*16))
	for ; n > 0; n-- {
		start := len(arena)
		arena, err = v.value(r, d.key, arena)
		if err != nil {
			return nil, err
		}
		keyEnd := len(arena)
		arena, err = v.value(r, d.elem, arena)
		if err != nil {
			return nil, err
		}
		entries = append(entries, entry{start, keyEnd, len(arena)})
	}
	slices.SortFunc(entries, func(a, b entry) int { return bytes.Compare(arena[a.start:a.keyEnd], arena[b.start:b.keyEnd]) })
	for _, e := range entries {
		out = append(out, arena[e.start:e.end]...)
	}
	return out, nil
}

// tokens copies a scalar sequence in one contiguous append. Each wire
// uint prefix still gets the same range validation as reader.uint.
func (v *values) tokens(r *reader, n uint64, out []byte) ([]byte, error) {
	start := r.i
	if n > uint64(len(r.b)-start) {
		return nil, errShort
	}
	for ; n > 0; n-- {
		if r.i >= len(r.b) {
			return nil, errShort
		}
		first := r.b[r.i]
		r.i++
		if first >= 0x80 {
			width := 256 - int(first)
			if width > 8 || width > len(r.b)-r.i {
				return nil, errShort
			}
			r.i += width
		}
	}
	return append(out, r.b[start:r.i]...), nil
}
