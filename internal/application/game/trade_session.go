// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"cmp"
	"fmt"
	"slices"

	"rimworld-go/internal/application/game/maps"
	"rimworld-go/internal/application/game/mind"
	"rimworld-go/internal/application/game/tradeview"
	"rimworld-go/internal/application/game/travel"
	"rimworld-go/internal/application/game/visit"
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/ai/pathing"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/trade"
	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/pawn/body"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/pawn/skill"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/pkg/mathx"
)

// Trade sessions (PLAY-024, agent S2). The Trade job walks a colonist up
// to the caravan's trader (re-pathing if it strolled), waits tradeWait
// ticks and opens the session; the job then holds in Do until CloseTrade,
// the trader leaves or falls, or the negotiator goes down or is pulled
// off the job (checkSessions). The orbital session opens from the comms
// dialog (comms.go) and is held by the UseComms job. The dialog reads
// TradeView and proposes deals (command.ProposeTrade); cmdProposeTrade
// settles each one against the live counts, since the sim runs while the
// dialog is open.

// Trade order refusals.
const (
	ReasonNoTrader    = "no trader here"
	ReasonCantTalk    = "can't talk"
	ReasonNoTrade     = "they won't trade with you"
	ReasonNotTrading  = "not trading now"
	ReasonTradeOpen   = "another trade is open"
	ReasonRadioInUse  = "the radio is in use"
	ReasonOfferBroken = "that offer makes no sense"
	ReasonNoGiftee    = "there is no one to give it to"
)

// tradeChase is how often a negotiator re-paths after a trader that
// moved before it gives up.
const tradeChase = 8

// tradeParty is the other side of an open session: the trader's stock
// (live, so a deal changes it), its name and kind, its faction (nil for
// the orbital ship) and when it leaves.
type tradeParty struct {
	stock *trade.Stock
	name  string
	kind  trade.TraderKind
	f     *faction.Faction
	until uint64
}

// party is session s's trader (ok false: it is gone).
func (g *Game) party(s *tradeSession) (tradeParty, bool) {
	if s.trader == 0 {
		o := g.trade.orbital
		if o == nil {
			return tradeParty{}, false
		}
		return tradeParty{stock: &o.Stock, name: o.Name, kind: o.Kind, until: o.Until}, true
	}
	c := caravanByTrader(&g.trade, s.trader)
	if c == nil {
		return tradeParty{}, false
	}
	return tradeParty{stock: &c.Stock, name: g.name(c.Trader), kind: c.Kind, f: g.factionByID(c.Faction), until: c.LeaveAt}, true
}

// termsFor are the price terms negotiator id gets from faction f (nil:
// the orbital ship, goodwill 0).
func (g *Game) termsFor(id entity.EntityID, f *faction.Faction) trade.Terms {
	t := trade.Terms{Social: int(g.skillOf(id, skill.Social).Level), Inspired: g.mind.Inspire.Has(id, mind.Trade)}
	if f != nil {
		t.Goodwill, t.Hostile, t.FactionType = f.Goodwill, f.Relation == faction.Hostile, f.Type
	}
	return t
}

// mapsMarket is the simulation adapter for physical market transactions.
type mapsMarket struct{ g *Game }

func (h mapsMarket) MarketTerms(id entity.EntityID, f *faction.Faction) trade.Terms {
	return h.g.termsFor(id, f)
}
func (h mapsMarket) MarketCargo() maps.MarketFreight { return travelOf(h.g) }
func (h mapsMarket) SplitCargo() maps.SplitFreight   { return travelOf(h.g) }
func (h mapsMarket) HeldMember(id, holder entity.EntityID) (bool, bool) {
	w := h.g.worldSt
	i, ok := slices.BinarySearchFunc(w.stashed, id, stashCmp)
	if !ok || w.stashed[i].Holder != holder || w.stashed[i].Beast != nil {
		return false, false
	}
	v, ok := h.g.vital.Get(id)
	if !ok || v.Status == health.Dead {
		return false, false
	}
	return true, !w.stashed[i].Prisoner && v.Status == health.Alive && !h.g.breaking(id)
}
func (h mapsMarket) TransferHeld(id, holder entity.EntityID) {
	if i, ok := slices.BinarySearchFunc(h.g.worldSt.stashed, id, stashCmp); ok {
		h.g.worldSt.stashed[i].Holder = holder
	}
}
func (h mapsMarket) MarketItems() item.Book { return h.g.sk.content.Items }
func (h mapsMarket) MarketCarriers(c *maps.Caravan) travel.TradeCarriers {
	var r travel.TradeCarriers
	for _, id := range c.C.Animals {
		i, ok := slices.BinarySearchFunc(h.g.worldSt.stashed, entity.EntityID(id), stashCmp)
		if !ok {
			continue
		}
		s := h.g.worldSt.stashed[i]
		if s.Holder == entity.EntityID(c.C.ID) && s.Beast != nil {
			r.Stashed = append(r.Stashed, travel.Beast{ID: s.ID, State: *s.Beast})
		}
	}
	slices.SortFunc(r.Stashed, func(a, b travel.Beast) int { return cmp.Compare(a.ID, b.ID) })
	return r
}
func (h mapsMarket) UseTradeInspiration(id entity.EntityID) {
	h.g.mind.Inspire.Use(id, mind.Trade)
}

