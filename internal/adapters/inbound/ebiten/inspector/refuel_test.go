package inspector

import (
	"strings"
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/lang"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/command"
)

func TestRefuelToggleShownForBurnersOnly(t *testing.T) {
	t.Parallel()
	tr := lang.English{}
	for _, tc := range []struct{ kind, icon string }{
		{"campfire", "wood"}, {"stove", "wood"}, {"generator", "wood"}, {"chemfuel generator", "chemfuel"},
	} {
		for _, auto := range []bool{true, false} {
			b := game.BuildingView{ID: 9, Kind: tc.kind, Fuel: true, FuelPct: 50, Refuelable: true, AutoRefuel: auto}
			c := autoRefuelCommand(tr, b)
			if c.ID != AutoRefuelID || !c.Toggle || c.On != auto || c.Label != "Auto refuel" {
				t.Errorf("%s auto=%v: button = %+v", tc.kind, auto, c)
			}
			if c.Icon != tc.icon {
				t.Errorf("%s icon = %q, want %q", tc.kind, c.Icon, tc.icon)
			}
			want := command.Command{Kind: command.SetAutoRefuel, Target: 9, Enabled: !auto}
			if len(c.Cmds) != 1 || c.Cmds[0] != want {
				t.Errorf("%s cmds = %+v, want %+v", tc.kind, c.Cmds, want)
			}
			line := fuelLine(tr, b)
			if has := strings.Contains(line, "auto refuel off"); has == auto {
				t.Errorf("%s auto=%v: fuel line %q", tc.kind, auto, line)
			}
		}
	}
	// a non-refuelable burner (fish tank, bed) never says it
	if line := fuelLine(tr, game.BuildingView{Kind: "bed", Fuel: true, FuelPct: 0}); strings.Contains(line, "auto refuel") {
		t.Errorf("fuel line = %q", line)
	}
}
