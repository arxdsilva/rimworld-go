package memo

import "testing"

func TestListShare(t *testing.T) {
	var m List[int]
	src := []int{1, 2, 3}
	at := func(i int) int { return src[i] }
	if m.Share(true, 0, at) != nil {
		t.Fatal("an empty list is nil")
	}
	a := m.Share(true, 3, at)
	if b := m.Share(true, 3, at); &a[0] != &b[0] {
		t.Fatal("an unchanged list is a new slice")
	}
	src[1] = 9
	if b := m.Share(true, 3, at); &a[0] == &b[0] || b[1] != 9 || a[1] != 2 {
		t.Fatalf("a changed list reuses or writes the old slice: %v %v", a, b)
	}
	var off List[int]
	x, y := off.Share(false, 3, at), off.Share(false, 3, at)
	if &x[0] == &y[0] {
		t.Fatal("memo off: every call is a fresh copy")
	}
}
