package creature

import (
	"math"
	"testing"
)

// Breathing (B.4) moves the top of every body by at least breatheBase
// tiles (about a pixel at 64 px tiles), a rat's as well as an elephant's.
func TestBreathingShowsOnEverySize(t *testing.T) {
	t.Parallel()
	for _, h := range []float64{0.45, 1.0, 2.2, 4.0} {
		lo, hi := math.Inf(1), math.Inf(-1)
		for p := uint64(0); p < 400; p++ {
			k := Breathe(h, p, 3)
			lo, hi = math.Min(lo, k), math.Max(hi, k)
		}
		if amp := (hi - lo) / 2 * h; amp < BreatheBase-1e-9 || amp > 0.1 {
			t.Fatalf("a body %.2f tiles tall breathes %.3f tiles", h, amp)
		}
	}
}
