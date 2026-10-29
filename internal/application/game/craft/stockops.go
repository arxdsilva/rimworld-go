package craft

import (
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/entity"
)

// Stack moves craft does itself over the host's plain stack reads and
// writes (PRD-3 moved them out of the game's adapters).

// driedStock is the colony's count of dried food.
func driedStock(h Host) int {
	n := 0
	for _, id := range h.ItemIDs() {
		if st, ok := h.Stack(id); ok && st.Dried {
			n += st.Qty
		}
	}
	return n
}

// merge adds carried stack from to carried stack into (same kind, the
// rot averaged) and deletes from: one load gathered from several stacks.
func merge(h Host, into, from entity.EntityID) {
	a, ok := h.Stack(into)
	f, fok := h.Stack(from)
	if !ok || !fok {
		return
	}
	a.Rot, a.Qty = item.MergeRot(a.Qty, a.Rot, f.Qty, f.Rot), a.Qty+f.Qty
	h.SetStack(into, a)
	h.DeleteItem(from)
}

// putDown sets carried stack id down at (x, y) as it is, never merged
// (an ingredient laid on its bench keeps its id and reservation).
func putDown(h Host, id entity.EntityID, x, y int) {
	if st, ok := h.Stack(id); ok {
		st.Carrier, st.X, st.Y = 0, x, y
		h.SetStack(id, st)
	}
}
