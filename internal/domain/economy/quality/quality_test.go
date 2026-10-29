package quality

import "testing"

func TestRollDeterministic(t *testing.T) {
	for s := 0; s <= MaxSkill; s++ {
		for _, r := range []uint32{0, 1, 1 << 31, 4294967295} {
			if Roll(s, r, false) != Roll(s, r, false) || Roll(s, r, true) != Roll(s, r, true) {
				t.Fatal("not deterministic")
			}
		}
	}
}

func TestRollExtremes(t *testing.T) {
	if Roll(0, 0, false) != Awful || Roll(20, 4294967295, false) != Masterwork || Roll(20, 4294967295, true) != Legendary {
		t.Fatal("extremes")
	}
	if Roll(-5, 0, false) != Roll(0, 0, false) || Roll(99, 7, true) != Roll(20, 7, true) {
		t.Fatal("clamp")
	}
}

func TestDistributionMatchesTable(t *testing.T) {
	const n = 20000
	for _, insp := range []bool{false, true} {
		for s := 0; s <= MaxSkill; s++ {
			var got [numLevels]int
			for i := 0; i < n; i++ {
				got[Roll(s, uint32(uint64(i)*(1<<32)/n), insp)]++
			}
			sum := 0
			for _, l := range All() {
				want := Share(s, insp, l) * n / sumShares(s, insp)
				if d := got[l] - want; d < -n/500 || d > n/500 {
					t.Fatalf("skill %d insp %v %v: got %d want %d", s, insp, l, got[l], want)
				}
				sum += Share(s, insp, l)
			}
			if sum < 9990 || sum > 10010 {
				t.Fatalf("skill %d shares sum %d", s, sum)
			}
		}
	}
}

func sumShares(s int, insp bool) int {
	n := 0
	for _, l := range All() {
		n += Share(s, insp, l)
	}
	return n
}

func expected(s int, insp bool) int {
	e := 0
	for _, l := range All() {
		e += int(l) * Share(s, insp, l)
	}
	return e
}

func TestMonotonicAndInspiration(t *testing.T) {
	for s := 0; s <= MaxSkill; s++ {
		if s > 0 && expected(s, false) < expected(s-1, false) {
			t.Fatalf("skill %d lowers expectation", s)
		}
		if expected(s, true) <= expected(s, false) {
			t.Fatalf("inspiration does not help at %d", s)
		}
	}
	for s := 0; s <= MaxSkill; s++ {
		for r := uint32(0); r < 1<<32-1<<22; r += 1 << 22 {
			if Roll(s, r, true) < Roll(s, r, false) {
				t.Fatalf("inspired roll lower at %d/%d", s, r)
			}
		}
	}
}

func TestFactorsComplete(t *testing.T) {
	for s := Stat(0); s < numStats; s++ {
		prev := -1 << 30
		for _, l := range All() {
			f := Factor(l, s)
			if s == Deterioration {
				f = -f
			}
			if f == 0 || f < prev {
				t.Fatalf("stat %d level %v factor %d not ascending/nonzero", s, l, f)
			}
			prev = f
		}
		if Factor(Normal, s) != 1000 {
			t.Fatalf("stat %d normal not 1000", s)
		}
	}
	if Factor(99, MarketValue) != 1000 || Factor(Good, 99) != 1000 {
		t.Fatal("unknown fallback")
	}
	if Apply(200, Legendary, MarketValue) != 1000 {
		t.Fatal("apply")
	}
}

func TestLabels(t *testing.T) {
	for _, l := range All() {
		if l.String() == "" || l.String() == "unknown" {
			t.Fatal(l)
		}
	}
	if Level(50).String() != "unknown" || Default != Normal {
		t.Fatal("misc")
	}
}

func TestInspiredShift(t *testing.T) {
	if Share(20, true, Legendary) != 6013 || Share(20, true, Awful) != 0 || Share(20, true, Poor) != 0 {
		t.Fatal(Share(20, true, Legendary))
	}
}
