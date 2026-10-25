package lighting

import "testing"

func TestCell(t *testing.T) {
	if Cell(0.9, false, 0.2) != 0.9 || Cell(0.9, true, 0.2) != 0.2 || Cell(0.1, false, 0.6) != 0.6 {
		t.Fatal("cell light")
	}
	if Cell(0.5, true, 3) != 1 || Cell(0.5, true, -1) != 0 {
		t.Fatal("clamped")
	}
}
