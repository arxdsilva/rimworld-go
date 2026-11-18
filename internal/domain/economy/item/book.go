// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package item

import "strings"

// Class is the item class name in plugin files, patch targets and a
// save's key table.
const Class = "item"

// The kind space (PLG-026): core's kinds come first, in Kind order, then
// the plugins' in load order. A Kind is a byte and None is no kind, so a
// run holds at most MaxKinds.
const (
	// CoreKinds is how many kinds the game itself defines.
	CoreKinds = int(numKinds)
	// MaxKinds is how many kinds a run may hold, core's included.
	MaxKinds = 255
	// None is no kind: what a saved reference to a gone plugin's item
	// becomes, and a "no material" sentinel. Every Kind method gives it
	// its neutral answer (as any kind past the run's book).
	None Kind = 255
)

// Def is one item kind's plain data (PLG-026): what the book answers for
// it. A core kind's is the game's tables (CoreDef) with the numbers a
// plugin may patch; a plugin kind's is all it has, a simple good with no
// behaviour of its own (no food, medicine, weapon, apparel or material).
type Def struct {
	Label string // display name
	Desc  string // one paragraph for the info card
	// Path is a plugin kind's storage filter category ("Manufactured/
	// Components"); core kinds' paths are the filter tree's own ("").
	Path    string
	Mass    int    // grams per unit
	Value   int    // market value per unit
	Stack   int    // how many share a tile
	Texture string // a plugin kind's sprite, "textures/….png" in its plugin
	// Smelt is the steel one unit gives back at a smelter (a plugin kind
	// only; 0 goes by the recipe that makes it, recipe.Book.SmeltYield).
	Smelt int
}

// CoreDef is the game's own data for core kind k.
func CoreDef(k Kind) Def {
	return Def{Label: k.String(), Mass: k.MassGrams(), Value: k.Value(), Stack: k.StackLimit()}
}

// Book is a run's frozen set of item kinds: core's first, in Kind order,
// then the plugins' in load order (PLG-026). The zero Book is core.
type Book struct {
	keys []string
	defs []Def
}

// Core is the game's own item kinds, keyed "core.<name>".
func Core() Book {
	keys, defs := make([]string, numKinds), make([]Def, numKinds)
	for k := range numKinds {
		keys[k], defs[k] = "core."+k.String(), CoreDef(k)
	}
	return Book{keys: keys, defs: defs}
}

// NewBook freezes defs under keys (index i is Kind i's): core's
// CoreKinds first, in Kind order, then at most MaxKinds-CoreKinds plugin
// kinds. The slices are copied.
func NewBook(defs []Def, keys []string) Book {
	return Book{keys: append([]string(nil), keys...), defs: append([]Def(nil), defs...)}
}

func (b Book) core() Book {
	if b.keys == nil {
		return Core()
	}
	return b
}

// Len is how many kinds the run holds.
func (b Book) Len() int {
	if b.keys == nil {
		return CoreKinds
	}
	return len(b.keys)
}

// Keys is every key in Kind order (index i is Kind i's).
func (b Book) Keys() []string { return append([]string(nil), b.core().keys...) }

// Key is kind k's key ("" outside the book).
func (b Book) Key(k Kind) string {
	if int(k) >= b.Len() {
		return ""
	}
	if b.keys == nil {
		return "core." + k.String()
	}
	return b.keys[k]
}

// KindOf is the kind keyed key (ok false when the run has none).
func (b Book) KindOf(key string) (Kind, bool) {
	if name, ok := strings.CutPrefix(key, "core."); ok {
		if k, ok := ParseKind(name); ok {
			return k, true
		}
	}
	for i := CoreKinds; i < len(b.keys); i++ {
		if b.keys[i] == key {
			return Kind(i), true
		}
	}
	return 0, false
}

// Plugin reports k is one of the run's plugin kinds.
func (b Book) Plugin(k Kind) bool { return int(k) >= CoreKinds && int(k) < b.Len() }

// Plugins is the run's plugin kinds, in Kind order (nil for core).
func (b Book) Plugins() []Kind {
	var out []Kind
	for i := CoreKinds; i < b.Len(); i++ {
		out = append(out, Kind(i))
	}
	return out
}

// Def is kind k's data (a kind past the book: the zero Def).
func (b Book) Def(k Kind) Def {
	switch {
	case int(k) < len(b.defs):
		return b.defs[k]
	case b.defs == nil && k < numKinds:
		return CoreDef(k)
	}
	return Def{}
}

// Name is the name views and commands carry for k: a core kind's own
// (Kind.String), a plugin kind's key ("unknown" past the book).
func (b Book) Name(k Kind) string {
	if b.Plugin(k) {
		return b.keys[k]
	}
	return k.String()
}

// Parse is the kind a view or command names (Name's inverse).
func (b Book) Parse(name string) (Kind, bool) {
	if k, ok := ParseKind(name); ok {
		return k, true
	}
	if !strings.Contains(name, ".") {
		return 0, false
	}
	k, ok := b.KindOf(name)
	return k, ok && b.Plugin(k)
}

// Label is k's display name.
func (b Book) Label(k Kind) string {
	if b.Plugin(k) {
		return b.defs[k].Label
	}
	return k.String()
}

// Value is the market value of one unit of k (a patched core kind's
// patched value).
func (b Book) Value(k Kind) int {
	if int(k) < len(b.defs) {
		return b.defs[k].Value
	}
	return k.Value()
}

// MassGrams is one unit of k's mass in grams (at least 1).
func (b Book) MassGrams(k Kind) int {
	if int(k) < len(b.defs) {
		return max(1, b.defs[k].Mass)
	}
	return k.MassGrams()
}

// StackLimit is how many units of k share a tile.
func (b Book) StackLimit(k Kind) int {
	if int(k) < len(b.defs) {
		return max(1, b.defs[k].Stack)
	}
	return k.StackLimit()
}

// Category is the resource list heading of k (Kind.Category; a plugin
// kind's from its storage path: raw resources are Materials, the rest
// Manufactured).
func (b Book) Category(k Kind) string {
	if !b.Plugin(k) {
		return k.Category()
	}
	if strings.HasPrefix(b.defs[k].Path, "Raw resources/") {
		return "Materials"
	}
	return "Manufactured"
}
