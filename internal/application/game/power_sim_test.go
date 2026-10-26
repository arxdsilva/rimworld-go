package game

import (
	"testing"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/building/power"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// PLAY-021 POWERSIM: the balance phase, the flicks, the saves and the
// views, on small open maps.

// psView is building id's view.
func psView(t *testing.T, g *Game, id entity.EntityID) BuildingView {
	t.Helper()
	for _, bv := range g.World().Buildings {
		if bv.ID == id {
			return bv
		}
	}
	t.Fatalf("no view of %d", id)
	return BuildingView{}
}

// psNet is net id's row in w (ok false: no such net).
func psNet(w WorldView, id int) (PowerNetView, bool) {
	for _, n := range w.PowerNets {
		if n.ID == id {
			return n, true
		}
	}
	return PowerNetView{}, false
}

// psConduits lays a conduit on row y from x0 to x1.
func psConduits(t *testing.T, g *Game, y, x0, x1 int) []entity.EntityID {
	t.Helper()
	var ids []entity.EntityID
	for x := x0; x <= x1; x++ {
		ids = append(ids, buildNow(t, g, construction.PowerConduit, item.Steel, construction.South, x, y))
	}
	return ids
}

// psFuelled builds a wood-fired generator at (x, y) with a full tank.
func psFuelled(t *testing.T, g *Game, x, y int) entity.EntityID {
	t.Helper()
	id := buildNow(t, g, construction.WoodGenerator, item.Steel, construction.South, x, y)
	g.setFuel(id, g.fuelCap(id))
	return id
}

// psWorker spawns a rested, fed colonist who does construction work.
func psWorker(t *testing.T, g *Game, x, y int) entity.EntityID {
	t.Helper()
	id, err := g.SpawnNamedPawn(x, y, "Ada")
	if err != nil {
		t.Fatal(err)
	}
	g.SetNeeds(id, needs.Full, needs.Full)
	return id
}

// psStepUntil steps one tick at a time until done, at most n ticks.
func psStepUntil(t *testing.T, g *Game, n int, what string, done func() bool) {
	t.Helper()
	for i := 0; i < n; i++ {
		if done() {
			return
		}
		g.Step(1)
	}
	if !done() {
		t.Fatalf("%s: not after %d ticks", what, n)
	}
}

// TestPowerGeneratorLightsLampThroughConduits: a fuelled generator feeds
// a lamp through a conduit line it links to; a lamp far from any conduit
// is not connected.
func TestPowerGeneratorLightsLampThroughConduits(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 40, Height: 16})
	gen := psFuelled(t, g, 2, 2)
	line := psConduits(t, g, 4, 2, 12)
	lamp := buildNow(t, g, construction.Lamp, item.Wood, construction.South, 14, 5)
	far := buildNow(t, g, construction.Lamp, item.Wood, construction.South, 30, 12)
	g.Step(powerInterval)

	if !g.powered(lamp) || g.powered(far) {
		t.Fatalf("powered: lamp %v, far lamp %v", g.powered(lamp), g.powered(far))
	}
	net := int(line[0])
	gv, lv, fv := psView(t, g, gen), psView(t, g, lamp), psView(t, g, far)
	if gv.PowerRole != "producer" || gv.PowerOut != 800 || gv.PowerNet != net || !gv.HasLink || gv.NoPowerReason != "" {
		t.Errorf("generator view %+v", gv)
	}
	if lv.PowerRole != "consumer" || !lv.Powered || lv.PowerUse != 50 || lv.PowerNet != net || lv.NoPowerReason != "" ||
		!lv.HasLink || lv.LinkX != 12 || lv.LinkY != 4 || !lv.PowerOn || !lv.PowerWantOn {
		t.Errorf("lamp view %+v", lv)
	}
	if fv.Powered || fv.PowerNet != 0 || fv.HasLink || fv.NoPowerReason != NoPowerNotConnected || fv.PowerUse != 0 {
		t.Errorf("far lamp view %+v", fv)
	}
	if cv := psView(t, g, line[3]); cv.PowerRole != "transmitter" || cv.PowerNet != net || cv.HasLink {
		t.Errorf("conduit view %+v", cv)
	}
	w := g.World()
	if len(w.PowerNets) != 1 {
		t.Fatalf("nets %+v", w.PowerNets)
	}
	if n := w.PowerNets[0]; n.ID != net || n.Gen != 800 || n.Load != 50 || n.Brownout || n.CapWh != 0 {
		t.Errorf("net %+v", n)
	}
	if w.SolarFlare || w.PowerVer == 0 {
		t.Errorf("flare %v, power ver %d", w.SolarFlare, w.PowerVer)
	}
}

