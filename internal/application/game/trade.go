// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"slices"

	"rimworld-go/internal/application/game/beacon"
	"rimworld-go/internal/application/game/store"
	"rimworld-go/internal/application/game/tradeview"
	"rimworld-go/internal/application/game/visit"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/trade"
	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
)

// Trade state (PLAY-024, agent S2): the trader caravan on the map, the
// orbital ship in radio range, drop pods bringing orbital purchases, the
// caravans asked for over the radio and each faction's request
// cooldowns, plus the open trade or comms session. Only one session is
// open at a time; sessions are never saved (a load closes them, and the
// held Trade/UseComms job ends on its next Do tick). Everything here is
// sorted slices (no maps), so the save and the hash are deterministic.
// tradeTick runs once a tick from phaseIncidents and returns at once
// while there is nothing to do.

const (
	tradeSaveVersion = 3
	// tradeWait is how long a negotiator or radio operator stands at the
	// trader or console before the session opens.
	tradeWait = 60
	// traderCaravanStay is how long a trader caravan camps near home().
	traderCaravanStay = clock.TicksPerDay * 3 / 4
	// traderCaravanRange is how far from home() its people stroll.
	traderCaravanRange = 6
	// tradePodFall is how long an orbital purchase takes to land.
	tradePodFall = 60
	// traderLostGoodwill is what killing or capturing a trader costs.
	traderLostGoodwill = 40
	// requestRetry: a requested caravan that can't come yet tries again.
	requestRetry = clock.TicksPerHour
)

// tradeState is the colony's trade.
type tradeState struct {
	caravan       *traderCaravan
	others        []*traderCaravan // other active visiting parties
	retired       *traderCaravan   // surviving guards after the trader's loss
	retiredOthers []*traderCaravan // other former traders with guards still present
	orbital       *orbitalShip
	pods          []tradePod        // by landing tick, then position
	cooldowns     []factionCooldown // ascending faction id
	requests      []caravanRequest  // by arrival tick, then faction
	aid           []aidCall         // military aid, by request (allies.go)
	session       *tradeSession
	comms         *commsSession
}

// traderCaravan is a trader caravan on the map (saved). Trader is the
// visitor carrying the goods, Members the whole group (ascending),
// Faction the faction id (faction.Faction.ID). Turned: its faction turned
// hostile while it was here (MAP-18); it trades no more, even if they calm
// down.
type traderCaravan struct {
	Trader  entity.EntityID
	Members []entity.EntityID
	Faction int
	Kind    trade.TraderKind
	Stock   trade.Stock
	LeaveAt uint64
	Turned  bool
	GroupID uint64
	Cargo   []visit.Cargo
}

// orbitalShip is an orbital trader in radio range until Until (saved).
type orbitalShip struct {
	Name  string
	Kind  trade.TraderKind
	Stock trade.Stock
	Until uint64
}

// tradePod is a drop pod with orbital purchases (saved): it falls until
// Tick, then lands a shell at X, Y with Items beside it; it stays listed
// for the renderer for arrivalKeep after landing.
type tradePod struct {
	X, Y   int
	Tick   uint64
	Landed bool
	Items  []trade.Line
	Reward bool // a quest's reward, not a purchase (QST-089)
}

// factionCooldown is faction id Faction's request cooldowns (saved).
type factionCooldown struct {
	Faction int
	CD      faction.Cooldowns
}

// caravanRequest is a caravan asked for over the radio, due at At.
type caravanRequest struct {
	Faction int
	At      uint64
}

// tradeSession is an open trade: the negotiating colonist and the trader
// (a visitor pawn, or 0 for the orbital ship), and why the last offer was
// refused.
type tradeSession struct {
	negotiator entity.EntityID
	trader     entity.EntityID
	reason     string
}

// commsSession is an open comms session: the operator and the console.
type commsSession struct {
	operator entity.EntityID
	console  entity.EntityID
}

