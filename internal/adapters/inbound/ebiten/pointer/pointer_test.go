package pointer

import (
	"math"
	"testing"
)

// TestPlacementHotspot: the 64 px master draws 32 logical px wide and
// its hotspot (7, 5 canvas px) lands on the pointer.
func TestPlacementHotspot(t *testing.T) {
	t.Parallel()
	x, y, k := Placement(100, 50, [2]float64{7, 5}, 64)
	if k != 0.5 {
		t.Fatalf("k = %v, want 0.5", k)
	}
	if math.Abs(x-96.5) > 1e-9 || math.Abs(y-47.5) > 1e-9 {
		t.Fatalf("corner (%v, %v), want (96.5, 47.5)", x, y)
	}
	// the hotspot, scaled, is back on the pointer
	if hx, hy := x+7*k, y+5*k; hx != 100 || hy != 50 {
		t.Fatalf("hotspot at (%v, %v)", hx, hy)
	}
	// a 32 px master (no @2x) draws unscaled
	if _, _, k := Placement(0, 0, [2]float64{3, 2}, 32); k != 1 {
		t.Fatalf("k = %v for a 32 px canvas", k)
	}
}
