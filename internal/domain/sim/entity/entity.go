package entity

// EntityID is a stable identifier for a simulation entity.
type EntityID uint64

// Allocator hands out monotonically increasing EntityIDs. IDs are never
// reused within a running world.
type Allocator struct {
	next EntityID
}

// NewAllocator returns an Allocator whose first allocated ID is 1.
// 0 is reserved to mean "no entity".
func NewAllocator() *Allocator {
	return &Allocator{next: 1}
}

// Peek is the ID Next would return (saved games).
func (a *Allocator) Peek() EntityID { return a.next }

// Resume continues allocating from next (loading a saved game).
func Resume(next EntityID) *Allocator {
	if next < 1 {
		next = 1
	}
	return &Allocator{next: next}
}

// Next allocates and returns the next unique EntityID.
func (a *Allocator) Next() EntityID {
	id := a.next
	a.next++
	return id
}
