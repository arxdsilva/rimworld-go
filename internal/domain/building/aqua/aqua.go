// Package aqua is the rules of fish farming in tanks (FISH-*): the stocks a
// tank raises and how a stock grows, suffers, dies and is netted. It holds
// no RNG and reads no clock: the caller steps a Tank over elapsed ticks with
// what it reads of the world.
//
// The building it lives in follows the wiki's Hydroponics basin (1x4, 70 W
// day and night, 100 steel + 1 component, 2,800 work; construction and
// power own those rows). Nothing in vanilla farms fish: every number in this
// package is original tuning.
package aqua

import (
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/world/temperature"
)

// Species is a stock a tank raises (saved by value: append only; the UI's
// TankOption.ID and command.SetTankStock's Count carry it).
type Species uint8

const (
	None Species = iota
	Tilapia
	Carp
	Trout
	NumSpecies
)

// Def is a stock's rules. Original tuning, all of it: harder stocks ask for
// more skill and take longer, and pay for it in fish.
type Def struct {
	Label string
	// GrowTicks is how long good conditions take to bring it to harvest.
	GrowTicks int
	// Yield is the fish a master nets from a ripe tank.
	Yield int
	// MinC and MaxC are the comfort band of the air around the tank, whole
	// degrees C, both inclusive.
	MinC, MaxC int
	// MinSkill is the Animals level a handler needs to work it.
	MinSkill int
}

var defs = [NumSpecies]Def{
	Tilapia: {Label: "tilapia", GrowTicks: 6 * clock.TicksPerDay, Yield: 48, MinC: 15, MaxC: 35, MinSkill: 0},
	Carp:    {Label: "carp", GrowTicks: 8 * clock.TicksPerDay, Yield: 72, MinC: 2, MaxC: 30, MinSkill: 4},
	Trout:   {Label: "trout", GrowTicks: 10 * clock.TicksPerDay, Yield: 110, MinC: 0, MaxC: 20, MinSkill: 8},
}

// DefOf is stock s's rules (the zero Def for no stock).
func DefOf(s Species) Def {
	if s >= NumSpecies {
		return Def{}
	}
	return defs[s]
}

// Valid reports a real stock (not None).
func (s Species) Valid() bool { return s > None && s < NumSpecies }

func (s Species) String() string { return DefOf(s).Label }

// All lists the stocks in the picker's order.
func All() []Species { return []Species{Tilapia, Carp, Trout} }

// Original tuning, like everything here.
const (
	// HealthFull is a healthy stock. It is a day's worth of points so that
	// no power, which costs a point a tick, kills in about a day (the
	// basin's "plants will die if power is cut").
	HealthFull = clock.TicksPerDay
	// WarnBelow starts the "dying" warning and WarnClear lets it fire again:
	// a stock hovering around one line warns once (hysteresis).
	WarnBelow = HealthFull / 2
	WarnClear = HealthFull * 3 / 4

	// Health lost per 1000 ticks of each trouble, and restored per 1000
	// ticks of good conditions: about 1, 2.5 and 4 days from full to dead,
	// 5 days from nothing to full.
	lossNoPower = 1000
	lossAir     = 400
	lossNoFeed  = 250
	restore     = 200

	// FeedItemTicks is how long one item of feed lasts a stock, 2 a day;
	// FeedItems a full tank. FeedBelow is where haulers bring more (a
	// quarter), and they fill to full, so feeding cannot flap.
	FeedItemTicks = clock.TicksPerDay / 2
	FeedItems     = 20
	FeedFull      = FeedItems * FeedItemTicks
	FeedBelow     = FeedFull / 4

	// StockWork and NetWork are the ticks a handler works a tank.
	StockWork = 300
	NetWork   = 450

	// A handler nets fumblePct of a harvest at Animals 0, plus skillPct per
	// level, up to all of it.
	fumblePct = 60
	skillPct  = 4
)

// Cause is why a stock suffers: the first trouble found, in this order.
type Cause uint8

const (
	Fine Cause = iota
	NoPower
	TooCold
	TooHot
	NoFeed
)

