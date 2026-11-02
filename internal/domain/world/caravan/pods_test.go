package caravan

import "testing"

// TestPodRangeAndFuel: a full launcher (150 chemfuel) sends one pod the
// wiki's 66 tiles at 2.25 a tile; two pods on the same fuel go half as far.
func TestPodRangeAndFuel(t *testing.T) {
	if r := PodRange(LauncherFuelCap, 1); r != 66 {
		t.Fatalf("range on a full tank %d, want 66", r)
	}
	if r := PodRange(LauncherFuelCap, 2); r != 33 {
		t.Fatalf("two pods' range %d, want 33", r)
	}
	if r := PodRange(9, 1); r != 4 {
		t.Fatalf("9 chemfuel reach %d tiles, want 4", r)
	}
	if f := PodFuel(10, 1); f != 23 {
		t.Fatalf("10 tiles burn %d chemfuel, want 23 (22.5 rounded up)", f)
	}
	if PodRange(0, 1) != 0 || PodRange(100, 0) != 0 {
		t.Fatal("no fuel or no pod flies nowhere")
	}
}
