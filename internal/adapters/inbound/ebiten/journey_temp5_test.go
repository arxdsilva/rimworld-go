package ebiten

// journey_temp5_test.go: the TEMP-5 journeys: the temperature overlay key,
// the vent pane's button and the climate chip.

import (
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/inputframe"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/platform/input"
)

// TestJourneyTemperatureOverlay (TEMP-5): Shift+T shows and hides the
// temperature overlay, and a frame draws with it on.
func TestJourneyTemperatureOverlay(t *testing.T) {
	t.Parallel()
	j := newJourney(t)
	ov := &j.a.renderer.overlays().Temp
	if ov.On {
		t.Fatal("the temperature overlay starts on")
	}
	j.key("T", input.ModShift)
	if !ov.On {
		t.Fatal("Shift+T did not show the temperature overlay")
	}
	j.draw(2)
	j.key("T", input.ModShift)
	if ov.On {
		t.Fatal("Shift+T again did not hide it")
	}
}

// TestJourneyVentPaneToggle (TEMP-5): a built vent's pane has an Open
// button that sends ToggleVent for that vent.
func TestJourneyVentPaneToggle(t *testing.T) {
	t.Parallel()
	j := archJourney(t)
	for _, x := range []int{8, 10} {
		if !j.g.BuildNow("wall", "wood", "south", x, 3) {
			t.Fatalf("no wall at %d", x)
		}
	}
	if !j.g.BuildNow("vent", "steel", "south", 9, 3) {
		t.Fatal("no vent")
	}
	vent, ok := t1Building(j, "vent", 9, 3)
	if !ok {
		t.Fatal("the vent has no view")
	}
	j.clickTile(9, 3, inputframe.Left, 0)
	if !j.clickID("cmd.vent.toggle") {
		t.Fatal("cmd.vent.toggle disabled")
	}
	c, ok := j.lastCmd(command.ToggleVent)
	if !ok || c.Enabled == vent.Open || c.Target != uint64(vent.ID) {
		t.Fatalf("ToggleVent %+v ok=%v, want the opposite of open=%v for vent %d", c, ok, vent.Open, vent.ID)
	}
}
