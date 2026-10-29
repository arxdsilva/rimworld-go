package ebiten

import (
	"strings"
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/selection"
	"rimworld-go/internal/application/game"
)

// PLAY-021 (POWERPANE): the Brownout, Not connected and widened Out of wood
// alerts.

// With the real simulation (and its view cache), a heater nobody wired up
// raises "not connected", wiring it clears the alert, and the pane of the
// heater says what the sim says.
func TestNotConnectedAlertFollowsTheRealGrid(t *testing.T) {
	t.Parallel()
	a, g, _ := liveColony(t, 0)
	if !g.BuildNow("heater", "steel", "south", 8, 8) {
		t.Skip("the heater did not build on the test map")
	}
	a.renderer.speed = speedPaused
	has := func() (alert, bool) {
		for _, al := range a.currentAlerts() {
			if al.key == "notconnected" {
				return al, true
			}
		}
		return alert{}, false
	}
	al, ok := has()
	if !ok || al.label != "Heater not connected" || len(al.targets) != 1 || al.targets[0] != [2]int{8, 8} {
		t.Fatalf("alert = %+v ok=%v (alerts %+v)", al, ok, a.currentAlerts())
	}
	var heater game.BuildingView
	for _, b := range a.renderer.world().Buildings {
		if b.Kind == "heater" {
			heater = b
		}
	}
	a.sel.Select(selection.Cand{Kind: selection.Building, ID: heater.ID})
	if got := bldLines(a.insp.Building().BuildingPane(heater.ID)); !strings.Contains(got, "No power: not connected") {
		t.Fatalf("the heater's pane: %q", got)
	}
	// wire it: a conduit within reach, and a generator on the same line
	for _, c := range []struct {
		kind string
		x, y int
	}{{"power conduit", 8, 10}, {"power conduit", 9, 10}, {"battery", 10, 10}} {
		if !g.BuildNow(c.kind, "steel", "south", c.x, c.y) {
			t.Skipf("%s did not build at %d,%d", c.kind, c.x, c.y)
		}
	}
	g.Step(1)
	if al, ok := has(); ok {
		t.Fatalf("still not connected after wiring: %+v", al)
	}
}
