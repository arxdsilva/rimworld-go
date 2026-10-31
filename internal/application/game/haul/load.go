package haul

import "rimworld-go/internal/domain/economy/item"

// Toppable reports a load of o's kind still fits on stack o: a full load
// on a stack of the plain limit; for a kind with a smaller wiki limit,
// any room (PRD-4; Capacity then caps the load).
func Toppable(o item.Stack) bool {
	lim := o.Kind.StackLimit()
	return o.Qty+item.CarryLimit <= lim || lim < item.StackLimit && o.Qty < lim
}

// Cell tallies the stacks lying on one storage cell for Room: how many,
// and the smallest of the hauled kind.
type Cell struct {
	n, low int
	same   bool
}

// Add counts one stack of qty on the cell; same: of the hauled kind.
func (c *Cell) Add(same bool, qty int) {
	c.n++
	if same && (!c.same || qty < c.low) {
		c.low, c.same = qty, true
	}
}

// Room is how much of a kind with stack limit lim the cell still takes
// on its own: a full stack while it has a free slot (slots: 1 on the
// ground, more on a shelf), else what tops up its smallest stack of the
// kind.
func (c Cell) Room(lim, slots int) int {
	switch {
	case c.n < slots:
		return lim
	case c.same:
		return max(0, lim-c.low)
	}
	return 0
}

// Capacity is the most one load of a kind with stack limit lim carries to
// a cell with room: what a colonist carries, a tile holds and the cell
// still takes, never below one.
func Capacity(lim, room int) int { return max(1, min(item.CarryLimit, lim, room)) }

// SplitLoad is the take-sized load split off stack st: same kind, place,
// age and drying, and the same body (species, who, when) for corpses. It
// is the colony's, allowed and unmarked (PLAY-017: picked up); the rest
// keeps st's flags.
func SplitLoad(st item.Stack, take int) item.Stack {
	return item.Stack{Kind: st.Kind, Qty: take, X: st.X, Y: st.Y, Rot: st.Rot, Dried: st.Dried,
		Animal: st.Animal, Of: st.Of, DiedAt: st.DiedAt, Hatch: st.Hatch, Tame: st.Tame, Poison: st.Poison,
		Quality: st.Quality, Wear: st.Wear, Stuff: st.Stuff, // PRD-2: the same made thing
		Ingredients: st.Ingredients, IngredientSpecies: st.IngredientSpecies} // FOOD-4: the same meal
}