// liveTradeParty is the shared admission gate at view, tick and commit.
func liveTradeParty(g *Game, s *tradeSession) (tradeParty, bool) {
	if !g.holdingJob(s.negotiator) || g.capacity(s.negotiator, body.Talking) <= 0 {
		return tradeParty{}, false
	}
	j, _ := g.jobs.Get(s.negotiator)
	if s.trader != 0 {
		if j.Kind != job.Trade || j.Target != s.trader || !g.traderOpen(s.trader) {
			return tradeParty{}, false
		}
	} else if j.Kind != job.UseComms || !g.consoleUsable(j.Target) {
		return tradeParty{}, false
	}
	p, ok := g.party(s)
	return p, ok && uint64(g.clock.Now()) < p.until
}

// TradeView is the open trade session (ok false while none is open).
func (g *Game) TradeView() (TradeView, bool) {
	s := g.trade.session
	if s == nil {
		return TradeView{}, false
	}
	v := TradeView{Negotiator: s.negotiator, NegotiatorName: g.name(s.negotiator),
		Reason: s.reason, Orbital: s.trader == 0}
	p, ok := liveTradeParty(g, s)
	if !ok {
		return v, true
	}
	factionID, factionName, factionType := -1, "", faction.Player
	if p.f != nil {
		factionID, factionName, factionType = p.f.ID, p.f.Name, p.f.Type
	}
	gd := g.colonyGoods(v.Orbital)
	return tradeview.Compose(v, tradeview.Market{
		Kind: p.kind, Stock: *p.stock, Terms: g.termsFor(s.negotiator, p.f),
		Colony: gd.Counts(), GiftStacks: gd.Stacks(), Items: g.sk.content.Items, Label: itemLabel,
		TraderName: p.name, FactionName: factionName, FactionID: factionID, FactionType: factionType,
		HoursLeft: tradeview.HoursLeft(p.until, uint64(g.clock.Now())),
	}), true
}

// cmdProposeTrade settles the offer encoded in offer (trade.EncodeOffer),
// as a gift when gift is set, against the live counts; a refusal is kept
// as the session's reason.
func (g *Game) cmdProposeTrade(offer string, gift bool, exact ...bool) {
	s := g.trade.session
	if s == nil {
		return
	}
	p, ok := liveTradeParty(g, s)
	if !ok {
		g.cmdCloseTrade()
		return
	}
	deltas, err := trade.DecodeOffer(g.sk.content.Items, offer)
	if err != nil {
		s.reason = ReasonOfferBroken
		return
	}
	orbital := s.trader == 0
	if gift && p.f == nil {
		s.reason = ReasonNoGiftee
		return
	}
	gd := g.colonyGoods(orbital)
	terms := g.termsFor(s.negotiator, p.f)
	if gift {
		terms.Inspired = false
	}
	selection := trade.LegacySelection
	if len(exact) != 0 && exact[0] {
		selection = trade.ExactSelection
	}
	res, plan, valid := trade.SettlePhysical(g.sk.content.Items, deltas, gd.Counts(),
		*p.stock, terms, gift, gd.Stacks(), selection)
	if !valid {
		s.reason = ReasonOfferBroken
		return
	}
	if !res.OK() {
		s.reason = res.Reject.String()
		return
	}
	debits, valid := gd.Debits(plan, g.stacks.Get)
	if !valid {
		s.reason = ReasonOfferBroken
		return
	}
	nextStock := cloneStock(*p.stock)
	if !nextStock.ApplyTrade(res) {
		s.reason = ReasonOfferBroken
		return
	}
	if !orbital {
		c := caravanByTrader(&g.trade, s.trader)
		if c == nil || !visit.SettleCargo(c.Cargo, c.Trader, res) {
			s.reason = ReasonOfferBroken
			return
		}
	}
	s.reason = ""
	for _, a := range debits {
		g.takeItems(entity.EntityID(a.ID), a.Stack, a.Qty)
	}
	*p.stock = nextStock
	now := uint64(g.clock.Now())
	pos, _ := g.positions.Get(s.negotiator)
	if gift {
		faction.Adjust(p.f, res.Goodwill, fmt.Sprintf("Day %d: gifts from the colony (%+d goodwill)", g.relDay(now)+1, res.Goodwill))
		g.message(pos.X, pos.Y, "%s accepted the gift (%+d goodwill).", p.f.Name, res.Goodwill)
		return
	}
	g.mind.Inspire.Use(s.negotiator, mind.Trade)
	if res.Silver < 0 {
		g.takeGoods(gd, item.Silver, -res.Silver)
	}
	deliver := res.Deliveries()
	if orbital {
		g.sendTradePod(deliver, now) // trade_orbital.go
	} else {
		for _, l := range deliver {
			g.dropGoods(l, pos.X, pos.Y, false)
		}
	}
	if p.f != nil {
		faction.TradeGoodwill(p.f, res.TradedValue(g.sk.content.Items),
			fmt.Sprintf("Day %d: goods traded with visitors", g.relDay(now)+1))
	}
	g.message(pos.X, pos.Y, "%s traded with %s.", g.name(s.negotiator), p.name)
}

