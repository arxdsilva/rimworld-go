package backstory

import "strings"

// Book is a frozen set of backstories: core's in catalog order, then the
// plugins' in load order (PLG-024). It is built once at start and passed
// in; there is no package-level registry to change. defs[0] is "none".
// The zero Book is the core set.
type Book struct {
	defs []Def
	keys []string // keys[i] is defs[i]'s namespaced key; keys[0] is ""
}

// Core is the game's own backstories, keyed "core.<slug of title>".
func Core() Book {
	keys := make([]string, len(defs))
	for i := 1; i < len(defs); i++ {
		keys[i] = "core." + Slug(defs[i].Title)
	}
	return Book{defs: defs, keys: keys}
}

// NewBook freezes list (no "none" entry) with its namespaced keys, one per
// def. Each def's ID becomes its position + 1, so core defs must come
// first and in catalog order for saves to read the same.
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
		return defs
	}
	return b.defs
}

// DefOf looks a backstory up; unknown IDs give the zero Def.
func (b Book) DefOf(id ID) Def {
	l := b.list()
	if int(id) >= len(l) {
		return Def{}
	}
	return l[id]
}

// All lists every backstory (a copy; the maps inside are shared, read only).
func (b Book) All() []Def { return append([]Def(nil), b.list()[1:]...) }

// Len is the number of backstories, "none" not counted.
func (b Book) Len() int { return len(b.list()) - 1 }

// Key is id's namespaced key ("" for none or unknown).
func (b Book) Key(id ID) string {
	l := b.list()
	if id == 0 || int(id) >= len(l) {
		return ""
	}
	if b.keys == nil {
		return "core." + Slug(l[id].Title)
	}
	return b.keys[id]
}

// Keys is every key in ID order (index i-1 is ID i's), what a save
// records to map its IDs onto another run's book (PLG-044).
func (b Book) Keys() []string {
	out := make([]string, b.Len())
	for i := range out {
		out[i] = b.Key(ID(i + 1))
	}
	return out
}

// IDOf is the ID of key in this book (false when the book lacks it).
func (b Book) IDOf(key string) (ID, bool) {
	for id := ID(1); int(id) <= b.Len(); id++ {
		if b.Key(id) == key {
			return id, true
		}
	}
	return 0, false
}

// Class is the backstory class name in plugin files, patch targets and a
// save's key table.
const Class = "backstory"

// Slug lowercases s and joins its letter and digit runs with "_"
// ("Ration-line runner" is "ration_line_runner").
func Slug(s string) string {
	var sb strings.Builder
	gap := false
	for _, r := range strings.ToLower(s) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			if gap && sb.Len() > 0 {
				sb.WriteByte('_')
			}
			sb.WriteRune(r)
			gap = false
			continue
		}
		gap = true
	}
	return sb.String()
}
