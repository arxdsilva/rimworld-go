package ebiten

import (
	"strconv"
	"strings"
	"testing"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/platform/input"
)

// PLAY-021 (POWERPANE): a power building's pane lines, bars and command
// buttons, the V hotkey, the Solar flare chip and the bills' "no power".

// The grid every pane test sits on: 800 W made, 450 W used, a 1200 Wh
// battery holding 900.
var pwrNets = []game.PowerNetView{{ID: 7, Gen: 800, Load: 450, StoredWh: 900, CapWh: 1200}}

// pwrApp is a colony with the buildings bs on the grid pwrNets, building
// sel selected.
func pwrApp(t *testing.T, sel int, bs ...game.BuildingView) (*App, *stubSim, *fakeInput) {
	t.Helper()
	for i := range bs {
		if bs[i].W == 0 {
			bs[i].W, bs[i].H = 1, 1
		}
	}
	s := bldStub(bs...)
	s.world.PowerNets = pwrNets
	a, f := bldApp(t, s, bs[sel].ID)
	return a, s, f
}

func pwrHeater() game.BuildingView {
	return game.BuildingView{ID: 10, Kind: "heater", X: 4, Y: 4, PowerRole: "consumer", PowerNet: 7, Powered: true,
		PowerOn: true, PowerWantOn: true, PowerUse: 180, ThermoTarget: 210}
}

func pwrGenerator() game.BuildingView {
	return game.BuildingView{ID: 11, Kind: "wood-fired generator", X: 6, Y: 4, PowerRole: "producer", PowerNet: 7,
		PowerOn: true, PowerWantOn: true, PowerOut: 800, Fuel: true, FuelPct: 55, Burning: true}
}

func pwrBattery() game.BuildingView {
	return game.BuildingView{ID: 12, Kind: "battery", X: 8, Y: 4, PowerRole: "storage", PowerNet: 7, PowerOn: true, PowerWantOn: true,
		StoredWh: 900, CapWh: 1200}
}

func pwrSwitch() game.BuildingView {
	return game.BuildingView{ID: 13, Kind: "power switch", X: 2, Y: 2, PowerRole: "switch", PowerNet: 7, PowerOn: true, PowerWantOn: true}
}

func pwrConduit() game.BuildingView {
	return game.BuildingView{ID: 14, Kind: "power conduit", X: 3, Y: 3, PowerRole: "transmitter", PowerNet: 7, PowerOn: true, PowerWantOn: true}
}

