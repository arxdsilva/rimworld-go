package game

import (
	"slices"
	"testing"

	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// PWR-*: wires to generators and batteries, and Reconnect.

// reconnectGrids is two separate grids and a lamp in range of both: it
// starts on A's end at (6,4), which has no generator; grid B, from (12,10),
// has one. All on a map where nothing else touches.
type reconnectGrids struct {
	g         *Game
	a, b      []entity.EntityID
	gen, lamp entity.EntityID
}

func newReconnectGrids(t *testing.T) reconnectGrids {
	t.Helper()
	g := New(Config{Seed: 1, Width: 40, Height: 24})
	r := reconnectGrids{g: g}
	r.a = psConduits(t, g, 4, 2, 6)
	r.b = psConduits(t, g, 10, 12, 16)
	r.gen = psFuelled(t, g, 14, 11)
	r.lamp = buildNow(t, g, construction.Lamp, item.Wood, construction.South, 9, 7)
	g.pwc.genHook = func(entity.EntityID) int { return 800 }
	g.Step(powerInterval)
	return r
}

func reconnect(g *Game, id entity.EntityID) {
	g.Enqueue(command.Command{Kind: command.ReconnectPower, Target: uint64(id)})
	g.Step(1)
}

// TestReconnectMovesToTheOtherGrid: the order works at once with no colonist
// on the map, the lamp is powered by its new grid, and a second one wraps.
func TestReconnectMovesToTheOtherGrid(t *testing.T) {
	t.Parallel()
	r := newReconnectGrids(t)
	g := r.g
	netA, netB := int(r.a[0]), int(r.b[0])
	v := psView(t, g, r.lamp)
	if v.PowerNet != netA || v.LinkX != 6 || v.LinkY != 4 || v.Powered || !v.PowerRelink {
		t.Fatalf("start: %+v", v)
	}
	ver := g.World().PowerVer

	reconnect(g, r.lamp)
	v = psView(t, g, r.lamp)
	if v.PowerNet != netB || v.LinkX != 12 || v.LinkY != 10 || !v.PowerRelink {
		t.Fatalf("after Reconnect: %+v", v)
	}
	w := g.World()
	if !v.Powered || w.PowerVer == ver {
		t.Fatalf("not at once: powered %v, ver %d", v.Powered, w.PowerVer)
	}
	if n, ok := psNet(w, netB); !ok || n.Load != 50 || n.Gen != 800 {
		t.Fatalf("grid B %+v", n)
	}
	if n, ok := psNet(w, netA); !ok || n.Load != 0 {
		t.Fatalf("grid A %+v", n)
	}
	if g.pw.pins[r.lamp] != uint64(r.b[0]) {
		t.Fatalf("pin %d, want %d", g.pw.pins[r.lamp], r.b[0])
	}

	reconnect(g, r.lamp)
	if v = psView(t, g, r.lamp); v.PowerNet != netA || v.LinkX != 6 || v.LinkY != 4 || v.Powered {
		t.Fatalf("wrapped: %+v", v)
	}
}

// TestReconnectRefused: not a device, an off-grid one, no other grid in
// range: nothing changes.
func TestReconnectRefused(t *testing.T) {
	t.Parallel()
	mk := func() (*Game, []entity.EntityID) {
		g := New(Config{Seed: 1, Width: 40, Height: 24})
		line := psConduits(t, g, 4, 2, 6)
		lamp := buildNow(t, g, construction.Lamp, item.Wood, construction.South, 9, 7)
		far := buildNow(t, g, construction.Lamp, item.Wood, construction.South, 30, 20)
		off := buildNow(t, g, construction.Lamp, item.Wood, construction.South, 4, 6)
		g.bldgs.Ref(off).OffGrid = true
		g.Step(powerInterval)
		return g, []entity.EntityID{line[0], lamp, far, off, 9999}
	}
	a, ids := mk()
	b, _ := mk()
	for _, id := range ids {
		a.Enqueue(command.Command{Kind: command.ReconnectPower, Target: uint64(id)})
	}
	a.Step(1)
	b.Step(1)
	if len(a.pw.pins) != 0 || a.StateHash() != b.StateHash() {
		t.Fatalf("pins %v, hash %#x vs %#x", a.pw.pins, a.StateHash(), b.StateHash())
	}
	if v := psView(t, a, ids[1]); !v.HasLink || v.PowerRelink {
		t.Fatalf("one grid in range: %+v", v)
	}
	if v := psView(t, a, ids[2]); v.HasLink || v.PowerRelink {
		t.Fatalf("no grid in range: %+v", v)
	}
}

// TestReconnectNoFlap: a nearer conduit built after the pin doesn't pull
// the wire, and over a day of ticks the wire and the graph hold still.
func TestReconnectNoFlap(t *testing.T) {
	t.Parallel()
	r := newReconnectGrids(t)
	g := r.g
	reconnect(g, r.lamp)
	buildNow(t, g, construction.PowerConduit, item.Steel, construction.South, 10, 8)
	g.Step(powerInterval)
	v := psView(t, g, r.lamp)
	if v.PowerNet != int(r.b[0]) || v.LinkX != 12 || v.LinkY != 10 || !v.Powered {
		t.Fatalf("pulled by a nearer conduit: %+v", v)
	}
	want, _ := g.powerGraph().graph.Link(uint64(r.lamp))
	graphs := g.pwc.graphs
	for i := 0; i < clock.TicksPerDay/powerInterval; i++ {
		g.Step(powerInterval)
		l, ok := g.powerGraph().graph.Link(uint64(r.lamp))
		if !ok || l != want || !g.powered(r.lamp) {
			t.Fatalf("tick %d: link %+v (%v), want %+v, powered %v", g.clock.Now(), l, ok, want, g.powered(r.lamp))
		}
	}
	if g.pwc.graphs != graphs {
		t.Fatalf("the graph was rebuilt %d times over a quiet day", g.pwc.graphs-graphs)
	}
}

// TestReconnectThroughASwitch: a wire pinned to a switch falls back while it
// is open and returns when it closes, and the pin stays through both.
func TestReconnectThroughASwitch(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 40, Height: 24})
	a := psConduits(t, g, 4, 2, 6)
	sw := buildNow(t, g, construction.PowerSwitch, item.Steel, construction.South, 12, 10)
	lamp := buildNow(t, g, construction.Lamp, item.Wood, construction.South, 9, 7)
	g.Step(powerInterval)
	reconnect(g, lamp)
	if v := psView(t, g, lamp); v.PowerNet != int(sw) || v.LinkX != 12 || v.LinkY != 10 {
		t.Fatalf("pinned: %+v", v)
	}
	g.cmdSetPowerWanted(sw, false)
	g.flick(sw)
	if v := psView(t, g, lamp); v.PowerNet != int(a[0]) || v.LinkX != 6 || v.LinkY != 4 {
		t.Fatalf("switch open: %+v", v)
	}
	g.cmdSetPowerWanted(sw, true)
	g.flick(sw)
	if v := psView(t, g, lamp); v.PowerNet != int(sw) || v.LinkX != 12 || v.LinkY != 10 {
		t.Fatalf("switch closed: %+v", v)
	}
}