// TestPowerColonyWithoutDevicesWritesNothing: a colony without producers,
// consumers or batteries (conduits alone included) keeps no power state,
// so its saves and hashes are as before; and once its last device is
// gone, a colony drops its power state again.
func TestPowerColonyWithoutDevicesWritesNothing(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 3, Width: 30, Height: 12})
	fire := buildNow(t, g, construction.Campfire, item.Wood, construction.South, 10, 8)
	g.setFuel(fire, g.fuelCap(fire))
	buildNow(t, g, construction.Wall, item.Wood, construction.South, 14, 8)
	psWorker(t, g, 5, 8)
	g.Step(120)
	psConduits(t, g, 1, 1, 6)
	g.Step(600)
	if g.pw != nil || g.exportPower() != nil {
		t.Fatal("conduits alone wrote power state")
	}
	if b := g.pwc.builds; b > 6 {
		t.Errorf("building list rebuilt %d times in 720 ticks", b)
	}

	lamp := buildNow(t, g, construction.Lamp, item.Wood, construction.South, 3, 2)
	g.Step(powerInterval)
	if g.pw == nil || g.exportPower() == nil {
		t.Fatal("a lamp starts the power state")
	}
	g.removeBuilding(lamp)
	g.Step(powerInterval)
	if g.pw != nil || g.exportPower() != nil || g.powerVer() != 0 || g.World().PowerVer != 0 {
		t.Fatal("the power state outlived the last device")
	}
}

// TestPowerBatteryChargesBySolarAndLightsAtNight: a solar panel charges a
// battery by day; at night the panel makes nothing and the battery keeps
// the lamps lit.
func TestPowerBatteryChargesBySolarAndLightsAtNight(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 24, Height: 12})
	panel := buildNow(t, g, construction.SolarPanel, item.Steel, construction.South, 2, 1)
	bat := buildNow(t, g, construction.Battery, item.Steel, construction.South, 7, 2)
	psConduits(t, g, 5, 2, 12)
	l1 := buildNow(t, g, construction.Lamp, item.Wood, construction.South, 10, 7)
	l2 := buildNow(t, g, construction.Lamp, item.Wood, construction.South, 12, 7)

	g.Step(3 * clock.TicksPerHour) // 11:00
	if v := psView(t, g, panel); v.PowerOut <= 500 {
		t.Fatalf("panel at 11:00 makes %d W", v.PowerOut)
	}
	g.Step(11 * clock.TicksPerHour) // 22:00
	dusk := psView(t, g, bat)
	if dusk.StoredWh < 1000 || dusk.CapWh != 1200 || dusk.PowerRole != "storage" {
		t.Fatalf("battery at 22:00: %+v", dusk)
	}
	g.Step(5 * clock.TicksPerHour) // 03:00
	if v := psView(t, g, panel); v.PowerOut != 0 {
		t.Fatalf("panel at night makes %d W", v.PowerOut)
	}
	for _, l := range []entity.EntityID{l1, l2} {
		if v := psView(t, g, l); !v.Powered {
			t.Fatalf("lamp %d dark at night: %+v", l, v)
		}
	}
	night := psView(t, g, bat)
	// 5 h of two lamps (500 Wh) and the battery's own leak (40 Wh)
	if lost := dusk.StoredWh - night.StoredWh; lost < 530 || lost > 545 {
		t.Fatalf("battery went from %d to %d Wh overnight", dusk.StoredWh, night.StoredWh)
	}
	n, ok := psNet(g.World(), night.PowerNet)
	if !ok || n.Gen != 0 || n.Load != 100 || n.StoredWh != night.StoredWh || n.CapWh != 1200 || n.Brownout {
		t.Fatalf("net at night %+v", n)
	}
}

