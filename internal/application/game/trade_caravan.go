// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"fmt"
	"slices"

	"rimworld-go/internal/application/game/letters"
	"rimworld-go/internal/application/game/tradeview"
	"rimworld-go/internal/application/game/visit"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/trade"
	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
)

// Trader caravans (PLAY-024, agent S2): incident.TraderCaravan (or a
// caravan asked for over the radio) brings a visitor group from a
// non-hostile faction: a trader with the goods and one to three guards.
// They are ordinary visitors (visitors.go) that stroll within
// traderCaravanRange of home() for traderCaravanStay, then walk out.
// Killing or capturing the trader drops its goods (silver too) forbidden
// where it fell and costs the faction traderLostGoodwill. Faction, kind
// and stock draws come from streamTrade; the edge and the strolls reuse
// the visitors' world stream.

// caravanFits is the map's check for a trader caravan: a colony with
// people, a non-hostile human faction (a legacy colony has none), and no
// caravan or visitors already here.
func (g *Game) caravanFits(now uint64) bool {
	return len(caravansOf(&g.trade)) < 4 && len(g.pawnIDs) > 0 && len(g.visitingFactions()) > 0
}

// caravanArrive sends a trader caravan in; false when it could not come.
func (g *Game) caravanArrive(now uint64) bool {
	facs := g.visitingFactions()
	if len(facs) == 0 {
		return false
	}
	return g.traderCaravanFrom(facs[g.stream(streamTrade).IntN(len(facs))], now, false)
}

// traderCaravanFrom brings faction ref's (1 + index) trader caravan in
// from a map edge; requested: it was asked for over the radio.
func (g *Game) traderCaravanFrom(ref int, now uint64, requested bool) bool {
	return traderCaravanFromKind(g, ref, now, requested, false)
}

func smallTraderArrive(g *Game, now uint64) bool {
	if len(caravansOf(&g.trade)) >= 4 {
		return false
	}
	facs := g.visitingFactions()
	if len(facs) == 0 {
		return false
	}
	return traderCaravanFromKind(g, facs[g.stream(streamTrade).IntN(len(facs))], now, false, true)
}

func traderCaravanFromKind(g *Game, ref int, now uint64, requested, small bool) bool {
	f := g.factionOf(ref)
	if f == nil || len(caravansOf(&g.trade)) >= 4 {
		return false
	}
	r := g.stream(streamTrade)
	n := 2 + r.IntN(3)
	kind := trade.BulkHauler
	duration := uint64(traderCaravanStay)
	if small {
		n = 1 + r.IntN(2)
		duration = clock.TicksPerDay / 2
	} else if r.IntN(2) == 1 {
		kind = trade.Quartermaster
	}
	arrival, ok := visitsOf(g).ArriveCaravan(ref, f, now, duration, small, n, nil)
	if !ok {
		return false
	}
	stock := trade.Generate(g.sk.content.Items, kind, faction.DefOf(f.Type).Tech, r)
	if small {
		visit.SmallStock(&stock)
	}
	c := &traderCaravan{GroupID: arrival.GroupID, Trader: arrival.Trader, Members: arrival.Members,
		Faction: f.ID, Kind: kind, Stock: stock, LeaveAt: now + duration}
	gr, _ := visitsOf(g).GroupOf(c.Trader)
	c.Cargo = visit.AssignCargo(stock, append(slices.Clone(c.Members), visit.CargoOwners(gr, c.Trader, g.animals.Has)...))
	if g.trade.caravan == nil {
		g.trade.caravan = c
	} else {
		g.trade.others = append(g.trade.others, c)
	}
	ids := arrival.Members
	why := "has come"
	if requested {
		why = "has come as you asked"
	}
	if small {
		letterf(g, letters.Info, "Visitors with goods", ids[0], "%s of %s has come to visit and brought a few things to trade.", g.name(ids[0]), f.Name)
	} else {
		letterf(g, letters.Info, "Trader caravan", ids[0], "%s, a %s of %s, %s to trade, with %d guards. They will camp near the colony for most of a day. Right-click the trader with a colonist selected to trade.",
			g.name(ids[0]), tradeview.LowerLabel(trade.DefOf(kind).Label), f.Name, why, len(ids)-1)
	}
	return true
}

