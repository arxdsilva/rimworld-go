package statehash

import (
	"bytes"
	"encoding/binary"
	"hash/fnv"
	"math"
	"reflect"
	"slices"
)

// Bytes is the FNV-64a hash of b.
func Bytes(b []byte) uint64 {
	h := fnv.New64a()
	_, _ = h.Write(b)
	return h.Sum64()
}

// Of is the FNV-64a hash of a canonical encoding of v: every exported
// struct field, slice element and map entry (maps in ascending order of
// their encoded keys), with lengths and nil-ness included so different
// shapes never collide by concatenation. Unexported fields, funcs and
// channels are skipped (they are not state a save carries).
func Of(v any) uint64 {
	var e encoder
	e.value(reflect.ValueOf(v))
	return Bytes(e.buf)
}

type encoder struct{ buf []byte }

func (e *encoder) u64(x uint64) { e.buf = binary.LittleEndian.AppendUint64(e.buf, x) }

func (e *encoder) tag(b byte) { e.buf = append(e.buf, b) }

func (e *encoder) value(v reflect.Value) {
	if !v.IsValid() {
		e.tag(0)
		return
	}
	switch v.Kind() {
	case reflect.Bool:
		if v.Bool() {
			e.tag(1)
		} else {
			e.tag(0)
		}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		e.u64(uint64(v.Int()))
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		e.u64(v.Uint())
	case reflect.Float32, reflect.Float64:
		e.u64(math.Float64bits(v.Float()))
	case reflect.Complex64, reflect.Complex128:
		c := v.Complex()
		e.u64(math.Float64bits(real(c)))
		e.u64(math.Float64bits(imag(c)))
	case reflect.String:
		e.u64(uint64(v.Len()))
		e.buf = append(e.buf, v.String()...)
	case reflect.Array:
		for i := 0; i < v.Len(); i++ {
			e.value(v.Index(i))
		}
	case reflect.Slice:
		if v.IsNil() {
			e.tag(0)
			return
		}
		e.tag(1)
		e.u64(uint64(v.Len()))
		if v.Type().Elem().Kind() == reflect.Uint8 {
			e.buf = append(e.buf, v.Bytes()...)
			return
		}
		for i := 0; i < v.Len(); i++ {
			e.value(v.Index(i))
		}
	case reflect.Map:
		e.mapValue(v)
	case reflect.Pointer, reflect.Interface:
		if v.IsNil() {
			e.tag(0)
			return
		}
		e.tag(1)
		e.value(v.Elem())
	case reflect.Struct:
		t := v.Type()
		for i := 0; i < v.NumField(); i++ {
			if t.Field(i).IsExported() {
				e.value(v.Field(i))
			}
		}
	default: // func, chan, unsafe pointer: not state
	}
}

func (e *encoder) mapValue(v reflect.Value) {
	if v.IsNil() {
		e.tag(0)
		return
	}
	e.tag(1)
	e.u64(uint64(v.Len()))
	type kv struct{ k, v []byte }
	entries := make([]kv, 0, v.Len())
	it := v.MapRange()
	for it.Next() {
		var ke, ve encoder
		ke.value(it.Key())
		ve.value(it.Value())
		entries = append(entries, kv{ke.buf, ve.buf})
	}
	slices.SortFunc(entries, func(a, b kv) int { return bytes.Compare(a.k, b.k) })
	for _, en := range entries {
		e.buf = append(e.buf, en.k...)
		e.buf = append(e.buf, en.v...)
	}
}

// OfFields hashes named values in ascending name order, skipping zero
// values (nil or empty slices and maps included): a field added at its
// zero value, or renamed, leaves the hash alone, so golden pins follow
// behaviour rather than a save format.
func OfFields(fields map[string]any) uint64 {
	names := make([]string, 0, len(fields))
	for n, v := range fields {
		if !isZero(reflect.ValueOf(v)) {
			names = append(names, n)
		}
	}
	slices.Sort(names)
	var e encoder
	for _, n := range names {
		e.value(reflect.ValueOf(n))
		e.value(reflect.ValueOf(fields[n]))
	}
	return Bytes(e.buf)
}

func isZero(v reflect.Value) bool {
	if !v.IsValid() {
		return true
	}
	switch v.Kind() {
	case reflect.Slice, reflect.Map:
		return v.Len() == 0
	}
	return v.IsZero()
}
