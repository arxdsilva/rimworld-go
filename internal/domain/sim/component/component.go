// Package component provides a sparse-set component store used by
// simulation systems to associate typed data with entities.
package component

import "rimworld-go/internal/domain/sim/entity"

// The sparse index is paged: entity IDs are small sequential integers, so
// an ID's dense position lives in a fixed-size page found by shifting,
// not in a hash map (lookups are the simulation's hottest operation).
// A page is allocated on its first ID and freed when its last one is
// deleted, so memory follows the live entities. IDs at or past farID
// (never handed out by entity.Allocator in practice) use a map.
const (
	pageShift = 8
	pageSize  = 1 << pageShift
	farID     = 1 << 26
)

// page holds dense index + 1 for pageSize consecutive IDs (0 = absent).
type page struct {
	n   int32 // IDs present
	idx [pageSize]int32
}

// Store holds one value of T per entity using a dense array for iteration
// and a sparse index for O(1) lookup. Iteration order follows insertion
// order and is therefore deterministic.
type Store[T any] struct {
	denseEntities []entity.EntityID
	denseValues   []T
	pages         []*page
	far           map[entity.EntityID]int
}

// NewStore returns an empty Store.
func NewStore[T any]() *Store[T] {
	return &Store[T]{}
}

// index is id's dense position.
func (s *Store[T]) index(id entity.EntityID) (int, bool) {
	if p := uint64(id >> pageShift); p < uint64(len(s.pages)) {
		if pg := s.pages[p]; pg != nil {
			if v := pg.idx[id&(pageSize-1)]; v != 0 {
				return int(v - 1), true
			}
		}
		return 0, false
	}
	return s.farIndex(id)
}

func (s *Store[T]) farIndex(id entity.EntityID) (int, bool) {
	if id < farID {
		return 0, false
	}
	idx, ok := s.far[id]
	return idx, ok
}

// setIndex records id at dense position idx.
func (s *Store[T]) setIndex(id entity.EntityID, idx int) {
	if id >= farID {
		if s.far == nil {
			s.far = make(map[entity.EntityID]int)
		}
		s.far[id] = idx
		return
	}
	p := int(id >> pageShift)
	if p >= len(s.pages) {
		s.pages = append(s.pages, make([]*page, p+1-len(s.pages))...)
	}
	pg := s.pages[p]
	if pg == nil {
		pg = new(page)
		s.pages[p] = pg
	}
	v := &pg.idx[id&(pageSize-1)]
	if *v == 0 {
		pg.n++
	}
	*v = int32(idx + 1)
}

// dropIndex forgets id (its page goes once empty).
func (s *Store[T]) dropIndex(id entity.EntityID) {
	if id >= farID {
		delete(s.far, id)
		return
	}
	p := uint64(id >> pageShift)
	if p >= uint64(len(s.pages)) || s.pages[p] == nil {
		return
	}
	pg := s.pages[p]
	if v := &pg.idx[id&(pageSize-1)]; *v != 0 {
		*v = 0
		if pg.n--; pg.n == 0 {
			s.pages[p] = nil
		}
	}
}

// Set assigns v to id, overwriting any existing value.
func (s *Store[T]) Set(id entity.EntityID, v T) {
	if idx, ok := s.index(id); ok {
		s.denseValues[idx] = v
		return
	}
	s.setIndex(id, len(s.denseEntities))
	s.denseEntities = append(s.denseEntities, id)
	s.denseValues = append(s.denseValues, v)
}

// Get returns the value for id and whether it was present.
func (s *Store[T]) Get(id entity.EntityID) (T, bool) {
	idx, ok := s.index(id)
	if !ok {
		var zero T
		return zero, false
	}
	return s.denseValues[idx], true
}

// Has reports whether id has a value (no copy of it is made).
func (s *Store[T]) Has(id entity.EntityID) bool {
	_, ok := s.index(id)
	return ok
}

// Ref returns a pointer to id's value for reading without copying it, or
// nil when id has none. It is valid only until the store next changes
// (a Set of a new ID or a Delete may move values); never write through it.
func (s *Store[T]) Ref(id entity.EntityID) *T {
	idx, ok := s.index(id)
	if !ok {
		return nil
	}
	return &s.denseValues[idx]
}

// Delete removes id from the store via swap-remove, preserving density.
func (s *Store[T]) Delete(id entity.EntityID) {
	idx, ok := s.index(id)
	if !ok {
		return
	}
	lastIdx := len(s.denseEntities) - 1
	lastEntity := s.denseEntities[lastIdx]

	s.denseEntities[idx] = lastEntity
	s.denseValues[idx] = s.denseValues[lastIdx]
	s.setIndex(lastEntity, idx)

	var zero T
	s.denseValues[lastIdx] = zero // release what the value references
	s.denseEntities = s.denseEntities[:lastIdx]
	s.denseValues = s.denseValues[:lastIdx]
	s.dropIndex(id)
}

// Len returns the number of entities currently stored.
func (s *Store[T]) Len() int {
	return len(s.denseEntities)
}

// All calls fn for every stored (id, value) pair in insertion order.
// Do not mutate the store from within fn.
func (s *Store[T]) All(fn func(entity.EntityID, T)) {
	for i, id := range s.denseEntities {
		fn(id, s.denseValues[i])
	}
}
