package zone

import (
	"reflect"
	"testing"
)

func stock() Zone { return Zone{Food: true, Wood: true, Stone: true} }

func cells(m *Map, id ID) int { z, _ := m.Zone(id); return z.Cells }

// consistent: the tile grid, the per-zone cell lists and counts agree, and
// no two touching zones are compatible (they would have merged).
func consistent(t *testing.T, m *Map) {
	t.Helper()
	total := 0
	for _, z := range m.Zones() {
		cs := m.Cells(z.ID)
		if len(cs) != z.Cells || z.Cells == 0 {
			t.Fatalf("zone %d: %d cells listed, count %d", z.ID, len(cs), z.Cells)
		}
		for _, c := range cs {
			if id, _ := m.At(c[0], c[1]); id != z.ID {
				t.Fatalf("zone %d lists %v owned by %d", z.ID, c, id)
			}
		}
		total += z.Cells
	}
	n := 0
	for y := 0; y < m.h; y++ {
		for x := 0; x < m.w; x++ {
			id, ok := m.At(x, y)
			if !ok {
				continue
			}
			n++
			a, _ := m.Zone(id)
			for _, d := range [2][2]int{{1, 0}, {0, 1}} {
				if o, ok := m.At(x+d[0], y+d[1]); ok && o != id {
					if b, _ := m.Zone(o); compatible(a, b) {
						t.Fatalf("compatible zones %d and %d touch at %d,%d", id, o, x, y)
					}
				}
			}
		}
	}
	if n != total {
		t.Fatalf("grid has %d zoned tiles, zones %d", n, total)
	}
}

func TestPaintSkipsTakenTilesOfAnyKind(t *testing.T) {
	m := NewMap(20, 20)
	s := m.Paint(KindStockpile, stock(), RectOf(0, 0, 4, 4), nil)
	g := m.Paint(KindGrow, Zone{Crop: 3}, RectOf(3, 3, 8, 8), nil)
	if s == 0 || g == 0 || s == g {
		t.Fatalf("ids %d %d", s, g)
	}
	if cells(m, s) != 25 || cells(m, g) != 36-4 {
		t.Fatalf("stockpile %d grow %d cells", cells(m, s), cells(m, g))
	}
	if id, _ := m.At(4, 4); id != s {
		t.Fatal("the old zone keeps its tiles")
	}
	if m.Paint(KindPen, Zone{}, RectOf(0, 0, 4, 4), nil) != 0 {
		t.Fatal("painting only taken tiles claims nothing")
	}
	consistent(t, m)
}

func TestPaintRespectsOkAndBounds(t *testing.T) {
	m := NewMap(5, 5)
	id := m.Paint(KindGrow, Zone{Crop: 1}, RectOf(-3, -3, 9, 9), func(x, y int) bool { return x != 2 || y != 2 })
	if cells(m, id) != 24 {
		t.Fatalf("cells %d", cells(m, id))
	}
	if _, ok := m.At(2, 2); ok {
		t.Fatal("ok refused the tile")
	}
	if m.Paint(KindGrow, Zone{}, RectOf(7, 7, 9, 9), nil) != 0 || m.Paint(0, Zone{}, RectOf(0, 0, 0, 0), nil) != 0 {
		t.Fatal("off-map or kindless paint claims nothing")
	}
}

func TestPaintMergesAdjacentSameKind(t *testing.T) {
	m := NewMap(20, 20)
	a := m.Paint(KindStockpile, Zone{Food: true}, RectOf(0, 0, 2, 2), nil)
	b := m.Paint(KindStockpile, Zone{Wood: true}, RectOf(3, 0, 5, 2), nil)
	if a != b || len(m.Zones()) != 1 || cells(m, a) != 18 {
		t.Fatalf("a %d b %d zones %+v", a, b, m.Zones())
	}
	if z, _ := m.Zone(a); !z.Food || z.Wood {
		t.Fatalf("the surviving zone keeps its settings: %+v", z)
	}
	// diagonal contact does not merge
	c := m.Paint(KindStockpile, stock(), RectOf(6, 3, 7, 4), nil)
	if c == a {
		t.Fatal("diagonal neighbours stay separate")
	}
	consistent(t, m)
}