func TestPowerPaneLines(t *testing.T) {
	t.Parallel()
	const grid = "Grid: +350 W (800 W made, 450 W used), stored 900 of 1200 Wh"
	with := func(b game.BuildingView, f func(*game.BuildingView)) game.BuildingView {
		f(&b)
		return b
	}
	cases := []struct {
		name      string
		b         game.BuildingView
		want, not []string
	}{
		{"powered consumer", pwrHeater(), []string{"Uses 180 W", "Target 21 °C", grid}, []string{"No power", "Flick pending"}},
		{"not connected", with(pwrHeater(), func(b *game.BuildingView) {
			b.Powered, b.PowerUse, b.PowerNet, b.NoPowerReason = false, 0, 0, game.NoPowerNotConnected
		}), []string{"No power: not connected"}, []string{"Uses", "Grid:"}},
		{"brownout", with(pwrHeater(), func(b *game.BuildingView) {
			b.Powered, b.PowerUse, b.NoPowerReason = false, 0, game.NoPowerBrownout
		}), []string{"No power: brownout", grid}, []string{"Uses"}},
		{"switched off", with(pwrHeater(), func(b *game.BuildingView) {
			b.Powered, b.PowerUse, b.PowerOn, b.PowerWantOn, b.NoPowerReason = false, 0, false, false, game.NoPowerSwitchedOff
		}), []string{"No power: switched off"}, []string{"Flick pending"}},
		{"solar flare", with(pwrHeater(), func(b *game.BuildingView) {
			b.Powered, b.PowerUse, b.NoPowerReason = false, 0, game.NoPowerSolarFlare
		}), []string{"No power: solar flare"}, nil},
		{"half degree target", with(pwrHeater(), func(b *game.BuildingView) { b.ThermoTarget = 215 }), []string{"Target 21.5 °C"}, nil},
		{"generator", pwrGenerator(), []string{"Produces 800 W", grid}, []string{"Not producing"}},
		{"generator without fuel", with(pwrGenerator(), func(b *game.BuildingView) {
			b.PowerOut, b.FuelPct, b.Burning, b.NoPowerReason = 0, 0, false, game.NoPowerNoFuel
		}), []string{"Not producing: no fuel", "Out of fuel"}, []string{"Produces 800"}},
		{"generator off the grid", with(pwrGenerator(), func(b *game.BuildingView) {
			b.PowerNet, b.NoPowerReason = 0, game.NoPowerNotConnected
		}), []string{"Produces 800 W", "Not connected:"}, []string{"Grid:"}},
		{"night panel", game.BuildingView{Kind: "solar panel", PowerRole: "producer", PowerNet: 7, PowerOn: true, PowerWantOn: true},
			[]string{"Produces 0 W"}, []string{"Not producing"}},
		{"battery", pwrBattery(), []string{"Holds 900 of 1200 Wh", grid}, nil},
		{"battery off the grid", with(pwrBattery(), func(b *game.BuildingView) {
			b.PowerNet, b.NoPowerReason = 0, game.NoPowerNotConnected
		}), []string{"Not connected:"}, nil},
		{"closed switch", pwrSwitch(), []string{"Closed (carries power)", grid}, []string{"Open", "Flick pending"}},
		{"open switch", with(pwrSwitch(), func(b *game.BuildingView) { b.PowerOn, b.PowerWantOn = false, false }),
			[]string{"Open (cuts the grid)"}, []string{"Closed"}},
		{"switch flick to open pending", with(pwrSwitch(), func(b *game.BuildingView) { b.PowerWantOn = false }),
			[]string{"Closed (carries power)", "Flick pending: a colonist will switch it off"}, nil},
		{"consumer flick to on pending", with(pwrHeater(), func(b *game.BuildingView) {
			b.Powered, b.PowerUse, b.PowerOn, b.NoPowerReason = false, 0, false, game.NoPowerSwitchedOff
		}), []string{"No power: switched off", "Flick pending: a colonist will switch it on"}, nil},
		{"conduit names its grid", pwrConduit(), []string{grid}, []string{"Uses", "Produces"}},
		{"a wall is not electrical", game.BuildingView{Kind: "wall"}, nil, []string{"Grid", "Uses", "Produces", "Flick"}},
	}
	for _, c := range cases {
		c.b.ID, c.b.W, c.b.H = 5, 1, 1
		if c.b.X == 0 {
			c.b.X, c.b.Y = 3, 3
		}
		s := bldStub(c.b)
		s.world.PowerNets = pwrNets
		a, _ := bldApp(t, s, 5)
		got := bldLines(a.buildingPane(5))
		for _, w := range c.want {
			if !strings.Contains(got, w) {
				t.Errorf("%s: lines %q lack %q", c.name, got, w)
			}
		}
		for _, n := range c.not {
			if strings.Contains(got, n) {
				t.Errorf("%s: lines %q hold %q", c.name, got, n)
			}
		}
	}
}

