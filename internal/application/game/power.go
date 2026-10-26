package game

import (
	"cmp"
	"slices"

	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/building/power"
	"rimworld-go/internal/domain/sim/entity"
)

// PLAY-021 electricity (POWERSIM owns this file): the colony's power
// state, the balance phase and the save round trip. The pure rules (nets,
// balance, shedding, the kinds' specs) are domain/building/power.
//
// phasePower balances every net every powerInterval ticks, integrating
// the exact time since the last balance. A flick, a solar flare's start
// and end and a short circuit's drain balance at once, in the call that
// changes the grid (doFlick, setFlare, drainNet); a building added or
// removed joins the next balance. Nothing waits on a flag across ticks, so
// a save between two ticks loads into the same next balance.
//
// In a colony without power devices (producers, consumers, batteries)
// the phase writes nothing and g.pw stays nil, so its saves and hashes are
// as before. The power buildings are listed once per structure (the
// light.go cache key: structVer, the building count, the store) and their
// nets are rebuilt from that list and the switches' state only when one
// of them changes: never a building scan per tick.
//
// Saved (powerSave): battery charge, wanted and actual on/off, the
// consumers the last balance left unpowered (every other consumer ran),
// shed timers, thermostats, the flare's end and the last balance's tick.
// Nets, outputs and the flick index are derived.

const (
	// powerInterval is the ticks between two balances.
	powerInterval = 50
	// thermoDefault is a new thermostat's target: 21 °C, in tenths.
	thermoDefault = 210
	// thermoMin and thermoMax bound a thermostat's target (tenths of °C).
	thermoMin, thermoMax = -500, 500
)

// powerState is the colony's electricity; Game.pw is nil until the
// colony has a power device (or the player sets a switch or thermostat),
// and again once every device is gone with nothing left to keep.
type powerState struct {
	charge  map[entity.EntityID]int64  // battery energy, W·ticks
	wantOff map[entity.EntityID]bool   // the player wants it off
	isOff   map[entity.EntityID]bool   // it is off (flicked)
	powered map[entity.EntityID]bool   // consumers the last balance ran
	shed    map[entity.EntityID]uint64 // shed consumers: held off until this tick
	thermo  map[entity.EntityID]thermoSave
	// flareUntil is the solar flare's end tick (0: none); last the tick of
	// the last balance.
	flareUntil, last uint64
	// flicks are the pending flicks (wanted ≠ actual), ascending; derived.
	flicks []entity.EntityID
	// ver bumps whenever a consumer's powered state, a device's actual or
	// wanted state or the flare changes; derived, never saved.
	ver uint64
}

// powerCache is the derived electricity (Game.pwc): the power buildings
// and their nets per structure, switch topology and store; never saved.
type powerCache struct {
	// The power buildings, ascending by id, per (structVer, building
	// count, store): ids, specs and the graph nodes (footprints).
	ok     bool
	sv     uint64
	n      int
	bldgs  any
	gen    uint64 // bumps on every list rebuild
	pruned uint64 // the list gen powerState was last pruned against
	ids    []entity.EntityID
	specs  []power.Spec
	nodes  []power.Node
	// devices counts the producers, consumers and batteries.
	devices int

	// The graph, per (list gen, topo): topo bumps when a switch flicks.
	gOK         bool
	gList, gTop uint64
	topo        uint64
	ggen        uint64 // bumps on every graph rebuild
	graph       power.Graph
	b           power.Builder
	// order: the connected devices' node indices by (net, id).
	order []int32
	nets  []uint64 // per order entry: its net

	devs []power.Dev // balance scratch

	// The view's PowerVer hash, per (pw.ver, graph gen); vers is the
	// highest ver handed out, so a new powerState never reuses one.
	hOK        bool
	hVer, hGen uint64
	hash       uint64
	vers       uint64

	builds, graphs int // rebuilds so far (tests)
	// genHook, when set (tests only), replaces genOutput.
	genHook func(entity.EntityID) int
}

// --- the power buildings and their nets --------------------------------

