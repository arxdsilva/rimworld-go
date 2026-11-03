package research

import "testing"

// TestAquaculture (FISH-*): an Industrial project after hydroponics that opens
// the fish tank, and whose text is what a player reads about it.
func TestAquaculture(t *testing.T) {
	t.Parallel()
	d := DefOf(Aquaculture)
	if d.Cost != 900 || d.Desc == "" {
		t.Fatalf("aquaculture: %+v", d)
	}
	needsHydro := false
	for _, p := range d.Requires {
		needsHydro = needsHydro || p == Hydroponics
	}
	if !needsHydro {
		t.Fatalf("aquaculture needs %v", d.Requires)
	}
	var p Progress
	if got, locked := p.Locks("build:fish tank"); !locked || got != Aquaculture {
		t.Fatalf("fish tank locked by %v, %v", got, locked)
	}
	if _, locked := Complete().Locks("build:fish tank"); locked {
		t.Fatal("still locked with everything researched")
	}
}
