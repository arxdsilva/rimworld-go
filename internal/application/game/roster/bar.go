package roster

import (
	"slices"

	"rimworld-go/internal/domain/sim/entity"
)

// BarOrder is the player's order of the colonist bar (CTL-011): nothing but
// the view reads it, so the colony's own lists keep their order, and AI and
// work never see this one. Empty until the player first moves someone: the bar
// is then the colony's order, and a save stores nothing. Otherwise ids runs
// first to last and may hold colonists away from the bar, who keep their slot
// for when they return; a colonist it does not hold stands after those it does.
type BarOrder struct {
	ids []entity.EntityID
}

// Order is on, the colonists on the bar, first to last. It may be on itself:
// read only.
func (b *BarOrder) Order(on []entity.EntityID) []entity.EntityID {
	if len(b.ids) == 0 {
		return on
	}
	out := make([]entity.EntityID, 0, len(on))
	for _, id := range b.ids {
		if slices.Contains(on, id) {
			out = append(out, id)
		}
	}
	for _, id := range on {
		if !slices.Contains(b.ids, id) {
			out = append(out, id)
		}
	}
	return out
}

// Places is each of ids' place on the bar of on, -1 for one not on it. Colony
// order finds the next id right after the last, so an unmoved bar walks once.
func (b *BarOrder) Places(on, ids []entity.EntityID) []int {
	bar, out, at := b.Order(on), make([]int, len(ids)), 0
	for i, id := range ids {
		if at >= len(bar) || bar[at] != id {
			at = slices.Index(bar, id)
		}
		out[i] = at
		at++
	}
	return out
}

// Move is the command: colonist id goes to place on the bar of on, clamped to
// the bar. Anyone not on it is refused; a move that changes nothing records
// nothing. Colonists away stay in their slots, and the ones on the bar fill the
// rest in the new order.
func (b *BarOrder) Move(on []entity.EntityID, id entity.EntityID, place int) {
	bar := b.Order(on)
	from := slices.Index(bar, id)
	if from < 0 {
		return
	}
	place = max(0, min(place, len(bar)-1))
	if place == from {
		return
	}
	bar = slices.Insert(slices.Delete(slices.Clone(bar), from, from+1), place, id)
	next := slices.Clone(b.ids)
	for _, v := range on {
		if !slices.Contains(next, v) {
			next = append(next, v)
		}
	}
	at := 0
	for i, v := range next {
		if slices.Contains(on, v) {
			next[i], at = bar[at], at+1
		}
	}
	b.ids = next
}

// Export is the order for a save (nil until the player moved someone).
func (b *BarOrder) Export() []entity.EntityID { return slices.Clone(b.ids) }

// Import restores a saved order (none in an older save).
func (b *BarOrder) Import(ids []entity.EntityID) { b.ids = slices.Clone(ids) }
