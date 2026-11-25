// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package savecanon

import "fmt"

// Type ids below firstUser are gob's predefined types (bool, int, string, the
// wire types themselves); a stream defines every other id it uses.
const (
	firstUser = 64

	idBool, idInt, idUint, idFloat, idBytes, idString, idComplex, idInterface = 1, 2, 3, 4, 5, 6, 7, 8
)

type kind int

const (
	kArray kind = iota
	kSlice
	kStruct
	kMap
	kOpaque // a GobEncoder, BinaryMarshaler or TextMarshaler: one byte string
)

type field struct {
	name string
	id   int
}

// typeDef is one wireType of the stream.
type typeDef struct {
	kind   kind
	name   string
	id     int
	elem   int
	key    int
	length int64
	fields []field
	wire   int // the wireType field that held it
}

// refs lists the ids a definition mentions, in a fixed order.
func (d *typeDef) refs() []int {
	switch d.kind {
	case kArray, kSlice:
		return []int{d.elem}
	case kMap:
		return []int{d.key, d.elem}
	case kStruct:
		out := make([]int, len(d.fields))
		for i, f := range d.fields {
			out[i] = f.id
		}
		return out
	}
	return nil
}

// parseTypeDef reads the wireType struct that follows a negative type id.
func parseTypeDef(r *reader) (*typeDef, error) {
	delta, err := r.uint()
	if err != nil || delta == 0 {
		return nil, fmt.Errorf("savecanon: empty wire type: %v", err)
	}
	var d *typeDef
	switch delta - 1 { // wireType: ArrayT, SliceT, StructT, MapT, GobEncoderT, BinaryMarshalerT, TextMarshalerT
	case 0:
		d, err = parseSeq(r, kArray)
	case 1:
		d, err = parseSeq(r, kSlice)
	case 2:
		d, err = parseStruct(r)
	case 3:
		d, err = parseMap(r)
	case 4, 5, 6:
		d, err = parseOpaque(r)
	default:
		return nil, fmt.Errorf("savecanon: unknown wire type field %d", delta-1)
	}
	if err != nil {
		return nil, err
	}
	if end, err := r.uint(); err != nil || end != 0 {
		return nil, fmt.Errorf("savecanon: wire type not closed: %v", err)
	}
	d.wire = int(delta - 1)
	return d, nil
}

// parseCommon reads CommonType{Name string; Id typeId}.
func parseCommon(r *reader, d *typeDef) error {
	last := -1
	for {
		delta, err := r.uint()
		if err != nil {
			return err
		}
		if delta == 0 {
			return nil
		}
		last += int(delta)
		switch last {
		case 0:
			n, err := r.uint()
			if err != nil {
				return err
			}
			raw, err := r.take(n)
			if err != nil {
				return err
			}
			d.name = string(raw)
		case 1:
			id, err := r.int()
			if err != nil {
				return err
			}
			d.id = int(id)
		default:
			return fmt.Errorf("savecanon: bad CommonType field %d", last)
		}
	}
}

// parseBody reads the struct fields of a wire type (field 0 is CommonType);
// each other field index goes to set.
func parseBody(r *reader, d *typeDef, set func(idx int) error) error {
	last := -1
	for {
		delta, err := r.uint()
		if err != nil {
			return err
		}
		if delta == 0 {
			return nil
		}
		last += int(delta)
		if last == 0 {
			if err := parseCommon(r, d); err != nil {
				return err
			}
			continue
		}
		if err := set(last); err != nil {
			return err
		}
	}
}

func parseOpaque(r *reader) (*typeDef, error) {
	d := &typeDef{kind: kOpaque}
	err := parseBody(r, d, func(int) error { return fmt.Errorf("savecanon: bad opaque wire type") })
	return d, err
}

