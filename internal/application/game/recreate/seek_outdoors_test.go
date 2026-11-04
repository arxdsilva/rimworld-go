package recreate

import (
	"slices"
	"testing"

	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
)

// fakeHost's defaults for the outdoors roles: everything indoors, the need full,
// the rule off, as the tests before CSE-001 had it.
func (f *fakeHost) Outside(int, int) bool             { return false }
func (f *fakeHost) OutdoorsPct(entity.EntityID) int   { return 100 }
func (f *fakeHost) OutdoorsJoy() bool                 { return false }
func (f *fakeHost) OnBedRest(id entity.EntityID) bool { return false }

// outdoorHost is fakeHost with a sky over the east half and a need to set.
type outdoorHost struct {
	*fakeHost
	pct     int
	off     bool
	bedRest bool
}

func (h *outdoorHost) Outside(x, _ int) bool             { return x >= 30 }
func (h *outdoorHost) OutdoorsPct(entity.EntityID) int   { return h.pct }
func (h *outdoorHost) OutdoorsJoy() bool                 { return !h.off }
func (h *outdoorHost) OnBedRest(id entity.EntityID) bool { return h.bedRest }

// outdoorScene: a colonist at (18, 18), a table beside it indoors, one out in
// the open farther off.
func outdoorScene(pct int) (*State, *outdoorHost) {
	f := newFake()
	f.add(10, construction.Building{Kind: construction.ChessTable, X: 20, Y: 20})
	f.add(11, construction.Building{Kind: construction.ChessTable, X: 32, Y: 20})
	f.pawn(1, 18, 18, 0)
	h := &outdoorHost{fakeHost: f, pct: pct}
	return New(h), h
}

// sought is the building the colonist's session went to.
func sought(t *testing.T, s *State, h *outdoorHost) entity.EntityID {
	t.Helper()
	if !s.Seek(1, clock.TicksPerHour) {
		t.Fatal("no session")
	}
	return h.jobs[1].Dest
}

func TestSeekPrefersOutdoorsWhenCraving(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		pct  int
		edit func(h *outdoorHost)
		want entity.EntityID
	}{
		{name: "craving goes out", pct: CravingOn - 1, want: 11},
		{name: "need met stays near", pct: CravingOn, want: 10},
		{name: "bed rest stays near", pct: 0, edit: func(h *outdoorHost) { h.bedRest = true }, want: 10},
		{name: "rule off stays near", pct: 0, edit: func(h *outdoorHost) { h.off = true }, want: 10},
	}
	for _, tc := range tests {
		s, h := outdoorScene(tc.pct)
		if tc.edit != nil {
			tc.edit(h)
		}
		if got := sought(t, s, h); got != tc.want {
			t.Errorf("%s: went to %d, want %d", tc.name, got, tc.want)
		}
	}
}

// TestSeekCravingFallsBackIndoors: with nothing out in the open, a craving still
// gets its recreation.
func TestSeekCravingFallsBackIndoors(t *testing.T) {
	t.Parallel()
	f := newFake()
	f.add(10, construction.Building{Kind: construction.ChessTable, X: 20, Y: 20})
	f.pawn(1, 18, 18, 0)
	h := &outdoorHost{fakeHost: f, pct: 0}
	if got := sought(t, New(h), h); got != 10 {
		t.Fatalf("went to %d", got)
	}
}

// TestOutdoorsCravingDoesNotFlap walks the need up and down between the two
// marks: the craving turns only when it crosses the far one, and the rule off
// records nothing.
func TestOutdoorsCravingDoesNotFlap(t *testing.T) {
	t.Parallel()
	s, h := outdoorScene(100)
	turns, was := 0, false
	feel := func(pct int) bool {
		h.pct = pct
		on := s.craves(1)
		if on != was {
			turns++
		}
		was = on
		return on
	}
	if feel(CravingOn) {
		t.Fatal("at the mark: not yet")
	}
	if !feel(CravingOn - 1) {
		t.Fatal("below the mark: craving")
	}
	for i := 0; i < 200; i++ {
		if !feel(CravingOn + i%(CravingOff-CravingOn)) {
			t.Fatalf("between the marks at %d: the craving holds", i)
		}
	}
	if turns != 1 {
		t.Fatalf("%d turns inside the band", turns)
	}
	if feel(CravingOff) {
		t.Fatal("at the upper mark: over")
	}
	for i := 0; i < 200; i++ {
		if feel(CravingOn + i%(CravingOff-CravingOn)) {
			t.Fatalf("between the marks at %d: stays over", i)
		}
	}
	if turns != 2 {
		t.Fatalf("%d turns over the walk, want 2", turns)
	}
	off, h2 := outdoorScene(0)
	h2.off = true
	if off.craves(1) || off.Export() != nil && len(off.Export().Craving) != 0 {
		t.Fatalf("rule off recorded a craving: %+v", off.Export())
	}
}

// TestBedRestKeepsCraving: bed rest overrides the choice, not the saved state.
func TestBedRestKeepsCraving(t *testing.T) {
	t.Parallel()
	s, h := outdoorScene(0)
	h.bedRest = true
	if s.craves(1) {
		t.Fatal("bed rest looks no further")
	}
	h.bedRest = false
	h.pct = CravingOn + 1
	if !s.craves(1) {
		t.Fatal("the craving was kept through bed rest")
	}
}

func TestCravingSaveRoundTrip(t *testing.T) {
	t.Parallel()
	s, h := outdoorScene(0)
	s.craves(1)
	out := s.Export()
	if out == nil || !slices.Equal(out.Craving, []entity.EntityID{1}) {
		t.Fatalf("export %+v", out)
	}
	back := Load(h, out)
	h.pct = CravingOn + 1 // inside the band: only the saved state keeps it on
	if !back.craves(1) {
		t.Fatal("the craving did not survive the round trip")
	}
	back.Forget(1)
	if e := back.Export(); e != nil && len(e.Craving) != 0 {
		t.Fatalf("forgotten pawn still craves: %+v", e)
	}
}
