package work

import "testing"

func TestPriorities(t *testing.T) {
	p := DefaultPriorities()
	if p[Haul] != DefaultPriority || p[Firefight] != 0 {
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
