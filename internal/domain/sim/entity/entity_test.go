package entity

import "testing"

func TestNextReturnsUniqueIncreasingIDs(t *testing.T) {
	a := NewAllocator()
	first := a.Next()
	second := a.Next()
	third := a.Next()

	if first == 0 {
		t.Fatal("Next() returned reserved zero ID")
	}
	if second <= first || third <= second {
		t.Fatalf("IDs not strictly increasing: %d, %d, %d", first, second, third)
	}
}

func TestAllocatorsAreIndependent(t *testing.T) {
	a := NewAllocator()
	b := NewAllocator()

	if a.Next() != b.Next() {
		t.Fatal("two fresh allocators should produce the same first ID")
	}
}
