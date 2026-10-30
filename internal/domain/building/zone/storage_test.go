package zone

import "testing"

// TestStoresByPriority: storage zones are listed under their priority in
// id order with their boxes, grow zones never, and the lists follow every
// edit (Ver) and a new map.
func TestStoresByPriority(t *testing.T) {
	m := NewMap(20, 20)
	a := m.Paint(KindStockpile, stock(), RectOf(0, 0, 3, 3), nil)
	b := m.Paint(KindDump, Zone{}, RectOf(10, 10, 12, 11), nil)
	m.Paint(KindGrow, Zone{Crop: 1}, RectOf(15, 0, 16, 1), nil)
	var s Stores
	got := s.Of(m, PriorityNormal)
	if len(got) != 2 || got[0].ID != a || got[1].ID != b || got[1].Box != (Rect{10, 10, 12, 11}) {
		t.Fatalf("Normal = %+v, want the stockpile then the dump", got)
	}
	m.Update(b, func(z *Zone) { z.Priority = PriorityCritical })
	if n, c := s.Of(m, PriorityNormal), s.Of(m, PriorityCritical); len(n) != 1 || len(c) != 1 || c[0].ID != b {
		t.Fatalf("after a priority change: Normal %+v, Critical %+v", n, c)
	}
	if s.Of(m, Priority(9)) != nil {
		t.Fatal("an invalid priority lists nothing")
	}
	if got := s.Of(NewMap(20, 20), PriorityCritical); len(got) != 0 {
		t.Fatal("a new map rebuilds the lists")
	}
}

func TestRectDist(t *testing.T) {
	r := Rect{2, 3, 5, 6}
	for _, c := range []struct{ x, y, d int }{{3, 4, 0}, {0, 4, 2}, {9, 9, 4}, {4, 0, 3}, {1, 8, 2}} {
		if got := r.Dist(c.x, c.y); got != c.d {
			t.Errorf("Dist(%d,%d) = %d, want %d", c.x, c.y, got, c.d)
		}
	}
}
