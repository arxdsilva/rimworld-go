// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package research

import (
	"slices"
	"strings"
)

// Class is the research class name in plugin files, patch targets and a
// save's key table.
const Class = "research"

// MaxBook is the most projects a book holds: a Project is a byte (the
// SetResearch command and the research view carry it as one).
const MaxBook = 256

// Book is a run's frozen set of research projects: core's in table order,
// then the plugins' in load order (PLG-026). It is built once at start and
// passed in; there is no package-level registry to change. A project's ID
// is its position, so core's keep theirs. The zero Book is core.
type Book struct {
	defs  []Def
	keys  []string             // keys[i] is defs[i]'s namespaced key
	gates map[string][]Project // unlock key -> the projects that gate it, in ID order (Locks)
}

// core is the game's own book, its unlock index built once (read only).
var core = newBook(defs[:], coreKeys())

func coreKeys() []string {
	keys := make([]string, len(defs))
	for i, d := range defs {
		keys[i] = "core." + d.Name
	}
	return keys
}

// Core is the game's own projects, keyed "core.<Name>" (shared, read only).
func Core() Book { return core }

// NewBook freezes list (core's first, in table order) with its namespaced
// keys, one per def. Each def's Project becomes its position, so saves
// read core's the same; Requires and Hidden must already name positions.
// A list longer than MaxBook is cut there (the decoder refuses the rest
// with a problem first).
func NewBook(list []Def, keys []string) Book {
	n := min(len(list), MaxBook, len(keys))
	out := make([]Def, n)
	for i := range n {
		out[i] = list[i]
		out[i].Project = Project(i)
	}
	return newBook(out, append([]string(nil), keys[:n]...))
}

func newBook(list []Def, keys []string) Book {
	b := Book{defs: list, keys: keys, gates: map[string][]Project{}}
	for _, d := range list {
		for _, u := range d.Unlocks {
			if l := b.gates[u]; len(l) == 0 || l[len(l)-1] != d.Project {
				b.gates[u] = append(l, d.Project)
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

// NameOf is the Def.Name of a project keyed key: a core project's name is
// its key without "core.", a plugin's is its whole key, so it never meets
// a core name.
func NameOf(key string) string {
	if rest, ok := strings.CutPrefix(key, "core."); ok {
		return rest
	}
	return key
}

// Len is the number of projects.
func (b Book) Len() int { return len(b.self().defs) }

// DefOf looks a project up; an unknown one gives the zero Def (Name "").
func (b Book) DefOf(p Project) Def {
	l := b.self().defs
	if int(p) >= len(l) {
		return Def{}
	}
	return l[p]
}

// All lists every project in ID order (shared, read only; clipped, so an
// append by a caller copies instead of writing into the book).
func (b Book) All() []Def { return slices.Clip(b.self().defs) }

// Key is p's namespaced key ("" for an unknown project).
func (b Book) Key(p Project) string {
	s := b.self()
	if int(p) >= len(s.keys) {
		return ""
	}
	return s.keys[p]
}

// Keys is every key in ID order (index i is project i's), what a save
// records to map its IDs onto another run's book (PLG-044).
func (b Book) Keys() []string { return append([]string(nil), b.self().keys...) }

// IDOf is the project keyed key in this book (false when the book lacks it).
func (b Book) IDOf(key string) (Project, bool) {
	for i, k := range b.self().keys {
		if k == key {
			return Project(i), true
		}
	}
	return 0, false
}

// Parse finds a project by Def.Name (a plugin project's is its key).
func (b Book) Parse(name string) (Project, bool) {
	for _, d := range b.self().defs {
		if d.Name == name {
			return d.Project, true
		}
	}
	return 0, false
}

// Done reports p finished in pr (its cost in this book reached).
func (b Book) Done(pr *Progress, p Project) bool {
	l := b.self().defs
	return int(p) < len(l) && pr[p] >= l[p].Cost
}

// Available reports p can be researched now in pr: not done, its
// prerequisites (drawn and hidden) done.
func (b Book) Available(pr *Progress, p Project) bool {
	if int(p) >= b.Len() || b.Done(pr, p) {
		return false
	}
	d := b.DefOf(p)
	for _, rs := range [2][]Project{d.Requires, d.Hidden} {
		for _, r := range rs {
			if !b.Done(pr, r) {
				return false
			}
		}
	}
	return true
}

// Locks returns the first project in ID order that must still be finished
// before unlock key is usable (ok false: nothing gates it, or every
// project that does is researched).
func (b Book) Locks(pr *Progress, key string) (Project, bool) {
	for _, p := range b.self().gates[key] {
		if !b.Done(pr, p) {
			return p, true
		}
	}
	return 0, false
}

// Add puts n points into p, reporting whether that finished it.
func (b Book) Add(pr *Progress, p Project, n int) bool {
	if int(p) >= b.Len() || b.Done(pr, p) {
		return false
	}
	pr[p] = min(b.DefOf(p).Cost, pr[p]+n)
	return b.Done(pr, p)
}

// Complete is every project of the book researched (sandboxes, tests).
func (b Book) Complete() Progress {
	var pr Progress
	for _, d := range b.self().defs {
		pr[d.Project] = d.Cost
	}
	return pr
}
