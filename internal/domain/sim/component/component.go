// Package component provides a sparse-set component store used by
// simulation systems to associate typed data with entities.
package component

import "rimworld-go/internal/domain/sim/entity"

// Store holds one value of T per entity using a dense array for iteration
// and a sparse index for O(1) lookup. Iteration order follows insertion
// order and is therefore deterministic.
type Store[T any] struct {
	denseEntities []entity.EntityID
	denseValues   []T
	sparse        map[entity.EntityID]int
}

// NewStore returns an empty Store.
func NewStore[T any]() *Store[T] {
	return &Store[T]{sparse: make(map[entity.EntityID]int)}
}

// Set assigns v to id, overwriting any existing value.
func (s *Store[T]) Set(id entity.EntityID, v T) {
	if idx, ok := s.sparse[id]; ok {
		s.denseValues[idx] = v
		return
	}
	s.sparse[id] = len(s.denseEntities)
	s.denseEntities = append(s.denseEntities, id)
	s.denseValues = append(s.denseValues, v)
}

// Get returns the value for id and whether it was present.
func (s *Store[T]) Get(id entity.EntityID) (T, bool) {
	idx, ok := s.sparse[id]
	if !ok {
		var zero T
		return zero, false
	}
	return s.denseValues[idx], true
}

// Delete removes id from the store via swap-remove, preserving density.
func (s *Store[T]) Delete(id entity.EntityID) {
	idx, ok := s.sparse[id]
	if !ok {
		return
	}
	lastIdx := len(s.denseEntities) - 1
	lastEntity := s.denseEntities[lastIdx]

	s.denseEntities[idx] = lastEntity
	s.denseValues[idx] = s.denseValues[lastIdx]
	s.sparse[lastEntity] = idx

	s.denseEntities = s.denseEntities[:lastIdx]
	s.denseValues = s.denseValues[:lastIdx]
	delete(s.sparse, id)
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
