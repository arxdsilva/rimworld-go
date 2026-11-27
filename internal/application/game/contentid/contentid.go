// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

// Package contentid fingerprints the game's content: the definitions in
// the content registries (research, recipes, buildings, biomes, factions,
// incidents, traits, drugs, backstories, quests, animals). A save carries
// the fingerprint (savefmt.Content), and a build that reads a save of its
// own format and build but another fingerprint says so (SAV-025).
package contentid

import (
	"cmp"
	"fmt"
	"hash/fnv"
	"slices"
	"strconv"

	"rimworld-go/internal/application/game/savefmt"
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/building/construction"
	itemkind "rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/recipe"
	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/incident"
	"rimworld-go/internal/domain/pawn/backstory"
	"rimworld-go/internal/domain/pawn/drug"
	"rimworld-go/internal/domain/pawn/mood"
	"rimworld-go/internal/domain/pawn/trait"
	"rimworld-go/internal/domain/quest"
	"rimworld-go/internal/domain/research"
	"rimworld-go/internal/domain/world/biome"
)

// Core is the plugin every build runs: the game's own content.
const Core = "core"

// Current is the build's own content with no plugin: its fingerprint and
// the plugin list ["core"].
func Current() savefmt.Content { return Of([]string{Core}, Books{}) }

// Books are the frozen books of the moddable classes a run plays with
// (the zero value is core's).
type Books struct {
	Backstories backstory.Book
	Recipes     recipe.Book
	Traits      trait.Book
	Research    research.Book
	Incidents   incident.Book
	Items       itemkind.Book // its kinds' keys: the items agent adds their defs (PLG-026)
	Thoughts    mood.Book
}

// Of is a run's content (PLG-043, PLG-050): plugins is the load order
// (core first) and the books are the frozen ones the run plays with, so
// the fingerprint covers what plugins added and patched. It walks every
// registry, so the composition root calls it once.
func Of(plugins []string, books Books) savefmt.Content {
	return savefmt.Content{Hash: fingerprintOf(registries(books)), Plugins: append([]string(nil), plugins...)}
}

// Fingerprint is an FNV-64a hash of the build's own definitions.
func Fingerprint() uint64 { return fingerprintOf(registries(Books{})) }

// list is one registry: its name and its definitions.
type list struct {
	name  string
	items []item
}

// item is one definition: its stable id and its printed value.
type item struct{ key, body string }

// listOf prints defs, each under key(def). %+v is deterministic for the
// registries' types: no funcs, pointers, interfaces or channels, and fmt
// prints maps in key order (TestDefsPrintDeterministically).
func listOf[T any](name string, defs []T, key func(T) string) list {
	l := list{name: name, items: make([]item, len(defs))}
	for i, d := range defs {
		l.items[i] = item{key(d), fmt.Sprintf("%+v", d)}
	}
	return l
}

func registries(books Books) []list {
	sprint := func(v any) string { return fmt.Sprint(v) }
	lists := []list{
		listOf("research", books.Research.All(), func(d research.Def) string { return sprint(d.Project) }),
		listOf("recipe", books.Recipes.All(), func(d recipe.Def) string { return sprint(d.ID) }),
		listOf("construction", construction.All(), func(d construction.Def) string { return sprint(d.Kind) }),
		listOf("biome", biome.All(), func(d biome.Def) string { return d.Key }),
		listOf("faction", faction.All(), func(d faction.Def) string { return d.Key }),
		listOf("incident", books.Incidents.All(), func(d incident.Def) string { return d.Key }),
		listOf("trait", books.Traits.All(), func(d trait.Def) string { return d.Key }),
		listOf("drug", drug.All(), func(d drug.Def) string { return sprint(d.Kind) }),
		listOf("backstory", books.Backstories.All(), func(d backstory.Def) string { return sprint(d.ID) }),
		listOf("quest", quest.Defs(), func(d quest.Def) string { return sprint(d.Kind) }),
		listOf("animal", animalDefs(), func(d animalDef) string { return d.Species.String() }),
	}
	if l := itemList(books.Items); len(l.items) > 0 { // core's own add nothing: its fingerprint stays (PLG-026)
		lists = append(lists, l)
	}
	if l := thoughtList(books.Thoughts); len(l.items) > 0 { // core's own add nothing
		lists = append(lists, l)
	}
	return lists
}

// thoughtList is the thoughts a plugin patched: each kind whose tuning
// differs from the game's own.
func thoughtList(b mood.Book) list {
	l := list{name: "thought"}
	core := mood.Core()
	for i := range b.Len() {
		k := mood.Kind(i) // #nosec G115 -- i < Len() <= numKinds, fits Kind
		if d := b.DefOf(k); d != core.DefOf(k) {
			l.items = append(l.items, item{b.Key(k), fmt.Sprintf("%+v", d)})
		}
	}
	return l
}

// itemList is the item kinds plugins added or patched: every plugin
// kind and each core kind whose data differs from the game's own.
func itemList(b itemkind.Book) list {
	l := list{name: "item"}
	for k := range b.Len() {
		kind := itemkind.Kind(k)
		if d := b.Def(kind); b.Plugin(kind) || d != itemkind.CoreDef(kind) {
			l.items = append(l.items, item{b.Key(kind), fmt.Sprintf("%+v", d)})
		}
	}
	return l
}

// animalDef is a species with its definition (animal.All lists species).
type animalDef struct {
	Species animal.Species
	Def     animal.Def
}

func animalDefs() []animalDef {
	all := animal.All()
	out := make([]animalDef, len(all))
	for i, s := range all {
		out[i] = animalDef{s, animal.DefOf(s)}
	}
	return out
}

// fingerprintOf hashes lists in their order, each list's items in key
// order (a registry that reorders its definitions keeps its fingerprint).
// Every string is length-prefixed, so no two different inputs run
// together into the same bytes.
func fingerprintOf(lists []list) uint64 {
	h := fnv.New64a()
	put := func(s string) {
		_, _ = h.Write(strconv.AppendInt(nil, int64(len(s)), 10))
		_, _ = h.Write([]byte{':'})
		_, _ = h.Write([]byte(s))
	}
	for _, l := range lists {
		put(l.name)
		put(strconv.Itoa(len(l.items)))
		items := slices.Clone(l.items)
		slices.SortStableFunc(items, func(a, b item) int { return cmp.Compare(a.key, b.key) })
		for _, it := range items {
			put(it.key)
			put(it.body)
		}
	}
	return h.Sum64()
}
