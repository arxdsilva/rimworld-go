package game

import (
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/entity"
)

// Feeding troughs (SIM-024): a pen's tame animals eat from them once the
// grass is gone; haulers fill them with feed matching the filter.

// Trough feed filters.
const (
	TroughPlants uint8 = iota // straw, berries, raw vegetables
	TroughMeat
	TroughAny
)

var troughFilterNames = [...]string{"plants", "meat", "any"}

// troughState is what one trough holds (saved).
type troughState struct {
	Filter uint8
	Food   map[item.Kind]int
}

// troughFill is how many food items trough id holds.
func (g *Game) troughFill(id entity.EntityID) int {
	n := 0
	for _, q := range g.troughs[id].Food {
		n += q
	}
	return n
}

// setTroughFilter applies a SetTroughFilter command.
func (g *Game) setTroughFilter(id entity.EntityID, f uint8) {
	if f > TroughAny {
		return
	}
	if b, ok := g.bldgs.Get(id); !ok || b.Kind.String() != "feeding trough" {
		return
	}
	t := g.troughs[id]
	t.Filter = f
	g.troughs[id] = t
}
