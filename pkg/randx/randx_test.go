package randx

import "testing"

func TestSameSeedProducesSameSequence(t *testing.T) {
	a := New(42)
	b := New(42)

	for i := 0; i < 20; i++ {
		x, y := a.IntN(1000), b.IntN(1000)
		if x != y {
			t.Fatalf("sequence diverged at index %d: %d != %d", i, x, y)
		}
	}
}

func TestDifferentSeedsCanDiffer(t *testing.T) {
	a := New(1)
	b := New(2)

	same := true
	for i := 0; i < 20; i++ {
		if a.IntN(1_000_000) != b.IntN(1_000_000) {
			same = false
			break
		}
	}
	if same {
		t.Fatal("different seeds produced an identical sequence across 20 draws")
	}
}

func TestIntNStaysInRange(t *testing.T) {
	r := New(7)
	for i := 0; i < 100; i++ {
		if v := r.IntN(5); v < 0 || v >= 5 {
			t.Fatalf("IntN(5) = %d, out of range", v)
		}
	}
}

func TestDeriveStreams(t *testing.T) {
	a, b := Derive(42, "plants"), Derive(42, "plants")
	c := Derive(42, "caves")
	same, diff := true, false
	for i := 0; i < 20; i++ {
		x, y, z := a.IntN(1<<30), b.IntN(1<<30), c.IntN(1<<30)
		same = same && x == y
		diff = diff || x != z
	}
	if !same || !diff {
		t.Fatalf("Derive: same name equal=%v, other name differs=%v", same, diff)
	}
	if f := Derive(1, "f").Float64(); f < 0 || f >= 1 {
		t.Fatalf("Float64 %v", f)
	}
}

// The values come from the copies these functions replaced (fire.Mix,
// pawngen.mix64). SplitMix64(0) is also the published first output of
// splitmix64 from seed 0.
func TestMixersNeverChange(t *testing.T) {
	t.Parallel()
	for _, c := range []struct{ in, split, mix uint64 }{
		{0x0, 0xe220a8397b1dcdaf, 0x0},
		{0x1, 0x910a2dec89025cc1, 0x5692161d100b05e5},
		{0x2a, 0xbdd732262feb6e95, 0xa759ea27d4727622},
		{0xdeadbeefcafef00d, 0x901d4f652fb472cb, 0x19104ae2406d51c3},
		{0xffffffffffffffff, 0xe4d971771b652c20, 0xb4d055fcf2cbbd7b},
	} {
		if got := SplitMix64(c.in); got != c.split {
			t.Errorf("SplitMix64(%#x) = %#x, want %#x", c.in, got, c.split)
		}
		if got := Mix64(c.in); got != c.mix {
			t.Errorf("Mix64(%#x) = %#x, want %#x", c.in, got, c.mix)
		}
	}
}
