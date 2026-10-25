package area

import (
	"bytes"
	"encoding/gob"
	"testing"
)

func TestPaintContainsCount(t *testing.T) {
	t.Parallel()
	s := New(10, 8)
	i := s.Add("home")
	if i != 0 || s.Count(0) != 0 {
		t.Fatal("add")
	}
	s.Paint(i, 3, 2, 1, 1, true) // reversed corners
	if s.Count(i) != 6 || !s.Contains(i, 1, 1) || !s.Contains(i, 3, 2) || s.Contains(i, 4, 2) {
		t.Fatalf("paint %d", s.Count(i))
	}
	s.Paint(i, -5, -5, 100, 100, true)
	if s.Count(i) != 80 {
		t.Fatalf("clip %d", s.Count(i))
	}
	s.Paint(i, 0, 0, 9, 3, false)
	if s.Count(i) != 40 || s.Contains(i, 0, 0) {
		t.Fatal("clear")
	}
	s.Paint(9, 0, 0, 1, 1, true) // invalid index: no-op
	if s.Contains(-1, 0, 0) || s.Contains(i, -1, 0) || s.Contains(i, 10, 0) || s.Count(5) != 0 {
		t.Fatal("bounds")
	}
}

func TestAddRenameMax(t *testing.T) {
	t.Parallel()
	s := New(4, 4)
	for i := 0; i < MaxAreas; i++ {
		if s.Add("a") != i {
			t.Fatal("index")
		}
	}
	if s.Add("x") != -1 {
		t.Fatal("max")
	}
	if !s.Rename(3, "farm") || s.Areas[3].Name != "farm" || s.Rename(10, "z") || s.Rename(-1, "z") {
		t.Fatal("rename")
	}
}

func TestExportImportGob(t *testing.T) {
	t.Parallel()
	if (*Set)(nil).Export() != nil || New(2, 2).Export() != nil {
		t.Fatal("empty export")
	}
	s := New(70, 3)
	s.Add("a")
	s.Paint(0, 60, 1, 69, 2, true)
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(s.Export()); err != nil {
		t.Fatal(err)
	}
	var back Set
	if err := gob.NewDecoder(&buf).Decode(&back); err != nil {
		t.Fatal(err)
	}
	r := Import(&back)
	s.Paint(0, 0, 0, 0, 0, true) // copy independence
	if r.Count(0) != 20 || !r.Contains(0, 69, 2) || r.Contains(0, 0, 0) {
		t.Fatal("roundtrip")
	}
}

// PLAY-015: stable ids, colours, delete and invert.
func TestIDsDeleteInvert(t *testing.T) {
	s := New(5, 3) // 15 cells: one partial word
	a, b := s.Add("A"), s.Add("B")
	if s.Areas[a].ID != 1 || s.Areas[b].ID != 2 || s.Areas[a].Color == s.Areas[b].Color {
		t.Fatalf("ids/colours %+v", s.Areas)
	}
	s.Paint(a, 0, 0, 1, 0, true)
	if !s.Invert(a) || s.Contains(a, 0, 0) || !s.Contains(a, 4, 2) || s.Count(a) != 13 {
		t.Fatalf("invert: count %d", s.Count(a))
	}
	if s.Areas[a].Bits[0]>>15 != 0 {
		t.Fatal("invert set padding bits")
	}
	if s.Invert(9) || s.Index(0) != -1 || s.Index(7) != -1 {
		t.Fatal("bad index handling")
	}
	if !s.Delete(1) || s.Delete(1) || len(s.Areas) != 1 || s.Index(2) != 0 {
		t.Fatal("delete")
	}
	if c := s.Add("C"); s.Areas[c].ID != 3 {
		t.Fatalf("ids are never reused: got %d", s.Areas[c].ID)
	}
	if ColorOf(0) != ColorOf(1) || ColorOf(11) != ColorOf(1) {
		t.Fatal("palette cycles")
	}
}

func TestImportGivesOldSetsIDs(t *testing.T) {
	old := &Set{W: 2, H: 2, Areas: []Area{{Name: "x", Bits: []uint64{1}}, {Name: "y", Bits: []uint64{2}}}}
	s := Import(old)
	if s.Areas[0].ID != 1 || s.Areas[1].ID != 2 || s.LastID != 2 || s.Areas[1].Color != ColorOf(2) {
		t.Fatalf("migrated %+v", s)
	}
	if old.Areas[0].ID != 0 {
		t.Fatal("import must not touch the saved set")
	}
	if Import(nil) != nil {
		t.Fatal("nil")
	}
}
