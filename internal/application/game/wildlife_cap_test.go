package game

import (
	"testing"

	"rimworld-go/internal/domain/animal"
)

// TestWildlifeCapCurve (NG-1 §6.6, D15): linear in area up to 120² (small
// maps unchanged), √area beyond, never above 300, continuous and
// non-decreasing.
func TestWildlifeCapCurve(t *testing.T) {
	for _, density := range []int{0, 5, 12, 25, 40, 100} {
		for _, sz := range [][2]int{{48, 40}, {80, 60}, {120, 120}} {
			a := sz[0] * sz[1]
			if got, want := animal.WildlifeCapacity(density, a), density*a/10000; got != want {
				t.Fatalf("density %d %dx%d: %d, want linear %d", density, sz[0], sz[1], got, want)
			}
		}
		prev := 0
		for side := 10; side <= 600; side++ {
			c := animal.WildlifeCapacity(density, side*side)
			if c < prev {
				t.Fatalf("density %d: cap fell from %d to %d at %d²", density, prev, c, side)
			}
			if c > animal.WildlifeMax {
				t.Fatalf("density %d: cap %d above %d at %d²", density, c, animal.WildlifeMax, side)
			}
			if side == 121 && c-prev > 1+density/50 {
				t.Fatalf("density %d: jump %d -> %d past 120²", density, prev, c)
			}
			prev = c
		}
		// √area: 250² carries density × 1.44 × 250/120 = 3·density, 500² twice that
		if got, want := animal.WildlifeCapacity(density, 250*250), min(animal.WildlifeMax, 3*density); got != want {
			t.Fatalf("density %d at 250²: %d, want %d", density, got, want)
		}
		if got, want := animal.WildlifeCapacity(density, 500*500), min(animal.WildlifeMax, 6*density); got != want {
			t.Fatalf("density %d at 500²: %d, want %d", density, got, want)
		}
	}
	for n := 0; n < 5000; n++ {
		r := animal.FloorSqrt(n)
		if r*r > n || (r+1)*(r+1) <= n {
			t.Fatalf("animal.FloorSqrt(%d) = %d", n, r)
		}
	}
}
