package game

import (
	"slices"

	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/building/power"
	"rimworld-go/internal/domain/sim/entity"
)

// PLAY-021 power read models (POWERSIM owns this file): the BuildingView
// power fields and WorldView.PowerNets, SolarFlare and PowerVer. Read
// only: filling them never balances. They are worked out from the saved
// state and the current nets (the derived graph is rebuilt if the
// structure moved, like the rooms), so a loaded game shows exactly what
// the one it was saved from does.

// PowerNetView is one power net for the overlay and the inspector: its id
// (BuildingView.PowerNet), W generated and W drawn now, the batteries'
// stored energy and capacity in Wh, and whether consumers are shed.
type PowerNetView struct {
	ID, Gen, Load, StoredWh, CapWh int
	Brownout                       bool
}

// Why a device has no power, or makes none (BuildingView.NoPowerReason).
const (
	NoPowerNotConnected = "not connected"
	NoPowerBrownout     = "brownout"
	NoPowerSwitchedOff  = "switched off"
	NoPowerSolarFlare   = "solar flare"
	NoPowerNoFuel       = "no fuel"
)

// fillPowerBuilding fills building id's power fields (PowerRole through
// ThermoTarget; Glow comes from light.go).
func (g *Game) fillPowerBuilding(bv *BuildingView, id entity.EntityID, b construction.Building) {
	s := power.SpecOf(b.Kind)
	if s.Role == power.None {
		return
	}
	c := g.powerGraph()
	net := c.graph.Net(uint64(id))
	bv.PowerRole, bv.PowerNet = s.Role.String(), int(net)
	bv.PowerOn, bv.PowerWantOn = g.powerOn(id), g.powerWantOn(id)
	if l, ok := c.graph.Link(uint64(id)); ok {
		bv.LinkX, bv.LinkY, bv.HasLink = l.X, l.Y, true
		bv.LinkFromX, bv.LinkFromY = l.FromX, l.FromY
	}
	switch s.Role {
	case power.Consumer:
		if bv.Powered = g.powered(id); bv.Powered {
			bv.PowerUse = g.powerWant(id, s)
		} else {
			bv.NoPowerReason = g.noPowerWhy(id, net, false)
		}
		if isThermostat(s) {
			bv.ThermoTarget, bv.ThermoActive = g.thermoTarget(id), g.thermoActive(id)
		}
	case power.Producer:
		bv.PowerMax = s.Peak
		if bv.PowerOut = g.liveOutput(id); bv.PowerOut == 0 {
			bv.NoPowerReason = g.noPowerWhy(id, net, b.Kind == construction.WoodGenerator && g.fuel[id] <= 0)
		} else if net == 0 {
			bv.NoPowerReason = NoPowerNotConnected
		}
	case power.Storage:
		bv.CapWh = s.CapWh
		if g.pw != nil {
			bv.StoredWh = int(g.pw.charge[id] / power.WTicksPerWh)
		}
		if net == 0 {
			bv.NoPowerReason = NoPowerNotConnected
		}
	}
}

// noPowerWhy says why device id (on net) has no power or makes none:
// switched off, not connected, the solar flare, out of fuel, or else a
// brownout ("" for a producer that is simply idle, a panel at night).
func (g *Game) noPowerWhy(id entity.EntityID, net uint64, noFuel bool) string {
	switch {
	case !g.powerOn(id):
		return NoPowerSwitchedOff
	case g.solarFlare():
		return NoPowerSolarFlare
	case noFuel:
		return NoPowerNoFuel
	case net == 0:
		return NoPowerNotConnected
	}
	if b := g.bldgs.Ref(id); b != nil && power.SpecOf(b.Kind).Role == power.Consumer {
		return NoPowerBrownout
	}
	return ""
}

// liveOutput is producer id's output now: 0 while it is switched off or a
// solar flare is on.
func (g *Game) liveOutput(id entity.EntityID) int {
	if !g.powerOn(id) || g.solarFlare() {
		return 0
	}
	return g.powerGen(id)
}

// fillPowerView fills the world's power nets, the solar flare flag and
// PowerVer.
func (g *Game) fillPowerView(w *WorldView) {
	c := g.powerGraph()
	nets := c.graph.Nets()
	if len(nets) > 0 {
		w.PowerNets = make([]PowerNetView, len(nets))
		for k, n := range nets {
			w.PowerNets[k].ID = int(n)
		}
		stored := make([]int64, len(nets))
		caps := make([]int64, len(nets))
		for k, i := range c.order {
			at, _ := slices.BinarySearch(nets, c.nets[k])
			nv := &w.PowerNets[at]
			id, s := c.ids[i], c.specs[i]
			switch s.Role {
			case power.Producer:
				nv.Gen += g.liveOutput(id)
			case power.Consumer:
				if g.powered(id) {
					nv.Load += g.powerWant(id, s)
				}
				if g.pw != nil && g.pw.shed[id] != 0 && g.powerOn(id) && !g.solarFlare() {
					nv.Brownout = true
				}
			case power.Storage:
				caps[at] += int64(s.CapWh) * power.WTicksPerWh
				if g.pw != nil {
					stored[at] += g.pw.charge[id]
				}
			}
		}
		for k := range w.PowerNets {
			w.PowerNets[k].StoredWh = int(stored[k] / power.WTicksPerWh)
			w.PowerNets[k].CapWh = int(caps[k] / power.WTicksPerWh)
		}
	}
	w.SolarFlare = g.solarFlare()
	w.PowerVer = g.powerViewVer(c)
}

// powerViewVer is WorldView.PowerVer: a hash of every power building's
// net and its powered, on and wanted state, and of the flare, so it moves
// whenever any of them does and is the same in a game and its reload (0:
// no power state). Cached per (pw.ver, graph).
func (g *Game) powerViewVer(c *powerCache) uint64 {
	pw := g.pw
	if pw == nil {
		return 0
	}
	if c.hOK && c.hVer == pw.ver && c.hGen == c.ggen {
		return c.hash
	}
	const prime = 1099511628211
	h := uint64(14695981039346656037)
	mix := func(v uint64) {
		for range 8 {
			h ^= v & 0xff
			h *= prime
			v >>= 8
		}
	}
	mix(uint64(len(c.ids)))
	for i, id := range c.ids {
		flags := uint64(0)
		if pw.powered[id] {
			flags |= 1
		}
		if pw.isOff[id] {
			flags |= 2
		}
		if pw.wantOff[id] {
			flags |= 4
		}
		mix(uint64(id))
		mix(c.graph.Net(c.nodes[i].ID))
		mix(flags)
	}
	mix(pw.flareUntil)
	h |= 1 // never 0 while there is power state
	c.hOK, c.hVer, c.hGen, c.hash = true, pw.ver, c.ggen, h
	return h
}