func TestPowerPaneGridLineVariants(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		net  game.PowerNetView
		want string
		more string
	}{
		{"no battery", game.PowerNetView{ID: 7, Gen: 800, Load: 450}, "Grid: +350 W (800 W made, 450 W used)", ""},
		{"deficit", game.PowerNetView{ID: 7, Gen: 100, Load: 450, StoredWh: 0, CapWh: 1200}, "Grid: -350 W (100 W made, 450 W used), stored 0 of 1200 Wh", ""},
		{"brownout", game.PowerNetView{ID: 7, Gen: 100, Load: 100, Brownout: true}, "Grid: +0 W (100 W made, 100 W used)", "Brownout: some devices are shed"},
	}
	for _, c := range cases {
		s := bldStub(pwrHeater())
		s.world.PowerNets = []game.PowerNetView{{ID: 99, Gen: 5}, c.net} // the right net is found by id
		a, _ := bldApp(t, s, 10)
		got := bldLines(a.buildingPane(10))
		if !strings.Contains(got, c.want) {
			t.Errorf("%s: lines %q lack %q", c.name, got, c.want)
		}
		if c.more != "" && !strings.Contains(got, c.more) {
			t.Errorf("%s: lines %q lack %q", c.name, got, c.more)
		}
		if c.more == "" && strings.Contains(got, "Brownout") {
			t.Errorf("%s: no brownout, yet %q", c.name, got)
		}
	}
	// a net the world does not list says nothing about a grid
	s := bldStub(pwrHeater())
	a, _ := bldApp(t, s, 10)
	if got := bldLines(a.buildingPane(10)); strings.Contains(got, "Grid:") {
		t.Errorf("an unknown net shows a grid line: %q", got)
	}
}

func TestPowerWarningLinesAreMarked(t *testing.T) {
	t.Parallel()
	for _, warn := range []string{"No power: brownout", "Not producing: no fuel", "Not connected: it makes power for nobody", "Brownout: some devices are shed"} {
		if !paneLineWarns(warn) {
			t.Errorf("%q should draw in the warning colour", warn)
		}
	}
	for _, calm := range []string{"Uses 180 W", "Produces 0 W", "Grid: +350 W (800 W made, 450 W used)", "Closed (carries power)", "Out of fuel", ""} {
		if paneLineWarns(calm) {
			t.Errorf("%q should draw in the body colour", calm)
		}
	}
}

func TestBatteryPaneChargeBar(t *testing.T) {
	t.Parallel()
	a, _, _ := pwrApp(t, 0, pwrBattery())
	bar := paneBarNamed(a.buildingPane(12), "Charge")
	if bar == nil || bar.Pct != 75 {
		t.Fatalf("charge bar = %+v in %+v", bar, a.buildingPane(12).Bars)
	}
	for stored, want := range map[int]int{0: 0, 1200: 100, 1500: 100, 600: 50} {
		b := pwrBattery()
		b.StoredWh = stored
		a, _, _ := pwrApp(t, 0, b)
		if bar := paneBarNamed(a.buildingPane(12), "Charge"); bar == nil || bar.Pct != want {
			t.Errorf("%d Wh: charge bar = %+v, want %d%%", stored, bar, want)
		}
	}
	// a building with no capacity has no charge bar
	if bar := paneBarNamed(paneOf(t, pwrHeater()), "Charge"); bar != nil {
		t.Errorf("a heater has a charge bar: %+v", bar)
	}
}

func paneOf(t *testing.T, b game.BuildingView) paneSpec {
	t.Helper()
	a, _, _ := pwrApp(t, 0, b)
	return a.buildingPane(b.ID)
}

func paneBarNamed(p paneSpec, label string) *paneBar {
	for i := range p.Bars {
		if p.Bars[i].Label == label {
			return &p.Bars[i]
		}
	}
	return nil
}

