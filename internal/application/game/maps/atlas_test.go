package maps

import (
	"slices"
	"testing"
)

// TestShouldCloseNoFlap: a map closes only after closeStreak empty hours
// in a row; anyone of ours (or a member still walking out) resets it.
func TestShouldCloseNoFlap(t *testing.T) {
	m := &Meta{Kind: Site}
	empty, ours := Census{}, Census{Ours: 1}
	for h := 0; h < 50; h++ { // alternating hours never close it
		c := empty
		if h%2 == 1 {
			c = ours
		}
		if ShouldClose(m, c) {
			t.Fatalf("closed at hour %d on alternating census results", h)
		}
	}
	if ShouldClose(m, Census{Exiting: true}) || ShouldClose(m, empty) || ShouldClose(m, empty) {
		t.Fatal("closed before closeStreak empty hours in a row")
	}
	if !ShouldClose(m, empty) {
		t.Fatal("did not close after closeStreak empty hours")
	}
}

// TestJobDoneNoFlap: the pack-up wants closeStreak done hours in a row;
// hostiles coming and going between hours never reach it.
func TestJobDoneNoFlap(t *testing.T) {
	m := &Meta{Kind: Site, Cells: []int{1, 2}}
	if JobDone(m, Census{Ore: 1}) || !JobDone(m, Census{}) || JobDone(m, Census{Hostiles: 1}) {
		t.Fatal("a lump site is done once mined out with no hostiles")
	}
	for h := 0; h < 40; h++ {
		c := Census{Hostiles: h % 2}
		if streak(&m.Done, JobDone(m, c)) {
			t.Fatalf("pack-up started at hour %d on a flapping fight", h)
		}
	}
	m.Hostile = true
	if !JobDone(m, Census{Ore: 5}) {
		t.Fatal("a won fight finishes the job")
	}
	if JobDone(&Meta{Kind: Colony}, Census{}) || JobDone(&Meta{Kind: Settlement}, Census{}) || !JobDone(&Meta{Kind: Settlement, Defeated: true}, Census{}) {
		t.Fatal("colonies are never done; a settlement once defeated")
	}
}

// TestSizesAndSeeds: temporary map sides (original tuning) and seeds that
// are pure, and differ by tile and by id.
func TestSizesAndSeeds(t *testing.T) {
	if SizeFor(Site, 250) != 150 || SizeFor(Settlement, 250) != 150 || SizeFor(Encounter, 250) != 100 || SizeFor(Colony, 250) != 250 {
		t.Fatal("map sides")
	}
	a := SeedFor(7, 100, 1)
	if a != SeedFor(7, 100, 1) || a == SeedFor(7, 101, 1) || a == SeedFor(7, 100, 2) || a == SeedFor(8, 100, 1) {
		t.Fatal("SeedFor is not a pure mix of seed, tile and id")
	}
	var at Atlas
	if !at.Empty() || at.newID() != 1 || at.newID() != 2 || at.Empty() {
		t.Fatal("map ids start at 1 and an atlas that handed one out is saved")
	}
}

// TestLumpCellsDeterministic: the lump fills the free cells nearest its
// spot, the same every time, never on the map's border.
func TestLumpCellsDeterministic(t *testing.T) {
	free := func(x, y int) bool { return (x+y)%7 != 0 }
	a, b := LumpCells(50, 40, 30, free), LumpCells(50, 40, 30, free)
	if len(a) != 30 || !slices.Equal(a, b) {
		t.Fatalf("lump cells %v vs %v", a, b)
	}
	for _, c := range a {
		if x, y := c%50, c/50; !free(x, y) || x == 0 || y == 0 || x == 49 || y == 39 {
			t.Fatalf("lump cell (%d, %d) is not free or on the border", x, y)
		}
	}
}

// TestVisibilityPoints: the wiki's two points and the fit beyond.
func TestVisibilityPoints(t *testing.T) {
	if Visibility(1) != 200 || Visibility(2) != 360 || Visibility(3) != 488 || Visibility(0) != 0 {
		t.Fatalf("visibility %d %d %d", Visibility(1), Visibility(2), Visibility(3))
	}
}
