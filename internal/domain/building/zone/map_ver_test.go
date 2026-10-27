package zone

import (
	"reflect"
	"testing"
)

// TestMapVerMovesOnEveryEdit: every exported mutator moves Ver (read-model
// caches key on it), reads leave it alone, and Export does not carry it.
func TestMapVerMovesOnEveryEdit(t *testing.T) {
	m := NewMap(12, 12)
	grow := Zone{Crop: 1}
	var a, b ID
	edits := []struct {
		name string
		do   func()
	}{
		{"Paint", func() { a = m.Paint(KindStockpile, stock(), RectOf(0, 0, 5, 3), nil) }},
		{"Paint grow", func() { b = m.Paint(KindGrow, grow, RectOf(0, 8, 3, 9), nil) }},
		{"Set", func() { m.Set(b, Zone{Crop: 2}) }},
		{"Update", func() { m.Update(a, func(z *Zone) { z.Name = "Pantry" }) }},
		{"AddCells", func() { m.AddCells(a, [][2]int{{6, 0}, {6, 1}}) }},
		{"RemoveCells", func() { m.RemoveCells(a, RectOf(2, 0, 2, 3)) }},
		{"Split", func() { m.Split(a) }},
		{"RemoveTiles", func() { m.RemoveTiles([][2]int{{0, 0}}) }},
		{"Clear", func() { m.Clear(RectOf(0, 1, 0, 1)) }},
		{"Remove", func() { m.Remove(b) }},
		{"Clear nothing", func() { m.Clear(RectOf(11, 11, 11, 11)) }}, // a no-op edit still moves it
	}
	for _, e := range edits {
		before := m.Ver()
		e.do()
		if m.Ver() == before {
			t.Errorf("%s did not move Ver", e.name)
		}
	}
	before := m.Ver()
	_, _ = m.At(1, 1)
	_ = m.Zones()
	_ = m.Cells(a)
	_, _ = m.Bounds(a)
	_, _ = m.Zone(a)
	m.Each(func(Zone) bool { return true })
	m.EachCell(a, func(int, int) bool { return true })
	if m.Ver() != before {
		t.Fatal("a read moved Ver")
	}
	if got, want := Import(m.Export()).Export(), m.Export(); !reflect.DeepEqual(got, want) {
		t.Fatal("Export/Import round trip changed")
	}
}