// psBrownoutGrid is a conduit line with a producer and lamps lamps beside
// it; the producer makes what gen says.
func psBrownoutGrid(t *testing.T, lamps int, gen func(now uint64) int) (*Game, []entity.EntityID) {
	t.Helper()
	g := New(Config{Seed: 1, Width: 44, Height: 10})
	src := buildNow(t, g, construction.SolarPanel, item.Steel, construction.South, 0, 0)
	psConduits(t, g, 3, 0, 43)
	var ids []entity.EntityID
	for i := 0; i < lamps; i++ {
		ids = append(ids, buildNow(t, g, construction.Lamp, item.Wood, construction.South, 2+2*i, 4))
	}
	g.pwc.genHook = func(id entity.EntityID) int {
		if id != src {
			return 0
		}
		return gen(uint64(g.clock.Now()))
	}
	return g, ids
}

// TestPowerBrownoutShedsNewestAndHolds: past what the net makes, the
// newest lamps go dark; they stay dark for the shed hold even when power
// comes back, then return.
func TestPowerBrownoutShedsNewestAndHolds(t *testing.T) {
	t.Parallel()
	out := 800
	g, lamps := psBrownoutGrid(t, 18, func(uint64) int { return out })
	g.Step(powerInterval)
	for i, l := range lamps {
		want := i < 16 // 900 W asked of 800: the two newest are shed
		v := psView(t, g, l)
		if v.Powered != want {
			t.Fatalf("lamp %d powered %v", i, v.Powered)
		}
		if !want && v.NoPowerReason != NoPowerBrownout {
			t.Fatalf("lamp %d reason %q", i, v.NoPowerReason)
		}
	}
	n := g.World().PowerNets[0]
	if !n.Brownout || n.Load != 800 || n.Gen != 800 {
		t.Fatalf("net %+v", n)
	}
	until := g.pw.shed[lamps[17]]
	if until == 0 || g.pw.shed[lamps[16]] != until {
		t.Fatalf("shed holds %v", g.pw.shed)
	}
	out = 2000
	psStepUntil(t, g, 2*power.ShedTicks, "shed lamps back", func() bool { return g.powered(lamps[17]) })
	if back := uint64(g.clock.Now()) - 1; back < until || back >= until+powerInterval {
		t.Fatalf("shed lamp back at tick %d, hold until %d", back, until)
	}
	if !g.powered(lamps[16]) || g.World().PowerNets[0].Brownout {
		t.Fatal("the shed lamps came back together, the brownout over")
	}
}

// TestPowerNoFlapOverADay: generation rippling ±5% around the lamps' load,
// with deep dips now and then, never flicks a lamp on and off: every dark
// spell of BuildingView.Powered lasts at least the shed hold.
func TestPowerNoFlapOverADay(t *testing.T) {
	t.Parallel()
	g, lamps := psBrownoutGrid(t, 16, func(now uint64) int {
		switch {
		case now%9000 < 100:
			return 500 // a deep dip
		case (now/700)%2 == 0:
			return 840
		}
		return 760
	})
	darkSince := make([]uint64, len(lamps))
	was := make([]bool, len(lamps))
	for i := range was {
		was[i] = true
	}
	flips := 0
	for i := 0; i < clock.TicksPerDay/powerInterval; i++ {
		g.Step(powerInterval)
		now := uint64(g.clock.Now())
		w := g.World()
		for k, l := range lamps {
			on := false
			for _, bv := range w.Buildings {
				if bv.ID == l {
					on = bv.Powered
				}
			}
			switch {
			case was[k] && !on:
				darkSince[k] = now
				flips++
			case !was[k] && on:
				if d := now - darkSince[k]; d < power.ShedTicks {
					t.Fatalf("lamp %d dark for only %d ticks (from %d)", k, d, darkSince[k])
				}
				flips++
			}
			was[k] = on
		}
	}
	// seven dips a day: each sheds a few lamps and brings them back once
	if flips == 0 || flips > 7*2*8 {
		t.Fatalf("%d on/off flips in a day", flips)
	}
}