// powerList is the cache with its building list current.
func (g *Game) powerList() *powerCache {
	c := &g.pwc
	if c.ok && c.sv == g.structVer && c.n == len(g.bldgIDs) && c.bldgs == any(g.bldgs) {
		return c
	}
	c.ids, c.specs, c.nodes, c.devices = c.ids[:0], c.specs[:0], c.nodes[:0], 0
	for _, id := range g.bldgIDs {
		b := g.bldgs.Ref(id)
		if b == nil {
			continue
		}
		s := power.SpecOf(b.Kind)
		if s.Role == power.None {
			continue
		}
		cells, _, _ := construction.Footprint(b.Kind, b.Facing, b.X, b.Y)
		c.ids = append(c.ids, id)
		c.specs = append(c.specs, s)
		c.nodes = append(c.nodes, power.Node{ID: uint64(id), Role: s.Role, Cells: cells})
		if isDevice(s.Role) {
			c.devices++
		}
	}
	if !slices.IsSorted(c.ids) { // building order is id order but for odd loads
		idx := make([]int, len(c.ids))
		for i := range idx {
			idx[i] = i
		}
		slices.SortFunc(idx, func(a, b int) int { return cmp.Compare(c.ids[a], c.ids[b]) })
		ids, specs, nodes := slices.Clone(c.ids), slices.Clone(c.specs), slices.Clone(c.nodes)
		for i, j := range idx {
			c.ids[i], c.specs[i], c.nodes[i] = ids[j], specs[j], nodes[j]
		}
	}
	c.ok, c.sv, c.n, c.bldgs = true, g.structVer, len(g.bldgIDs), g.bldgs
	c.gen++
	c.builds++
	return c
}

// isDevice: a role the balance works on (not a conduit or switch).
func isDevice(r power.Role) bool {
	return r == power.Producer || r == power.Consumer || r == power.Storage
}

// powerNode is building id's index in the list (ok false: not a power
// building).
func (c *powerCache) powerNode(id entity.EntityID) (int, bool) {
	return slices.BinarySearch(c.ids, id)
}

// powerGraph is the cache with its list and nets current.
func (g *Game) powerGraph() *powerCache {
	c := g.powerList()
	if c.gOK && c.gList == c.gen && c.gTop == c.topo {
		return c
	}
	for i := range c.nodes {
		if c.nodes[i].Role == power.Switch {
			c.nodes[i].Closed = g.powerOn(c.ids[i])
		}
	}
	c.graph = c.b.Build(g.grid.Width(), g.grid.Height(), c.nodes)
	c.order, c.nets = c.order[:0], c.nets[:0]
	for i, n := range c.nodes {
		if !isDevice(n.Role) {
			continue
		}
		if net := c.graph.Net(n.ID); net != 0 {
			c.order = append(c.order, int32(i))
		}
	}
	slices.SortStableFunc(c.order, func(a, b int32) int {
		return cmp.Compare(c.graph.Net(c.nodes[a].ID), c.graph.Net(c.nodes[b].ID))
	})
	for _, i := range c.order {
		c.nets = append(c.nets, c.graph.Net(c.nodes[i].ID))
	}
	c.gOK, c.gList, c.gTop = true, c.gen, c.topo
	c.ggen++
	c.graphs++
	return c
}

// --- the phase ------------------------------------------------------------

// phasePower balances every power net (before phaseClimate): every
// powerInterval ticks, and at the tick a solar flare ends.
func (g *Game) phasePower() {
	now := uint64(g.clock.Now())
	if now%powerInterval != 0 && (g.pw == nil || g.pw.flareUntil == 0 || now != g.pw.flareUntil) {
		return
	}
	g.balancePower(now)
}

