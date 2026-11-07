package overlay

import (
	"math"
	"testing"
)

// TestBracketGeomFollowsTheCell (UIS-092): an arm is a third of a cell and
// the stroke max(1.5, cell/14), so the brackets follow the zoom.
func TestBracketGeomFollowsTheCell(t *testing.T) {
	t.Parallel()
	for _, c := range []struct{ cell, arm, stroke float64 }{
		{9, 3, 1.5}, {22.5, 7.5, 1.6071}, {49, 16.333, 3.5},
	} {
		arm, stroke := BracketGeom(c.cell)
		if math.Abs(arm-c.arm) > 1e-3 || math.Abs(stroke-c.stroke) > 1e-3 {
			t.Errorf("cell %v: arm %v stroke %v, want %v %v", c.cell, arm, stroke, c.arm, c.stroke)
		}
	}
}
