// Package deepeq is reflect.DeepEqual with the same answers, made cheap for
// the large read models tests compare tick after tick.
//
// reflect.DeepEqual walks every field of every element through reflect,
// which the race detector slows about tenfold. A Comparer compiles one
// comparison per type instead, and compares a value whose type holds no
// pointer, slice, map, interface, func or chan (a "plain" type: numbers,
// bools, strings, and arrays and structs of those) with Go's ==, which on
// such types is exactly DeepEqual's rule; slices of plain elements go in
// array chunks. Everything else is walked like DeepEqual does, including
// its nil-versus-empty and same-pointer rules and its cycle guard. Values
// reached through unexported fields cannot be boxed by reflect, so they
// are walked field by field.
package deepeq

import "reflect"

// chunkBytes is about how much of a plain slice one == compares; chunk
// lengths are powers of two, so the array types stay few.
const chunkBytes = 4096

// Equal reports whether a and b are deeply equal, as reflect.DeepEqual does.
// A loop comparing many values should keep one Comparer instead.
func Equal(a, b any) bool { return New().Equal(a, b) }

// Comparer compares values as reflect.DeepEqual does, remembering what it
// compiled for each type. It is not safe for concurrent use.
type Comparer struct {
	funcs map[reflect.Type]eqFunc
	plain map[reflect.Type]bool
	seen  map[visit]bool
}

type eqFunc func(c *Comparer, v1, v2 reflect.Value) bool

// New returns an empty Comparer.
func New() *Comparer {
	return &Comparer{funcs: map[reflect.Type]eqFunc{}, plain: map[reflect.Type]bool{}}
}

// Equal reports whether a and b are deeply equal, as reflect.DeepEqual does.
func (c *Comparer) Equal(a, b any) bool {
	if a == nil || b == nil {
		return a == b
	}
	v1, v2 := reflect.ValueOf(a), reflect.ValueOf(b)
	if v1.Type() != v2.Type() {
		return false
	}
	c.seen = map[visit]bool{}
	return c.funcOf(v1.Type())(c, v1, v2)
}

// visit is one pair of references already being compared (DeepEqual's
// cycle guard). A false anywhere ends the whole comparison, so a pair seen
// again is either still being compared (a cycle) or was equal.
type visit struct {
	a1, a2 uintptr
	n      int // slice length: one backing array holds many slices
	typ    reflect.Type
}

// seenBefore reports whether the non-nil reference pair v1, v2 is already
// being compared, and marks it.
func (c *Comparer) seenBefore(v1, v2 reflect.Value, n int) bool {
	a1, a2 := v1.Pointer(), v2.Pointer()
	if a1 > a2 {
		a1, a2 = a2, a1
	}
	k := visit{a1, a2, n, v1.Type()}
	if c.seen[k] {
		return true
	}
	c.seen[k] = true
	return false
}

// isPlain: t holds no reference of any kind, so == compares all of it as
// DeepEqual would.
func (c *Comparer) isPlain(t reflect.Type) bool {
	if p, ok := c.plain[t]; ok {
		return p
	}
	p := false
	switch t.Kind() {
	case reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr,
		reflect.Float32, reflect.Float64, reflect.Complex64, reflect.Complex128, reflect.String:
		p = true
	case reflect.Array:
		p = c.isPlain(t.Elem())
	case reflect.Struct:
		p = true
		for i := range t.NumField() {
			if !c.isPlain(t.Field(i).Type) {
				p = false
				break
			}
		}
	}
	c.plain[t] = p
	return p
}

// funcOf returns t's comparison, compiling it on first use. Both values
// handed to it have type t.
func (c *Comparer) funcOf(t reflect.Type) eqFunc {
	if f, ok := c.funcs[t]; ok {
		return f
	}
	var f eqFunc
	c.funcs[t] = func(c *Comparer, v1, v2 reflect.Value) bool { return f(c, v1, v2) } // a recursive type
	f = c.compile(t)
	c.funcs[t] = f
	return f
}

