package inventory

import "rimworld-go/internal/domain/economy/item"

// Slot holds one spare weapon: Qty of item Kind (Qty > 1 only for a
// grenade belt). The zero value is empty. Fields are exported for the
// save (gob).
type Slot struct {
	Kind item.Kind
	Qty  int
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
