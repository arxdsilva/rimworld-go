package item

import "testing"

// TestSameLoad: piles join one carried load only when they are the same
// thing in the same state and make; place, carrier and marks don't matter
// (HAUL-002), and CanMerge still asks for them.
func TestSameLoad(t *testing.T) {
	base := Stack{Kind: Club, Qty: 2, X: 1, Y: 1, Quality: 3, Stuff: Steel, Wear: 4}
	far := base
	far.X, far.Y, far.Carrier, far.Unclaimed, far.HaulMarked = 9, 9, 7, true, true
	if !SameLoad(base, far) {
		t.Fatal("the same club elsewhere, carried or marked, is the same load")
	}
	if CanMerge(base, far) {
		t.Fatal("CanMerge still needs one tile and no carrier")
	}
	for name, edit := range map[string]func(*Stack){
		"kind":    func(s *Stack) { s.Kind = Mace },
		"quality": func(s *Stack) { s.Quality = 4 },
		"stuff":   func(s *Stack) { s.Stuff = Wood },
		"wear":    func(s *Stack) { s.Wear = 5 },
		"dried":   func(s *Stack) { s.Dried = true },
		"of":      func(s *Stack) { s.Of = 3 },
		"animal":  func(s *Stack) { s.Animal = 2 },
		"tame":    func(s *Stack) { s.Tame = true },
	} {
		o := base
		edit(&o)
		if SameLoad(base, o) {
			t.Errorf("a %s mismatch joined the load", name)
		}
	}
	meat := Stack{Kind: Meat, Qty: 5}
	rotten := meat
	rotten.Rot = Meat.ShelfTicks()
	if SameLoad(meat, rotten) {
		t.Error("rotten meat joined a fresh load")
	}
}