func TestFuelBarAndLinesForGeneratorAndTorch(t *testing.T) {
	t.Parallel()
	torch := game.BuildingView{ID: 20, Kind: "torch lamp", X: 5, Y: 5, Fuel: true, FuelPct: 30, Burning: true, Glow: true, GlowRadius: 4}
	cases := []struct {
		name string
		b    game.BuildingView
		pct  int
		want string
		not  string
	}{
		{"generator", pwrGenerator(), 55, "Burning", "Banked"},
		{"empty generator", func() game.BuildingView { b := pwrGenerator(); b.FuelPct, b.Burning = 0, false; return b }(), 0, "Out of fuel", "Banked"},
		{"torch", torch, 30, "Burning", "Banked"},
		{"empty torch", func() game.BuildingView { b := torch; b.FuelPct, b.Burning = 0, false; return b }(), 0, "Out of fuel", "Banked"},
		{"doused torch", func() game.BuildingView { b := torch; b.Burning = false; return b }(), 30, "Not burning", "Banked"},
		{"banked campfire", game.BuildingView{ID: 21, Kind: "campfire", Fuel: true, FuelPct: 70}, 70, "Banked (warm enough)", "Not burning"},
	}
	for _, c := range cases {
		c.b.W, c.b.H = 1, 1
		a, _, _ := pwrApp(t, 0, c.b)
		p := a.buildingPane(c.b.ID)
		bar := paneBarNamed(p, "Fuel")
		if bar == nil || bar.Pct != c.pct {
			t.Errorf("%s: fuel bar = %+v, want %d%%", c.name, bar, c.pct)
		}
		if got := bldLines(p); !strings.Contains(got, c.want) || strings.Contains(got, c.not) {
			t.Errorf("%s: lines %q, want %q and no %q", c.name, got, c.want, c.not)
		}
	}
}

// ---- the command buttons ----------------------------------------------------

func TestPowerToggleCommand(t *testing.T) {
	t.Parallel()
	for _, mk := range []func() game.BuildingView{pwrHeater, pwrGenerator, pwrSwitch} {
		b := mk()
		a, s, _ := pwrApp(t, 0, b)
		cs := a.buildingCommands(b.ID)
		c, ok := bldCmd(cs, "cmd.power.toggle")
		if !ok {
			t.Fatalf("%s: no power toggle in %v", b.Kind, bldCmdIDs(cs))
		}
		if c.Label != "Power" || c.Icon != "power_toggle" || c.Hotkey != input.Power || !c.Toggle || !c.On || c.Disabled {
			t.Errorf("%s: toggle = %+v", b.Kind, c)
		}
		if !strings.Contains(c.Desc, "colonist") {
			t.Errorf("%s: the tip should say a colonist flicks it: %q", b.Kind, c.Desc)
		}
		// on now: a click asks for off
		a.applyCommandClick("cmd.power.toggle")
		got := cmdsOf(s, command.SetPowerWanted)
		if len(got) != 1 || got[0].Target != uint64(b.ID) || got[0].Enabled {
			t.Fatalf("%s: click sent %+v, want SetPowerWanted off for %d", b.Kind, got, b.ID)
		}
		// the player's wish is the button's state, not the device's: a flick
		// pending shows the wish, and the pane says so
		s.world.Buildings[0].PowerWantOn = false
		c, _ = bldCmd(a.buildingCommands(b.ID), "cmd.power.toggle")
		if c.On {
			t.Errorf("%s: toggle still on after the wish went off", b.Kind)
		}
		if !strings.Contains(bldLines(a.buildingPane(b.ID)), "Flick pending") {
			t.Errorf("%s: no flick pending line: %q", b.Kind, a.buildingPane(b.ID).Lines)
		}
		// wished off: a click asks for on
		a.applyCommandClick("cmd.power.toggle")
		got = cmdsOf(s, command.SetPowerWanted)
		if len(got) != 2 || !got[1].Enabled || got[1].Target != uint64(b.ID) {
			t.Fatalf("%s: second click sent %+v", b.Kind, got)
		}
	}
}

func TestPowerToggleIsOnlyOnToggleDevices(t *testing.T) {
	t.Parallel()
	for _, b := range []game.BuildingView{pwrBattery(), pwrConduit(),
		{ID: 30, Kind: "wall", X: 1, Y: 1}, {ID: 31, Kind: "torch lamp", X: 1, Y: 1, Fuel: true}} {
		a, _, _ := pwrApp(t, 0, b)
		for _, c := range a.buildingCommands(b.ID) {
			if strings.HasPrefix(c.ID, "cmd.power.") {
				t.Errorf("%s has %s", b.Kind, c.ID)
			}
		}
	}
	// and only heaters and coolers have a thermostat
	for _, b := range []game.BuildingView{pwrGenerator(), pwrSwitch()} {
		a, _, _ := pwrApp(t, 0, b)
		if _, ok := bldCmd(a.buildingCommands(b.ID), "cmd.power.temp_up"); ok {
			t.Errorf("%s has a thermostat", b.Kind)
		}
	}
}

