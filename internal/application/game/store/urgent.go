package store

import (
	"slices"

	"rimworld-go/internal/domain/sim/entity"
)

// Urgent is the stacks marked to haul urgently (STO-2, the vanilla "haul
// urgently" designation): haulers carry them to storage before any other
// hauling. A mark has no threshold, so nothing can flap: it lasts until
// the stack lies in storage that takes it, or is gone (Keep).
type Urgent struct {
	ids []entity.EntityID // sorted, no repeats
}

// Has reports stack id is marked.
func (u *Urgent) Has(id entity.EntityID) bool {
	_, ok := slices.BinarySearch(u.ids, id)
	return ok
}

// Any reports any stack is marked.
func (u *Urgent) Any() bool { return len(u.ids) > 0 }

// IDs is the marked stacks in id order (shared: read only, until the
// next change).
func (u *Urgent) IDs() []entity.EntityID { return u.ids }

// Mark marks (on) or unmarks stack id.
func (u *Urgent) Mark(id entity.EntityID, on bool) {
	i, ok := slices.BinarySearch(u.ids, id)
	switch {
	case on && !ok:
		u.ids = slices.Insert(u.ids, i, id)
	case !on && ok:
		u.ids = slices.Delete(u.ids, i, i+1)
	}
}

// MarkRect marks (on) or unmarks every loose stack lying in the
// rectangle x1..x2, y1..y2 (any corner order).
func (u *Urgent) MarkRect(it Items, x1, y1, x2, y2 int, on bool) {
	minX, maxX, minY, maxY := min(x1, x2), max(x1, x2), min(y1, y2), max(y1, y2)
	for _, id := range it.ItemIDs() {
		if st, ok := it.Stack(id); ok && st.Carrier == 0 && st.X >= minX && st.X <= maxX && st.Y >= minY && st.Y <= maxY {
			u.Mark(id, on)
		}
	}
}

// Keep drops every mark keep refuses (a stack gone or stored).
func (u *Urgent) Keep(keep func(id entity.EntityID) bool) {
	u.ids = slices.DeleteFunc(u.ids, func(id entity.EntityID) bool { return !keep(id) })
}

// Export is the marks for the save (nil when none).
func (u *Urgent) Export() []entity.EntityID {
	if len(u.ids) == 0 {
		return nil
	}
	return slices.Clone(u.ids)
}

// Import restores saved marks (sorted and de-duplicated).
func (u *Urgent) Import(ids []entity.EntityID) {
	u.ids = slices.Compact(slices.Sorted(slices.Values(ids)))
	if len(u.ids) == 0 {
		u.ids = nil
	}
}
