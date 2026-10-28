package ebiten

import (
	"slices"
	"strings"
	"testing"

	"rimworld-go/internal/application/game"
)

// PLAY-021 (POWERPANE): the Brownout, Not connected and widened Out of wood
// alerts.

func alertByKeyPrefix(als []alert, prefix string) []alert {
	var out []alert
	for _, a := range als {
		if a.key == prefix || strings.HasPrefix(a.key, prefix+".") {
			out = append(out, a)
		}
	}
	return out
}

func TestBrownoutAlert(t *testing.T) {
	t.Parallel()
	w := game.WorldView{
		PowerNets: []game.PowerNetView{{ID: 3, Gen: 800, Load: 800, Brownout: true}, {ID: 9, Gen: 900, Load: 100}},
		Buildings: []game.BuildingView{
			{ID: 1, Kind: "electric stove", X: 4, Y: 5, PowerRole: "consumer", PowerNet: 3, Powered: true, PowerUse: 250},
			{ID: 2, Kind: "heater", X: 8, Y: 9, PowerRole: "consumer", PowerNet: 3, NoPowerReason: game.NoPowerBrownout},
			{ID: 3, Kind: "cooler", X: 1, Y: 2, PowerRole: "consumer", PowerNet: 3, NoPowerReason: game.NoPowerBrownout},
			{ID: 4, Kind: "lamp", X: 6, Y: 6, PowerRole: "consumer", PowerNet: 3, Powered: true, PowerUse: 50},
			{ID: 5, Kind: "heater", X: 12, Y: 12, PowerRole: "consumer", PowerNet: 9, Powered: true, PowerUse: 180},
			{ID: 6, Kind: "heater", X: 14, Y: 14, PowerRole: "consumer", PowerNet: 9, NoPowerReason: game.NoPowerSwitchedOff},
		},
	}
	got := alertByKeyPrefix(buildAlerts(nil, w), "brownout")
	if len(got) != 1 {
		t.Fatalf("brownout alerts = %+v", got)
	}
	al := got[0]
	if !al.danger {
		t.Error("a brownout is a danger")
	}
	if !strings.Contains(al.label, "Brownout") || !strings.Contains(al.label, "electric stove") {
		t.Errorf("label %q should say brownout and name the biggest consumer", al.label)
	}
	if !slices.Equal(al.targets, [][2]int{{8, 9}, {1, 2}}) {
		t.Errorf("targets = %v, want the shed heater and cooler", al.targets)
	}
	if al.explain == "" || al.explain == "Select this alert to look at the cause." {
		t.Errorf("explain = %q", al.explain)
	}
	// dangers come first
	if all := buildAlerts(nil, w); len(all) == 0 || !strings.HasPrefix(all[0].key, "brownout") {
		t.Errorf("order = %+v", all)
	}
}

func TestBrownoutAlertPerNetAndWithoutAConsumerName(t *testing.T) {
	t.Parallel()
	w := game.WorldView{
		PowerNets: []game.PowerNetView{{ID: 3, Brownout: true}, {ID: 9, Brownout: true}},
		Buildings: []game.BuildingView{
			{ID: 2, Kind: "heater", X: 8, Y: 9, PowerRole: "consumer", PowerNet: 3, NoPowerReason: game.NoPowerBrownout},
			{ID: 6, Kind: "cooler", X: 14, Y: 14, PowerRole: "consumer", PowerNet: 9, NoPowerReason: game.NoPowerBrownout},
		},
	}
	got := alertByKeyPrefix(buildAlerts(nil, w), "brownout")
	if len(got) != 2 || got[0].key == got[1].key {
		t.Fatalf("one alert per net, with its own key: %+v", got)
	}
	for _, al := range got {
		if !strings.Contains(al.label, "Brownout") || len(al.targets) != 1 {
			t.Errorf("alert %+v", al)
		}
	}
	if !strings.Contains(got[0].label, "grid") || got[0].label == got[1].label {
		t.Errorf("two grids need telling apart: %q / %q", got[0].label, got[1].label)
	}
	// no net in brownout, no alert
	w.PowerNets = []game.PowerNetView{{ID: 3}, {ID: 9}}
	if got := alertByKeyPrefix(buildAlerts(nil, w), "brownout"); len(got) != 0 {
		t.Fatalf("alerts without a brownout: %+v", got)
	}
}

