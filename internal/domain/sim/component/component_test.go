package component

import (
	"testing"

	"rimworld-go/internal/domain/sim/entity"
)

func TestSetGet(t *testing.T) {
	s := NewStore[int]()
	s.Set(1, 42)

	got, ok := s.Get(1)
	if !ok || got != 42 {
		t.Fatalf("Get(1) = %d, %v, want 42, true", got, ok)
	}
}

func TestGetMissingReturnsFalse(t *testing.T) {
	s := NewStore[int]()
	if _, ok := s.Get(99); ok {
		t.Fatal("Get on missing entity should return ok=false")
	}
}

func TestSetOverwritesExistingValue(t *testing.T) {
	s := NewStore[string]()
	s.Set(1, "a")
	s.Set(1, "b")

	if got, _ := s.Get(1); got != "b" {
		t.Fatalf("Get(1) = %q, want %q", got, "b")
	}
	if s.Len() != 1 {
		t.Fatalf("Len() = %d, want 1 after overwrite", s.Len())
	}
}

func TestDeleteRemovesEntity(t *testing.T) {
	s := NewStore[int]()
	s.Set(1, 10)
	s.Set(2, 20)
	s.Delete(1)

	if _, ok := s.Get(1); ok {
		t.Fatal("Get(1) should fail after Delete")
	}
	if got, ok := s.Get(2); !ok || got != 20 {
		t.Fatalf("Get(2) = %d, %v, want 20, true", got, ok)
	}
	if s.Len() != 1 {
		t.Fatalf("Len() = %d, want 1", s.Len())
	}
}

func TestAllIteratesInInsertionOrder(t *testing.T) {
	s := NewStore[int]()
	s.Set(3, 30)
	s.Set(1, 10)
	s.Set(2, 20)

	var ids []entity.EntityID
	s.All(func(id entity.EntityID, v int) {
		ids = append(ids, id)
	})

	want := []entity.EntityID{3, 1, 2}
	if len(ids) != len(want) {
		t.Fatalf("visited %d entities, want %d", len(ids), len(want))
	}
	for i, id := range want {
		if ids[i] != id {
			t.Fatalf("ids[%d] = %d, want %d", i, ids[i], id)
		}
	}
}

func TestAnyRefStopsAtFirstMatchInInsertionOrder(t *testing.T) {
	s := NewStore[int]()
	s.Set(3, 30)
	s.Set(1, 10)
	s.Set(2, 20)

	var seen []entity.EntityID
	found := s.AnyRef(func(id entity.EntityID, v *int) bool {
		seen = append(seen, id)
		return *v == 10
	})
	if !found || len(seen) != 2 || seen[0] != 3 || seen[1] != 1 {
		t.Fatalf("AnyRef = %v after %v, want true after [3 1]", found, seen)
	}
	if s.AnyRef(func(entity.EntityID, *int) bool { return false }) {
		t.Fatal("AnyRef with no match = true")
	}
}