// TestPowerSwitchCutsNetAndColonistFlicks: opening a switch splits the
// net and darkens the lamp beyond it, once a colonist has flicked it; it
// takes a flick back to close it.
func TestPowerSwitchCutsNetAndColonistFlicks(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 30, Height: 12})
	psFuelled(t, g, 2, 2)
	psConduits(t, g, 4, 2, 8)
	sw := buildNow(t, g, construction.PowerSwitch, item.Steel, construction.South, 9, 4)
	psConduits(t, g, 4, 10, 16)
	lamp := buildNow(t, g, construction.Lamp, item.Wood, construction.South, 17, 5)
	ada := psWorker(t, g, 5, 8)
	g.Step(powerInterval)
	if !g.powered(lamp) || len(g.World().PowerNets) != 1 {
		t.Fatal("the closed switch carries power")
	}
	if v := psView(t, g, sw); v.PowerRole != "switch" || !v.PowerOn || v.PowerNet == 0 {
		t.Fatalf("switch view %+v", v)
	}

	g.Enqueue(command.Command{Kind: command.SetPowerWanted, Target: uint64(sw), Enabled: false})
	g.Step(1)
	if v := psView(t, g, sw); v.PowerWantOn || !v.PowerOn || !g.flickPending(sw) || !g.powered(lamp) {
		t.Fatalf("asked off, before the flick: %+v", v)
	}
	topo, sawJob := g.pwc.topo, false
	psStepUntil(t, g, 3000, "flick off", func() bool {
		if j, _ := g.jobs.Get(ada); j.Kind == job.Flick {
			sawJob = true
		}
		return !g.powerOn(sw)
	})
	if !sawJob || g.pwc.topo == topo || g.flickPending(sw) {
		t.Fatalf("flick job seen %v, topology %d→%d", sawJob, topo, g.pwc.topo)
	}
	if g.powered(lamp) {
		t.Fatal("the open switch still carries power to the lamp")
	}
	w := g.World()
	if len(w.PowerNets) != 2 {
		t.Fatalf("an open switch splits the net: %+v", w.PowerNets)
	}
	if v := psView(t, g, sw); v.PowerOn || v.PowerWantOn || v.PowerNet != 0 {
		t.Fatalf("open switch view %+v", v)
	}
	if v := psView(t, g, lamp); v.NoPowerReason != NoPowerBrownout {
		t.Fatalf("lamp beyond the open switch: %+v", v)
	}

	g.Enqueue(command.Command{Kind: command.SetPowerWanted, Target: uint64(sw), Enabled: true})
	psStepUntil(t, g, 3000, "flick on", func() bool { return g.powerOn(sw) })
	if !g.powered(lamp) || len(g.World().PowerNets) != 1 {
		t.Fatal("closing the switch rejoins the net at once")
	}
}

