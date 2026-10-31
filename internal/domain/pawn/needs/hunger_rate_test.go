package needs

import "testing"

// TestHungerRate: offsets scale the rate additively and never below 0.
func TestHungerRate(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ hr, off, want int32 }{
		{1000, 0, 1000}, {1000, StonedHunger, 1300}, {1500, 1000, 3000}, {1000, -2000, 0},
	} {
		if got := HungerRate(tc.hr, tc.off); got != tc.want {
			t.Errorf("HungerRate(%d, %d) = %d, want %d", tc.hr, tc.off, got, tc.want)
		}
	}
}

// TestImmunityGainByBand (wiki: Saturation): Fed and Hungry x100 %,
// Ravenously hungry x90 %, Malnourished x70 %.
func TestImmunityGainByBand(t *testing.T) {
	t.Parallel()
	for b, want := range map[Band]int32{Fed: 1000, Hungry: 1000, Ravenous: 900, Malnourished: 700} {
		if got := ImmunityGain(b); got != want {
			t.Errorf("band %d: %d, want %d", b, got, want)
		}
	}
}

// TestImmunityGainNoFlap: food hovering just around the ravenous edge
// (eating a crumb, losing it again) keeps the immunity factor steady once
// the pawn is ravenous: the band leaves only BandGap past its edge.
func TestImmunityGainNoFlap(t *testing.T) {
	t.Parallel()
	p := Pair{Hunger: RavenousAt + 1}.Settle()
	if ImmunityGain(p.Band) != 1000 {
		t.Fatal("hungry above the edge: full gain")
	}
	p.Hunger = RavenousAt
	p = p.Settle()
	changes, last := 0, ImmunityGain(p.Band)
	for i := range 200 {
		p.Hunger = RavenousAt - BandGap/2 + i%2*BandGap // swings across the edge, inside the gap
		p = p.Settle()
		if g := ImmunityGain(p.Band); g != last {
			changes, last = changes+1, g
		}
	}
	if changes != 0 || last != 900 {
		t.Fatalf("immunity flapped %d times (last %d)", changes, last)
	}
	p.Hunger = RavenousAt + BandGap + 1
	if ImmunityGain(p.Settle().Band) != 1000 {
		t.Fatal("past the gap: full gain again")
	}
}
