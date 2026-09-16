package item

import "testing"

func TestTakeRejectsOverdraw(t *testing.T) {
	if _, ok := Take(2, 3); ok {
		t.Fatal("must not take more than qty")
	}
}

func TestTakeReducesQty(t *testing.T) {
	got, ok := Take(5, 2)
	if !ok || got != 3 {
		t.Fatalf("got qty=%d ok=%v", got, ok)
	}
}

func TestAddRespectsLimit(t *testing.T) {
	got := Add(StackLimit-1, 5)
	if got != StackLimit {
		t.Fatalf("qty=%d", got)
	}
}
