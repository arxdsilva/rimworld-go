package modding

import "rimworld-go/internal/domain/pawn/backstory"

// Catalog is the frozen content of one run: the load order, every
// moddable class's book, and every def's tree with its provenance.
type Catalog struct {
	Order       Order
	Backstories backstory.Book
	Defs        []Def
}

// Build runs the whole pipeline over the installed plugins. Owner: lead
// (wave 2).
func Build(found []Plugin, enabled []string, gameVersion string) (Catalog, []Problem) {
	o, probs := Resolve(nil, enabled, gameVersion)
	return Catalog{Order: o, Backstories: backstory.Core()}, probs
}