func TestPaintBridgeMergesIntoLowestID(t *testing.T) {
	m := NewMap(20, 20)
	a := m.Paint(KindPen, Zone{}, RectOf(0, 0, 2, 2), nil)
	b := m.Paint(KindPen, Zone{}, RectOf(6, 0, 8, 2), nil)
	m.Paint(KindStockpile, stock(), RectOf(10, 10, 11, 11), nil)
	if a == b {
		t.Fatal("apart: two zones")
	}
	got := m.Paint(KindPen, Zone{}, RectOf(3, 1, 5, 1), nil)
	if got != a || len(m.Zones()) != 2 || cells(m, a) != 21 {
		t.Fatalf("bridge -> %d (a %d), zones %+v", got, a, m.Zones())
	}
	if _, ok := m.Zone(b); ok {
		t.Fatal("the higher id is merged away")
	}
	consistent(t, m)
}

// A drag split by refused tiles bridges its pieces through an old zone.
func TestPaintPiecesJoinThroughTouchedZone(t *testing.T) {
	m := NewMap(10, 10)
	a := m.Paint(KindStockpile, stock(), RectOf(0, 0, 9, 0), nil)
	got := m.Paint(KindStockpile, stock(), RectOf(0, 1, 9, 1), func(x, _ int) bool { return x != 5 })
	if got != a || len(m.Zones()) != 1 {
		t.Fatalf("zones %+v", m.Zones())
	}
	// without a zone to join, the pieces are separate zones
	p := m.Paint(KindPen, Zone{}, RectOf(0, 5, 9, 5), func(x, _ int) bool { return x != 5 })
	if len(m.Zones()) != 3 || cells(m, p) != 5 {
		t.Fatalf("zones %+v", m.Zones())
	}
	consistent(t, m)
}

func TestGrowZonesMergeOnlyWithSameCrop(t *testing.T) {
	m := NewMap(20, 20)
	a := m.Paint(KindGrow, Zone{Crop: 1}, RectOf(0, 0, 2, 2), nil)
	b := m.Paint(KindGrow, Zone{Crop: 2}, RectOf(3, 0, 5, 2), nil)
	c := m.Paint(KindGrow, Zone{Crop: 1}, RectOf(0, 3, 2, 4), nil)
	if a == b || c != a {
		t.Fatalf("a %d b %d c %d", a, b, c)
	}
	s := m.Paint(KindStockpile, stock(), RectOf(6, 0, 7, 2), nil)
	if s == b {
		t.Fatal("different kinds never merge")
	}
	consistent(t, m)
	// switching b to crop 1 joins it to a (the lower id)
	m.Set(b, Zone{Crop: 1})
	if _, ok := m.Zone(b); ok || cells(m, a) != 24 {
		t.Fatalf("zones %+v", m.Zones())
	}
	consistent(t, m)
}

func TestClearSplitsAndDeletes(t *testing.T) {
	m := NewMap(20, 20)
	a := m.Paint(KindStockpile, Zone{Food: true}, RectOf(0, 0, 8, 2), nil)
	m.Clear(RectOf(3, 0, 4, 2)) // cut in two
	zs := m.Zones()
	if len(zs) != 2 || zs[0].ID != a || zs[0].Cells != 9 || zs[1].Cells != 12 {
		t.Fatalf("zones %+v", zs)
	}
	if !zs[1].Food || zs[1].Kind != KindStockpile {
		t.Fatal("the split piece copies the settings")
	}
	if id, _ := m.At(0, 0); id != a {
		t.Fatal("the piece with the first tile keeps the id")
	}
	consistent(t, m)
	// a three-way split
	p := m.Paint(KindPen, Zone{}, RectOf(0, 10, 8, 10), nil)
	m.Clear(RectOf(2, 10, 2, 10))
	m.Clear(RectOf(5, 10, 5, 10))
	if len(m.Zones()) != 5 || cells(m, p) != 2 {
		t.Fatalf("zones %+v", m.Zones())
	}
	consistent(t, m)
	m.Clear(RectOf(0, 0, 19, 19))
	if len(m.Zones()) != 0 {
		t.Fatal("cleared zones are deleted")
	}
	m.Clear(RectOf(30, 30, 40, 40)) // off the map: nothing
}