// TestPowerWantedOffLampAfterFlick: asking a lamp off changes nothing
// until a colonist flicks it; then it is off ("switched off") and draws
// nothing. Things without a toggle ignore the ask.
func TestPowerWantedOffLampAfterFlick(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 30, Height: 12})
	psFuelled(t, g, 2, 2)
	line := psConduits(t, g, 4, 2, 10)
	lamp := buildNow(t, g, construction.Lamp, item.Wood, construction.South, 8, 5)
	bat := buildNow(t, g, construction.Battery, item.Steel, construction.South, 12, 3)
	psWorker(t, g, 5, 8)
	g.Step(powerInterval)

	g.Enqueue(command.Command{Kind: command.SetPowerWanted, Target: uint64(bat), Enabled: false})
	g.Enqueue(command.Command{Kind: command.SetPowerWanted, Target: uint64(line[0]), Enabled: false})
	g.Enqueue(command.Command{Kind: command.SetPowerWanted, Target: uint64(lamp), Enabled: false})
	g.Step(1)
	if !g.powerWantOn(bat) || !g.powerWantOn(line[0]) || len(g.pw.flicks) != 1 {
		t.Fatalf("only toggles take a wanted state: flicks %v", g.pw.flicks)
	}
	if v := psView(t, g, lamp); !v.Powered || v.PowerWantOn || !v.PowerOn {
		t.Fatalf("pending flick: %+v", v)
	}
	psStepUntil(t, g, 3000, "lamp flicked off", func() bool { return !g.powerOn(lamp) })
	v := psView(t, g, lamp)
	if v.Powered || v.PowerOn || v.PowerUse != 0 || v.NoPowerReason != NoPowerSwitchedOff {
		t.Fatalf("lamp switched off: %+v", v)
	}
	if n, _ := psNet(g.World(), v.PowerNet); n.Load != 0 {
		t.Fatalf("a switched-off lamp draws %d W", n.Load)
	}
	// asking it on again, then off before anyone came, leaves nothing to do
	g.cmdSetPowerWanted(lamp, true)
	g.cmdSetPowerWanted(lamp, false)
	if g.flickPending(lamp) {
		t.Fatal("wanted back as it is: no flick")
	}
}

// TestPowerForcedFlickOrder: Prioritize → Flick sends the colonist at
// once, forced; with nothing to flick the order is refused.
func TestPowerForcedFlickOrder(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 30, Height: 12})
	psFuelled(t, g, 2, 2)
	psConduits(t, g, 4, 2, 10)
	lamp := buildNow(t, g, construction.Lamp, item.Wood, construction.South, 8, 5)
	ada := psWorker(t, g, 5, 8)
	g.Step(powerInterval)
	if _, why := g.flickJob(ada, 8, 5); why != ReasonGone {
		t.Fatalf("nothing pending: %q", why)
	}
	if g.flickAt(8, 5) != 0 {
		t.Fatal("no flick to offer")
	}
	g.cmdSetPowerWanted(lamp, false)
	if g.flickAt(8, 5) != lamp {
		t.Fatal("the pending flick is offered on its cell")
	}
	g.Enqueue(command.Command{Kind: command.Prioritize, Pawn: uint64(ada), Order: command.OrderFlick, X: 8, Y: 5})
	g.Step(1)
	if j, _ := g.jobs.Get(ada); j.Kind != job.Flick || !j.Forced || j.Target != lamp {
		t.Fatalf("forced flick: %+v", j)
	}
	if owner, _ := g.reserve.EntityOwner(lamp); owner != ada {
		t.Fatal("the flick holds the lamp")
	}
	psStepUntil(t, g, 2000, "forced flick", func() bool { return !g.powerOn(lamp) })
}

