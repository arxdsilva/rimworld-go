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

func TestSlotKeepsTheMake(t *testing.T) {
	t.Parallel()
	var s Slot
	st := item.Stack{Kind: item.Mace, Qty: 1, Quality: 6, Stuff: item.Silver, Wear: 3}
	if got := s.Stack(1, 2); got.Qty != 0 || got.X != 1 {
		t.Fatalf("empty slot stack %+v", got)
	}
	if !s.Stow(st) || s.Stow(st) {
		t.Fatal("one stow only")
	}
	if got := s.Stack(4, 5); got != (item.Stack{Kind: item.Mace, Qty: 1, X: 4, Y: 5, Quality: 6, Stuff: item.Silver, Wear: 3}) {
		t.Fatalf("stack %+v", got)
	}
	if _, _, ok := s.Take(); !ok || !s.Empty() || s.Quality != 0 {
		t.Fatalf("take clears the make: %+v", s)
	}
}
