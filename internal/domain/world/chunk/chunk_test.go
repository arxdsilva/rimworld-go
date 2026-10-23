package chunk

import "testing"

func TestGridGeometry(t *testing.T) {
	g := New(40, 17) // 3×2 chunks, the last ones clipped
	if g.CW != 3 || g.CH != 2 || g.Count() != 6 {
		t.Fatalf("layout %+v", g)
	}
	if c := g.Of(39, 16); c != 5 {
		t.Fatalf("Of(39,16) = %d", c)
	}
	if c := g.OfIndex(16*40 + 17); c != 4 {
		t.Fatalf("OfIndex = %d", c)
	}
	x0, y0, x1, y1 := g.Bounds(5)
	if x0 != 32 || y0 != 16 || x1 != 40 || y1 != 17 {
		t.Fatalf("Bounds(5) = %d,%d,%d,%d", x0, y0, x1, y1)
	}
	if g.Left(3) != -1 || g.Left(4) != 3 || g.Up(2) != -1 || g.Up(5) != 2 {
		t.Fatal("neighbours")
	}
}

func TestSetOrder(t *testing.T) {
	s := NewSet(130)
	for _, c := range []int{129, 3, 64, 3, -1, 500} {
		s.Add(c)
	}
	var got []int
	s.Each(func(c int) { got = append(got, c) })
	if len(got) != 3 || got[0] != 3 || got[1] != 64 || got[2] != 129 || !s.Has(64) || s.Has(5) || !s.Any() {
		t.Fatalf("set = %v", got)
	}
	s.Clear()
	if s.Any() {
		t.Fatal("cleared set not empty")
	}
}