func TestIDsNeverReused(t *testing.T) {
	m := NewMap(10, 10)
	a := m.Paint(KindPen, Zone{}, RectOf(0, 0, 1, 1), nil)
	m.Clear(RectOf(0, 0, 1, 1))
	b := m.Paint(KindPen, Zone{}, RectOf(0, 0, 1, 1), nil)
	if b <= a {
		t.Fatalf("id %d reused after %d", b, a)
	}
	// and not after a save either
	n := Import(m.Export())
	n.Clear(RectOf(0, 0, 9, 9))
	if c := n.Paint(KindPen, Zone{}, RectOf(5, 5, 5, 5), nil); c <= b {
		t.Fatalf("id %d after import reuses %d", c, b)
	}
}

func TestExportImportRoundTrip(t *testing.T) {
	m := NewMap(12, 9)
	m.Paint(KindStockpile, Zone{Wood: true}, RectOf(0, 0, 3, 3), nil)
	m.Paint(KindGrow, Zone{Crop: 4}, RectOf(5, 0, 8, 4), nil)
	m.Paint(KindPen, Zone{}, RectOf(0, 6, 11, 8), nil)
	m.Clear(RectOf(6, 6, 6, 8))
	n := Import(m.Export())
	if !reflect.DeepEqual(m.Export(), n.Export()) || !reflect.DeepEqual(m.Zones(), n.Zones()) {
		t.Fatal("round trip differs")
	}
	for _, z := range m.Zones() {
		if !reflect.DeepEqual(m.Cells(z.ID), n.Cells(z.ID)) {
			t.Fatalf("zone %d cells differ", z.ID)
		}
		a, _ := m.Bounds(z.ID)
		b, _ := n.Bounds(z.ID)
		if a != b {
			t.Fatalf("bounds %v vs %v", a, b)
		}
	}
	consistent(t, n)
}

func TestBoundsAndEach(t *testing.T) {
	m := NewMap(10, 10)
	id := m.Paint(KindGrow, Zone{}, RectOf(2, 3, 5, 6), func(x, y int) bool { return x+y != 8 || x == 2 })
	b, ok := m.Bounds(id)
	if !ok || b != (Rect{2, 3, 5, 6}) {
		t.Fatalf("bounds %v", b)
	}
	m.Clear(RectOf(4, 0, 9, 9))
	if b, _ = m.Bounds(id); b != (Rect{2, 3, 3, 6}) {
		t.Fatalf("bounds after clear %v", b)
	}
	if _, ok := m.Bounds(99); ok {
		t.Fatal("unknown zone")
	}
	n := 0
	m.EachCell(id, func(x, y int) bool { n++; return n < 3 })
	visits := 0
	m.Each(func(Zone) bool { visits++; return false })
	if n != 3 || visits != 1 {
		t.Fatalf("each stops early: %d %d", n, visits)
	}
}

func TestDeterministic(t *testing.T) {
	run := func() Saved {
		m := NewMap(30, 30)
		for i := 0; i < 40; i++ {
			x, y := (i*7)%27, (i*11)%27
			kind := Kind(i%3 + 1)
			m.Paint(kind, Zone{Crop: uint8(i % 2)}, RectOf(x, y, x+3, y+2), func(x, y int) bool { return (x*y)%7 != 3 })
			if i%5 == 4 {
				m.Clear(RectOf(y, x, y+1, x+4))
			}
		}
		consistent(t, m)
		return m.Export()
	}
	if !reflect.DeepEqual(run(), run()) {
		t.Fatal("same paints, different maps")
	}
}