func TestThermostatCommands(t *testing.T) {
	t.Parallel()
	cooler := pwrHeater()
	cooler.ID, cooler.Kind, cooler.ThermoTarget = 15, "cooler", 195
	for _, b := range []game.BuildingView{pwrHeater(), cooler} {
		a, s, _ := pwrApp(t, 0, b)
		cs := a.buildingCommands(b.ID)
		down, ok1 := bldCmd(cs, "cmd.power.temp_down")
		up, ok2 := bldCmd(cs, "cmd.power.temp_up")
		if !ok1 || !ok2 {
			t.Fatalf("%s: thermostat buttons missing in %v", b.Kind, bldCmdIDs(cs))
		}
		if down.Label != "-1 °C" || down.Icon != "thermostat_down" || up.Label != "+1 °C" || up.Icon != "thermostat_up" {
			t.Errorf("%s: down %+v up %+v", b.Kind, down, up)
		}
		if down.Desc == "" || up.Desc == "" || !strings.Contains(up.Desc, fmtTarget(b.ThermoTarget)) {
			t.Errorf("%s: tips should state the target: %q / %q", b.Kind, down.Desc, up.Desc)
		}
		a.applyCommandClick("cmd.power.temp_down")
		a.applyCommandClick("cmd.power.temp_up")
		got := cmdsOf(s, command.SetThermostat)
		if len(got) != 2 || got[0].Target != uint64(b.ID) || got[0].Count != b.ThermoTarget-10 ||
			got[1].Target != uint64(b.ID) || got[1].Count != b.ThermoTarget+10 {
			t.Fatalf("%s: thermostat commands = %+v (target %d)", b.Kind, got, b.ThermoTarget)
		}
	}
}

func TestPowerCommandsComeBeforeMoveAndKeepTheirHotkeyLabel(t *testing.T) {
	t.Parallel()
	a, _, _ := pwrApp(t, 0, pwrHeater())
	ids := strings.Join(bldCmdIDs(a.buildingCommands(10)), ",")
	if ids != "cmd.power.toggle,cmd.power.temp_down,cmd.power.temp_up,bldg.move,bldg.copy,bldg.deconstruct" {
		t.Fatalf("heater commands = %s", ids)
	}
	c, _ := bldCmd(a.buildingCommands(10), "cmd.power.toggle")
	if got := a.commandKeyLabel(0, c); got != "V" {
		t.Fatalf("the toggle's key label = %q, want V", got)
	}
	if d, ok := input.Lookup(input.Power); !ok || d.Reserved != "" {
		t.Fatalf("the power action must be live: %+v", d)
	}
}

func TestVHotkeyFlicksTheSelectedDevice(t *testing.T) {
	t.Parallel()
	a, s, f := pwrApp(t, 0, pwrHeater())
	f.key(t, a, "V", 0)
	got := cmdsOf(s, command.SetPowerWanted)
	if len(got) != 1 || got[0].Target != 10 || got[0].Enabled {
		t.Fatalf("V sent %+v, want off for 10", got)
	}
	if len(a.liveToasts()) != 0 {
		t.Fatalf("V toasted %+v", a.liveToasts())
	}
	// wished off: V asks for on
	s.world.Buildings[0].PowerWantOn = false
	f.key(t, a, "V", 0)
	if got = cmdsOf(s, command.SetPowerWanted); len(got) != 2 || !got[1].Enabled {
		t.Fatalf("second V sent %+v", got)
	}
	// V on a building with no toggle does nothing
	b, _, g := pwrApp(t, 0, pwrBattery())
	g.key(t, b, "V", 0)
	if n := len(cmdsOf(b.renderer.game.(*stubSim), command.SetPowerWanted)); n != 0 {
		t.Fatalf("V on a battery sent %d commands", n)
	}
	// and on a colonist
	c, cs, h := inputColony(t, [2]int{5, 5})
	c.renderer.Select(1)
	h.key(t, c, "V", 0)
	if n := len(cmdsOf(cs, command.SetPowerWanted)); n != 0 {
		t.Fatalf("V on a colonist sent %d commands", n)
	}
}