// TestPowerThermostatSetAndSaved: a heater's thermostat starts at 21 °C,
// takes a new target (clamped) and keeps it and its working flag through
// a save; other buildings have none.
func TestPowerThermostatSetAndSaved(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 20, Height: 10})
	heater := buildNow(t, g, construction.Heater, item.Steel, construction.South, 3, 3)
	cooler := buildNow(t, g, construction.Heater, item.Steel, construction.South, 8, 3)
	lamp := buildNow(t, g, construction.Lamp, item.Wood, construction.South, 12, 3)
	if g.thermoTarget(heater) != thermoDefault || psView(t, g, heater).ThermoTarget != 210 {
		t.Fatal("default target 21 °C")
	}
	g.setThermoTarget(heater, 185)
	g.setThermoTarget(cooler, 9999)
	g.setThermoTarget(lamp, 100)
	g.setThermoActive(heater, true)
	g.setThermoActive(lamp, true)
	if g.thermoTarget(heater) != 185 || g.thermoTarget(cooler) != thermoMax || !g.thermoActive(heater) ||
		g.thermoActive(lamp) || len(g.pw.thermo) != 2 {
		t.Fatalf("thermostats %+v", g.pw.thermo)
	}
	g.Step(powerInterval)
	h := saveLoad(t, g)
	if h.thermoTarget(heater) != 185 || h.thermoTarget(cooler) != thermoMax || !h.thermoActive(heater) || h.thermoActive(cooler) {
		t.Fatalf("loaded thermostats %+v", h.pw.thermo)
	}
	if v := psView(t, h, heater); v.ThermoTarget != 185 {
		t.Fatalf("heater view %+v", v)
	}
	if v := psView(t, h, lamp); v.ThermoTarget != 0 {
		t.Fatalf("lamp view %+v", v)
	}
	// back to the default and idle: nothing kept
	g.setThermoTarget(heater, thermoDefault)
	g.setThermoActive(heater, false)
	if _, ok := g.pw.thermo[heater]; ok {
		t.Fatal("a default thermostat is not kept")
	}
}

// TestPowerSolarFlare: a flare takes every net down (consumers dark,
// generators idle, batteries frozen) until its end tick exactly.
func TestPowerSolarFlare(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 30, Height: 12})
	gen := psFuelled(t, g, 2, 2)
	psConduits(t, g, 4, 2, 12)
	bat := buildNow(t, g, construction.Battery, item.Steel, construction.South, 10, 2)
	lamp := buildNow(t, g, construction.Lamp, item.Wood, construction.South, 8, 5)
	g.Step(3 * clock.TicksPerHour)
	until := uint64(g.clock.Now()) + 1234
	g.setFlare(until)
	stored := g.pw.charge[bat]
	if !g.solarFlare() || g.flareUntil() != until || g.powered(lamp) {
		t.Fatal("the flare takes the grid down at once")
	}
	w := g.World()
	if v := psView(t, g, lamp); v.NoPowerReason != NoPowerSolarFlare {
		t.Fatalf("lamp in the flare %+v", v)
	}
	if v := psView(t, g, gen); v.PowerOut != 0 || v.NoPowerReason != NoPowerSolarFlare {
		t.Fatalf("generator in the flare %+v", v)
	}
	if !w.SolarFlare || w.PowerNets[0].Gen != 0 || w.PowerNets[0].Load != 0 {
		t.Fatalf("world in the flare %+v", w.PowerNets)
	}
	g.Step(1000)
	if g.pw.charge[bat] != stored || g.powered(lamp) {
		t.Fatal("batteries hold and lamps stay dark during the flare")
	}
	psStepUntil(t, g, 300, "flare end", func() bool { return uint64(g.clock.Now()) > until })
	if g.solarFlare() || g.flareUntil() != 0 || !g.powered(lamp) || g.World().SolarFlare {
		t.Fatal("the grid comes back at the flare's end tick")
	}
}

// TestPowerShortCircuitHooks: storedNets finds a charged net, netConduits
// its visible conduits, and drainNet empties its batteries.
func TestPowerShortCircuitHooks(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 30, Height: 12})
	psFuelled(t, g, 2, 2)
	line := psConduits(t, g, 4, 2, 9)
	hidden := buildNow(t, g, construction.HiddenConduit, item.Steel, construction.South, 10, 4)
	bat := buildNow(t, g, construction.Battery, item.Steel, construction.South, 8, 2)
	if g.powerDevices() != 2 {
		t.Fatalf("devices %d", g.powerDevices())
	}
	g.Step(2 * clock.TicksPerHour)
	net := g.netOf(bat)
	if wh := g.netStoredWh(net); wh < 500 {
		t.Fatalf("stored %d Wh", wh)
	}
	if got := g.storedNets(100); len(got) != 1 || got[0] != net {
		t.Fatalf("charged nets %v", got)
	}
	if got := g.netConduits(net); len(got) != len(line) || got[0] != line[0] {
		t.Fatalf("visible conduits %v (hidden %d)", got, hidden)
	}
	stored := g.netStoredWh(net)
	// it balances up to now first: a little more than was stored
	if got := g.drainNet(net); got < stored || got > stored+20 || g.netStoredWh(net) != 0 || g.pw.charge[bat] != 0 {
		t.Fatalf("drained %d of %d Wh, left %d", got, stored, g.netStoredWh(net))
	}
	if len(g.storedNets(100)) != 0 {
		t.Fatal("a drained net holds nothing")
	}
}

