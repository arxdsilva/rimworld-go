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
