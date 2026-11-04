package haul

import (
	"slices"
	"testing"

	"rimworld-go/internal/domain/sim/entity"
)

func TestSweepArmDisarm(t *testing.T) {
	t.Parallel()
	var s Sweep
	if s.Armed(3) {
		t.Fatal("an empty sweep has a mark")
	}
	s.Arm(3)
	s.Arm(3)
	s.Arm(1)
	if !s.Armed(3) || !s.Armed(1) || s.Armed(2) {
		t.Fatalf("marks %v, want 1 and 3", s.Export())
	}
	s.Disarm(3)
	s.Disarm(2) // not marked: nothing happens
	if s.Armed(3) || !s.Armed(1) {
		t.Fatalf("marks %v, want 1", s.Export())
	}
	s.Disarm(1)
	if s.Export() != nil {
		t.Fatalf("no mark left exports %v, want nil", s.Export())
	}
}

// TestSweepExportIsSortedAndImportNormalises: the saved ids are in
// ascending order whatever order they were marked in; an import sorts,
// drops repeats and does not share the caller's slice.
func TestSweepExportIsSortedAndImportNormalises(t *testing.T) {
	t.Parallel()
	var s Sweep
	for _, id := range []entity.EntityID{9, 2, 5, 7} {
		s.Arm(id)
	}
	if got := s.Export(); !slices.Equal(got, []entity.EntityID{2, 5, 7, 9}) {
		t.Fatalf("export %v, want ascending", got)
	}
	in := []entity.EntityID{8, 4, 8, 6}
	var r Sweep
	r.Import(in)
	in[0] = 99
	if got := r.Export(); !slices.Equal(got, []entity.EntityID{4, 6, 8}) {
		t.Fatalf("import kept %v, want [4 6 8]", got)
	}
	r.Import(nil)
	if r.Export() != nil || r.Armed(4) {
		t.Fatal("importing none kept marks")
	}
	out := s.Export()
	out[0] = 99
	if !s.Armed(2) {
		t.Fatal("the export shares the sweep's own slice")
	}
}

func TestSweepNextUnarmedLeavesTheHostAlone(t *testing.T) {
	t.Parallel()
	var s Sweep
	f := &fakeField{ripe: 4, rx: 1, ry: 1}
	if s.Next(f, 1) {
		t.Fatal("an unmarked pawn swept")
	}
	if f.asked != 0 || f.position != 0 || len(f.started) != 0 {
		t.Fatal("an unmarked pawn asked the host")
	}
}

func TestSweepNextGrowOffDisarms(t *testing.T) {
	t.Parallel()
	var s Sweep
	s.Arm(1)
	f := &fakeField{growOff: true, ripe: 4, rx: 1, ry: 1}
	if s.Next(f, 1) || s.Armed(1) || len(f.started) != 0 {
		t.Fatal("a pawn with Grow off went on sweeping")
	}
}

// TestSweepNextRadius: the next plant must lie within Radius (Chebyshev,
// as the plant search measures) of the pawn; the mark holds while it
// sweeps and goes when nothing is in reach.
func TestSweepNextRadius(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		ripe   entity.EntityID
		rx, ry int
		want   bool
	}{
		{"none", 0, 0, 0, false},
		{"edge diagonal", 4, 10 + Radius, 10 - Radius, true},
		{"just out", 4, 10 + Radius + 1, 10, false},
		{"far", 4, 30, 10, false},
		{"here", 4, 10, 10, true},
	}
	for _, c := range cases {
		var s Sweep
		s.Arm(1)
		f := &fakeField{pos: [2]int{10, 10}, ripe: c.ripe, rx: c.rx, ry: c.ry}
		if got := s.Next(f, 1); got != c.want || s.Armed(1) != c.want {
			t.Fatalf("%s: next %v armed %v, want %v", c.name, got, s.Armed(1), c.want)
		}
		if c.want && !slices.Equal(f.started, []entity.EntityID{c.ripe}) {
			t.Fatalf("%s: started %v", c.name, f.started)
		}
		if !c.want && len(f.started) != 0 {
			t.Fatalf("%s: started %v out of reach", c.name, f.started)
		}
	}
}

func TestSweepNextStartFailsDisarms(t *testing.T) {
	t.Parallel()
	var s Sweep
	s.Arm(1)
	f := &fakeField{ripe: 4, refuse: true}
	if s.Next(f, 1) || s.Armed(1) {
		t.Fatal("a harvest that could not start kept the mark")
	}
}

// TestSweepNextOnlyItsPawn: one pawn's mark leaves the others' alone.
func TestSweepNextOnlyItsPawn(t *testing.T) {
	t.Parallel()
	var s Sweep
	s.Arm(1)
	s.Arm(2)
	f := &fakeField{}
	if s.Next(f, 1) || s.Armed(1) || !s.Armed(2) {
		t.Fatalf("marks %v, want only 2", s.Export())
	}
}
