// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package savefmt

import (
	"reflect"
	"slices"

	"rimworld-go/internal/domain/building/zone"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/command"
)

// ItemKinds maps saved item kinds onto the running content (PLG-026): the
// kind of the same key now, or item.None for one the run lacks (its
// plugin is no longer loaded). Core kinds keep their places, so only
// plugin kinds ever move; kinds past the saved keys (item.None, the
// "no material" sentinel) stay as they are.
type ItemKinds struct {
	to []item.Kind // nil: every kind stays
}

// Items is the item map onto book. A save without item keys (before
// PLG-026: core only), or whose every key keeps its kind (core's, or the
// same plugins' with more appended), keeps its kinds.
func (k Keys) Items(book item.Book) ItemKinds {
	saved, ok := k[item.Class]
	if !ok {
		return ItemKinds{}
	}
	to := make([]item.Kind, len(saved))
	same := true // kinds a run appends never move the saved ones
	for i, key := range saved {
		to[i] = item.None
		if j, ok := book.KindOf(key); ok {
			to[i] = j
		}
		same = same && to[i] == item.Kind(i)
	}
	if same {
		return ItemKinds{}
	}
	return ItemKinds{to: to}
}

// Same reports nothing moves.
func (m ItemKinds) Same() bool { return m.to == nil }

// Kind is saved kind k now (item.None: gone).
func (m ItemKinds) Kind(k item.Kind) item.Kind {
	if int(k) < len(m.to) {
		return m.to[k]
	}
	return k
}

// gone reports saved kind k lost its key.
func (m ItemKinds) gone(k item.Kind) bool { return k != item.None && m.Kind(k) == item.None }

// Walk remaps every item kind reachable from v, a pointer to a save's
// state, in place: each item.Kind (fields, array, slice and map
// elements, map keys), each zone.Filter's bits. A gone kind's stack is
// removed (a slice or map element that is an item.Stack, or holds one as
// a field, of a gone kind), as are a gone kind's slice elements and map
// entries; a lone field naming a gone kind becomes item.None, which every
// Kind method answers neutrally. A slice of kinds sorted before stays
// sorted. Kinds stored as plain integers are the caller's.
func (m ItemKinds) Walk(v any) {
	if m.Same() {
		return
	}
	w := walker{m: m, seen: map[reflect.Type]bool{}}
	w.walk(reflect.ValueOf(v).Elem())
}

// walker is one Walk: the map and which types can hold a kind.
type walker struct {
	m    ItemKinds
	seen map[reflect.Type]bool // type → may hold an item kind (memo)
}

var (
	kindType   = reflect.TypeFor[item.Kind]()
	stackType  = reflect.TypeFor[item.Stack]()
	filterType = reflect.TypeFor[zone.Filter]()
)

// holds reports values of type t may hold an item kind.
func (w *walker) holds(t reflect.Type) bool {
	if v, ok := w.seen[t]; ok {
		return v
	}
	w.seen[t] = false // a recursive type holds one only through its other parts
	out := false
	switch t.Kind() {
	case reflect.Uint8:
		out = t == kindType
	case reflect.Array:
		out = t == filterType || w.holds(t.Elem())
	case reflect.Slice, reflect.Pointer:
		out = w.holds(t.Elem())
	case reflect.Map:
		out = w.holds(t.Key()) || w.holds(t.Elem())
	case reflect.Struct:
		for i := range t.NumField() {
			if f := t.Field(i); f.IsExported() && w.holds(f.Type) {
				out = true
			}
		}
	}
	w.seen[t] = out
	return out
}

// walk remaps v in place and reports v is (or directly holds) a stack of
// a gone kind, or is a gone kind, so its slice or map drops it.
func (w *walker) walk(v reflect.Value) (drop bool) {
	t := v.Type()
	if !w.holds(t) {
		return false
	}
	switch {
	case t == kindType:
		k := item.Kind(v.Uint())
		v.SetUint(uint64(w.m.Kind(k)))
		return w.m.gone(k)
	case t == filterType:
		v.Set(reflect.ValueOf(w.filter(v.Interface().(zone.Filter))))
		return false
	}
	switch t.Kind() {
	case reflect.Pointer:
		if !v.IsNil() {
			return w.walk(v.Elem())
		}
	case reflect.Struct:
		for i := range t.NumField() {
			f := t.Field(i)
			if !f.IsExported() {
				continue
			}
			gone := w.walk(v.Field(i))
			if gone && (t == stackType && f.Name == "Kind" || f.Type == stackType) {
				drop = true
			}
		}
	case reflect.Array:
		for i := range v.Len() {
			w.walk(v.Index(i))
		}
	case reflect.Slice:
		w.slice(v)
	case reflect.Map:
		w.mapOf(v)
	}
	return drop
}

// slice remaps a slice, dropping the elements walk drops.
func (w *walker) slice(v reflect.Value) {
	sorted := v.Type().Elem() == kindType && slices.IsSorted(v.Interface().([]item.Kind))
	n := 0
	for i := range v.Len() {
		if w.walk(v.Index(i)) {
			continue
		}
		if n != i {
			v.Index(n).Set(v.Index(i))
		}
		n++
	}
	if n < v.Len() {
		v.Set(v.Slice(0, n))
	}
	if sorted {
		slices.Sort(v.Interface().([]item.Kind))
	}
}

// mapOf rebuilds a map with its keys and values remapped, without the
// entries walk drops (surviving keys stay distinct: the map is one to
// one).
func (w *walker) mapOf(v reflect.Value) {
	if v.IsNil() {
		return
	}
	t := v.Type()
	out := reflect.MakeMapWithSize(t, v.Len())
	it := v.MapRange()
	for it.Next() {
		k, e := reflect.New(t.Key()).Elem(), reflect.New(t.Elem()).Elem()
		k.Set(it.Key())
		e.Set(it.Value())
		if w.walk(k) || w.walk(e) {
			continue
		}
		out.SetMapIndex(k, e)
	}
	v.Set(out)
}

// filter moves f's plugin bits to their kinds now, clearing gone ones.
func (w *walker) filter(f zone.Filter) zone.Filter {
	out := f
	for k := item.CoreKinds; k < int(item.None); k++ {
		out = out.With(item.Kind(k), false)
	}
	for k := item.CoreKinds; k < int(item.None); k++ {
		if f.Has(item.Kind(k)) {
			if to := w.m.Kind(item.Kind(k)); to != item.None {
				out = out.With(to, true)
			}
		}
	}
	return out
}

// Commands remaps the commands that carry an item kind as a plain byte
// (SetBillIngredient's Stuff), in place, dropping one whose kind is gone
// (cmds itself when none is). Stuff elsewhere is a building material,
// never a plugin kind; filter commands name kinds by key.
func (m ItemKinds) Commands(cmds []command.Command) []command.Command {
	if m.Same() {
		return cmds
	}
	out := cmds[:0:0]
	for _, c := range cmds {
		if c.Kind == command.SetBillIngredient {
			k := item.Kind(c.Stuff)
			if m.gone(k) {
				continue
			}
			c.Stuff = uint8(m.Kind(k))
		}
		out = append(out, c)
	}
	return out
}
