// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package trait

// Class is the trait class name in plugin files, patch targets and a
// save's key table.
const Class = "trait"

// MaxBook is how many traits a book holds, None not counted: an ID is a
// byte.
const MaxBook = 255

// Book is a run's frozen set of traits: core's first, in ID order, then
// the plugins' in load order (PLG-026). It is built once at start and
// passed in; there is no package-level registry to change. The zero Book
// is core.
type Book struct {
	defs []Def    // defs[0] is None
	keys []string // keys[i] is defs[i]'s namespaced key; keys[0] is ""
}

// Core is the game's own traits, keyed "core.<Key>".
func Core() Book {
	keys := make([]string, len(defs))
	for i := 1; i < len(defs); i++ {
		keys[i] = "core." + defs[i].Key
	}
	return Book{defs: defs[:], keys: keys}
}

// NewBook freezes list (no None entry) with its namespaced keys, one per
// def. Each def's ID becomes its position + 1, so core defs must come
// first and in ID order for saves to read the same. At most MaxBook defs.
func NewBook(list []Def, keys []string) Book {
	b := Book{defs: make([]Def, 1, len(list)+1), keys: make([]string, 1, len(list)+1)}
	for i, d := range list {
		d.ID = ID(i + 1)
		b.defs = append(b.defs, d)
		b.keys = append(b.keys, keys[i])
	}
	return b
}

func (b Book) list() []Def {
	if b.defs == nil {
		return defs[:]
	}
	return b.defs
}

// DefOf looks a trait up; unknown IDs give the zero Def.
func (b Book) DefOf(id ID) Def {
	l := b.list()
	if int(id) >= len(l) {
		return Def{}
	}
	return l[id]
}

// All lists every trait but None, in ID order (a copy; the maps inside are
// shared, read only).
func (b Book) All() []Def { return append([]Def(nil), b.list()[1:]...) }

// Len is the number of traits, None not counted.
func (b Book) Len() int { return len(b.list()) - 1 }

// Key is id's namespaced key ("" for None or unknown).
func (b Book) Key(id ID) string {
	l := b.list()
	if id == None || int(id) >= len(l) {
		return ""
	}
	if b.keys == nil {
		return "core." + l[id].Key
	}
	return b.keys[id]
}

// Keys is every key in ID order (index i-1 is ID i's), what a save records
// to map its IDs onto another run's book (PLG-044).
func (b Book) Keys() []string {
	out := make([]string, b.Len())
	for i := range out {
		out[i] = b.Key(ID(i + 1))
	}
	return out
}

// IDOf is the ID of the namespaced key in this book (false when the book
// lacks it).
func (b Book) IDOf(key string) (ID, bool) {
	for id := ID(1); int(id) <= b.Len(); id++ {
		if b.Key(id) == key {
			return id, true
		}
	}
	return None, false
}

// Parse finds a trait by its Def key: a core trait's short key ("sunny")
// or any trait's namespaced key ("core.sunny", "myplugin.brave").
func (b Book) Parse(key string) (ID, bool) {
	l := b.list()
	for i := 1; i < len(l); i++ {
		if l[i].Key == key || b.Key(ID(i)) == key {
			return ID(i), true
		}
	}
	return None, false
}

// Generated lists the traits pawn generation may roll, in ID order: under
// legacy rules only the original set (so existing seeds keep their pawns),
// otherwise every trait, plugins' included.
func (b Book) Generated(legacy bool) []ID {
	n := min(max(b.Len(), 0), MaxBook)
	if legacy {
		n = min(n, int(legacyTraits)-1)
	}
	out := make([]ID, 0, n)
	for i := 1; i <= n; i++ {
		out = append(out, ID(min(i, MaxBook)))
	}
	return out
}
