// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package incident

// Class is the incident class name in plugin files, patch targets and a
// save's key table.
const Class = "incident"

// Book is a run's frozen set of incidents: core's first, then the
// plugins' in load order (PLG-026). The zero Book is core.
// CONTRACT STUB (wave 0): the incidents agent fills it in.
type Book struct {
	defs []Def
	keys []string
}

// Core is the game's own incidents, keyed "core.<Key>".
func Core() Book {
	keys := make([]string, len(defs))
	for i, d := range defs {
		keys[i] = "core." + d.Key
	}
	return Book{defs: defs[:], keys: keys}
}

func (b Book) self() Book {
	if b.defs == nil {
		return Core()
	}
	return b
}

// All lists every incident in ID order (as the package's All).
func (b Book) All() []Def { return append([]Def(nil), b.self().defs...) }

// Keys is every key in ID order (index i is ID i's).
func (b Book) Keys() []string { return append([]string(nil), b.self().keys...) }
