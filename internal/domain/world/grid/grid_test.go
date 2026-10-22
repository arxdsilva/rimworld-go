package grid

import "testing"

func TestInBounds(t *testing.T) {
	g := New(4, 3)

	cases := []struct {
		x, y int
		want bool
	}{
		{0, 0, true},
		{3, 2, true},
		{4, 0, false},
		{0, 3, false},
		{-1, 0, false},
	}
	for _, c := range cases {
		if got := g.InBounds(c.x, c.y); got != c.want {
			t.Errorf("InBounds(%d, %d) = %v, want %v", c.x, c.y, got, c.want)
		}
	}
}

func TestWalkableDefaultsTrueWithinBounds(t *testing.T) {
	g := New(2, 2)
	if !g.Walkable(0, 0) {
		t.Fatal("tiles should be walkable by default")
	}
}

func TestWalkableFalseOutOfBounds(t *testing.T) {
	g := New(2, 2)
	if g.Walkable(5, 5) {
		t.Fatal("out-of-bounds tiles must never be walkable")
	}
}

func TestSetWalkableBlocksAndUnblocks(t *testing.T) {
	g := New(2, 2)
	g.SetWalkable(1, 1, false)
	if g.Walkable(1, 1) {
		t.Fatal("tile should be blocked after SetWalkable(false)")
	}
	g.SetWalkable(1, 1, true)
	if !g.Walkable(1, 1) {
		t.Fatal("tile should be walkable again after SetWalkable(true)")
	}
}

func TestNewPanicsOnNonPositiveSize(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("New should panic for non-positive dimensions")
		}
	}()
	New(0, 5)
}

func TestVersionChangesOnlyWithWalkability(t *testing.T) {
	g := New(3, 3)
	r0 := g.Version()
	g.SetWalkable(1, 1, true) // already walkable
	if g.Version() != r0 {
		t.Fatal("a no-op SetWalkable changed Version")
	}
	g.SetWalkable(1, 1, false)
	r1 := g.Version()
	if r1 == r0 {
		t.Fatal("blocking a tile did not change Version")
	}
	g.SetWalkable(1, 1, false)
	if g.Version() != r1 {
		t.Fatal("re-blocking a blocked tile changed Version")
	}
	g.SetWalkable(1, 1, true)
	if g.Version() == r1 {
		t.Fatal("unblocking a tile did not change Version")
	}
	g.SetWalkable(9, 9, false) // out of bounds
	if g.Version() != r1+1 {
		t.Fatal("an out-of-bounds SetWalkable changed Version")
	}
}
