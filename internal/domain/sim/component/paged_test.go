package component

import (
	"testing"

	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/pkg/randx"
)

// mapStore is the map-indexed store the paged index replaced: the model
// a Store must match operation for operation (values and dense order).
type mapStore struct {
	ids  []entity.EntityID
	vals []int
	idx  map[entity.EntityID]int
}

func (m *mapStore) set(id entity.EntityID, v int) {
	if i, ok := m.idx[id]; ok {
		m.vals[i] = v
		return
	}
	m.idx[id] = len(m.ids)
	m.ids = append(m.ids, id)
	m.vals = append(m.vals, v)
}

func (m *mapStore) del(id entity.EntityID) {
	i, ok := m.idx[id]
	if !ok {
		return
	}
	last := len(m.ids) - 1
	m.ids[i], m.vals[i] = m.ids[last], m.vals[last]
	m.idx[m.ids[i]] = i
	m.ids, m.vals = m.ids[:last], m.vals[:last]
	delete(m.idx, id)
}

// TestPagedMatchesMap: random sets, gets and deletes over near, sparse
// and far IDs leave the paged store identical to the map model.
func TestPagedMatchesMap(t *testing.T) {
	r := randx.New(9)
	s := NewStore[int]()
	m := &mapStore{idx: map[entity.EntityID]int{}}
	pick := func() entity.EntityID {
		switch r.IntN(4) {
		case 0:
			return entity.EntityID(r.IntN(600)) // a few pages, 0 included
		case 1:
			return entity.EntityID(r.IntN(200_000))
		case 2:
			return entity.EntityID(farID - 300 + r.IntN(600)) // both sides of farID
		default:
			return entity.EntityID(1<<40 + r.IntN(50))
		}
	}
	for step := 0; step < 200_000; step++ {
		id := pick()
		switch r.IntN(3) {
		case 0, 1:
			v := r.IntN(1000)
			s.Set(id, v)
			m.set(id, v)
		default:
			s.Delete(id)
			m.del(id)
		}
		id2 := pick()
		got, ok := s.Get(id2)
		i, want := m.idx[id2]
		if ok != want || ok && got != m.vals[i] {
			t.Fatalf("step %d: Get(%d) = %d %v, model %v", step, id2, got, ok, want)
		}
		if ref := s.Ref(id2); s.Has(id2) != want || (ref != nil) != want || ref != nil && *ref != got {
			t.Fatalf("step %d: Has/Ref(%d) disagree with Get", step, id2)
		}
	}
	if s.Len() != len(m.ids) {
		t.Fatalf("Len %d, model %d", s.Len(), len(m.ids))
	}
	k := 0
	s.All(func(id entity.EntityID, v int) {
		if id != m.ids[k] || v != m.vals[k] {
			t.Fatalf("dense %d: (%d, %d), model (%d, %d)", k, id, v, m.ids[k], m.vals[k])
		}
		k++
	})
}

// TestPagesFreed: a page goes when its last ID is deleted, so the index
// follows the live entities.
func TestPagesFreed(t *testing.T) {
	s := NewStore[int]()
	for id := entity.EntityID(1000); id < 1000+3*pageSize; id++ {
		s.Set(id, int(id))
	}
	for id := entity.EntityID(1000); id < 1000+3*pageSize; id++ {
		s.Delete(id)
	}
	for p, pg := range s.pages {
		if pg != nil {
			t.Fatalf("page %d kept after its IDs were deleted", p)
		}
	}
	s.Delete(5)       // never set: no page
	s.Delete(1 << 40) // never set: far
	if s.Len() != 0 {
		t.Fatalf("Len = %d", s.Len())
	}
}
