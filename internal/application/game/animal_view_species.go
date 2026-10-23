package game

import (
	"sync"

	"rimworld-go/internal/domain/animal"
)

// speciesView is what an animal's read model takes from its species
// alone. The species tables are constant, so it is derived once per
// species instead of once per animal per view (Label allocates, and the
// definition lookups copy whole rows): a big map's views hold thousands
// of animals.
type speciesView struct {
	label, attack, diet string
	size                float64
	maxHP               int
}

var speciesViews = sync.OnceValue(func() []speciesView {
	all := animal.All()
	out := make([]speciesView, len(all))
	for i, s := range all {
		out[i] = speciesViewFor(s)
	}
	return out
})

func speciesViewFor(s animal.Species) speciesView {
	return speciesView{
		label: s.Label(), attack: animal.RosterOf(s).Attack.String(), diet: animal.EcologyOf(s).Diet.String(),
		size: animal.BodyLength(s), maxHP: animal.DefOf(s).MaxHP,
	}
}

// speciesViewOf is s's speciesView (computed for a species outside the
// table, as the lookups it replaces fall back).
func speciesViewOf(s animal.Species) *speciesView {
	if t := speciesViews(); int(s) < len(t) {
		return &t[s]
	}
	v := speciesViewFor(s)
	return &v
}
