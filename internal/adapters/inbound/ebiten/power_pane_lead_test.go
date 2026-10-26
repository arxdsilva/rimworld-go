package ebiten

import (
	"slices"
	"testing"

	"rimworld-go/internal/application/game"
)

// PLAY-021: the pane says whether a thermostat is working now, and a
// solar panel or wind turbine its peak beside what it makes now.
func TestPowerPaneThermostatStateAndPeak(t *testing.T) {
	t.Parallel()
	w := game.WorldView{}
	h := pwrHeater()
	h.ThermoActive = true
	if got := powerLines(h, w); !slices.Contains(got, "Heating now") {
		t.Errorf("active heater lines %q", got)
	}
	h.ThermoActive = false
	if got := powerLines(h, w); !slices.Contains(got, "Idle (at target)") {
		t.Errorf("idle heater lines %q", got)
	}
	c := pwrHeater()
	c.Kind, c.ThermoActive = "cooler", true
	if got := powerLines(c, w); !slices.Contains(got, "Cooling now") {
		t.Errorf("active cooler lines %q", got)
	}

	s := pwrGenerator()
	s.Kind, s.PowerOut, s.PowerMax = "solar panel", 450, 900
	if got := powerLines(s, w); !slices.Contains(got, "Up to 900 W") {
		t.Errorf("solar panel lines %q", got)
	}
	gen := pwrGenerator()
	gen.PowerMax = gen.PowerOut
	if got := powerLines(gen, w); slices.ContainsFunc(got, func(l string) bool { return len(l) > 6 && l[:6] == "Up to " }) {
		t.Errorf("a steady generator needs no peak line: %q", got)
	}
}
