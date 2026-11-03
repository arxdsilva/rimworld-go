package mathx

import (
	"math"
	"testing"
)

func TestAbs(t *testing.T) {
	t.Parallel()
	for v, want := range map[int]int{0: 0, 1: 1, -1: 1, 42: 42, -42: 42, math.MaxInt: math.MaxInt, -math.MaxInt: math.MaxInt, math.MinInt: math.MinInt} {
		if got := Abs(v); got != want {
			t.Errorf("Abs(%d) = %d, want %d", v, got, want)
		}
	}
}

func TestClamp(t *testing.T) {
	t.Parallel()
	for _, c := range []struct{ v, lo, hi, want int }{
		{5, 0, 10, 5},
		{-1, 0, 10, 0},
		{11, 0, 10, 10},
		{0, 0, 10, 0},
		{10, 0, 10, 10},
		{-5, -10, -1, -5},
		{7, 3, 3, 3},
		{5, 10, 0, 0}, // lo > hi: the documented result
	} {
		if got := Clamp(c.v, c.lo, c.hi); got != c.want {
			t.Errorf("Clamp(%d, %d, %d) = %d, want %d", c.v, c.lo, c.hi, got, c.want)
		}
	}
}