// TestReconnectPinSurvivesSaveAndLoad: same state, same hash and same wire,
// also running on.
func TestReconnectPinSurvivesSaveAndLoad(t *testing.T) {
	t.Parallel()
	r := newReconnectGrids(t)
	g := r.g
	reconnect(g, r.lamp)
	h := saveLoad(t, g)
	h.pwc.genHook = g.pwc.genHook
	sameState(t, "after load", g, h)
	if g.StateHash() != h.StateHash() {
		t.Fatal("state hash after load")
	}
	if h.pw.pins[r.lamp] != uint64(r.b[0]) || psView(t, h, r.lamp).LinkX != 12 {
		t.Fatalf("pins %v, view %+v", h.pw.pins, psView(t, h, r.lamp))
	}
	for i := 0; i < 4; i++ {
		g.Step(311)
		h.Step(311)
		if a, b := g.StateHash(), h.StateHash(); a != b {
			t.Fatalf("step %d: hash %#x vs %#x", i, a, b)
		}
	}
	if g.World().PowerVer != h.World().PowerVer {
		t.Fatal("power version differs after the reload")
	}
}

// TestReconnectPinPruned: a device's pin goes with it, and a stale pin on a
// removed conductor falls back without losing the device.
func TestReconnectPinPruned(t *testing.T) {
	t.Parallel()
	r := newReconnectGrids(t)
	g := r.g
	reconnect(g, r.lamp)
	g.removeBuilding(r.b[0])
	g.Step(powerInterval)
	if v := psView(t, g, r.lamp); g.pw.pins[r.lamp] != uint64(r.b[0]) || v.LinkX != 6 || v.LinkY != 4 {
		t.Fatalf("pin %d, view %+v", g.pw.pins[r.lamp], v)
	}
	topo := g.pwc.topo
	g.removeBuilding(r.lamp)
	g.Step(powerInterval)
	if len(g.pw.pins) != 0 || g.exportPower().Pins != nil || g.pwc.topo == topo {
		t.Fatalf("pins %v after the device went (topo %d, was %d)", g.pw.pins, g.pwc.topo, topo)
	}
}

