// Package animalview holds what an animal's read model takes from its
// species alone. The species tables are constant, so it is derived once per
// species instead of once per animal per view (Label allocates, and the
// definition lookups copy whole rows): a big map's views hold thousands of
// animals.
package animalview

import (
	"sync"

	"rimworld-go/internal/domain/animal"
)

// Species is a species' share of an animal's read model.
type Species struct {
	Label, Attack, Diet string
	Size                float64
	MaxHP               int
}

var table = sync.OnceValue(func() []Species {
	all := animal.All()
	out := make([]Species, len(all))
	for i, s := range all {
		out[i] = of(s)
	}
	return out
})

func of(s animal.Species) Species {
	return Species{
		Label: s.Label(), Attack: animal.RosterOf(s).Attack.String(), Diet: animal.EcologyOf(s).Diet.String(),
		Size: animal.BodyLength(s), MaxHP: animal.DefOf(s).MaxHP,
	}
}

// Of is s's view (computed for a species outside the table, as the lookups
// it replaces fall back).
func Of(s animal.Species) *Species {
	if t := table(); int(s) < len(t) {
		return &t[s]
	}
	v := of(s)
	return &v
}