// A group of devices: the merged toggle puts them all on unless all already
// are (the group rules of mergeCommands). Buildings cannot be box-selected
// into a group yet (groupable), but their commands merge like any kind's.
func TestPowerToggleOverAGroup(t *testing.T) {
	t.Parallel()
	on, off := pwrHeater(), pwrGenerator()
	off.PowerOn, off.PowerWantOn, off.PowerOut = false, false, 0
	a, s, _ := pwrApp(t, 0, on, off)
	a.clearSelections(selNone)
	a.group = selGroup{kind: selBuilding, ids: []entity.EntityID{on.ID, off.ID}}
	merged := func() cmdButton {
		c, ok := bldCmd(a.groupCommands(), "cmd.group.power.toggle")
		if !ok {
			t.Fatalf("no merged toggle in %v", bldCmdIDs(a.groupCommands()))
		}
		return c
	}
	if c := merged(); !c.Mixed || c.On {
		t.Fatalf("a mixed group: %+v", c)
	}
	a.runCommand(merged())
	got := cmdsOf(s, command.SetPowerWanted)
	if len(got) != 1 || got[0].Target != uint64(off.ID) || !got[0].Enabled {
		t.Fatalf("a mixed group switches the off device on: %+v", got)
	}
	// all on: the toggle switches both off
	s.world.Buildings[1].PowerWantOn, s.world.Buildings[1].PowerOn = true, true
	a.hst.cmds.memo.ok = false
	if c := merged(); !c.On || c.Mixed {
		t.Fatalf("an all-on group: %+v", c)
	}
	s.cmds = nil
	a.runCommand(merged())
	got = cmdsOf(s, command.SetPowerWanted)
	if len(got) != 2 || got[0].Enabled || got[1].Enabled {
		t.Fatalf("an all-on group switches both off: %+v", got)
	}
}

// ---- layout at every UI scale ----------------------------------------------------

// The power buildings' panes are up, their commands each have a grid cell
// and a hit target, and no region overlaps another, at UI scales 1, 1.5 and
// 2 on the default, a big and a small screen. It sets the UI scale: not
// parallel.
func TestPowerPaneAndGridAtUIScales(t *testing.T) {
	bs := []game.BuildingView{pwrHeater(), pwrGenerator(), pwrBattery(), pwrSwitch(), pwrConduit()}
	for _, ui := range []float64{1, 1.5, 2} {
		for _, sz := range layoutScreens {
			s := busyStub(6)
			for i := range bs {
				bs[i].W, bs[i].H = 1, 1
			}
			s.world.Buildings = bs
			s.world.PowerNets = pwrNets
			a, f := colonyApp(t, s)
			sw, sh := layoutScreen(t, a, sz[0], sz[1], ui)
			for _, b := range bs {
				name := b.Kind + " ui" + strconv.FormatFloat(ui, 'g', -1, 64) + " " + strconv.Itoa(sw) + "x" + strconv.Itoa(sh)
				a.selectCand(selCand{selBuilding, b.ID})
				f.frame(t, a)
				l := a.hud()
				if l.Pane.Empty() || !a.paneShown() {
					t.Fatalf("%s: no pane", name)
				}
				if bad := layoutOverlaps(l, sw, sh); len(bad) > 0 {
					t.Fatalf("%s:\n\t%s", name, strings.Join(bad, "\n\t"))
				}
				assertChromeIsTheLayout(t, a)
				cs := a.selectionCommands()
				if len(cs) == 0 || len(l.GridCells) != len(cs) {
					t.Fatalf("%s: %d commands, %d cells", name, len(cs), len(l.GridCells))
				}
				hits := append(a.layoutHits(a.sw, a.sh), a.colonyButtons(a.sw, a.sh)...)
				for i, cm := range cs {
					hb, ok := hasButton(hits, cm.ID)
					if !ok {
						t.Errorf("%s: %s has no hit target", name, cm.ID)
						continue
					}
					if got := (rect{X: hb.x, Y: hb.y, W: hb.w, H: hb.h}); got != l.GridCells[i] {
						t.Errorf("%s: %s is at %+v, its cell is %+v", name, cm.ID, got, l.GridCells[i])
					}
				}
			}
		}
	}
}

