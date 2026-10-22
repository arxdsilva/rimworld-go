package noise

import (
	"testing"

	"rimworld-go/pkg/randx"
)

func TestValueDeterministic(t *testing.T) {
	t.Parallel()
	a := New(randx.New(9), 40, 30, 7)
	b := New(randx.New(9), 40, 30, 7)
	for y := 0; y < 30; y++ {
		for x := 0; x < 40; x++ {
			if a.At(x, y) != b.At(x, y) {
				t.Fatalf("(%d,%d) differs: %v vs %v", x, y, a.At(x, y), b.At(x, y))
			}
		}
	}
}

func TestValueInUnitRangeAndVaries(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		w, h int
		cell float64
	}{
		{"fine", 50, 40, 4},
		{"coarse", 80, 60, 24},
		{"cell larger than map", 10, 10, 32},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			n := New(randx.New(3), tc.w, tc.h, tc.cell)
			lo, hi := 2.0, -1.0
			for y := 0; y < tc.h; y++ {
				for x := 0; x < tc.w; x++ {
					v := n.At(x, y)
					if v < 0 || v > 1 {
						t.Fatalf("At(%d,%d) = %v outside [0,1]", x, y, v)
					}
					lo, hi = min(lo, v), max(hi, v)
				}
			}
			if hi-lo < 0.01 {
				t.Fatalf("noise is flat: [%v, %v]", lo, hi)
			}
		})
	}
}

func TestValueIsSmooth(t *testing.T) {
	t.Parallel()
	n := New(randx.New(5), 60, 60, 12)
	for y := 0; y < 60; y++ {
		for x := 1; x < 60; x++ {
			if d := n.At(x, y) - n.At(x-1, y); d > 0.25 || d < -0.25 {
				t.Fatalf("jump of %v between (%d,%d) and (%d,%d)", d, x-1, y, x, y)
			}
		}
	}
}

func TestValueDrawsOneLatticePointPerCell(t *testing.T) {
	t.Parallel()
	// The lattice draw count is part of the contract: callers that share
	// one RNG across several layers rely on it to keep their output stable.
	r := randx.New(11)
	New(r, 20, 10, 5) // (20/5+2) * (10/5+2) = 24 draws
	ref := randx.New(11)
	for i := 0; i < 24; i++ {
		ref.IntN(1 << 20)
	}
	if r.IntN(1<<30) != ref.IntN(1<<30) {
		t.Fatal("New consumed an unexpected number of RNG draws")
	}
}
