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
	"rimworld-go/internal/domain/economy/recipe"
	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/incident"
	"rimworld-go/internal/domain/pawn/backstory"
	"rimworld-go/internal/domain/pawn/drug"
	"rimworld-go/internal/domain/pawn/trait"
	"rimworld-go/internal/domain/quest"
	"rimworld-go/internal/domain/research"
	"rimworld-go/internal/domain/world/biome"
)

// Core is the plugin every build runs: the game's own content. There is
// no mod system yet, so it is the only one.
const Core = "core"

// Current is the running build's content: its fingerprint and plugins in
// load order. It walks every registry, so the composition root calls it
// once.
func Current() savefmt.Content {
	return savefmt.Content{Hash: Fingerprint(), Plugins: []string{Core}}
}

// Fingerprint is an FNV-64a hash of every registry's definitions.
func Fingerprint() uint64 { return fingerprintOf(registries()) }

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

func registries() []list {
	sprint := func(v any) string { return fmt.Sprint(v) }
	return []list{
		listOf("research", research.All(), func(d research.Def) string { return sprint(d.Project) }),
		listOf("recipe", recipe.All(), func(d recipe.Def) string { return sprint(d.ID) }),
		listOf("construction", construction.All(), func(d construction.Def) string { return sprint(d.Kind) }),
		listOf("biome", biome.All(), func(d biome.Def) string { return d.Key }),
		listOf("faction", faction.All(), func(d faction.Def) string { return d.Key }),
		listOf("incident", incident.All(), func(d incident.Def) string { return d.Key }),
		listOf("trait", trait.All(), func(d trait.Def) string { return d.Key }),
		listOf("drug", drug.All(), func(d drug.Def) string { return sprint(d.Kind) }),
		listOf("backstory", backstory.All(), func(d backstory.Def) string { return sprint(d.ID) }),
		listOf("quest", quest.Defs(), func(d quest.Def) string { return sprint(d.Kind) }),
		listOf("animal", animalDefs(), func(d animalDef) string { return d.Species.String() }),
	}
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
		h.Write(strconv.AppendInt(nil, int64(len(s)), 10))
		h.Write([]byte{':'})
		h.Write([]byte(s))
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
