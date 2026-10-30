package inventory

import (
	"testing"

	"rimworld-go/internal/domain/economy/item"
)

func TestSlotHoldsOneSpare(t *testing.T) {
	t.Parallel()
	var s Slot
	if !s.Empty() {
		t.Fatal("the zero slot is empty")
	}
	if _, _, ok := s.Take(); ok {
		t.Fatal("nothing to take from an empty slot")
	}
	if s.Put(item.Knife, 0) || !s.Empty() {
		t.Fatal("nothing stows no weapon")
	}
	if !s.Put(item.Knife, 1) || s.Empty() {
		t.Fatal("a knife stows")
	}
	if s.Put(item.Club, 1) || s.Kind != item.Knife {
		t.Fatal("one spare only")
	}
	if k, n, ok := s.Take(); !ok || k != item.Knife || n != 1 || !s.Empty() {
		t.Fatalf("take: %v %d %v %+v", k, n, ok, s)
	}
}
