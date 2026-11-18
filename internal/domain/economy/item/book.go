// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package item

// Class is the item class name in plugin files, patch targets and a
// save's key table.
const Class = "item"

// Book is a run's frozen set of item kinds: core's first, in Kind order,
// then the plugins' in load order (PLG-026). The zero Book is core.
// CONTRACT STUB (wave 0): the items agent designs and fills it in.
type Book struct {
	keys []string
}

// Core is the game's own item kinds, keyed "core.<name>".
func Core() Book {
	keys := make([]string, numKinds)
	for k := range numKinds {
		keys[k] = "core." + Kind(k).String()
	}
	return Book{keys: keys}
}

// Keys is every key in Kind order (index i is Kind i's).
func (b Book) Keys() []string {
	if b.keys == nil {
		return Core().Keys()
	}
	return append([]string(nil), b.keys...)
}
