package work

import (
	"slices"
	"testing"
)

func TestPriorities(t *testing.T) {
	p := DefaultPriorities()
	if p[Haul] != DefaultPriority || p[Firefight] != DefaultPriority || p[Warden] != 0 {
		t.Fatalf("defaults %v", p)
	}
	p = p.Set(Grow, 1).Set(Mine, 0).Set(Cook, 9)
	if p[Grow] != 1 || p[Cook] != Lowest {
		t.Fatalf("set %v", p)
	}
	m := p.Mask()
	if m.Enabled(Mine) || !m.Enabled(Grow) {
		t.Fatal("mask should follow priorities")
	}
}

// TestOrderIsTheWikis pins work.Order against the wiki's Work page (WRK-001):
// the tie-break and the Work tab's left-to-right order.
func TestOrderIsTheWikis(t *testing.T) {
	want := []Type{Firefight, Doctor, Warden, Handle, Cook, Hunt, Construct, Grow, Mine, PlantCut, Craft, Haul, Clean, Research}
	if !slices.Equal(Order, want) {
		t.Fatalf("Order %v, want %v", Order, want)
	}
	if len(Order) != int(NumTypes) {
		t.Fatalf("Order has %d of %d work types", len(Order), NumTypes)
	}
}

// TestFirefightFirst: firefighting (PLAY-015) is on by default and the
// first work type within a priority.
func TestFirefightFirst(t *testing.T) {
	if !DefaultColonist().Enabled(Firefight) {
		t.Fatal("firefighting should be on by default")
	}
	if len(Order) == 0 || Order[0] != Firefight {
		t.Fatalf("Order %v: Firefight should come first", Order)
	}
	seen := map[Type]bool{}
	for _, w := range Order {
		if seen[w] {
			t.Fatalf("Order repeats %d", w)
		}
		seen[w] = true
	}
}
