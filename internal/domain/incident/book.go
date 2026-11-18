// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package incident

import (
	"slices"

	"rimworld-go/internal/domain/world/biome"
)

// Class is the incident class name in plugin files, patch targets and a
// save's key table.
const Class = "incident"

// MaxBook is the most incidents a book holds: an ID is a byte.
const MaxBook = 256

// Book is a run's frozen set of incidents: core's first, then the
// plugins' in load order (PLG-026). An incident's ID is its position, so
// core's keep theirs. The zero Book is core.
type Book struct {
	defs []Def
	keys []string // keys[i] is defs[i]'s namespaced key
}

// Core is the game's own incidents, keyed "core.<Key>".
func Core() Book {
	keys := make([]string, len(defs))
	for i, d := range defs {
		keys[i] = "core." + d.Key
	}
	return Book{defs: defs[:], keys: keys}
}

// NewBook freezes list (core's first, in catalog order) with its
// namespaced keys, one per def. Each def's ID becomes its position. A list
// longer than MaxBook is cut there (the decoder refuses the rest first).
func NewBook(list []Def, keys []string) Book {
	n := min(len(list), len(keys), MaxBook)
	out := make([]Def, n)
	for i := range n {
		out[i] = list[i]
		out[i].ID = ID(i)
	}
	return Book{defs: out, keys: append([]string(nil), keys[:n]...)}
}

func (b Book) self() Book {
	if b.defs == nil {
		return Core()
	}
	return b
}

// list is the incidents Pick and Total draw among (core's package array
// for the zero Book: no copy).
func (b Book) list() []Def {
	if b.defs == nil {
		return defs[:]
	}
	return b.defs
}

// DefOf looks an incident up; an unknown ID gives the zero Def.
func (b Book) DefOf(id ID) Def {
	l := b.defs
	if l == nil {
		return DefOf(id)
	}
	if int(id) >= len(l) {
		return Def{}
	}
	return l[id]
}

// Like is the core event whose behaviour id runs (id itself for a core
// event or an unknown one).
func (b Book) Like(id ID) ID {
	if d := b.DefOf(id); d.Key != "" {
		return d.Like
	}
	return id
}

// All lists every incident in ID order (a copy).
func (b Book) All() []Def { return append([]Def(nil), b.self().defs...) }

// Len is the number of incidents.
func (b Book) Len() int {
	if b.defs == nil {
		return len(defs)
	}
	return len(b.defs)
}

// Key is id's namespaced key ("" for an unknown ID).
func (b Book) Key(id ID) string {
	s := b.self()
	if int(id) >= len(s.keys) {
		return ""
	}
	return s.keys[id]
}

// Keys is every key in ID order (index i is ID i's).
func (b Book) Keys() []string { return append([]string(nil), b.self().keys...) }

// IDOf finds an incident by namespaced key.
func (b Book) IDOf(key string) (ID, bool) {
	i := slices.Index(b.self().keys, key)
	return ID(i), i >= 0
}

// ByName finds an incident by its Def.Key or its namespaced key.
func (b Book) ByName(name string) (Def, bool) {
	s := b.self()
	for i, d := range s.defs {
		if d.Key == name || s.keys[i] == name {
			return d, true
		}
	}
	return Def{}, false
}

// Pick is the package's PickOdds over this book's incidents.
func (b Book) Pick(c Category, st State, now uint64, bi biome.ID, o Odds, roll int, ok func(ID) bool) (ID, bool) {
	return pickOdds(b.list(), c, st, now, bi, o, roll, ok)
}

// Total is the package's TotalOdds over this book's incidents.
func (b Book) Total(c Category, st State, now uint64, bi biome.ID, o Odds, ok func(ID) bool) int {
	return totalOdds(b.list(), c, st, now, bi, o, ok)
}