func parseSeq(r *reader, k kind) (*typeDef, error) {
	d := &typeDef{kind: k}
	err := parseBody(r, d, func(idx int) error {
		v, err := r.int()
		switch idx {
		case 1:
			d.elem = int(v)
		case 2:
			d.length = v
		default:
			return fmt.Errorf("savecanon: bad sequence field %d", idx)
		}
		return err
	})
	return d, err
}

func parseMap(r *reader) (*typeDef, error) {
	d := &typeDef{kind: kMap}
	err := parseBody(r, d, func(idx int) error {
		v, err := r.int()
		switch idx {
		case 1:
			d.key = int(v)
		case 2:
			d.elem = int(v)
		default:
			return fmt.Errorf("savecanon: bad map field %d", idx)
		}
		return err
	})
	return d, err
}

func parseStruct(r *reader) (*typeDef, error) {
	d := &typeDef{kind: kStruct}
	err := parseBody(r, d, func(idx int) error {
		if idx != 1 {
			return fmt.Errorf("savecanon: bad struct field %d", idx)
		}
		n, err := r.uint()
		for ; err == nil && n > 0; n-- {
			var f field
			if f, err = parseFieldType(r); err != nil {
				break
			}
			d.fields = append(d.fields, f)
		}
		return err
	})
	return d, err
}

// parseFieldType reads fieldType{Name string; Id typeId}.
func parseFieldType(r *reader) (field, error) {
	var f field
	last := -1
	for {
		delta, err := r.uint()
		if err != nil {
			return f, err
		}
		if delta == 0 {
			return f, nil
		}
		last += int(delta)
		switch last {
		case 0:
			n, err := r.uint()
			if err != nil {
				return f, err
			}
			raw, err := r.take(n)
			if err != nil {
				return f, err
			}
			f.name = string(raw)
		case 1:
			id, err := r.int()
			if err != nil {
				return f, err
			}
			f.id = int(id)
		default:
			return f, fmt.Errorf("savecanon: bad fieldType field %d", last)
		}
	}
}

// structW writes a gob struct field by field, omitting nothing itself.
type structW struct {
	b    []byte
	last int
}

func newStructW() *structW { return &structW{last: -1} }

func (w *structW) at(idx int) {
	w.b = appendUint(w.b, uint64(idx-w.last))
	w.last = idx
}

func (w *structW) end() []byte { return appendUint(w.b, 0) }

func commonBytes(name string, id int) []byte {
	w := newStructW()
	if name != "" {
		w.at(0)
		w.b = appendUint(w.b, uint64(len(name)))
		w.b = append(w.b, name...)
	}
	w.at(1)
	w.b = appendInt(w.b, int64(id))
	return w.end()
}

// encode is the definition message body for the definition under id,
// ids rewritten through remap.
func (d *typeDef) encode(id int, remap func(int) int) []byte {
	out := appendInt(nil, -int64(id))
	body := newStructW()
	body.at(0)
	body.b = append(body.b, commonBytes(d.name, id)...)
	switch d.kind {
	case kArray, kSlice:
		body.at(1)
		body.b = appendInt(body.b, int64(remap(d.elem)))
		if d.kind == kArray && d.length != 0 {
			body.at(2)
			body.b = appendInt(body.b, d.length)
		}
	case kMap:
		body.at(1)
		body.b = appendInt(body.b, int64(remap(d.key)))
		body.at(2)
		body.b = appendInt(body.b, int64(remap(d.elem)))
	case kStruct:
		if len(d.fields) > 0 {
			body.at(1)
			body.b = appendUint(body.b, uint64(len(d.fields)))
			for _, f := range d.fields {
				fw := newStructW()
				fw.at(0)
				fw.b = appendUint(fw.b, uint64(len(f.name)))
				fw.b = append(fw.b, f.name...)
				fw.at(1)
				fw.b = appendInt(fw.b, int64(remap(f.id)))
				body.b = append(body.b, fw.end()...)
			}
		}
	}
	wt := newStructW()
	wt.at(d.wire)
	wt.b = append(wt.b, body.end()...)
	return append(out, wt.end()...)
}
