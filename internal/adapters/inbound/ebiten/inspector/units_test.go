package inspector

import (
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/units"
)

func TestFmtTargetInFollowsTheUnit(t *testing.T) {
	f := units.FromUnit(units.UnitF)
	if got := FmtTargetIn(f, 210); got != "69.8 °F" {
		t.Errorf("F: %q", got)
	}
	if got := FmtTargetIn(units.Celsius, 215); got != "21.5 °C" || got != FmtTarget(215) {
		t.Errorf("C: %q", got)
	}
	if d, u := thermoStepLabels(f); d != "-1.8 °F" || u != "+1.8 °F" {
		t.Errorf("steps %q %q", d, u)
	}
	if d, u := thermoStepLabels(units.Celsius); d != "-1 °C" || u != "+1 °C" {
		t.Errorf("steps %q %q", d, u)
	}
}