// balancePower integrates every net from the last balance to now: each
// net's generation, load and batteries (power.Balance), the consumers'
// powered and shed state, the batteries' charge. During a solar flare
// every consumer is down, nothing is generated and the batteries hold
// their charge. A colony without devices writes nothing.
func (g *Game) balancePower(now uint64) {
	c := g.powerList()
	if g.pw == nil && c.devices == 0 {
		return
	}
	pw := g.ensurePower(now)
	g.prunePower(c)
	if c.devices == 0 && pw.idle() {
		g.dropPower()
		return
	}
	from := min(pw.last, now)
	if pw.flareUntil != 0 && now >= pw.flareUntil {
		from = max(from, min(pw.flareUntil, now)) // the batteries were frozen
		pw.flareUntil = 0
		g.bumpPower()
	}
	pw.last = now
	c = g.powerGraph()
	if pw.flareUntil != 0 { // the flare: everything down, nothing moves
		for i, s := range c.specs {
			if s.Role == power.Consumer {
				g.setPowered(c.ids[i], false)
			}
		}
		return
	}
	dt := int(now - from)
	connected := 0
	for lo := 0; lo < len(c.order); {
		hi := lo + 1
		for hi < len(c.order) && c.nets[hi] == c.nets[lo] {
			hi++
		}
		g.balanceNet(c, c.order[lo:hi], dt, now)
		connected += hi - lo
		lo = hi
	}
	if connected < c.devices { // devices on no net: consumers unpowered
		for i, s := range c.specs {
			if s.Role == power.Consumer && c.graph.Net(c.nodes[i].ID) == 0 {
				g.setPowered(c.ids[i], false)
			}
		}
	}
}

// balanceNet balances one net's devices (node indices) over dt ticks.
//
// A dead net (nothing generated, nothing stored) puts no new shed holds
// on its consumers: they are simply unpowered, and run as soon as the net
// has power (a generator finished, a switch closed), not an hour later.
// Holds already running are kept, so a net that keeps dying and coming
// back still cannot flick its consumers.
func (g *Game) balanceNet(c *powerCache, net []int32, dt int, now uint64) {
	pw := g.pw
	devs := c.devs[:0]
	live := false
	for _, i := range net {
		id, s := c.ids[i], c.specs[i]
		d := power.Dev{ID: uint64(id), Role: s.Role, Class: s.Class}
		switch s.Role {
		case power.Producer:
			if g.powerOn(id) {
				d.Out = g.powerGen(id)
			}
		case power.Consumer:
			if g.powerOn(id) {
				d.Want = g.powerWant(id, s)
			}
			d.ShedUntil = pw.shed[id]
		case power.Storage:
			d.Stored, d.Cap = pw.charge[id], int64(s.CapWh)*power.WTicksPerWh
		}
		live = live || d.Out > 0 || d.Stored > 0
		devs = append(devs, d)
	}
	c.devs = devs
	power.Balance(devs, dt, now)
	for k := range devs {
		d := &devs[k]
		id := entity.EntityID(d.ID)
		switch d.Role {
		case power.Consumer:
			g.setPowered(id, d.Powered)
			if !live && pw.shed[id] == 0 {
				d.ShedUntil = 0 // a dead net: unpowered, no hold
			}
			switch {
			case d.ShedUntil == 0:
				if _, ok := pw.shed[id]; ok {
					delete(pw.shed, id)
				}
			case pw.shed[id] != d.ShedUntil:
				pw.shed[id] = d.ShedUntil
			}
		case power.Storage:
			if d.Stored == 0 {
				if _, ok := pw.charge[id]; ok {
					delete(pw.charge, id)
				}
			} else {
				pw.charge[id] = d.Stored
			}
		}
	}
}

// powerGen is producer id's output in W before switches and flares
// (power_gen.go).
func (g *Game) powerGen(id entity.EntityID) int {
	if h := g.pwc.genHook; h != nil {
		return h(id)
	}
	return g.genOutput(id)
}

// powerWant is the W consumer id would draw now while on: a lamp always
// its draw; a heater or cooler its draw while its thermostat has it
// working, its idle draw otherwise; an electric bench its draw while
// someone holds it for work, a tenth otherwise.
func (g *Game) powerWant(id entity.EntityID, s power.Spec) int {
	switch s.Class {
	case power.ClassHeater, power.ClassCooler:
		if g.thermoActive(id) {
			return s.Draw
		}
		return s.Idle
	case power.ClassBench:
		if _, held := g.reserve.EntityOwner(id); held {
			return s.Draw
		}
		return s.Idle
	}
	return s.Draw
}

