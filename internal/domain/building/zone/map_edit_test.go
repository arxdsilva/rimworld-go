package zone

import (
	"reflect"
	"testing"
)

// PLAY-020 zone management: Remove, AddCells, RemoveCells, Update and the
// merge rule for zones that carry the new settings.

// rectCells lists the tiles of r row-major, as AddCells takes them.
func rectCells(x1, y1, x2, y2 int) [][2]int {
	var out [][2]int
	for y := y1; y <= y2; y++ {
		for x := x1; x <= x2; x++ {
			out = append(out, [2]int{x, y})
		}
	}
	return out
}

func owner(m *Map, x, y int) ID { id, _ := m.At(x, y); return id }

func TestRemoveDeletesTheZoneAndFreesItsTiles(t *testing.T) {
	m := NewMap(20, 20)
	a := m.Paint(KindGrow, Zone{Crop: 1}, RectOf(0, 0, 3, 3), nil)
	b := m.Paint(KindStockpile, stock(), RectOf(10, 10, 12, 12), nil)
	m.Remove(a)
	if _, ok := m.Zone(a); ok || len(m.Zones()) != 1 || owner(m, 1, 1) != 0 {
		t.Fatalf("zone %d not removed: %+v", a, m.Zones())
	}
	if cells(m, b) != 9 || owner(m, 11, 11) != b {
		t.Fatal("other zones keep their tiles")
	}
	m.Remove(a) // already gone
	m.Remove(99)
	if len(m.Zones()) != 1 {
		t.Fatal("removing an unknown zone changes nothing")
	}
	if c := m.Paint(KindGrow, Zone{Crop: 1}, RectOf(0, 0, 0, 0), nil); c <= b {
		t.Fatalf("id %d reused", c)
	}
	consistent(t, m)
}

func TestAddCellsJoinsTheZoneKeepingItsSettings(t *testing.T) {
	m := NewMap(20, 20)
	id := m.Paint(KindGrow, Zone{Crop: 3, NoSow: true, NoCut: true, Hidden: true, Name: "Corn"}, RectOf(2, 2, 4, 4), nil)
	m.AddCells(id, rectCells(5, 2, 6, 4)) // a block against its east side
	z, _ := m.Zone(id)
	if len(m.Zones()) != 1 || z.Cells != 15 || owner(m, 6, 3) != id {
		t.Fatalf("zones %+v", m.Zones())
	}
	if z.Crop != 3 || !z.NoSow || !z.NoCut || !z.Hidden || z.Name != "Corn" {
		t.Fatalf("settings lost: %+v", z)
	}
	if b, _ := m.Bounds(id); b != (Rect{2, 2, 6, 4}) {
		t.Fatalf("bounds %v", b)
	}
	consistent(t, m)
}

func TestAddCellsDetachedBlockBecomesANewZoneWithTheSameSettings(t *testing.T) {
	m := NewMap(20, 20)
	id := m.Paint(KindGrow, Zone{Crop: 3, NoSow: true, Name: "Corn"}, RectOf(2, 2, 4, 4), nil)
	// one block touching the zone, two apart from it and from each other
	cs := append(rectCells(5, 2, 5, 2), rectCells(10, 10, 11, 11)...)
	cs = append(cs, rectCells(15, 3, 15, 4)...)
	m.AddCells(id, cs)
	zs := m.Zones()
	if len(zs) != 3 {
		t.Fatalf("zones %+v", zs)
	}
	if zs[0].ID != id || zs[0].Cells != 10 {
		t.Fatalf("the touching cell joins the zone: %+v", zs[0])
	}
	for _, z := range zs[1:] {
		if z.Crop != 3 || !z.NoSow || z.Name != "Corn" || z.Kind != KindGrow {
			t.Fatalf("a detached block copies the settings: %+v", z)
		}
	}
	// blocks are numbered by their first tile, row-major: (15,3) before (10,10)
	if zs[1].Cells != 2 || zs[2].Cells != 4 || zs[1].ID <= id {
		t.Fatalf("detached blocks in row-major order: %+v", zs)
	}
	if owner(m, 15, 4) != zs[1].ID || owner(m, 10, 10) != zs[2].ID {
		t.Fatal("tiles of the wrong zone")
	}
	consistent(t, m)
}

