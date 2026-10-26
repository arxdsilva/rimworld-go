package power

import (
	"testing"

	"rimworld-go/internal/domain/sim/clock"
)

// TestUnitsAndRoleNames pins the contract the simulation and the views
// build on (PLAY-021): stored energy in W·ticks, an hour of 1 W per Wh,
// and the role names BuildingView.PowerRole shows.
func TestUnitsAndRoleNames(t *testing.T) {
	t.Parallel()
	if WTicksPerWh != clock.TicksPerHour || ConnectRange != 3 {
		t.Fatalf("units: %d W·ticks per Wh, range %d", WTicksPerWh, ConnectRange)
	}
	want := map[Role]string{None: "", Producer: "producer", Consumer: "consumer", Storage: "storage",
		Transmitter: "transmitter", Switch: "switch"}
	for r, s := range want {
		if r.String() != s {
			t.Errorf("Role(%d) = %q, want %q", r, r.String(), s)
		}
	}
	if !(ClassNone < ClassLamp && ClassLamp < ClassBench && ClassBench < ClassCooler && ClassCooler < ClassHeater) {
		t.Fatal("shed order: lamps, benches, coolers, heaters")
	}
}