// tradeSave is the saved trade state (versioned), nil while there is
// nothing to keep, so older saves and colonies that never traded hash as
// before.
type tradeSave struct {
	Version         int
	Caravan         *traderCaravan
	Retired         *traderCaravan
	Caravans        []*traderCaravan
	RetiredCaravans []*traderCaravan
	Orbital         *orbitalShip
	Pods            []tradePod
	Cooldowns       []factionCooldown
	Requests        []caravanRequest
	Aid             []aidCall // allies.go (Phase 4): nil without aid
}

// exportTrade is the Trade save field (nil: nothing to keep).
func (g *Game) exportTrade() *tradeSave {
	t := &g.trade
	if t.caravan == nil && len(t.others) == 0 && t.retired == nil && len(t.retiredOthers) == 0 && t.orbital == nil && len(t.pods) == 0 && len(t.cooldowns) == 0 && len(t.requests) == 0 && len(t.aid) == 0 {
		return nil
	}
	s := &tradeSave{Version: tradeSaveVersion, Caravan: cloneCaravan(t.caravan),
		Retired: cloneCaravan(t.retired), Orbital: cloneShip(t.orbital),
		Pods: clonePodsT(t.pods), Cooldowns: slices.Clone(t.cooldowns), Requests: slices.Clone(t.requests), Aid: slices.Clone(t.aid)}
	for _, c := range t.others {
		s.Caravans = append(s.Caravans, cloneCaravan(c))
	}
	for _, c := range t.retiredOthers {
		s.RetiredCaravans = append(s.RetiredCaravans, cloneCaravan(c))
	}
	return s
}

// importTrade restores exportTrade's state (nil: none); open sessions
// close on load.
func (g *Game) importTrade(s *tradeSave) {
	g.trade = tradeState{}
	if s == nil {
		return
	}
	g.trade = tradeState{caravan: cloneCaravan(s.Caravan), retired: cloneCaravan(s.Retired),
		orbital: cloneShip(s.Orbital), pods: clonePodsT(s.Pods),
		cooldowns: slices.Clone(s.Cooldowns), requests: slices.Clone(s.Requests), aid: slices.Clone(s.Aid)}
	for _, c := range s.Caravans {
		g.trade.others = append(g.trade.others, cloneCaravan(c))
	}
	for _, c := range s.RetiredCaravans {
		g.trade.retiredOthers = append(g.trade.retiredOthers, cloneCaravan(c))
	}
	if s.Version < 2 && g.trade.caravan != nil {
		c := g.trade.caravan
		c.Cargo = visit.AssignCargo(c.Stock, c.Members)
	}
}

func cloneStock(s trade.Stock) trade.Stock { return s.Clone() }

func cloneCaravan(c *traderCaravan) *traderCaravan {
	if c == nil {
		return nil
	}
	out := *c
	out.Members = slices.Clone(c.Members)
	out.Stock = cloneStock(c.Stock)
	out.Cargo = visit.CloneCargo(c.Cargo)
	return &out
}

func cloneShip(s *orbitalShip) *orbitalShip {
	if s == nil {
		return nil
	}
	out := *s
	out.Stock = cloneStock(s.Stock)
	return &out
}

func clonePodsT(ps []tradePod) []tradePod {
	out := slices.Clone(ps)
	for i := range out {
		out[i].Items = slices.Clone(out[i].Items)
	}
	return out
}

// tradeTick runs the trade once a tick (phaseIncidents): requested
// caravans come, the ship leaves, pods land and sessions whose people or
// trader are gone close.
func (g *Game) tradeTick(now uint64) {
	wanderersTick(g, now) // world_stash.go (MAP-19)
	t := &g.trade
	if t.caravan == nil && len(t.others) == 0 && t.orbital == nil && len(t.pods) == 0 && len(t.requests) == 0 && len(t.aid) == 0 && t.session == nil && t.comms == nil {
		return
	}
	if len(t.aid) > 0 {
		g.aidTick(now) // allies.go (Phase 4): military aid
	}
	if len(t.requests) > 0 && now >= t.requests[0].At {
		g.requestedCaravanDue(now) // trade_caravan.go
	}
	if t.orbital != nil && now >= t.orbital.Until {
		g.orbitalLeaves() // trade_orbital.go
	}
	if len(t.pods) > 0 {
		g.tradePodsTick(now) // trade_orbital.go
	}
	g.checkSessions()
}