func TestAddCellsSkipsZonedAndOffMapTiles(t *testing.T) {
	m := NewMap(10, 10)
	a := m.Paint(KindGrow, Zone{Crop: 1}, RectOf(0, 0, 2, 2), nil)
	other := m.Paint(KindStockpile, stock(), RectOf(3, 0, 4, 2), nil)
	cs := append(rectCells(2, 2, 6, 3), [2]int{-1, 0}, [2]int{0, -1}, [2]int{10, 5}, [2]int{5, 10}, [2]int{2, 3}) // 2,3 twice
	m.AddCells(a, cs)
	if owner(m, 3, 2) != other || cells(m, other) != 6 {
		t.Fatal("tiles of another zone are skipped")
	}
	if cells(m, a) != 9+7 { // the 7 free tiles of the rect, joined through (2,3)
		t.Fatalf("zone a has %d cells", cells(m, a))
	}
	consistent(t, m)
	// nothing new and an unknown zone are no-ops
	before := m.Export()
	m.AddCells(a, rectCells(0, 0, 2, 2))
	m.AddCells(99, rectCells(8, 8, 9, 9))
	m.AddCells(a, nil)
	if !reflect.DeepEqual(before, m.Export()) {
		t.Fatal("no-ops changed the map")
	}
}

func TestAddCellsTakenTilesStayWithTheirZone(t *testing.T) {
	m := NewMap(12, 6)
	a := m.Paint(KindGrow, Zone{Crop: 1}, RectOf(0, 0, 1, 1), nil)
	b := m.Paint(KindPen, Zone{}, RectOf(2, 0, 3, 0), nil)
	m.AddCells(a, rectCells(2, 0, 5, 1)) // (2,0) and (3,0) are the pen's
	if cells(m, a) != 4+6 || cells(m, b) != 2 || owner(m, 4, 0) != a || owner(m, 2, 0) != b {
		t.Fatalf("zones %+v", m.Zones())
	}
	consistent(t, m)
}

// Expanding a zone against a compatible zone merges them, and the zone
// that was expanded survives with its own settings.
func TestAddCellsMergesTouchingCompatibleZonesIntoTheExpandedOne(t *testing.T) {
	m := NewMap(20, 20)
	a := m.Paint(KindGrow, Zone{Crop: 1, Name: "low"}, RectOf(0, 0, 2, 2), nil)
	b := m.Paint(KindGrow, Zone{Crop: 1, NoSow: true, Name: "high"}, RectOf(5, 0, 7, 2), nil)
	c := m.Paint(KindGrow, Zone{Crop: 2}, RectOf(0, 5, 2, 7), nil)
	if a == b {
		t.Fatal("apart: two zones")
	}
	m.AddCells(b, rectCells(3, 1, 4, 1)) // bridges a and b
	z, ok := m.Zone(b)
	if _, alive := m.Zone(a); alive || !ok || z.Cells != 9+9+2 || z.Name != "high" || !z.NoSow {
		t.Fatalf("the expanded zone absorbs the touched one: %+v", m.Zones())
	}
	if owner(m, 0, 0) != b {
		t.Fatal("the lower id merges away")
	}
	// a different crop is never merged
	m.AddCells(b, rectCells(0, 3, 2, 4))
	if _, ok := m.Zone(c); !ok || cells(m, c) != 9 || owner(m, 0, 4) != b {
		t.Fatalf("zones %+v", m.Zones())
	}
	consistent(t, m)
}

