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

func TestBlockedRoundTripsThroughRestore(t *testing.T) {
	g := New(70, 3) // 210 tiles: spans four words, the last partial
	want := []int{0, 1, 63, 64, 65, 127, 128, 200, 209}
	for _, i := range want {
		g.SetWalkable(i%70, i/70, false)
	}
	got := g.Blocked()
	if len(got) != len(want) {
		t.Fatalf("Blocked() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("Blocked() = %v, want %v", got, want)
		}
	}
	r := Restore(70, 3, append(got, -1, 210, 99999)) // out of range: ignored
	for y := 0; y < 3; y++ {
		for x := 0; x < 70; x++ {
			if r.Walkable(x, y) != g.Walkable(x, y) {
				t.Fatalf("restored (%d,%d) differs", x, y)
			}
		}
	}
	if len(r.Blocked()) != len(want) {
		t.Fatalf("restored Blocked() = %v", r.Blocked())
	}
}

func TestWalkableNegativeCoordinates(t *testing.T) {
	g := New(3, 3)
	if g.Walkable(-1, 1) || g.Walkable(1, -1) || g.Walkable(3, 0) || g.Walkable(0, 3) {
		t.Fatal("out-of-bounds tile walkable")
	}
}

func TestEmptyBlocked(t *testing.T) {
	if b := New(5, 5).Blocked(); len(b) != 0 || b == nil {
		t.Fatalf("Blocked() = %#v, want empty non-nil", b)
	}
}

func BenchmarkWalkable(b *testing.B) {
	g := New(500, 500)
	for i := 0; i < 500; i += 3 {
		g.SetWalkable(i, i, false)
	}
	n := 0
	for i := 0; i < b.N; i++ {
		if g.Walkable(i%500, (i/500)%500) {
			n++
		}
	}
	_ = n
}