// TestPowerSaveLoadMidBalance: a colony saved between two balances
// (mid-flick, mid-brownout, a lamp built since the last balance, a flare
// running) carries on exactly as the unbroken one.
func TestPowerSaveLoadMidBalance(t *testing.T) {
	t.Parallel()
	for _, flare := range []bool{false, true} {
		g := New(Config{Seed: 7, Width: 40, Height: 16})
		psFuelled(t, g, 2, 2)
		buildNow(t, g, construction.SolarPanel, item.Steel, construction.South, 2, 9)
		psConduits(t, g, 6, 2, 20)
		sw := buildNow(t, g, construction.PowerSwitch, item.Steel, construction.South, 21, 6)
		psConduits(t, g, 6, 22, 30)
		buildNow(t, g, construction.Battery, item.Steel, construction.South, 26, 4)
		buildNow(t, g, construction.Battery, item.Steel, construction.South, 8, 4)
		heater := buildNow(t, g, construction.Heater, item.Steel, construction.South, 12, 7)
		for x := 4; x < 36; x += 2 {
			buildNow(t, g, construction.Lamp, item.Wood, construction.South, x, 7+x%4)
		}
		psWorker(t, g, 15, 12)
		g.setThermoTarget(heater, 230)
		g.setThermoActive(heater, true)
		g.Step(2*clock.TicksPerHour + 38)
		g.cmdSetPowerWanted(sw, false)
		g.Step(13)
		buildNow(t, g, construction.Lamp, item.Wood, construction.South, 30, 7)
		if flare {
			g.setFlare(uint64(g.clock.Now()) + 777)
		}
		if uint64(g.clock.Now())%powerInterval == 0 {
			t.Fatal("save between two balances")
		}
		h := saveLoad(t, g)
		sameState(t, "after load", g, h)
		if g.StateHash() != h.StateHash() {
			t.Fatal("state hash after load")
		}
		for i := 0; i < 8; i++ {
			g.Step(311)
			h.Step(311)
			if a, b := g.StateHash(), h.StateHash(); a != b {
				t.Fatalf("flare %v, step %d: hash %#x vs %#x", flare, i, a, b)
			}
		}
		sameState(t, "running on", g, h)
		if g.pw == nil || g.World().PowerVer != h.World().PowerVer {
			t.Fatal("power version differs after the reload")
		}
	}
}

// TestPowerVerFollowsPoweredState: PowerVer moves when a lamp loses
// power, and stays put while nothing changes.
func TestPowerVerFollowsPoweredState(t *testing.T) {
	t.Parallel()
	out := 1000
	g, lamps := psBrownoutGrid(t, 4, func(uint64) int { return out })
	g.Step(powerInterval)
	v0, l0 := g.World().PowerVer, g.light.ver
	g.Step(powerInterval)
	if g.World().PowerVer != v0 || g.light.ver != l0 {
		t.Fatal("nothing changed: the versions hold")
	}
	out = 120
	g.Step(powerInterval)
	if g.powered(lamps[3]) || g.World().PowerVer == v0 || g.light.ver == l0 {
		t.Fatal("a lamp went dark: both versions move")
	}
}