func TestAddCellsDetachedBlockTouchingAnotherZoneJoinsIt(t *testing.T) {
	m := NewMap(20, 20)
	a := m.Paint(KindGrow, Zone{Crop: 1}, RectOf(0, 0, 2, 2), nil)
	b := m.Paint(KindGrow, Zone{Crop: 1, Name: "far"}, RectOf(10, 0, 12, 2), nil)
	m.AddCells(a, rectCells(10, 3, 12, 3)) // under b, apart from a
	if cells(m, b) != 12 || cells(m, a) != 9 || len(m.Zones()) != 2 {
		t.Fatalf("the block joins the zone it touches: %+v", m.Zones())
	}
	if z, _ := m.Zone(b); z.Name != "far" {
		t.Fatal("it takes that zone's settings")
	}
	consistent(t, m)
}

func TestAddCellsBlocksBridgedThroughAnAbsorbedZone(t *testing.T) {
	m := NewMap(20, 20)
	a := m.Paint(KindGrow, Zone{Crop: 1}, RectOf(0, 5, 1, 6), nil)
	q := m.Paint(KindGrow, Zone{Crop: 1}, RectOf(5, 5, 6, 6), nil)
	// the block above q comes first (row-major) and touches only q; the
	// block between a and q then joins them, so the first one ends up in a
	m.AddCells(a, append(rectCells(5, 4, 6, 4), rectCells(2, 6, 4, 6)...))
	if _, alive := m.Zone(q); alive || cells(m, a) != 4+4+2+3 || len(m.Zones()) != 1 {
		t.Fatalf("zones %+v", m.Zones())
	}
	consistent(t, m)
}

func TestRemoveCellsTouchesOnlyThatZone(t *testing.T) {
	m := NewMap(20, 20)
	a := m.Paint(KindGrow, Zone{Crop: 1, NoCut: true}, RectOf(0, 0, 5, 3), nil)
	s := m.Paint(KindStockpile, stock(), RectOf(6, 0, 8, 3), nil)
	m.RemoveCells(a, RectOf(4, 1, 8, 2)) // spans the stockpile too
	if cells(m, a) != 24-4 || cells(m, s) != 12 || owner(m, 6, 1) != s {
		t.Fatalf("zones %+v", m.Zones())
	}
	if owner(m, 4, 1) != 0 || owner(m, 5, 2) != 0 || owner(m, 3, 1) != a {
		t.Fatal("only the zone's own tiles in the rect are freed")
	}
	if z, _ := m.Zone(a); !z.NoCut {
		t.Fatal("settings kept")
	}
	if b, _ := m.Bounds(a); b != (Rect{0, 0, 5, 3}) {
		t.Fatalf("bounds %v", b)
	}
	consistent(t, m)
}

func TestRemoveCellsSplitKeepsSettingsAndTheBiggestPieceKeepsTheID(t *testing.T) {
	m := NewMap(20, 20)
	a := m.Paint(KindGrow, Zone{Crop: 2, NoSow: true, Hidden: true, Name: "North"}, RectOf(0, 0, 9, 1), nil)
	m.RemoveCells(a, RectOf(3, 0, 3, 1)) // 3 tiles x 2 rows west, 6 x 2 east
	zs := m.Zones()
	if len(zs) != 2 {
		t.Fatalf("zones %+v", zs)
	}
	if zs[0].ID != a || zs[0].Cells != 12 || owner(m, 9, 1) != a {
		t.Fatalf("the bigger east piece keeps the id: %+v", zs)
	}
	west := zs[1]
	if west.Cells != 6 || owner(m, 0, 0) != west.ID || west.ID <= a {
		t.Fatalf("west piece %+v", west)
	}
	for _, z := range zs {
		if z.Crop != 2 || !z.NoSow || !z.Hidden || z.Name != "North" || z.Kind != KindGrow {
			t.Fatalf("a piece keeps the settings: %+v", z)
		}
	}
	consistent(t, m)
}

func TestRemoveCellsEqualPiecesKeepTheFirstRowMajorOne(t *testing.T) {
	m := NewMap(20, 20)
	a := m.Paint(KindPen, Zone{}, RectOf(0, 0, 6, 0), nil)
	m.RemoveCells(a, RectOf(3, 0, 3, 0))
	if owner(m, 0, 0) != a || owner(m, 4, 0) == a || cells(m, a) != 3 {
		t.Fatalf("zones %+v", m.Zones())
	}
	consistent(t, m)
}