// TestPowerSaveWithNoPins: no pins write nothing, and loading gives a usable
// empty map.
func TestPowerSaveWithNoPins(t *testing.T) {
	t.Parallel()
	r := newReconnectGrids(t)
	if p := r.g.exportPower().Pins; p != nil {
		t.Fatalf("pins saved: %v", p)
	}
	h := saveLoad(t, r.g)
	sameState(t, "after load", r.g, h)
	if h.StateHash() != r.g.StateHash() || h.pw.pins == nil || len(h.pw.pins) != 0 {
		t.Fatalf("pins %v, hash %#x vs %#x", h.pw.pins, h.StateHash(), r.g.StateHash())
	}
	reconnect(h, r.lamp)
	if len(h.pw.pins) != 1 {
		t.Fatal("Reconnect on a loaded game")
	}
}

// TestPowerLampWiresToGenerator: no conduit needed: a lamp 5 tiles from a
// generator is powered through it, and the generator has no wire of its own.
func TestPowerLampWiresToGenerator(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 40, Height: 24})
	gen := psFuelled(t, g, 5, 5) // 2×2
	lamp := buildNow(t, g, construction.Lamp, item.Wood, construction.South, 11, 5)
	g.Step(powerInterval)
	v := psView(t, g, lamp)
	if !v.Powered || !g.powered(lamp) || v.PowerNet != int(gen) || !v.HasLink || v.LinkX != 6 || v.LinkY != 5 {
		t.Fatalf("lamp %+v", v)
	}
	if gv := psView(t, g, gen); gv.PowerNet != int(gen) || gv.HasLink || gv.NoPowerReason != "" {
		t.Fatalf("generator %+v", gv)
	}
}

// TestPowerDevicePairOnAGrid: a battery touching a solar panel near a conduit
// is on that grid through the panel's one wire: it charges from the panel and
// feeds a lamp on the line once the panel stops.
func TestPowerDevicePairOnAGrid(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 40, Height: 24})
	line := psConduits(t, g, 10, 2, 8)
	panel := buildNow(t, g, construction.SolarPanel, item.Steel, construction.South, 11, 12) // 3×3
	bat := buildNow(t, g, construction.Battery, item.Steel, construction.South, 14, 12)      // touches it
	lamp := buildNow(t, g, construction.Lamp, item.Wood, construction.South, 3, 12)
	out := 800
	g.pwc.genHook = func(entity.EntityID) int { return out }
	g.Step(powerInterval)
	net := int(line[0])
	pv, bv, lv := psView(t, g, panel), psView(t, g, bat), psView(t, g, lamp)
	if pv.PowerNet != net || !pv.HasLink || pv.LinkX != 8 || pv.LinkY != 10 || pv.LinkFromX != 11 || pv.LinkFromY != 12 {
		t.Fatalf("panel %+v", pv)
	}
	if bv.PowerNet != net || bv.HasLink || bv.PowerRelink || bv.NoPowerReason != "" {
		t.Fatalf("battery %+v", bv)
	}
	if lv.PowerNet != net || !lv.Powered {
		t.Fatalf("lamp %+v", lv)
	}
	g.Step(clock.TicksPerHour)
	stored := g.pw.charge[bat]
	if stored == 0 {
		t.Fatal("the battery didn't charge from the panel")
	}
	out = 0
	g.Step(clock.TicksPerHour)
	if left := g.pw.charge[bat]; left >= stored || left == 0 || !g.powered(lamp) {
		t.Fatalf("battery %d of %d W·ticks, lamp powered %v", left, stored, g.powered(lamp))
	}
}

