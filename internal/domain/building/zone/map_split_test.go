package zone

import "testing"

// PLAY-023: RemoveTiles and Split (cells lost under a wall or a floor),
// the dumping stockpile kind and the storage priority.

func TestRemoveTilesSplitsDeterministically(t *testing.T) {
	t.Parallel()
	build := func() (*Map, ID) {
		m := NewMap(12, 6)
		id := m.Paint(KindGrow, Zone{Crop: 2, Name: "Field"}, RectOf(0, 0, 8, 2), nil)
		return m, id
	}
	m, id := build()
	// a wall down column 3 cuts the field into 3 wide (left) and 5 wide (right)
	hit := m.RemoveTiles([][2]int{{3, 0}, {3, 1}, {3, 2}, {11, 5}, {-1, 0}})
	if len(hit) != 1 || hit[0] != id {
		t.Fatalf("hit = %v, want [%d]", hit, id)
	}
	if owner(m, 3, 1) != 0 {
		t.Fatal("removed tile still zoned")
	}
	if owner(m, 5, 1) != id {
		t.Fatal("the bigger piece keeps the id")
	}
	left := owner(m, 0, 0)
	if left == 0 || left == id {
		t.Fatalf("left piece = %d: want a new zone", left)
	}
	z, _ := m.Zone(left)
	if z.Kind != KindGrow || z.Crop != 2 || z.Name != "Field" || z.Cells != 9 {
		t.Fatalf("new piece %+v: want the old settings and 9 tiles", z)
	}
	if big, _ := m.Zone(id); big.Cells != 15 {
		t.Fatalf("kept piece has %d tiles, want 15", big.Cells)
	}
	// same cut, same ids
	m2, _ := build()
	m2.RemoveTiles([][2]int{{3, 2}, {3, 0}, {3, 1}})
	if owner(m2, 0, 0) != left || owner(m2, 5, 1) != id {
		t.Fatal("the split is not deterministic")
	}
	// removing every tile deletes the zone
	m.RemoveTiles(rectCells(0, 0, 2, 2))
	if _, ok := m.Zone(left); ok {
		t.Fatal("emptied zone not deleted")
	}
}

func TestSplitPublic(t *testing.T) {
	t.Parallel()
	m := NewMap(10, 3)
	id := m.Paint(KindStockpile, stock(), RectOf(0, 0, 9, 0), nil)
	if got := m.Split(id); got != nil {
		t.Fatalf("whole zone split into %v", got)
	}
	if m.Split(99) != nil {
		t.Fatal("unknown zone split")
	}
}

func TestDumpKindAndPriority(t *testing.T) {
	t.Parallel()
	if !KindStockpile.Storage() || !KindDump.Storage() || KindGrow.Storage() || KindPen.Storage() {
		t.Fatal("storage kinds")
	}
	if KindDump != 4 {
		t.Fatal("kinds are saved: KindDump must stay 4")
	}
	var z Zone
	if z.Priority != PriorityNormal || PriorityNormal != 0 {
		t.Fatal("the zero priority is Normal")
	}
	for i := 1; i < len(Priorities); i++ {
		if Priorities[i] >= Priorities[i-1] || !Priorities[i].Valid() {
			t.Fatal("Priorities runs highest first")
		}
	}
	if Priority(4).Valid() || Priority(-2).Valid() || PriorityCritical.String() != "Critical" || PriorityLow.String() != "Low" {
		t.Fatal("priority names and range")
	}
	m := NewMap(10, 3)
	s := m.Paint(KindStockpile, stock(), RectOf(0, 0, 2, 0), nil)
	d := m.Paint(KindDump, Zone{Corpses: true, Chunks: true}, RectOf(3, 0, 5, 0), nil)
	if s == d {
		t.Fatal("a dumping stockpile never merges into a stockpile")
	}
	m.Update(d, func(z *Zone) { z.Priority = PriorityLow })
	if z, _ := m.Zone(d); z.Priority != PriorityLow || !z.Corpses || !z.Chunks {
		t.Fatalf("dump settings %+v", z)
	}
	// round trip
	m2 := Import(m.Export())
	if z, _ := m2.Zone(d); z.Kind != KindDump || z.Priority != PriorityLow || !z.Chunks {
		t.Fatalf("import lost dump settings: %+v", z)
	}
}
