package work

import "testing"

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
