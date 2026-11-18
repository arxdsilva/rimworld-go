package recipe

import (
	"slices"
	"strings"

	"rimworld-go/internal/domain/building/construction"
)

// Class is the recipe class name in plugin files, patch targets and a
// save's key table.
const Class = "recipe"

// MaxBook is the most recipes a book holds: an ID is a byte (bills, the
// AddBill command and the bills read model carry it as one).
const MaxBook = 256

// Book is a frozen set of recipes: core's in catalog order, then the
// plugins' in load order (PLG-025). It is built once at start and passed
// in; there is no package-level registry to change. A recipe's ID is its
// position, so core's keep theirs. The zero Book is the core set.
type Book struct {
	defs  []Def
	keys  []string // keys[i] is defs[i]'s namespaced key
	bench [][]Def  // bench[k]: the recipes a bench of kind k runs, legacy ones excluded (For)
}

// core is the game's own book, its bench index built once (read only).
var core = newBook(defs[:], coreKeys())

func coreKeys() []string {
	keys := make([]string, len(defs))
	for i, d := range defs {
		keys[i] = "core." + d.Name
	}
	return keys
}

// Core is the game's own recipes, keyed "core.<Name>" (shared, read only).
func Core() Book { return core }

// NewBook freezes list (core's first, in catalog order) with its
// namespaced keys, one per def. Each def's ID becomes its position, so
// saves read core's the same. A list longer than MaxBook is cut there
// (the decoder refuses the rest with a problem first).
func NewBook(list []Def, keys []string) Book {
	n := min(len(list), MaxBook)
	out := make([]Def, n)
	for i := range n {
		out[i] = list[i]
		out[i].ID = ID(i)
	}
	return newBook(out, append([]string(nil), keys[:n]...))
}

func newBook(list []Def, keys []string) Book {
	n := 0
	for construction.Kind(n).Valid() {
		n++
	}
	b := Book{defs: list, keys: keys, bench: make([][]Def, n)}
	for k := range b.bench {
		for _, d := range list {
			if d.At(construction.Kind(k)) && !d.Legacy {
				b.bench[k] = append(b.bench[k], d)
			}
		}
	}
	return b
}

func (b Book) self() Book {
	if b.defs == nil {
		return core
	}
	return b
}

// DefOf looks a recipe up; unknown IDs give the zero Def (Name "").
func (b Book) DefOf(id ID) Def {
	l := b.self().defs
	if int(id) >= len(l) {
		return Def{}
	}
	return l[id]
}

// All lists every recipe in ID order (shared, read only; clipped, so an
// append by a caller copies instead of writing into the book).
func (b Book) All() []Def { return slices.Clip(b.self().defs) }

// For lists the recipes a bench of kind k runs (At; legacy ones
// excluded), in ID order: an index built with the book, shared and read
// only, so work discovery allocates nothing.
func (b Book) For(k construction.Kind) []Def {
	idx := b.self().bench
	if int(k) >= len(idx) {
		return nil
	}
	return slices.Clip(idx[k])
}

// Upgrade maps a legacy recipe to the one that replaced it.
func (b Book) Upgrade(id ID) ID {
	if id == ButcherDustbuck || id == ButcherRidgewolf {
		return ButcherAnimal
	}
	return id
}

// Len is the number of recipes.
func (b Book) Len() int { return len(b.self().defs) }

// Key is id's namespaced key ("" for an unknown ID).
func (b Book) Key(id ID) string {
	s := b.self()
	if int(id) >= len(s.keys) {
		return ""
	}
	return s.keys[id]
}

// Keys is every key in ID order (index i is ID i's), what a save records
// to map its IDs onto another run's book (PLG-044).
func (b Book) Keys() []string { return append([]string(nil), b.self().keys...) }

// IDOf is the ID of key in this book (false when the book lacks it).
func (b Book) IDOf(key string) (ID, bool) {
	for i, k := range b.self().keys {
		if k == key {
			return ID(i), true
		}
	}
	return 0, false
}

// NameOf is the Def.Name of a recipe keyed key: a core recipe's name is
// its key without "core." (what research unlocks name), a plugin's is its
// whole key, so it never meets a core name.
func NameOf(key string) string {
	if rest, ok := strings.CutPrefix(key, "core."); ok {
		return rest
	}
	return key
}