// ensurePower is g.pw, made (balanced from now on) if there is none.
func (g *Game) ensurePower(now uint64) *powerState {
	if g.pw == nil {
		g.pw = newPowerState()
		g.pw.last = now
		g.bumpPower()
	}
	return g.pw
}

func newPowerState() *powerState {
	return &powerState{
		charge: map[entity.EntityID]int64{}, wantOff: map[entity.EntityID]bool{}, isOff: map[entity.EntityID]bool{},
		powered: map[entity.EntityID]bool{}, shed: map[entity.EntityID]uint64{}, thermo: map[entity.EntityID]thermoSave{},
	}
}

// idle: nothing worth keeping without devices (no switch or thermostat
// set, no flare).
func (pw *powerState) idle() bool {
	return len(pw.wantOff) == 0 && len(pw.isOff) == 0 && len(pw.thermo) == 0 && pw.flareUntil == 0
}

// dropPower forgets the power state (every device is gone).
func (g *Game) dropPower() {
	if g.pw == nil {
		return
	}
	g.bumpPower()
	g.pw = nil
}

// bumpPower moves pw.ver (and the cache's version floor).
func (g *Game) bumpPower() {
	c := &g.pwc
	c.vers++
	if g.pw != nil {
		g.pw.ver = c.vers
	}
}

// prunePower drops the state of buildings that are gone (or no longer
// take it), once per building list.
func (g *Game) prunePower(c *powerCache) {
	pw := g.pw
	if pw == nil || c.pruned == c.gen {
		return
	}
	c.pruned = c.gen
	role := func(id entity.EntityID) power.Spec {
		if i, ok := c.powerNode(id); ok {
			return c.specs[i]
		}
		return power.Spec{}
	}
	for id := range pw.charge {
		if role(id).Role != power.Storage {
			delete(pw.charge, id)
		}
	}
	for id := range pw.powered {
		if role(id).Role != power.Consumer {
			delete(pw.powered, id)
		}
	}
	for id := range pw.shed {
		if role(id).Role != power.Consumer {
			delete(pw.shed, id)
		}
	}
	for id := range pw.thermo {
		if !isThermostat(role(id)) {
			delete(pw.thermo, id)
		}
	}
	topo := false
	for id := range pw.isOff {
		if s := role(id); !s.Toggle {
			delete(pw.isOff, id)
			topo = topo || s.Role == power.Switch
		}
	}
	for id := range pw.wantOff {
		if !role(id).Toggle {
			delete(pw.wantOff, id)
		}
	}
	if topo {
		c.topo++
	}
	pw.flicks = slices.DeleteFunc(pw.flicks, func(id entity.EntityID) bool { return !role(id).Toggle })
}

// isThermostat: a heater or a cooler.
func isThermostat(s power.Spec) bool {
	return s.Class == power.ClassHeater || s.Class == power.ClassCooler
}

// --- hooks for the consumers, generators and events ---------------------

// powered reports consumer id running on power now (lamps, benches,
// heaters, coolers): the last balance ran it and it is still on a net.
func (g *Game) powered(id entity.EntityID) bool {
	if g.pw == nil || !g.pw.powered[id] {
		return false
	}
	return g.netOf(id) != 0
}

// setPowered records consumer id's powered state, moving the power
// version and, for a lamp, the light grid.
func (g *Game) setPowered(id entity.EntityID, on bool) {
	pw := g.pw
	if pw.powered[id] == on {
		return
	}
	if on {
		pw.powered[id] = true
	} else {
		delete(pw.powered, id)
	}
	g.bumpPower()
	if b := g.bldgs.Ref(id); b != nil && power.SpecOf(b.Kind).Class == power.ClassLamp {
		g.light.ver++ // light.go: a lamp lights only while powered
	}
}

// powerOn reports building id's actual on/off state (true for anything
// that is not switched off, a non-toggle included).
func (g *Game) powerOn(id entity.EntityID) bool { return g.pw == nil || !g.pw.isOff[id] }

// powerWantOn reports the state the player wants for building id.
func (g *Game) powerWantOn(id entity.EntityID) bool { return g.pw == nil || !g.pw.wantOff[id] }

// netOf is building id's power net (0: none, or not electrical).
func (g *Game) netOf(id entity.EntityID) int {
	c := g.powerGraph()
	return int(c.graph.Net(uint64(id)))
}

