package site

import "testing"

func TestHillsLabel(t *testing.T) {
	want := map[Hills]string{HillsDefault: "Default", Flat: "Flat", SmallHills: "Small hills", LargeHills: "Large hills", Mountainous: "Mountainous", 99: ""}
	for h, w := range want {
		if got := h.Label(); got != w {
			t.Errorf("Hills(%d).Label() = %q, want %q", h, got, w)
		}
	}
}

func TestRiverLabel(t *testing.T) {
	want := map[RiverSize]string{NoRiver: "", Creek: "Creek", River: "River", LargeRiver: "Large river", 9: ""}
	for r, w := range want {
		if got := r.Label(); got != w {
			t.Errorf("RiverSize(%d).Label() = %q, want %q", r, got, w)
		}
	}
}

func TestZeroAndDirSet(t *testing.T) {
	if !(Site{}).Zero() {
		t.Fatal("zero Site must be Zero")
	}
	for _, s := range []Site{{Hills: Flat}, {River: RiverPath{Size: Creek}}, {Coast: 1}, {Lakes: 1}, {Caves: true}, {Road: 4}} {
		if s.Zero() {
			t.Fatalf("%+v reported Zero", s)
		}
	}
	s := Dir8Set(1<<2 | 1<<7)
	if !s.Has(2) || !s.Has(7) || s.Has(0) {
		t.Fatal("Dir8Set.Has")
	}
}