func TestNotConnectedAlert(t *testing.T) {
	t.Parallel()
	w := game.WorldView{Buildings: []game.BuildingView{
		{ID: 1, Kind: "heater", X: 2, Y: 3, PowerRole: "consumer", NoPowerReason: game.NoPowerNotConnected},
		{ID: 2, Kind: "heater", X: 5, Y: 6, PowerRole: "consumer", PowerNet: 4, Powered: true},
		{ID: 3, Kind: "heater", X: 7, Y: 7, PowerRole: "consumer", PowerNet: 4, NoPowerReason: game.NoPowerSwitchedOff},
	}}
	got := alertByKeyPrefix(buildAlerts(nil, w), "notconnected")
	if len(got) != 1 {
		t.Fatalf("alerts = %+v", got)
	}
	al := got[0]
	if al.danger || al.label != "Heater not connected" || !slices.Equal(al.targets, [][2]int{{2, 3}}) {
		t.Errorf("single device: %+v", al)
	}
	// several devices, any role (a generator and a battery count too)
	w.Buildings = append(w.Buildings,
		game.BuildingView{ID: 4, Kind: "battery", X: 9, Y: 9, PowerRole: "storage", NoPowerReason: game.NoPowerNotConnected},
		game.BuildingView{ID: 5, Kind: "solar panel", X: 10, Y: 10, PowerRole: "producer", PowerOut: 600, NoPowerReason: game.NoPowerNotConnected})
	al = alertByKeyPrefix(buildAlerts(nil, w), "notconnected")[0]
	if al.label != "3 devices not connected" || !slices.Equal(al.targets, [][2]int{{2, 3}, {9, 9}, {10, 10}}) {
		t.Errorf("three devices: %+v", al)
	}
	if al.explain == "" || al.explain == "Select this alert to look at the cause." {
		t.Errorf("explain = %q", al.explain)
	}
	// a colony with all devices connected raises nothing
	w.Buildings = w.Buildings[1:3]
	if got := alertByKeyPrefix(buildAlerts(nil, w), "notconnected"); len(got) != 0 {
		t.Fatalf("alert without a loose device: %+v", got)
	}
}

func TestNoFuelAlertCoversGeneratorsAndTorches(t *testing.T) {
	t.Parallel()
	camp := game.BuildingView{ID: 1, Kind: "campfire", X: 1, Y: 1, Fuel: true}
	torch := game.BuildingView{ID: 2, Kind: "torch lamp", X: 2, Y: 2, Fuel: true}
	gen := game.BuildingView{ID: 3, Kind: "wood-fired generator", X: 3, Y: 3, Fuel: true, PowerRole: "producer", NoPowerReason: game.NoPowerNoFuel}
	full := game.BuildingView{ID: 4, Kind: "torch lamp", X: 4, Y: 4, Fuel: true, FuelPct: 60}
	one := func(bs ...game.BuildingView) alert {
		t.Helper()
		got := alertByKeyPrefix(buildAlerts(nil, game.WorldView{Buildings: bs}), "nofuel")
		if len(got) != 1 {
			t.Fatalf("nofuel alerts for %d buildings = %+v", len(bs), got)
		}
		return got[0]
	}
	if al := one(camp, full); al.label != "Campfire out of wood" || len(al.targets) != 1 {
		t.Errorf("campfire alone: %+v", al)
	}
	if al := one(torch); al.label != "Torch lamp out of wood" || !slices.Equal(al.targets, [][2]int{{2, 2}}) {
		t.Errorf("torch alone: %+v", al)
	}
	if al := one(gen); al.label != "Wood-fired generator out of wood" || !slices.Equal(al.targets, [][2]int{{3, 3}}) {
		t.Errorf("generator alone: %+v", al)
	}
	if al := one(camp, torch); al.label != "Out of wood (2)" || len(al.targets) != 2 || al.danger {
		t.Errorf("a campfire and a torch: %+v", al)
	}
	two := camp
	two.ID, two.X = 9, 9
	if al := one(camp, two); al.label != "2 campfires out of wood" {
		t.Errorf("two campfires: %+v", al)
	}
	// a generator that reports no fuel but whose view lacks the Fuel flag still counts
	bare := gen
	bare.Fuel = false
	if al := one(bare); len(al.targets) != 1 {
		t.Errorf("generator by reason: %+v", al)
	}
	// and it is counted once
	if al := one(gen, camp); len(al.targets) != 2 || al.label != "Out of wood (2)" {
		t.Errorf("generator and campfire: %+v", al)
	}
	if got := alertByKeyPrefix(buildAlerts(nil, game.WorldView{Buildings: []game.BuildingView{full}}), "nofuel"); len(got) != 0 {
		t.Errorf("a torch with fuel raises %+v", got)
	}
	if e := alertExplain("nofuel"); !strings.Contains(e, "generator") || !strings.Contains(e, "torch") {
		t.Errorf("explanation = %q", e)
	}
}

// The alert memo is keyed on the power data too, so an alert follows a net
// that browns out between two ticks.
func TestAlertsKeyFollowsPowerData(t *testing.T) {
	t.Parallel()
	a, g, _ := liveColony(t, 0)
	_ = g
	r := a.renderer
	w := r.world()
	k0 := newAlertsKey(r, r.game.Snapshot(), w)
	w1 := w
	w1.PowerVer = w.PowerVer + 1
	if newAlertsKey(r, r.game.Snapshot(), w1) == k0 {
		t.Fatal("PowerVer is not in the key")
	}
	w2 := w
	w2.PowerNets = []game.PowerNetView{{ID: 1, Brownout: true}}
	if newAlertsKey(r, r.game.Snapshot(), w2) == k0 {
		t.Fatal("the nets are not in the key")
	}
	w3 := w
	w3.SolarFlare = !w.SolarFlare
	if newAlertsKey(r, r.game.Snapshot(), w3) == k0 {
		t.Fatal("the flare is not in the key")
	}
}

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
	a.selectCand(selCand{Kind: selBuilding, ID: heater.ID})
	if got := bldLines(a.buildingPane(heater.ID)); !strings.Contains(got, "No power: not connected") {
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
