package inventory

import (
	"slices"

	"rimworld-go/internal/domain/economy/item"
)

// Entry is one stack in a pawn's pack. The stack is off the map: X, Y and
// Carrier are zero, so it joins no stockpile, index, reservation or carrier
// filter until it leaves through the pawn's hands.
type Entry struct {
	ID     uint32 // stable within this pack, for commands
	Stack  item.Stack
	Unload bool // marked to go to storage
}

// Pack is what a pawn carries on its back (CSE-036). It is a value type with no
// simulation: game/sense decides what goes in and comes out. Items keep the
// order they first went in, so every read of them is deterministic.
type Pack struct {
	Items []Entry
	Next  uint32 // the last ID handed out; an ID is never used twice in a pack
}

// Add puts st in the pack and returns its entry's ID, joining an entry it may
// be carried with (item.SameLoad): rot, poison and wear average by quantity as
// on the ground. A stack in a pack has no place, holder or map marks. Nothing
// to add returns 0, which is no entry's ID.
func (p *Pack) Add(st item.Stack) uint32 {
	if st.Qty <= 0 {
		return 0
	}
	st.X, st.Y, st.Carrier = 0, 0, 0
	st.Forbidden, st.Unclaimed, st.HaulMarked = false, false, false
	for i := range p.Items {
		e := &p.Items[i]
		if item.SameLoad(e.Stack, st) {
			e.Stack = item.MergeFood(e.Stack, st, st.Qty)
			e.Stack.Qty += st.Qty
			return e.ID
		}
	}
	p.Next++
	p.Items = append(p.Items, Entry{ID: p.Next, Stack: st})
	return p.Next
}

// Take removes up to n items from entry id and returns them; an emptied entry
// goes, the others keep their IDs and order. False: no such entry, or n is not
// positive.
func (p *Pack) Take(id uint32, n int) (item.Stack, bool) {
	i := slices.IndexFunc(p.Items, func(e Entry) bool { return e.ID == id })
	if i < 0 || n <= 0 {
		return item.Stack{}, false
	}
	e := &p.Items[i]
	out := e.Stack
	out.Qty = min(n, e.Stack.Qty)
	e.Stack.Qty -= out.Qty
	if e.Stack.Qty == 0 {
		p.Items = slices.Delete(p.Items, i, i+1)
	}
	return out, true
}

// Grams is the pack's mass: each item's mass times its quantity.
func (p Pack) Grams() int {
	total := 0
	for _, e := range p.Items {
		total += e.Stack.Kind.MassGrams() * e.Stack.Qty
	}
	return total
}

// Find is the first entry holding kind k.
func (p Pack) Find(k item.Kind) (uint32, bool) {
	for _, e := range p.Items {
		if e.Stack.Kind == k {
			return e.ID, true
		}
	}
	return 0, false
}
