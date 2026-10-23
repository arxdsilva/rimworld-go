package noise3

import (
	"math"
	"testing"
)

func TestSameSeedSameValues(t *testing.T) {
	a, b := New(42), New(42)
	for i := 0; i < 200; i++ {
		x, y, z := float64(i)*0.37, float64(i)*-0.11, float64(i)*0.73
		if a.At(x, y, z) != b.At(x, y, z) || a.FBm(x, y, z, 6, 1.6, 0.5) != b.FBm(x, y, z, 6, 1.6, 0.5) {
			t.Fatalf("seed 42 diverged at sample %d", i)
		}
	}
}

func TestDifferentSeedsDiffer(t *testing.T) {
	a, b := New(1), New(2)
	same := 0
	for i := 0; i < 100; i++ {
		x := float64(i) * 0.53
		if a.At(x, 0.2, -x) == b.At(x, 0.2, -x) {
			same++
		}
	}
	if same > 5 {
		t.Fatalf("seeds 1 and 2 agree on %d of 100 samples", same)
	}
}

func TestRangeAndLatticeValues(t *testing.T) {
	f := New(7)
	for i := 0; i < 2000; i++ {
		x, y, z := math.Sin(float64(i))*9, math.Cos(float64(i)*1.3)*9, float64(i%37)-18
		for _, v := range []float64{f.At(x, y, z), f.FBm(x, y, z, 6, 1.6, 0.5), f.Ridged(x, y, z, 4, 2, 0.5)} {
			if v < 0 || v > 1 || math.IsNaN(v) {
				t.Fatalf("value %v out of [0,1] at %d", v, i)
			}
		}
	}
	// At integer points the noise is exactly the lattice value.
	if f.At(3, -2, 5) != f.lattice(3, -2, 5) {
		t.Fatal("At at a lattice point must equal the lattice value")
	}
}

func TestContinuous(t *testing.T) {
	f := New(99)
	for i := 0; i < 500; i++ {
		x, y, z := float64(i)*0.071, float64(i)*0.033, -float64(i)*0.052
		d := math.Abs(f.At(x, y, z) - f.At(x+1e-4, y, z))
		if d > 1e-3 {
			t.Fatalf("jump %v over a 1e-4 step at %d", d, i)
		}
	}
}

func TestFBmSpread(t *testing.T) {
	f := New(5)
	lo, hi := 1.0, 0.0
	for i := 0; i < 5000; i++ {
		th, ph := float64(i)*0.7, float64(i)*0.31
		v := f.FBm(math.Sin(th)*math.Cos(ph), math.Sin(th)*math.Sin(ph), math.Cos(th), 6, 1.6, 0.5)
		lo, hi = math.Min(lo, v), math.Max(hi, v)
	}
	if hi-lo < 0.4 {
		t.Fatalf("fBm on the sphere spans only [%.3f, %.3f]", lo, hi)
	}
	if New(1).FBm(0.1, 0.2, 0.3, 0, 1, 0.5) != 0 {
		t.Fatal("zero octaves must give 0")
	}
}

func BenchmarkFBm6(b *testing.B) {
	f := New(1)
	for i := 0; i < b.N; i++ {
		_ = f.FBm(0.3, float64(i%100)*0.01, 0.7, 6, 1.6, 0.5)
	}
}