// powerVer moves whenever a consumer's powered state, a device's actual
// or wanted state or the flare changes (0: no power state).
func (g *Game) powerVer() uint64 {
	if g.pw == nil {
		return 0
	}
	return g.pw.ver
}

// powerBuildings lists the power buildings (any role), ascending by id,
// with their specs: read only, valid until the structure changes.
func (g *Game) powerBuildings() ([]entity.EntityID, []power.Spec) {
	c := g.powerList()
	return c.ids, c.specs
}

// powerDevices counts the colony's producers, consumers and batteries.
func (g *Game) powerDevices() int { return g.powerList().devices }

// thermoTarget is heater or cooler id's target temperature, tenths of °C.
func (g *Game) thermoTarget(id entity.EntityID) int {
	if g.pw != nil {
		if t, ok := g.pw.thermo[id]; ok {
			return t.Target
		}
	}
	return thermoDefault
}

// setThermoTarget sets heater or cooler id's target (tenths of °C,
// clamped); anything else is ignored.
func (g *Game) setThermoTarget(id entity.EntityID, tenths int) {
	if !g.isThermostatBldg(id) {
		return
	}
	g.putThermo(id, func(t *thermoSave) { t.Target = min(max(tenths, thermoMin), thermoMax) })
}

// thermoActive reports heater or cooler id working (its hysteresis
// state, power_heat.go).
func (g *Game) thermoActive(id entity.EntityID) bool {
	return g.pw != nil && g.pw.thermo[id].Active
}

// setThermoActive records heater or cooler id working or idle.
func (g *Game) setThermoActive(id entity.EntityID, on bool) {
	if g.thermoActive(id) == on || !g.isThermostatBldg(id) {
		return
	}
	g.putThermo(id, func(t *thermoSave) { t.Active = on })
}

func (g *Game) isThermostatBldg(id entity.EntityID) bool {
	b := g.bldgs.Ref(id)
	return b != nil && isThermostat(power.SpecOf(b.Kind))
}

// putThermo edits id's thermostat, keeping only non-default ones.
func (g *Game) putThermo(id entity.EntityID, edit func(*thermoSave)) {
	pw := g.ensurePower(uint64(g.clock.Now()))
	t, ok := pw.thermo[id]
	if !ok {
		t.Target = thermoDefault
	}
	edit(&t)
	if t == (thermoSave{Target: thermoDefault}) {
		delete(pw.thermo, id)
		return
	}
	pw.thermo[id] = t
}

// solarFlare reports a solar flare in progress: every net is down.
func (g *Game) solarFlare() bool {
	return g.pw != nil && uint64(g.clock.Now()) < g.pw.flareUntil
}

// flareUntil is the solar flare's end tick (0: none).
func (g *Game) flareUntil() uint64 {
	if g.pw == nil {
		return 0
	}
	return g.pw.flareUntil
}

// setFlare starts (or extends) a solar flare until tick until: the grid
// is balanced up to now, then every net is down until then (EVENTS).
func (g *Game) setFlare(until uint64) {
	now := uint64(g.clock.Now())
	if until <= now {
		return
	}
	g.balancePower(now)
	pw := g.ensurePower(now)
	if pw.flareUntil != until {
		pw.flareUntil = until
		g.bumpPower()
	}
	g.balancePower(now)
}

// storedNets lists the nets holding at least minWh, ascending.
func (g *Game) storedNets(minWh int) []int {
	if g.pw == nil {
		return nil
	}
	c := g.powerGraph()
	var out []int
	for lo := 0; lo < len(c.order); {
		hi, e := lo, int64(0)
		for ; hi < len(c.order) && c.nets[hi] == c.nets[lo]; hi++ {
			if i := c.order[hi]; c.specs[i].Role == power.Storage {
				e += g.pw.charge[c.ids[i]]
			}
		}
		if e > 0 && e >= int64(minWh)*power.WTicksPerWh {
			out = append(out, int(c.nets[lo]))
		}
		lo = hi
	}
	return out
}