// cmdCloseTrade ends the trade session.
func (g *Game) cmdCloseTrade() { g.trade.session = nil }

// holdingJob reports id is conscious and holding an open session's job
// (Trade or UseComms, past its wait).
func (g *Game) holdingJob(id entity.EntityID) bool {
	if v, ok := g.vital.Get(id); !ok || v.Status != health.Alive {
		return false
	}
	j, _ := g.jobs.Get(id)
	return (j.Kind == job.Trade || j.Kind == job.UseComms) && j.State == job.Do && j.Count == 1
}

// sessionBusy is why id can't open a session now ("" when it can): only
// one trade or comms session is open at a time.
func (g *Game) sessionBusy(id entity.EntityID) string {
	if s := g.trade.session; s != nil && s.negotiator != id {
		return ReasonTradeOpen
	}
	if c := g.trade.comms; c != nil && c.operator != id {
		return ReasonRadioInUse
	}
	return ""
}

// doTrade runs a Trade job's Do phase: catch up with the trader, wait,
// open the session, then hold it (Count 1) until it closes.
func (g *Game) doTrade(id entity.EntityID, j job.Record) {
	if j.Count == 1 {
		if s := g.trade.session; s != nil && s.negotiator == id && s.trader == j.Target {
			return // trading
		}
		j.State = job.Complete
		g.jobs.Set(id, j)
		return
	}
	if !g.traderOpen(j.Target) {
		j.State, j.Fail = job.Fail, job.TargetGone
		g.jobs.Set(id, j)
		return
	}
	pos, _ := g.positions.Get(id)
	tp, _ := g.positions.Get(j.Target)
	if max(mathx.Abs(pos.X-tp.X), mathx.Abs(pos.Y-tp.Y)) > 2 { // it strolled off: follow
		sx, sy, ok := g.besideSpot(id, tp.X, tp.Y)
		var path []pathing.Point
		if ok {
			path, ok = g.pathTo(id, sx, sy)
		}
		if j.DestX++; !ok || j.DestX > tradeChase {
			j.State, j.Fail = job.Fail, job.Unreachable
			g.jobs.Set(id, j)
			return
		}
		j.TX, j.TY, j.Path, j.PathI, j.State = sx, sy, path, 0, job.Goto
		if len(path) == 0 {
			j.State = job.Do
		}
		g.jobs.Set(id, j)
		return
	}
	if g.sessionBusy(id) != "" {
		j.State, j.Fail = job.Fail, job.Reserved
		g.jobs.Set(id, j)
		return
	}
	if j.WorkLeft--; j.WorkLeft > 0 {
		g.jobs.Set(id, j)
		return
	}
	g.trade.session = &tradeSession{negotiator: id, trader: j.Target}
	j.Count = 1
	g.jobs.Set(id, j)
}

// tradeJob builds the forced Trade job toward the trader at x, y, or the
// reason it can't start.
func (g *Game) tradeJob(id entity.EntityID, x, y int) (job.Record, string) {
	tid := g.traderAt(x, y)
	if tid == 0 {
		return job.Record{}, ReasonNoTrader
	}
	if g.capacity(id, body.Talking) <= 0 {
		return job.Record{}, ReasonCantTalk
	}
	c := caravanByTrader(&g.trade, tid)
	if c == nil {
		return job.Record{}, ReasonNoTrade
	}
	if f := g.factionByID(c.Faction); f == nil || f.Relation == faction.Hostile {
		return job.Record{}, ReasonNoTrade
	}
	if !g.traderOpen(tid) {
		return job.Record{}, ReasonNotTrading
	}
	if why := g.sessionBusy(id); why != "" {
		return job.Record{}, why
	}
	if r := g.busyBy(id, tid); r != "" {
		return job.Record{}, r
	}
	tx, ty, ok := g.besideSpot(id, x, y)
	if !ok {
		return job.Record{}, ReasonUnreachable
	}
	return job.Record{Kind: job.Trade, Target: tid, TX: tx, TY: ty, WorkLeft: tradeWait}, ""
}

// tradeOrders lists "Trade with <name>" for a trader at x, y (OrdersAt's
// add asks tradeJob whether it is allowed).
func (g *Game) tradeOrders(add func(string, command.Order, construction.Layer), x, y int) {
	if tid := g.traderAt(x, y); tid != 0 {
		add("Trade with "+g.name(tid), command.OrderTrade, construction.LayerObject)
	}
}
