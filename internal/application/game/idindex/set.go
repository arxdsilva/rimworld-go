package idindex

import "rimworld-go/internal/domain/sim/entity"

// Set is an insertion-ordered set of ids built from one store.
type Set struct {
	IDs   []entity.EntityID // in insertion order
	in    map[entity.EntityID]struct{}
	src   any
	built bool
}

// From reports whether s was built from src (and not reset since).
func (s *Set) From(src any) bool { return s.built && s.src == src }

// Reset empties s for a build from src.
func (s *Set) Reset(src any) {
	s.IDs, s.src, s.built = s.IDs[:0], src, true
	if s.in == nil {
		s.in = map[entity.EntityID]struct{}{}
	} else {
		clear(s.in)
	}
}

// Add appends id unless s holds it.
func (s *Set) Add(id entity.EntityID) {
	if _, ok := s.in[id]; !ok {
		s.in[id] = struct{}{}
		s.IDs = append(s.IDs, id)
	}
}

// Keep drops every id ok rejects (in place, order kept).
func (s *Set) Keep(ok func(entity.EntityID) bool) {
	kept := s.IDs[:0]
	for _, id := range s.IDs {
		if ok(id) {
			kept = append(kept, id)
		} else {
			delete(s.in, id)
		}
	}
	clear(s.IDs[len(kept):])
	s.IDs = kept
}