// netStoredWh is net's stored energy in whole Wh.
func (g *Game) netStoredWh(net int) int {
	if g.pw == nil {
		return 0
	}
	c := g.powerGraph()
	e := int64(0)
	for k, i := range c.order {
		if int(c.nets[k]) == net && c.specs[i].Role == power.Storage {
			e += g.pw.charge[c.ids[i]]
		}
	}
	return int(e / power.WTicksPerWh)
}

// netConduits lists net's visible conduits, ascending (a short circuit
// picks one).
func (g *Game) netConduits(net int) []entity.EntityID {
	c := g.powerGraph()
	var out []entity.EntityID
	for i, s := range c.specs {
		if s.Role == power.Transmitter && !s.Hidden && int(c.graph.Net(c.nodes[i].ID)) == net {
			out = append(out, c.ids[i])
		}
	}
	return out
}

// drainNet empties net's batteries (a short circuit), balancing up to now
// first and again after; it returns the Wh drained.
func (g *Game) drainNet(net int) int {
	if g.pw == nil || net == 0 {
		return 0
	}
	now := uint64(g.clock.Now())
	g.balancePower(now)
	if g.pw == nil {
		return 0
	}
	c := g.powerGraph()
	e := int64(0)
	for k, i := range c.order {
		if int(c.nets[k]) == net && c.specs[i].Role == power.Storage {
			e += g.pw.charge[c.ids[i]]
			delete(g.pw.charge, c.ids[i])
		}
	}
	g.balancePower(now)
	return int(e / power.WTicksPerWh)
}

// --- saves ----------------------------------------------------------------

// exportPower is the Power save field (nil: no power state). Only the
// buildings still standing are written; Unpowered is every consumer the
// last balance did not run, a consumer built since included.
func (g *Game) exportPower() *powerSave {
	pw := g.pw
	if pw == nil {
		return nil
	}
	c := g.powerList()
	s := &powerSave{FlareUntil: pw.flareUntil, Last: pw.last}
	for i, id := range c.ids {
		sp := c.specs[i]
		if v := pw.charge[id]; v != 0 && sp.Role == power.Storage {
			s.Charge = append(s.Charge, entry[int64]{ID: id, V: v})
		}
		if sp.Toggle && pw.wantOff[id] {
			s.WantOff = append(s.WantOff, id)
		}
		if sp.Toggle && pw.isOff[id] {
			s.IsOff = append(s.IsOff, id)
		}
		if sp.Role != power.Consumer {
			continue
		}
		if !pw.powered[id] {
			s.Unpowered = append(s.Unpowered, id)
		}
		if v := pw.shed[id]; v != 0 {
			s.Shed = append(s.Shed, entry[uint64]{ID: id, V: v})
		}
		if t, ok := pw.thermo[id]; ok && isThermostat(sp) {
			s.Thermo = append(s.Thermo, entry[thermoSave]{ID: id, V: t})
		}
	}
	return s
}

// importPower restores exportPower's state (nil: an older save, or no
// power) and rebuilds the flick index.
func (g *Game) importPower(s *powerSave) {
	g.pwc.hOK = false
	g.pwc.topo++
	if s == nil {
		g.pw = nil
		return
	}
	pw := newPowerState()
	pw.flareUntil, pw.last = s.FlareUntil, s.Last
	for _, e := range s.Charge {
		pw.charge[e.ID] = e.V
	}
	for _, id := range s.WantOff {
		pw.wantOff[id] = true
	}
	for _, id := range s.IsOff {
		pw.isOff[id] = true
	}
	for _, e := range s.Shed {
		pw.shed[e.ID] = e.V
	}
	for _, e := range s.Thermo {
		pw.thermo[e.ID] = e.V
	}
	g.pw = pw
	c := g.powerList()
	unpowered := map[entity.EntityID]bool{}
	for _, id := range s.Unpowered {
		unpowered[id] = true
	}
	for i, id := range c.ids {
		if c.specs[i].Role == power.Consumer && !unpowered[id] {
			pw.powered[id] = true
		}
		if c.specs[i].Toggle && pw.wantOff[id] != pw.isOff[id] {
			pw.flicks = append(pw.flicks, id)
		}
	}
	g.bumpPower()
}