// ---- the Solar flare chip ----------------------------------------------------

func TestSolarFlareChip(t *testing.T) {
	t.Parallel()
	if chips := statusChips(game.WorldView{}); len(chips) != 0 {
		t.Fatalf("no flare, yet chips %+v", chips)
	}
	chips := statusChips(game.WorldView{SolarFlare: true})
	if len(chips) != 1 || chips[0].label != "Solar flare" || chips[0].edge != uiWarning {
		t.Fatalf("flare chip = %+v", chips)
	}
	if !strings.HasPrefix(chips[0].tip, "Solar flare\n") || !strings.Contains(chips[0].tip, "Batteries") {
		t.Fatalf("flare tip = %q", chips[0].tip)
	}
	// with a heat wave and a permanent condition: the weather, the flare, then the conditions
	all := statusChips(game.WorldView{Extreme: "heat_wave", SolarFlare: true, Conditions: []string{"ashfall_sky"}})
	if len(all) != 3 || all[0].label != "Heat wave" || all[1].label != "Solar flare" || all[2].label != "Ashfall sky" {
		t.Fatalf("chips = %+v", all)
	}
}

func TestSolarFlareChipIsInTheGlobalControls(t *testing.T) {
	t.Parallel()
	s := bldStub()
	s.world.SolarFlare = true
	a, f := colonyApp(t, s)
	f.frame(t, a)
	rs := a.chipRects(statusChips(a.renderer.world()), a.sw)
	if len(rs) != 1 || rs[0].Empty() {
		t.Fatalf("chip rects = %+v", rs)
	}
	if _, ok := a.uiSt().find("hud.chip.0"); !ok {
		t.Fatal("the flare chip has no hover card")
	}
}

// ---- the letters ----------------------------------------------------------------

// The sim posts the flare as a caution letter and the short circuit as a
// threat; the letter stack shows each kind with its own icon and colour.
func TestPowerLettersShowWithTheirKind(t *testing.T) {
	t.Parallel()
	if letterKinds[letterKind("caution")].icon != "letter_caution" || letterKinds[letterKind("threat")].icon != "letter_threat" {
		t.Fatalf("kinds: caution %+v threat %+v", letterKinds[letterKind("caution")], letterKinds[letterKind("threat")])
	}
	if letterKind("caution") == letterKind("threat") {
		t.Fatal("caution and threat share a kind")
	}
}

// ---- the bills of an unpowered bench ----------------------------------------------

func TestUnpoweredBenchBillsSayNoPower(t *testing.T) {
	t.Parallel()
	b := game.BuildingView{ID: 40, Kind: "electric stove", X: 3, Y: 3, W: 2, H: 1, Bench: true, PowerRole: "consumer", PowerOn: true, PowerWantOn: true,
		NoPowerReason: game.NoPowerNotConnected,
		Bills:         []game.BillView{{Recipe: "simple_meal", Label: "Cook simple meal", Mode: "forever", ModeID: 1, Status: game.BillNoPower}}}
	if got := billLine(b.Bills[0]); !strings.Contains(strings.ToLower(got), "no power") {
		t.Fatalf("bill line = %q", got)
	}
	a, _, _ := pwrApp(t, 0, b)
	p := a.buildingPane(40)
	if _, ok := bldTab(p, "inspector.bills"); !ok {
		t.Fatalf("no Bills tab on an electric bench: %+v", p.Tabs)
	}
	body := billsBody(b, 500)
	if len(body.labels) != 1 || !strings.Contains(strings.ToLower(body.labels[0].text), "no power") {
		t.Fatalf("bills body labels = %+v", body.labels)
	}
	if len(body.fills) != 1 {
		t.Fatalf("a held bill is marked: %+v", body.fills)
	}
	if !strings.Contains(bldLines(p), "No power: not connected") {
		t.Fatalf("pane lines = %q", p.Lines)
	}
	// a powered bench's bill has no such note
	b.NoPowerReason, b.Powered, b.PowerUse = "", true, 250
	b.Bills[0].Status = ""
	if strings.Contains(strings.ToLower(billLine(b.Bills[0])), "no power") {
		t.Fatal("an active bill says no power")
	}
}