// BenchmarkPhasePower is one balance of a large colony grid: 8 nets of
// 190 conduits, each with 2 producers, 4 batteries and 40 lamps, on a
// 200×200 map.
func BenchmarkPhasePower(b *testing.B) {
	t := &testing.T{}
	g := New(Config{Seed: 1, Width: 200, Height: 200})
	for n := 0; n < 8; n++ {
		y := 10 + n*24
		psConduits(t, g, y, 2, 191)
		for k := 0; k < 2; k++ {
			buildNow(t, g, construction.SolarPanel, item.Steel, construction.South, 4+k*80, y-4)
		}
		for k := 0; k < 4; k++ {
			buildNow(t, g, construction.Battery, item.Steel, construction.South, 20+k*30, y-3)
		}
		for k := 0; k < 40; k++ {
			buildNow(t, g, construction.Lamp, item.Wood, construction.South, 4+k*4, y+1+k%2)
		}
	}
	g.pwc.genHook = func(entity.EntityID) int { return 900 }
	now := uint64(g.clock.Now())
	g.balancePower(now)
	g.balancePower(now + powerInterval)
	if g.powerDevices() != 8*46 || len(g.World().PowerNets) != 8 {
		b.Fatalf("devices %d", g.powerDevices())
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		now += powerInterval
		g.balancePower(now)
	}
}

// TestPowerDrawByKind: a heater draws its full 180 W only while its
// thermostat has it working, an electric bench its 250 W only while held
// for work; each idles at its small draw otherwise.
func TestPowerDrawByKind(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 30, Height: 12})
	psFuelled(t, g, 2, 2)
	psConduits(t, g, 4, 2, 14)
	heater := buildNow(t, g, construction.Heater, item.Steel, construction.South, 8, 5)
	stove := buildNow(t, g, construction.ElectricStove, item.Steel, construction.South, 11, 5)
	g.Step(powerInterval)
	if h, s := psView(t, g, heater), psView(t, g, stove); h.PowerUse != 20 || s.PowerUse != 25 || !h.Powered || !s.Powered {
		t.Fatalf("idle: heater %d W, stove %d W", h.PowerUse, s.PowerUse)
	}
	g.setThermoActive(heater, true)
	ada := psWorker(t, g, 5, 9)
	g.reserve.TryEntity(ada, stove)
	h, s := psView(t, g, heater), psView(t, g, stove)
	if h.PowerUse != 180 || s.PowerUse != 250 {
		t.Fatalf("working: heater %d W, stove %d W", h.PowerUse, s.PowerUse)
	}
	if n, _ := psNet(g.World(), h.PowerNet); n.Load != 430 || n.Gen != 800 {
		t.Fatalf("net %+v", n)
	}
}

// TestPowerFlickTargetGone: a building removed while a colonist walks to
// flick it ends the job, and its pending flick is forgotten.
func TestPowerFlickTargetGone(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 30, Height: 12})
	psFuelled(t, g, 2, 2)
	psConduits(t, g, 4, 2, 24)
	lamp := buildNow(t, g, construction.Lamp, item.Wood, construction.South, 22, 5)
	ada := psWorker(t, g, 2, 9)
	g.Step(powerInterval)
	g.cmdSetPowerWanted(lamp, false)
	psStepUntil(t, g, 200, "flick job", func() bool {
		j, _ := g.jobs.Get(ada)
		return j.Kind == job.Flick
	})
	g.removeBuilding(lamp)
	g.Step(powerInterval)
	if g.flickPending(lamp) || len(g.pw.flicks) != 0 || len(g.pw.wantOff) != 0 {
		t.Fatalf("pending flicks %v", g.pw.flicks)
	}
	// like any job, it finds the target gone where it arrives
	psStepUntil(t, g, 400, "flick ended", func() bool {
		j, _ := g.jobs.Get(ada)
		return j.Kind != job.Flick || j.State == job.Fail
	})
	if j, _ := g.jobs.Get(ada); j.Kind == job.Flick && j.Fail != job.TargetGone {
		t.Fatalf("flick of a gone lamp: %+v", j)
	}
	if len(g.pw.flicks) != 0 {
		t.Fatalf("pending flicks %v", g.pw.flicks)
	}
}
