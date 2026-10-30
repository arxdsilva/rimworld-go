package inventory

import "rimworld-go/internal/domain/economy/item"

// Slot holds one spare weapon: Qty of item Kind (Qty > 1 only for a
// grenade belt), with its Quality, Stuff and Wear (CBT-3, append only: a
// stowed weapon keeps how it was made). The zero value is empty. Fields
// are exported for the save (gob).
type Slot struct {
	Kind    item.Kind
	Qty     int
	Quality uint8
	Stuff   item.Kind
	Wear    int
}

// Empty reports that nothing is carried.
func (s Slot) Empty() bool { return s.Qty <= 0 }

// Put stows qty of k; false (and nothing changes) when the slot is taken
// or qty is not positive.
func (s *Slot) Put(k item.Kind, qty int) bool {
	if !s.Empty() || qty <= 0 {
		return false
	}
	s.Kind, s.Qty = k, qty
	return true
}

// Take empties the slot and returns what it held (ok false: it was empty).
func (s *Slot) Take() (k item.Kind, qty int, ok bool) {
	if s.Empty() {
		return 0, 0, false
	}
	k, qty = s.Kind, s.Qty
	*s = Slot{}
	return k, qty, true
}

// Stow is Put for a whole stack: st's kind and size, with its quality,
// stuff and wear (CBT-3).
func (s *Slot) Stow(st item.Stack) bool {
	if !s.Put(st.Kind, st.Qty) {
		return false
	}
	s.Quality, s.Stuff, s.Wear = st.Quality, st.Stuff, st.Wear
	return true
}

// Stack is what the slot holds as a stack at (x, y) (Qty 0: empty).
func (s Slot) Stack(x, y int) item.Stack {
	if s.Empty() {
		return item.Stack{X: x, Y: y}
	}
	return item.Stack{Kind: s.Kind, Qty: s.Qty, X: x, Y: y, Quality: s.Quality, Stuff: s.Stuff, Wear: s.Wear}
}