// traderOpen reports trader id is ready to trade now: the caravan's
// trader, conscious, on the map and not yet leaving.
func (g *Game) traderOpen(id entity.EntityID) bool {
	c := caravanByTrader(&g.trade, id)
	if c == nil || c.Turned || !g.visitor(id) || uint64(g.clock.Now()) >= c.LeaveAt {
		return false
	}
	if !visitsOf(g).Trading(id, uint64(g.clock.Now())) {
		return false
	}
	v, ok := g.vital.Get(id)
	return ok && v.Status == health.Alive
}

// traderAt is the trader standing at x, y (0: none). One that turned
// (MAP-18) still counts, so the order menu says why it won't trade.
func (g *Game) traderAt(x, y int) entity.EntityID {
	for _, c := range caravansOf(&g.trade) {
		if !c.Turned && !g.visitor(c.Trader) {
			continue
		}
		if p, ok := g.positions.Get(c.Trader); ok && p.X == x && p.Y == y {
			return c.Trader
		}
	}
	return 0
}

// traderHeld reports visitor id is the trader while a colonist walks up
// to it or trades with it: it stays put (no new stroll).
func (g *Game) traderHeld(id entity.EntityID) bool {
	c := caravanByTrader(&g.trade, id)
	if c == nil {
		return false
	}
	if s := g.trade.session; s != nil && s.trader == id {
		return true
	}
	_, taken := g.reserve.EntityOwner(id)
	return taken
}

// caravanStrollRange is how far from home() visitor id strolls.
func (g *Game) caravanStrollRange(id entity.EntityID) int {
	for _, c := range caravansOf(&g.trade) {
		if _, ok := slices.BinarySearch(c.Members, id); ok {
			return traderCaravanRange
		}
	}
	return visitorRange
}

// traderVisitorGone processes each carrier once. The trader's departure
// closes negotiations; surviving guards retain their own cargo until they
// also depart, are killed or are taken prisoner.
func (g *Game) traderVisitorGone(id entity.EntityID) {
	c := caravanByMember(&g.trade, id)
	if c == nil {
		return
	}
	i, ok := slices.BinarySearch(c.Members, id)
	if !ok {
		return
	}
	c.Members = slices.Delete(c.Members, i, i+1)
	if id == c.Trader {
		retireCaravan(&g.trade, c)
		if s := g.trade.session; s != nil && s.trader == id {
			g.trade.session = nil
		}
	}
	departed := false
	if p := g.personOf(id); p != nil {
		departed = p.Where == whereOffMap
	}
	if id == c.Trader && !departed {
		g.traderLost(c)
	} else {
		carrierGone(g, c, id, !departed)
	}
	if c.GroupID == 0 && len(c.Members) == 0 && len(c.Cargo) == 0 {
		removeCaravan(&g.trade, c)
	}
}

func carrierGone(g *Game, c *traderCaravan, id entity.EntityID, lost bool) {
	remaining, lines, silver := visit.LoseCargo(&c.Stock, c.Cargo, id)
	c.Cargo = remaining
	if lost {
		dropCaravanCargo(g, id, lines, silver)
	}
}

func dropCaravanCargo(g *Game, id entity.EntityID, lines []trade.Line, silver int) {
	x, y := g.home()
	if p, ok := g.positions.Get(id); ok {
		x, y = p.X, p.Y
	}
	for _, l := range lines {
		g.dropGoods(l, x, y, true)
	}
	if silver > 0 {
		g.dropGoods(trade.Line{Kind: item.Silver, Qty: silver}, x, y, true)
	}
}

// traderLost removes the trader's own stock. Remaining carriers' shares
// stay assigned and can later depart or be lost independently.
func (g *Game) traderLost(c *traderCaravan) {
	carrierGone(g, c, c.Trader, true)
	f := g.factionByID(c.Faction)
	if f == nil {
		return
	}
	if c.Turned {
		letterf(g, letters.Info, "Trader lost", c.Trader, "%s, the trader of %s, won't reach home. Its goods lie where it fell.", g.name(c.Trader), f.Name)
		return
	}
	now := uint64(g.clock.Now())
	faction.Adjust(f, -traderLostGoodwill, fmt.Sprintf("Day %d: trader killed or taken (%d goodwill)", g.relDay(now)+1, -traderLostGoodwill))
	letterf(g, letters.Caution, "Trader lost", c.Trader, "%s, the trader of %s, is lost to them. Its goods lie where it fell. %s will not forget it (%d goodwill).",
		g.name(c.Trader), f.Name, f.Name, -traderLostGoodwill)
}