// TestPowerLoneDeviceNotConnected: a generator or battery with nothing
// touching or wired to it still warns, and is on no net.
func TestPowerLoneDeviceNotConnected(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 40, Height: 24})
	gen := psFuelled(t, g, 5, 5)
	bat := buildNow(t, g, construction.Battery, item.Steel, construction.South, 25, 15)
	g.Step(powerInterval)
	if v := psView(t, g, gen); v.PowerNet != 0 || v.HasLink || v.NoPowerReason != NoPowerNotConnected || v.PowerOut == 0 {
		t.Fatalf("generator %+v", v)
	}
	if v := psView(t, g, bat); v.PowerNet != 0 || v.HasLink || v.NoPowerReason != NoPowerNotConnected {
		t.Fatalf("battery %+v", v)
	}
	if w := g.World(); len(w.PowerNets) != 0 {
		t.Fatalf("nets %+v", w.PowerNets)
	}
	if len(g.powerGraph().order) != 0 {
		t.Fatal("a lone device was balanced")
	}
}

// TestPowerConductingDevicesBalanceOnce: a generator and battery on a grid
// are in the balance once each, and a short circuit still picks conduits.
func TestPowerConductingDevicesBalanceOnce(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 40, Height: 24})
	line := psConduits(t, g, 4, 2, 8)
	gen := psFuelled(t, g, 2, 2)                                                           // touches
	touching := buildNow(t, g, construction.Battery, item.Steel, construction.South, 6, 5) // touches
	wired := buildNow(t, g, construction.Battery, item.Steel, construction.South, 12, 4)   // 4 away
	g.Step(powerInterval)
	c := g.powerGraph()
	var ids []entity.EntityID
	for _, i := range c.order {
		ids = append(ids, c.ids[i])
	}
	slices.Sort(ids)
	if want := []entity.EntityID{gen, touching, wired}; !slices.Equal(ids, want) {
		t.Fatalf("balanced %v, want %v", ids, want)
	}
	net := g.netOf(line[0])
	if net != int(line[0]) || g.netOf(gen) != net || g.netOf(wired) != net || g.netOf(touching) != net {
		t.Fatalf("nets: line %d, generator %d, batteries %d %d", net, g.netOf(gen), g.netOf(touching), g.netOf(wired))
	}
	got := g.netConduits(net)
	if !slices.Equal(got, line) {
		t.Fatalf("short circuit candidates %v, want the line %v", got, line)
	}
}

// TestReconnectBattery: a wired battery moves between grids: it leaves the
// first, and a touching one has no wire to move.
func TestReconnectBattery(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 40, Height: 24})
	a := psConduits(t, g, 4, 2, 6)
	b := psConduits(t, g, 4, 14, 18)
	bat := buildNow(t, g, construction.Battery, item.Steel, construction.South, 10, 4) // 4 from both
	touching := buildNow(t, g, construction.Battery, item.Steel, construction.South, 16, 5)
	g.Step(powerInterval)
	if v := psView(t, g, bat); v.PowerNet != int(a[0]) || !v.PowerRelink {
		t.Fatalf("battery %+v", v)
	}
	reconnect(g, bat)
	if v := psView(t, g, bat); v.PowerNet != int(b[0]) || v.LinkX != 14 {
		t.Fatalf("battery moved: %+v", v)
	}
	if g.netOf(a[0]) == g.netOf(b[0]) {
		t.Fatal("the battery bridged the grids")
	}
	reconnect(g, touching)
	if v := psView(t, g, touching); v.HasLink || v.PowerRelink || g.pw.pins[touching] != 0 {
		t.Fatalf("touching battery %+v, pins %v", v, g.pw.pins)
	}
}