func TestRemoveCellsThreeWaySplitNumbersPiecesRowMajor(t *testing.T) {
	m := NewMap(20, 20)
	a := m.Paint(KindStockpile, Zone{Food: true}, RectOf(0, 0, 8, 0), nil)
	m.RemoveCells(a, RectOf(2, 0, 2, 0))
	m.RemoveCells(a, RectOf(6, 0, 6, 0)) // west 2, middle 3 (x 3..5), east 2
	if len(m.Zones()) != 3 || owner(m, 4, 0) != a || cells(m, a) != 3 {
		t.Fatalf("zones %+v", m.Zones())
	}
	consistent(t, m)
}

func TestRemoveCellsEmptiedZoneIsDeleted(t *testing.T) {
	m := NewMap(20, 20)
	a := m.Paint(KindGrow, Zone{Crop: 1}, RectOf(2, 2, 4, 4), nil)
	b := m.Paint(KindStockpile, stock(), RectOf(10, 10, 11, 11), nil)
	m.RemoveCells(a, RectOf(0, 0, 19, 19))
	if _, ok := m.Zone(a); ok || len(m.Zones()) != 1 || cells(m, b) != 4 || owner(m, 3, 3) != 0 {
		t.Fatalf("zones %+v", m.Zones())
	}
	consistent(t, m)
}

func TestRemoveCellsNoOps(t *testing.T) {
	m := NewMap(10, 10)
	a := m.Paint(KindGrow, Zone{Crop: 1}, RectOf(2, 2, 4, 4), nil)
	before := m.Export()
	m.RemoveCells(a, RectOf(7, 7, 9, 9)) // none of its tiles
	m.RemoveCells(a, RectOf(20, 20, 30, 30))
	m.RemoveCells(99, RectOf(0, 0, 9, 9))
	if !reflect.DeepEqual(before, m.Export()) {
		t.Fatal("no-ops changed the map")
	}
	m.RemoveCells(a, RectOf(4, 4, 30, 30)) // clipped to the map
	if cells(m, a) != 9-1 {
		t.Fatalf("cells %d", cells(m, a))
	}
}

func TestUpdateChangesSettingsButNotIdentityOrCrop(t *testing.T) {
	m := NewMap(20, 20)
	a := m.Paint(KindGrow, Zone{Crop: 1}, RectOf(0, 0, 2, 2), nil)
	b := m.Paint(KindGrow, Zone{Crop: 2}, RectOf(3, 0, 5, 2), nil)
	m.Update(b, func(z *Zone) {
		z.NoSow, z.NoCut, z.Hidden, z.Name = true, true, true, "Rice"
		z.ID, z.Kind, z.Cells, z.Crop = 77, KindPen, 1, 1 // none of these stick
	})
	z, _ := m.Zone(b)
	if z.ID != b || z.Kind != KindGrow || z.Cells != 9 || z.Crop != 2 || !z.NoSow || !z.NoCut || !z.Hidden || z.Name != "Rice" {
		t.Fatalf("zone %+v", z)
	}
	if _, ok := m.Zone(a); !ok || len(m.Zones()) != 2 {
		t.Fatal("Update never merges zones")
	}
	m.Update(99, func(z *Zone) { z.NoSow = true }) // unknown: no call, no panic
	consistent(t, m)
}

