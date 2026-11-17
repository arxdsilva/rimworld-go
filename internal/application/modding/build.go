package modding

import (
	"slices"
	"strings"

	"rimworld-go/internal/domain/pawn/backstory"
)

// Catalog is the frozen content of one run: the load order, the loaded
// plugins' manifests, every moddable class's book, and every def's tree
// with its provenance (MASTER_SPEC §52).
type Catalog struct {
	Order       Order
	Plugins     []Manifest // the loaded plugins in load order, core excluded
	Backstories backstory.Book
	Defs        []Def // every typed def after inheritance and patches
}

// IDs is the load order's plugin ids, core first: what a save records.
func (c Catalog) IDs() []string {
	out := make([]string, len(c.Order.Loaded))
	for i, p := range c.Order.Loaded {
		out[i] = p.ID
	}
	return out
}

// Classes are the moddable classes, in the order their books are built.
func Classes() []string { return []string{BackstoryClass} }

// Build runs the whole pipeline over the installed plugins (found, as a
// Source lists them) for the enabled ids:
//
//	resolve the load order
//	→ core's defs as trees, then each loaded plugin's defs files
//	→ inheritance → patches, in load order
//	→ type each class into its book
//
// A broken def or patch drops only itself; a plugin whose manifest or
// dependencies fail does not load (see Resolve). Every problem names its
// plugin, file, def and field.
func Build(found []Plugin, enabled []string, gameVersion string) (Catalog, []Problem) {
	manifests := make([]Manifest, len(found))
	byID := make(map[string]int, len(found))
	for i, p := range found {
		manifests[i] = p.Manifest
		byID[p.Manifest.ID] = i
	}
	order, probs := Resolve(manifests, enabled, gameVersion)
	cat := Catalog{Order: order}

	defs := EncodeBackstories(backstory.Core())
	var ops []Op
	for _, placed := range order.Loaded {
		if placed.ID == Core {
			continue
		}
		p := found[byID[placed.ID]] // Resolve loads only ids installed once
		cat.Plugins = append(cat.Plugins, p.Manifest)
		probs = append(probs, p.Problems...)
		d, dp := CollectDefs(p)
		defs = append(defs, d...)
		probs = append(probs, dp...)
		o, op := ParseOps(p)
		ops = append(ops, o...)
		probs = append(probs, op...)
	}
	defs, probs = known(defs, probs)
	defs, ip := Inherit(defs)
	probs = append(probs, ip...)
	defs, ap := Apply(defs, ops, order.Has)
	probs = append(probs, ap...)

	var bs []Def
	for _, d := range defs {
		if d.Class == BackstoryClass {
			bs = append(bs, d)
		}
	}
	book, bp := DecodeBackstories(bs)
	probs = append(probs, bp...)
	cat.Backstories = book
	cat.Defs = defs
	return cat, probs
}

// known drops defs of a class no book reads, each with a problem.
func known(defs []Def, probs []Problem) ([]Def, []Problem) {
	out := defs[:0:0]
	for _, d := range defs {
		if slices.Contains(Classes(), d.Class) {
			out = append(out, d)
			continue
		}
		probs = append(probs, Problem{Plugin: d.Plugin, File: d.File, Def: d.Ref(),
			Msg: "unknown definition class " + d.Class + " (known: " + strings.Join(Classes(), ", ") + ")"})
	}
	return out, probs
}

// Errors reports whether probs holds anything but warnings.
func Errors(probs []Problem) bool {
	return slices.ContainsFunc(probs, func(p Problem) bool { return !p.Warn })
}
