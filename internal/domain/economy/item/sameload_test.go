// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

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

// Market goods merge according to their physical identity. The source
// of an otherwise identical commodity never changes its stackability.
func TestStackIdentityForMarketGoods(t *testing.T) {
	cloth := Stack{Kind: Cloth, Qty: 2}
	more := cloth
	more.Qty = 3
	if !SameLoad(cloth, more) || !CanMerge(cloth, more) {
		t.Fatal("identical commodities should stack regardless of where they were bought")
	}
	crate := Stack{Kind: MinifiedBuilding, Qty: 1, Building: 4, Stuff: Steel}
	otherCrate := crate
	otherCrate.Building = 5
	if SameLoad(crate, otherCrate) || CanMerge(crate, otherCrate) {
		t.Fatal("different minified buildings shared a stack")
	}
	egg := Stack{Kind: FertileEgg, Qty: 2, Animal: 4, Hatch: 150, Tame: true}
	otherEgg := egg
	otherEgg.Hatch = 300
	if SameLoad(egg, otherEgg) || CanMerge(egg, otherEgg) {
		t.Fatal("different egg incubation stages shared a stack")
	}
	book := Stack{Kind: Novel, Qty: 1, Book: 11, Quality: 3}
	otherBook := book
	otherBook.Book = 12
	if SameLoad(book, otherBook) || CanMerge(book, otherBook) {
		t.Fatal("different books shared a stack")
	}
	apparel := Stack{Kind: Parka, Qty: 1, Stuff: Cloth, Quality: 3}
	otherApparel := apparel
	otherApparel.Quality = 4
	if SameLoad(apparel, otherApparel) {
		t.Fatal("different-quality goods shared a stack")
	}
}