// Settings and names never decide whether zones merge: painting against a
// zone joins it and takes the surviving (lowest id) zone's settings.
func TestPaintMergesIgnoreSettingsAndNames(t *testing.T) {
	m := NewMap(20, 20)
	a := m.Paint(KindGrow, Zone{Crop: 1, NoSow: true, Name: "First"}, RectOf(0, 0, 2, 2), nil)
	got := m.Paint(KindGrow, Zone{Crop: 1}, RectOf(3, 0, 4, 2), nil) // fresh paint: default settings
	if got != a || len(m.Zones()) != 1 {
		t.Fatalf("zones %+v", m.Zones())
	}
	if z, _ := m.Zone(a); !z.NoSow || z.Name != "First" || z.Cells != 15 {
		t.Fatalf("the painted tiles take the zone's settings: %+v", z)
	}
	// a bridge between two zones with different settings: the lowest id wins
	b := m.Paint(KindGrow, Zone{Crop: 1, NoCut: true, Hidden: true, Name: "Second"}, RectOf(8, 0, 10, 2), nil)
	if b == a {
		t.Fatal("apart: two zones")
	}
	m.Paint(KindGrow, Zone{Crop: 1}, RectOf(5, 1, 7, 1), nil)
	z, _ := m.Zone(a)
	if _, alive := m.Zone(b); alive || len(m.Zones()) != 1 || !z.NoSow || z.NoCut || z.Hidden || z.Name != "First" {
		t.Fatalf("the lowest id keeps its settings: %+v", m.Zones())
	}
	consistent(t, m)
}

// Changing a crop onto a touching zone's merges into the lower id, which
// keeps its own settings.
func TestSetCropMergeKeepsTheSurvivorsSettings(t *testing.T) {
	m := NewMap(20, 20)
	a := m.Paint(KindGrow, Zone{Crop: 1, Name: "Low"}, RectOf(0, 0, 2, 2), nil)
	b := m.Paint(KindGrow, Zone{Crop: 2, NoSow: true}, RectOf(3, 0, 5, 2), nil)
	z, _ := m.Zone(b)
	z.Crop = 1
	m.Set(b, z)
	got, ok := m.Zone(a)
	if _, alive := m.Zone(b); alive || !ok || got.Cells != 18 || got.NoSow || got.Name != "Low" {
		t.Fatalf("zones %+v", m.Zones())
	}
	consistent(t, m)
}

func TestExportImportKeepsPLAY020Settings(t *testing.T) {
	m := NewMap(12, 9)
	a := m.Paint(KindGrow, Zone{Crop: 4, NoSow: true, NoCut: true, Hidden: true, Name: "Rice"}, RectOf(0, 0, 3, 3), nil)
	m.AddCells(a, rectCells(8, 0, 9, 1))
	m.RemoveCells(a, RectOf(1, 0, 1, 3)) // splits it: the east piece keeps the id
	n := Import(m.Export())
	if !reflect.DeepEqual(m.Zones(), n.Zones()) || len(n.Zones()) != 3 {
		t.Fatalf("round trip differs: %+v vs %+v", m.Zones(), n.Zones())
	}
	for _, z := range n.Zones() {
		if !z.NoSow || !z.NoCut || !z.Hidden || z.Name != "Rice" {
			t.Fatalf("settings lost: %+v", z)
		}
	}
	consistent(t, n)
}

func TestEditsAreDeterministic(t *testing.T) {
	run := func() Saved {
		m := NewMap(30, 30)
		ids := []ID{
			m.Paint(KindGrow, Zone{Crop: 1}, RectOf(2, 2, 8, 6), nil),
			m.Paint(KindGrow, Zone{Crop: 1, NoSow: true}, RectOf(14, 2, 20, 6), nil),
			m.Paint(KindStockpile, stock(), RectOf(2, 12, 6, 16), nil),
		}
		for i := 0; i < 30; i++ {
			x, y := (i*7)%24, (i*11)%24
			id := ids[i%len(ids)]
			switch i % 3 {
			case 0:
				m.AddCells(id, rectCells(x, y, x+4, y+2))
			case 1:
				m.RemoveCells(id, RectOf(x, y, x+2, y+3))
			default:
				if i%9 == 8 {
					m.Remove(id)
					ids[i%len(ids)] = m.Paint(KindGrow, Zone{Crop: 1}, RectOf(x, y, x+3, y+3), nil)
				}
			}
			// a removed or merged-away id is a no-op
		}
		consistent(t, m)
		return m.Export()
	}
	if !reflect.DeepEqual(run(), run()) {
		t.Fatal("same edits, different maps")
	}
}
