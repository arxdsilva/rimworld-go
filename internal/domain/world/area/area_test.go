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
