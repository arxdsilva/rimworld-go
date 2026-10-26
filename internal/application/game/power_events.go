package game

import (
	"fmt"

	"rimworld-go/internal/domain/incident"
	"rimworld-go/internal/domain/sim/clock"
)

// PLAY-021 EVENTS: the two power events of the Nature pool. Both fit only
// a colony that has electricity, so a colony without power devices adds
// nothing to the pool, draws nothing and plays as it always did. Their
// randomness comes from the colony's own power stream (streamPower),
// never from g.rng. The catalogue (domain/incident) holds their weights,
// cooldowns and first days; the grid itself (nets, balance, the flare's
// window) is power.go.
//
// Solar flare: for flareMinHours..flareMaxHours whole hours every net is
// down (power.go setFlare). A caution letter opens it and a plain message
// closes it. The end is found without any state of ours: on the last tick
// of the window (the one before the power phase clears it) the incident
// phase sees the end one tick away and posts the message, so a save in
// the middle of a flare loads into the same end.
//
// Short circuit: a net holding at least shortMinWh and a visible conduit
// loses its stored energy, one of its visible conduits blows (removed,
// no refund) and a fire starts there, bigger for more lost energy. A
// buried (hidden) conduit is never chosen.

const (
	// flareMinHours and flareMaxHours bound a flare's length, in whole
	// hours drawn from the power stream.
	flareMinHours = 3
	flareMaxHours = 8
	// shortMinWh is the stored energy a net needs for a short circuit.
	shortMinWh = 100
	// shortFireWh is the lost energy that adds one to the fire's size
	// (size 1 + lost Wh / shortFireWh).
	shortFireWh = 500
)

// flareFits: the map has power devices and no flare is on.
func (g *Game) flareFits() bool {
	return g.powerDevices() > 0 && !g.solarFlare()
}

// startSolarFlare begins a flare of 3-8 hours now. It reports false when
// it does not fit (nothing is drawn then).
func (g *Game) startSolarFlare(now uint64) bool {
	if !g.flareFits() {
		return false
	}
	hours := flareMinHours + g.stream(streamPower).IntN(flareMaxHours-flareMinHours+1)
	g.setFlare(now + uint64(hours)*clock.TicksPerHour) // power.go: everything is down until then
	x, y := g.home()
	g.post(MessageView{Tick: now, X: x, Y: y, Letter: true, Title: "Solar flare", Kind: letterCaution,
		Text: fmt.Sprintf("A solar flare is washing over the land. Every electrical device will stop for about %d hours, "+
			"and the batteries will hold what they have. Torches and campfires keep burning.", hours)})
	return true
}

// trackFlare posts the flare's end. It runs every tick (phaseIncidents)
// and needs no memory: pw.flareUntil is saved, and the power phase clears
// it at the end tick, so the tick before is the last one that still sees
// it. The message is stamped now, one tick before the power returns.
func (g *Game) trackFlare(now uint64) {
	if u := g.flareUntil(); u != 0 && now+1 == u {
		x, y := g.home()
		g.message(x, y, "The solar flare has passed. Power is coming back.")
	}
}

// shortCircuitNets are the nets a short circuit can hit: at least
// shortMinWh stored and a visible conduit, ascending.
func (g *Game) shortCircuitNets() []int {
	var out []int
	for _, net := range g.storedNets(shortMinWh) { // power.go
		if len(g.netConduits(net)) > 0 {
			out = append(out, net)
		}
	}
	return out
}

// shortCircuit blows one visible conduit of one charged net: the net's
// batteries empty, the conduit is destroyed (nothing comes back) and a
// fire starts where it stood. The net and the conduit are picked from the
// sorted candidates by the power stream. It reports false when no net
// fits (nothing is drawn then).
func (g *Game) shortCircuit(now uint64) bool {
	nets := g.shortCircuitNets()
	if len(nets) == 0 {
		return false
	}
	rng := g.stream(streamPower)
	net := nets[rng.IntN(len(nets))]
	conduits := g.netConduits(net)
	id := conduits[rng.IntN(len(conduits))]
	b, ok := g.bldgs.Get(id)
	if !ok {
		return false
	}
	lost := g.drainNet(net)       // power.go: the batteries empty, the net rebalances
	g.removeBuildingAs(id, false) // placement.go: a short circuit refunds nothing
	g.startFire(b.X, b.Y, 1+lost/shortFireWh)
	g.threat("Short circuit", b.X, b.Y,
		"A power conduit shorted out and burst into flames near %d,%d. The %d Wh stored on its grid are gone.", b.X, b.Y, lost)
	return true
}

// powerEventFits is incidentFits for the two power events.
func (g *Game) powerEventFits(id incident.ID) bool {
	switch id {
	case incident.SolarFlare:
		return g.flareFits()
	case incident.ShortCircuit:
		return len(g.shortCircuitNets()) > 0
	}
	return false
}
