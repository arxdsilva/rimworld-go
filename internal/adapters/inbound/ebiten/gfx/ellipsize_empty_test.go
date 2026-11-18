package gfx

import "testing"

// An empty string stays empty however narrow the box: a checkbox with no
// label once drew a lone "…" beside itself (PLG-070 review).
func TestEllipsizeEmptyStaysEmpty(t *testing.T) {
	for _, w := range []float64{-20, 0, 5, 100} {
		if got := Ellipsize("", Body, w); got != "" {
			t.Fatalf("Ellipsize(\"\", %v) = %q", w, got)
		}
	}
}
