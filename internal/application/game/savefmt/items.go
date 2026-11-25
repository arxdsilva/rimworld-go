// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package savefmt

import (
	"reflect"
	"slices"

	"rimworld-go/internal/application/game/visit"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/building/zone"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/trade"
	"rimworld-go/internal/domain/sim/command"
)

// ItemKinds maps saved item kinds onto the running content (PLG-026): the
// kind of the same key now, or item.None for one the run lacks (its
// plugin is no longer loaded). Core kinds keep their places, but adding core kinds can overlap old
// plugin slots; kinds past the saved keys (item.None, the
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
// sorted. Stock and carrier ledgers retain their Goods ordering; saved
// item commands retain their key identity or are removed as a whole.
func (m ItemKinds) Walk(v any) {
	if m.Same() {
		return
	}
	w := walker{m: m, seen: map[reflect.Type]bool{}}
	w.walk(reflect.ValueOf(v).Elem())
}

// MigrateConstructionStacks gives all old physical crates their original
// construction basis, including nested packs, markets, holds and flights.
// Reuse the item remap traversal; only historical load invokes this.
func MigrateConstructionStacks(v any, version int) {
	if version >= 21 {
		return
	}
	w := walker{seen: map[reflect.Type]bool{}, constructionVersion: version}
	w.walk(reflect.ValueOf(v).Elem())
}

// walker is one Walk: the map and which types can hold a kind.
type walker struct {
	constructionVersion int
	m                   ItemKinds
	seen                map[reflect.Type]bool // type → may hold an item kind (memo)
}

var (
	kindType    = reflect.TypeFor[item.Kind]()
	stackType   = reflect.TypeFor[item.Stack]()
	filterType  = reflect.TypeFor[zone.Filter]()
	lineType    = reflect.TypeFor[trade.Line]()
	stockType   = reflect.TypeFor[trade.Stock]()
	cargoType   = reflect.TypeFor[visit.Cargo]()
	commandType = reflect.TypeFor[command.Command]()
)

// holds reports whether t can contain a kind or a command's encoded
// reference. Cache complete reachability results, never an in-progress
// false result: saveState recursively contains extra map saveStates.
func (w *walker) holds(t reflect.Type) bool {
	if v, ok := w.seen[t]; ok {
		return v
	}
	out := w.reaches(t, make(map[reflect.Type]bool))
	w.seen[t] = out
	return out
}

func (w *walker) reaches(t reflect.Type, visited map[reflect.Type]bool) bool {
	if v, ok := w.seen[t]; ok {
		return v
	}
	if visited[t] {
		return false
	}
	visited[t] = true
	if t == kindType || t == filterType || t == commandType {
		return true
	}
	switch t.Kind() {
	case reflect.Array, reflect.Slice, reflect.Pointer:
		return w.reaches(t.Elem(), visited)
	case reflect.Map:
		return w.reaches(t.Key(), visited) || w.reaches(t.Elem(), visited)
	case reflect.Struct:
		for i := range t.NumField() {
			if f := t.Field(i); f.IsExported() && w.reaches(f.Type, visited) {
				return true
			}
		}
	}
	return false
}

// walk remaps v in place and reports v is (or directly holds) a stack of
// a gone kind, or is a gone kind, so its slice or map drops it.
func (w *walker) walk(v reflect.Value) (drop bool) {
	t := v.Type()
	if !w.holds(t) {
		return false
	}
	switch {
	case t == stackType && w.constructionVersion > 0:
		st := v.Interface().(item.Stack)
		if st.Kind == item.MinifiedBuilding && st.MaterialBasis == (item.MaterialBasis{}) {
			st.MaterialBasis = construction.HistoricalBasis(construction.Kind(st.Building), w.constructionVersion)
			v.Set(reflect.ValueOf(st))
		}
		return false
	case t == commandType:
		c, ok := w.m.Command(v.Interface().(command.Command))
		if ok {
			v.Set(reflect.ValueOf(c))
		}
		return !ok
	case t == lineType && w.m.gone(v.Interface().(trade.Line).Kind):
		return true
	case t == kindType:
		k := item.Kind(v.Uint()) // #nosec G115 -- v's type is item.Kind (uint8), so v.Uint() fits
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
			if gone && (t == stackType && f.Name == "Kind" || f.Type == stackType || f.Type == commandType || f.Type == lineType) {
				drop = true
			}
		}
		// Only inventory owners have a Goods-sorted contract. Do not
		// reorder unrelated delivery and reward sequences.
		if t == stockType || t == cargoType {
			lines := v.FieldByName("Lines").Interface().([]trade.Line)
			slices.SortStableFunc(lines, compareLines)
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

func compareLines(a, b trade.Line) int { return a.Goods().Compare(b.Goods()) }

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

// filter reconstructs permissions in the saved key namespace. A historical
// plugin slot can now be a native kind, so the current core boundary is
// not a valid source boundary. Preserve the out-of-band sentinel.
func (w *walker) filter(f zone.Filter) zone.Filter {
	if w.m.Same() {
		return f
	}
	out := zone.Filter{}.With(item.None, f.Has(item.None))
	for i, to := range w.m.to {
		if i < int(item.None) && to != item.None && f.Has(item.Kind(i)) {
			out = out.With(to, true)
		}
	}
	return out
}

// Command remaps saved item intent without changing its quote or
// silently selecting another physical lot. A missing key or ambiguous
// world lot invalidates the entire command, never just one offer line.
func (m ItemKinds) Command(c command.Command) (command.Command, bool) {
	if m.Same() {
		return c, true
	}
	switch c.Kind {
	case command.SetBillIngredient:
		k := item.Kind(c.Stuff)
		if m.gone(k) {
			return c, false
		}
		c.Stuff = uint8(m.Kind(k))
	case command.ProposeTrade, command.WorldCaravanTrade:
		name, ok := trade.RemapOffer(c.Name, len(m.to), m.Kind, c.Kind == command.WorldCaravanTrade)
		if !ok {
			return c, false
		}
		c.Name = name
	}
	return c, true
}

// Commands remaps a list of queued commands, dropping an invalid intent
// as a whole. Walk also applies Command inside nested saved orders.
func (m ItemKinds) Commands(cmds []command.Command) []command.Command {
	if m.Same() {
		return cmds
	}
	out := cmds[:0:0]
	for _, c := range cmds {
		if moved, ok := m.Command(c); ok {
			out = append(out, moved)
		}
	}
	return out
}
