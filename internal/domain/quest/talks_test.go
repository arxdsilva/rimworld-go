package quest

import (
	"math/rand/v2"
	"testing"
)

// publishedOdds are QST-096's odds in tenths of a percent, as published
// (the Social 12 row sums to 99.9 %).
func publishedOdds() map[int][5]int {
	return map[int][5]int{
		0:  {42, 230, 235, 329, 164},
		3:  {57, 316, 242, 257, 128},
		6:  {80, 438, 231, 167, 84},
		9:  {105, 575, 190, 87, 43},
		12: {119, 655, 152, 49, 24},
		15: {133, 729, 106, 21, 11},
	}
}

func TestNegotiate_AnchorsOver200kRolls(t *testing.T) {
	const n = 200_000
	for social, want := range publishedOdds() {
		r := rand.New(rand.NewPCG(uint64(social), 96))
		var got [5]int
		for range n {
			got[Negotiate(social, r.IntN(1000), r.IntN(1000)).Result]++
		}
		for k := range got {
			// ±0.5 % is ±5 permille: |got/n×1000 − want| ≤ 5.
			diff := got[k]*1000 - want[k]*n
			if diff < 0 {
				diff = -diff
			}
			if diff > 5*n {
				t.Errorf("social %d result %d: %.2f %%, want %.1f %% ± 0.5", social, k, float64(got[k])*100/n, float64(want[k])/10)
			}
		}
	}
}

func TestNegotiate_ExactOverEveryRoll(t *testing.T) {
	for social, want := range publishedOdds() {
		var got [5]int
		for roll := range 1000 {
			got[Negotiate(social, roll, 0).Result]++
		}
		for k := range got {
			if d := got[k] - want[k]; d < -1 || d > 1 {
				t.Errorf("social %d result %d: %d of 1000 rolls, want %d", social, k, got[k], want[k])
			}
		}
	}
}

func TestNegotiate_LinearAndMonotone(t *testing.T) {
	prev := -1
	for social := -3; social <= 20; social++ {
		best := 0
		for roll := range 1000 {
			if r := Negotiate(social, roll, 0).Result; r <= TalkSuccess {
				best++
			}
		}
		if best < prev {
			t.Fatalf("social %d: good outcomes %d fell from %d", social, best, prev)
		}
		prev = best
	}
	// Social 1 is a third of the way from 0 to 3: Triumph 42 + (57−42)/3 = 47.
	tri := 0
	for roll := range 1000 {
		if Negotiate(1, roll, 0).Result == TalkTriumph {
			tri++
		}
	}
	if tri != 47 {
		t.Fatalf("social 1 triumph = %d, want 47", tri)
	}
}

func TestNegotiate_Outcomes(t *testing.T) {
	cases := []struct {
		res        TalkResult
		lo, hi     int
		gLo, gHi   int
		ambush     bool
		roll       int
		socialSkew int
	}{
		{TalkTriumph, 100, 110, 500, 1200, false, 0, 15},
		{TalkSuccess, 60, 70, 0, 0, false, 200, 15},
		{TalkFlounder, 0, 0, 0, 0, false, 900, 15},
		{TalkBackfire, -20, -10, 0, 0, false, 980, 15},
		{TalkDisaster, -50, -40, 0, 0, true, 999, 15},
	}
	for _, c := range cases {
		lo, hi := Negotiate(c.socialSkew, c.roll, 0), Negotiate(c.socialSkew, c.roll, 999)
		if lo.Result != c.res || hi.Result != c.res {
			t.Fatalf("roll %d: results %d %d, want %d", c.roll, lo.Result, hi.Result, c.res)
		}
		gw := []int{lo.Goodwill, hi.Goodwill}
		if min(gw[0], gw[1]) != c.lo || max(gw[0], gw[1]) != c.hi {
			t.Errorf("result %d goodwill %v, want %d..%d", c.res, gw, c.lo, c.hi)
		}
		if lo.Gift != c.gLo || hi.Gift != c.gHi {
			t.Errorf("result %d gift %d..%d, want %d..%d", c.res, lo.Gift, hi.Gift, c.gLo, c.gHi)
		}
		if lo.Ambush != c.ambush || hi.Ambush != c.ambush {
			t.Errorf("result %d ambush %v, want %v", c.res, lo.Ambush, c.ambush)
		}
	}
	// Out-of-range rolls are clamped, not wrapped.
	if Negotiate(15, -5, -5) != Negotiate(15, 0, 0) || Negotiate(0, 5000, 5000) != Negotiate(0, 999, 999) {
		t.Fatal("rolls out of [0, 1000) must clamp")
	}
}
