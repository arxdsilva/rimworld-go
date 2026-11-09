package spatial

// IDSet is a set of small ids as bits (entity ids are sequential), for a
// membership test in O(1) where a list would be scanned. Reset clears
// only the ids added since the last Reset, so a set rebuilt each tick
// costs its members, not its range. Not saved.
type IDSet[T ~uint32 | ~uint64] struct {
	bits []uint64
	ids  []T
}

// Reset empties the set.
func (s *IDSet[T]) Reset() {
	for _, id := range s.ids {
		s.bits[id>>6] &^= 1 << (id & 63)
	}
	s.ids = s.ids[:0]
}

// Add puts id in the set.
func (s *IDSet[T]) Add(id T) {
	w := int(id >> 6)
	if w >= len(s.bits) {
		s.bits = append(s.bits, make([]uint64, w+1-len(s.bits))...)
	}
	if s.bits[w]&(1<<(id&63)) == 0 {
		s.bits[w] |= 1 << (id & 63)
		s.ids = append(s.ids, id)
	}
}

// Has reports whether id is in the set.
func (s *IDSet[T]) Has(id T) bool {
	w := int(id >> 6)
	return w < len(s.bits) && s.bits[w]&(1<<(id&63)) != 0
}
