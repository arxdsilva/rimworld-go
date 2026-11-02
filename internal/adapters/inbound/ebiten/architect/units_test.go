package architect

import (
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/units"
)

func TestFmtTempInFollowsTheUnit(t *testing.T) {
	if got := FmtTempIn(units.FromUnit(units.UnitF), 0); got != "32F" {
		t.Errorf("F: %q", got)
	}
	if FmtTempIn(units.Celsius, -25) != "-3C" || FmtTempIn(units.Celsius, 25) != "3C" {
		t.Error("Celsius keeps rounding half away from zero")
	}
}