func (c *Comparer) compile(t reflect.Type) eqFunc {
	switch t.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return func(_ *Comparer, v1, v2 reflect.Value) bool { return v1.Int() == v2.Int() }
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return func(_ *Comparer, v1, v2 reflect.Value) bool { return v1.Uint() == v2.Uint() }
	case reflect.String:
		return func(_ *Comparer, v1, v2 reflect.Value) bool { return v1.String() == v2.String() }
	case reflect.Bool:
		return func(_ *Comparer, v1, v2 reflect.Value) bool { return v1.Bool() == v2.Bool() }
	case reflect.Float32, reflect.Float64:
		return func(_ *Comparer, v1, v2 reflect.Value) bool { return v1.Float() == v2.Float() }
	case reflect.Complex64, reflect.Complex128:
		return func(_ *Comparer, v1, v2 reflect.Value) bool { return v1.Complex() == v2.Complex() }
	case reflect.Chan, reflect.UnsafePointer:
		return func(_ *Comparer, v1, v2 reflect.Value) bool { return v1.Pointer() == v2.Pointer() }
	case reflect.Func:
		return func(_ *Comparer, v1, v2 reflect.Value) bool { return v1.IsNil() && v2.IsNil() }
	case reflect.Array:
		return c.compileArray(t)
	case reflect.Struct:
		return c.compileStruct(t)
	case reflect.Slice:
		return c.compileSlice(t)
	case reflect.Map:
		return c.compileMap(t)
	case reflect.Pointer:
		elem := c.funcOf(t.Elem())
		return func(c *Comparer, v1, v2 reflect.Value) bool {
			if v1.UnsafePointer() == v2.UnsafePointer() {
				return true
			}
			if v1.IsNil() || v2.IsNil() || c.seenBefore(v1, v2, 0) {
				return v1.IsNil() == v2.IsNil()
			}
			return elem(c, v1.Elem(), v2.Elem())
		}
	case reflect.Interface:
		return func(c *Comparer, v1, v2 reflect.Value) bool {
			if v1.IsNil() || v2.IsNil() {
				return v1.IsNil() == v2.IsNil()
			}
			e1, e2 := v1.Elem(), v2.Elem()
			if e1.Type() != e2.Type() {
				return false
			}
			return c.funcOf(e1.Type())(c, e1, e2)
		}
	}
	return func(_ *Comparer, v1, v2 reflect.Value) bool { // not reached: every kind is above
		return reflect.DeepEqual(v1.Interface(), v2.Interface())
	}
}

func (c *Comparer) compileArray(t reflect.Type) eqFunc {
	elem, n, plain := c.funcOf(t.Elem()), t.Len(), c.isPlain(t)
	return func(c *Comparer, v1, v2 reflect.Value) bool {
		if plain && v1.CanInterface() {
			return v1.Interface() == v2.Interface()
		}
		for i := range n {
			if !elem(c, v1.Index(i), v2.Index(i)) {
				return false
			}
		}
		return true
	}
}

func (c *Comparer) compileStruct(t reflect.Type) eqFunc {
	n := t.NumField()
	fields := make([]eqFunc, n)
	for i := range n {
		fields[i] = c.funcOf(t.Field(i).Type)
	}
	plain := c.isPlain(t)
	return func(c *Comparer, v1, v2 reflect.Value) bool {
		if plain && v1.CanInterface() {
			return v1.Interface() == v2.Interface()
		}
		for i, f := range fields {
			if !f(c, v1.Field(i), v2.Field(i)) {
				return false
			}
		}
		return true
	}
}

func (c *Comparer) compileMap(t reflect.Type) eqFunc {
	elem := c.funcOf(t.Elem())
	return func(c *Comparer, v1, v2 reflect.Value) bool {
		if v1.IsNil() != v2.IsNil() || v1.Len() != v2.Len() {
			return false
		}
		if v1.UnsafePointer() == v2.UnsafePointer() || c.seenBefore(v1, v2, 0) {
			return true
		}
		iter := v1.MapRange()
		for iter.Next() {
			e2 := v2.MapIndex(iter.Key())
			if !e2.IsValid() || !elem(c, iter.Value(), e2) {
				return false
			}
		}
		return true
	}
}

func (c *Comparer) compileSlice(t reflect.Type) eqFunc {
	elem := c.funcOf(t.Elem())
	var arrays []reflect.Type // arrays[k]: 1<<k elements
	if c.isPlain(t.Elem()) {
		size := max(int(t.Elem().Size()), 1)
		for n := 1; n <= max(chunkBytes/size, 1); n *= 2 {
			arrays = append(arrays, reflect.ArrayOf(n, t.Elem()))
		}
	}
	return func(c *Comparer, v1, v2 reflect.Value) bool {
		if v1.IsNil() != v2.IsNil() || v1.Len() != v2.Len() {
			return false
		}
		n := v1.Len()
		if v1.UnsafePointer() == v2.UnsafePointer() || n == 0 || c.seenBefore(v1, v2, n) {
			return true
		}
		if arrays == nil || !v1.CanInterface() {
			for i := range n {
				if !elem(c, v1.Index(i), v2.Index(i)) {
					return false
				}
			}
			return true
		}
		for i, k := 0, len(arrays)-1; i < n; {
			for 1<<k > n-i {
				k--
			}
			j := i + 1<<k
			if v1.Slice(i, j).Convert(arrays[k]).Interface() != v2.Slice(i, j).Convert(arrays[k]).Interface() {
				return false
			}
			i = j
		}
		return true
	}
}