// ---- the electric benches ---------------------------------------------------------

// An electric bench takes its base bench's bills, and an electric research
// bench counts as the research bench.
func TestElectricBenchesListTheirBaseRecipes(t *testing.T) {
	t.Parallel()
	a, _, _ := pwrApp(t, 0, pwrHeater())
	for _, c := range []struct {
		electric, base string
		needRecipes    bool
	}{
		{"electric stove", "stove", true},
		{"electric tailoring bench", "tailoring bench", true},
		{"electric research bench", "research bench", false},
	} {
		got, want := a.recipesFor(c.electric), a.recipesFor(c.base)
		if c.needRecipes && len(want) == 0 {
			t.Fatalf("the %s has no recipes to copy", c.base)
		}
		if len(got) != len(want) {
			t.Errorf("%s lists %d recipes, %s %d", c.electric, len(got), c.base, len(want))
			continue
		}
		for i := range got {
			if got[i].ID != want[i].ID {
				t.Errorf("%s recipe %d is %q, the %s's is %q", c.electric, i, got[i].Name, c.base, want[i].Name)
			}
		}
	}
	if recipeBenchName("heater") != "heater" || recipeBenchName("stove") != "stove" || recipeBenchName("nonsense") != "nonsense" {
		t.Fatal("a kind with no base bench is its own")
	}
	if len(a.recipesFor("heater")) != 0 {
		t.Fatal("a heater is no bench")
	}
}

func TestElectricResearchBenchCountsAsAResearchBench(t *testing.T) {
	t.Parallel()
	for kind, want := range map[string]bool{"research bench": true, "electric research bench": true, "stove": false, "electric stove": false} {
		s := bldStub(game.BuildingView{ID: 3, Kind: kind, X: 3, Y: 3, W: 2, H: 1})
		a, _ := colonyApp(t, s)
		if got := a.hasResearchBench(); got != want {
			t.Errorf("a %s: hasResearchBench = %v, want %v", kind, got, want)
		}
	}
}

// The pane draws a power warning as a line of its body (the colour is
// paneLineWarns' call) at every UI scale. It sets the UI scale: not parallel.
func TestPaneDrawsThePowerWarning(t *testing.T) {
	for _, ui := range []float64{1, 1.5, 2} {
		b := pwrHeater()
		b.W, b.H = 1, 1
		b.Powered, b.PowerUse, b.NoPowerReason = false, 0, game.NoPowerNotConnected
		b.PowerNet = 0
		s := bldStub(b)
		s.world.PowerNets = pwrNets
		a, f := bldApp(t, s, b.ID)
		img := winScreen(t, a, 1280, 720, ui)
		f.frame(t, a)
		found := false
		for _, e := range winTrace(a, img) {
			if e.Kind == "text" && e.Text == "No power: not connected" {
				found = true
			}
		}
		if !found {
			t.Errorf("ui %g: the pane does not draw the no-power line", ui)
		}
	}
}

func TestFmtTarget(t *testing.T) {
	t.Parallel()
	for tenths, want := range map[int]string{210: "21 °C", 215: "21.5 °C", 0: "0 °C", -50: "-5 °C", -55: "-5.5 °C", 5: "0.5 °C"} {
		if got := fmtTarget(tenths); got != want {
			t.Errorf("fmtTarget(%d) = %q, want %q", tenths, got, want)
		}
	}
}
