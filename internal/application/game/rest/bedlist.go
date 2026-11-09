package rest

import "rimworld-go/internal/domain/sim/entity"

// BedList is the sleeping buildings of a map in building order, cached on
// the structure version and the building count (never saved). The
// claimant is not part of it: a caller reads each bed's slots live, so a
// claim or release never stales the list.
type BedList struct {
	ok    bool
	ver   uint64
	n     int
	store any // the building store it was built from (a load swaps it)
	ids   []entity.EntityID
	// Builds counts rebuilds (tests).
	Builds int
}

// Get is the cached list for (ver, n, store); ok is false when it must be
// rebuilt with Set first.
func (c *BedList) Get(ver uint64, n int, store any) ([]entity.EntityID, bool) {
	return c.ids, c.ok && c.ver == ver && c.n == n && c.store == store
}

// Reuse is the old backing array, emptied, for a rebuild.
func (c *BedList) Reuse() []entity.EntityID { return c.ids[:0] }

// Seal stores ids as the list built at (ver, n, store) and returns it.
func (c *BedList) Seal(ver uint64, n int, store any, ids []entity.EntityID) []entity.EntityID {
	c.ok, c.ver, c.n, c.store, c.ids = true, ver, n, store, ids
	c.Builds++
	return ids
}