// String is the player's words for it ("" when fine).
func (c Cause) String() string {
	switch c {
	case NoPower:
		return "No power"
	case TooCold:
		return "Too cold"
	case TooHot:
		return "Too hot"
	case NoFeed:
		return "No feed"
	}
	return ""
}

// loss is the health cause c costs per 1000 ticks.
func (c Cause) loss() int {
	switch c {
	case NoPower:
		return lossNoPower
	case TooCold, TooHot:
		return lossAir
	case NoFeed:
		return lossNoFeed
	}
	return 0
}

// Conditions is what the world gives a tank right now.
type Conditions struct {
	Power bool
	Air   temperature.Temp
	Fed   bool
}

// Trouble is what is wrong for stock s under c (Fine: nothing).
func (s Species) Trouble(c Conditions) Cause {
	d := DefOf(s)
	switch {
	case !c.Power:
		return NoPower
	case c.Air < temperature.C(d.MinC):
		return TooCold
	case c.Air > temperature.C(d.MaxC):
		return TooHot
	case !c.Fed:
		return NoFeed
	}
	return Fine
}

// Tank is one tank's state: the stock chosen for it, and what lives in it.
// Species stays chosen when a stock dies or is netted.
type Tank struct {
	Species Species
	Stocked bool
	// Growth is ticks of good conditions so far, at most GrowTicks.
	Growth int
	// Health is 0..HealthFull while stocked.
	Health int
	// Warned: the dying warning has fired and not yet been cleared.
	Warned bool
}

// Outcome is what a Step has to tell the player.
type Outcome struct {
	// Cause is the trouble the stock suffered this step.
	Cause Cause
	// Warn: the stock just began dying. Died: it just died of Cause.
	Warn, Died bool
}

// Fill is t freshly stocked.
func (t Tank) Fill() Tank { return Tank{Species: t.Species, Stocked: true, Health: HealthFull} }

// Netted is t after a harvest: the tank restocks by itself and grows again.
func (t Tank) Netted() Tank {
	t.Growth = 0
	return t
}

// Ripe reports a stock ready to net.
func (t Tank) Ripe() bool {
	return t.Stocked && t.Species.Valid() && t.Growth >= DefOf(t.Species).GrowTicks
}

// GrowthPct is the way to harvest in percent.
func (t Tank) GrowthPct() int {
	if d := DefOf(t.Species); t.Stocked && d.GrowTicks > 0 {
		return min(100, t.Growth*100/d.GrowTicks)
	}
	return 0
}

// HealthPct is the stock's health in percent, 1 at least while it lives.
func (t Tank) HealthPct() int {
	if !t.Stocked {
		return 0
	}
	return max(1, t.Health*100/HealthFull)
}

// Step is t after elapsed ticks under c. Trouble costs health and pauses
// growth; good conditions grow the stock and slowly restore it. A stock at 0
// dies and leaves the tank empty, its species still chosen.
func (t Tank) Step(elapsed int, c Conditions) (Tank, Outcome) {
	if !t.Stocked || elapsed <= 0 {
		return t, Outcome{}
	}
	cause := t.Species.Trouble(c)
	if cause == Fine {
		t.Growth = min(DefOf(t.Species).GrowTicks, t.Growth+elapsed)
		t.Health = min(HealthFull, t.Health+elapsed*restore/1000)
		if t.Health >= WarnClear {
			t.Warned = false
		}
		return t, Outcome{}
	}
	out := Outcome{Cause: cause}
	t.Health = max(0, t.Health-elapsed*cause.loss()/1000)
	if t.Health == 0 {
		out.Died = true
		return Tank{Species: t.Species}, out
	}
	if !t.Warned && t.Health < WarnBelow {
		t.Warned, out.Warn = true, true
	}
	return t, out
}

// NetYield is the fish a handler with Animals level nets from a ripe stock
// of s: an unskilled hand fumbles part of it.
func NetYield(s Species, level int) int {
	if !s.Valid() {
		return 0
	}
	return max(1, DefOf(s).Yield*min(100, fumblePct+skillPct*max(0, level))/100)
}
