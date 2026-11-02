package overlay

import (
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/units"
)

func TestTempLabelInFollowsTheUnit(t *testing.T) {
	if got := TempLabelIn(units.FromUnit(units.UnitF), 210); got != "70 °F" {
		t.Errorf("F: %q", got)
	}
	if TempLabelIn(units.Celsius, -5) != TempLabel(-5) || TempLabel(-5) != "-1 °C" {
		t.Errorf("C floors: %q", TempLabel(-5))
	}
}