// requestedCaravanDue sends the first requested caravan whose time has
// come; one that can't come yet (visitors or another caravan here) tries
// again later, one whose faction turned hostile or fell never comes.
func (g *Game) requestedCaravanDue(now uint64) {
	t := &g.trade
	req := t.requests[0]
	t.requests = slices.Delete(t.requests, 0, 1)
	f := g.factionByID(req.Faction)
	if f == nil || f.Defeated || f.Relation == faction.Hostile {
		return
	}
	busy := len(caravansOf(t)) >= 4
	if !busy && g.traderCaravanFrom(g.factionRef(req.Faction), now, true) {
		return
	}
	g.addRequest(caravanRequest{Faction: req.Faction, At: now + requestRetry})
}

// addRequest queues r in arrival order (ties: lower faction id first).
func (g *Game) addRequest(r caravanRequest) {
	rs := g.trade.requests
	i, _ := slices.BinarySearchFunc(rs, r, func(a, b caravanRequest) int {
		if a.At != b.At {
			if a.At < b.At {
				return -1
			}
			return 1
		}
		return a.Faction - b.Faction
	})
	g.trade.requests = slices.Insert(rs, i, r)
}

// requestPending reports a caravan from faction id is on its way.
func (g *Game) requestPending(id int) bool {
	for _, r := range g.trade.requests {
		if r.Faction == id {
			return true
		}
	}
	return false
}

// Trade listings are visited at order, arrival and departure boundaries.
func caravansOf(t *tradeState) []*traderCaravan {
	out := make([]*traderCaravan, 0, 1+len(t.others))
	if t.caravan != nil {
		out = append(out, t.caravan)
	}
	return append(out, t.others...)
}

func caravanByTrader(t *tradeState, id entity.EntityID) *traderCaravan {
	for _, c := range caravansOf(t) {
		if c.Trader == id {
			return c
		}
	}
	return nil
}

func findCaravan(t *tradeState, match func(*traderCaravan) bool) *traderCaravan {
	for _, c := range caravansOf(t) {
		if match(c) {
			return c
		}
	}
	if c := t.retired; c != nil && match(c) {
		return c
	}
	for _, c := range t.retiredOthers {
		if match(c) {
			return c
		}
	}
	return nil
}

func caravanByMember(t *tradeState, id entity.EntityID) *traderCaravan {
	return findCaravan(t, func(c *traderCaravan) bool { _, found := slices.BinarySearch(c.Members, id); return found })
}

func caravanByGroup(t *tradeState, id uint64) *traderCaravan {
	return findCaravan(t, func(c *traderCaravan) bool { return c.GroupID == id })
}

func removeCaravan(t *tradeState, c *traderCaravan) {
	switch {
	case t.caravan == c:
		t.caravan = nil
		if len(t.others) > 0 {
			t.caravan = t.others[0]
			t.others = slices.Delete(t.others, 0, 1)
		}
	case t.retired == c:
		t.retired = nil
	default:
		if i := slices.Index(t.others, c); i >= 0 {
			t.others = slices.Delete(t.others, i, i+1)
		} else if i := slices.Index(t.retiredOthers, c); i >= 0 {
			t.retiredOthers = slices.Delete(t.retiredOthers, i, i+1)
		}
	}
}

func retireCaravan(t *tradeState, c *traderCaravan) {
	if t.caravan == c || slices.Contains(t.others, c) {
		removeCaravan(t, c)
		if t.retired == nil {
			t.retired = c
		} else {
			t.retiredOthers = append(t.retiredOthers, c)
		}
	}
}

func packCargoLost(g *Game, id entity.EntityID) {
	if g.worldSt == nil {
		return
	}
	groupID := visit.CargoLossGroup(g.worldSt.visitGroups, id)
	if groupID == 0 {
		return
	}
	if c := caravanByGroup(&g.trade, groupID); c != nil {
		carrierGone(g, c, id, true)
	}
}