// checkSessions closes a session whose people are no longer at it: the
// negotiator or operator down, off the job, or the trader gone.
func (g *Game) checkSessions() {
	t := &g.trade
	if s := t.session; s != nil {
		_, ok := liveTradeParty(g, s)
		if !ok {
			t.session = nil
		}
	}
	if c := t.comms; c != nil && (!g.holdingJob(c.operator) || !g.consoleUsable(c.console)) {
		t.comms = nil
	}
}

// TradeStatus is the trade alerts' read model: the orbital ship in radio
// range (Present) and the trader caravan on the map (CaravanVisiting,
// CaravanFaction, CaravanHoursLeft). ok: either is here. It reads no
// session, so it works whether or not a dialog is open.
func (g *Game) TradeStatus() (OrbitalView, bool) {
	v := g.orbitalView(uint64(g.clock.Now()))
	if c := g.trade.caravan; c != nil && !c.Turned {
		v.CaravanVisiting = true
		if f := g.factionByID(c.Faction); f != nil {
			v.CaravanFaction = f.Name
		}
		v.CaravanHoursLeft = tradeview.HoursLeft(c.LeaveAt, uint64(g.clock.Now()))
	}
	return v, v.Present || v.CaravanVisiting
}

// factionByID resolves the live diplomacy record by saved faction ID.
func (g *Game) factionByID(id int) *faction.Faction { return faction.FindByID(g.ng.factions, id) }

// factionRef is the 1-based faction index for visitor and raid records.
func (g *Game) factionRef(id int) int { return faction.RefByID(g.ng.factions, id) }

// cooldownOf is faction id's request cooldowns.
func (g *Game) cooldownOf(id int) faction.Cooldowns {
	cs := g.trade.cooldowns
	if i, ok := slices.BinarySearchFunc(cs, id, func(c factionCooldown, id int) int { return c.Faction - id }); ok {
		return cs[i].CD
	}
	return faction.Cooldowns{}
}

// setCooldown records faction id's request cooldowns.
func (g *Game) setCooldown(id int, cd faction.Cooldowns) {
	cs := g.trade.cooldowns
	i, ok := slices.BinarySearchFunc(cs, id, func(c factionCooldown, id int) int { return c.Faction - id })
	if ok {
		cs[i].CD = cd
		return
	}
	g.trade.cooldowns = slices.Insert(cs, i, factionCooldown{Faction: id, CD: cd})
}

// beaconIDs lists the powered trade beacons, ascending.
func (g *Game) beaconIDs() []entity.EntityID {
	return slices.Collect(beacon.Powered(g.bldgIDs, construction.TradeBeacon, g.bldgs.Ref, g.powered))
}

// colonyGoods selects the colony's tradeable stockpile, or the orbital
// beacon inventory when an orbital ship is negotiating.
func (g *Game) colonyGoods(orbital bool) store.TradeGoods {
	if orbital {
		return beaconGoods(g)
	}
	return g.collectGoods(g.stockpiled)
}

// collectGoods delegates physical stack selection and ordering to store.
func (g *Game) collectGoods(in func(x, y int) bool) store.TradeGoods {
	return store.CollectTrade(g.itemIDs, g.stacks.Get, func(st item.Stack) bool {
		return !g.fogged(st.X, st.Y) && in(st.X, st.Y)
	})
}

// takeGoods spends the selected physical stacks against their live state.
func (g *Game) takeGoods(gd store.TradeGoods, k item.Kind, n int) {
	gd.Take(k, n, g.stacks.Get, func(id entity.EntityID, st item.Stack, qty int) {
		g.takeItems(id, st, qty)
	})
}

// dropGoods puts exact traded physical lots down in map stack limits,
// keeping quality, material, wear and book identity after a purchase.
func (g *Game) dropGoods(l trade.Line, x, y int, forbid bool) {
	for _, physical := range trade.DropStacks(g.sk.content.Items, l) {
		sid, err := g.SpawnStack(physical.Kind, physical.Qty, x, y)
		if err != nil {
			return
		}
		physical.X, physical.Y = x, y
		physical.Forbidden = forbid
		g.mergeOrSet(sid, physical)
	}
}
