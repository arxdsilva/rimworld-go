// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"fmt"

	"rimworld-go/internal/application/game/settlement"
	"rimworld-go/internal/application/game/tradeview"
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
	ReasonNoTrade     = tradeview.ReasonNoTrade
	ReasonNotTrading  = "not trading now"
	ReasonTradeOpen   = tradeview.ReasonTradeOpen
	ReasonRadioInUse  = "the radio is in use"
	ReasonOfferBroken = tradeview.ReasonOfferBroken
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
	c := g.trade.caravan
	if c == nil || c.Trader != s.trader {
		return tradeParty{}, false
	}
	return tradeParty{stock: &c.Stock, name: g.name(c.Trader), kind: c.Kind, f: g.factionByID(c.Faction), until: c.LeaveAt}, true
}

// termsFor are the price terms negotiator id gets from faction f (nil:
// the orbital ship, goodwill 0).
func (g *Game) termsFor(id entity.EntityID, f *faction.Faction) trade.Terms {
	t := trade.Terms{Social: int(g.skillOf(id, skill.Social).Level)}
	if f != nil {
		t.Goodwill, t.Hostile = f.Goodwill, f.Relation == faction.Hostile
	}
	return t
}

// TradeView is the open trade session (ok false while none is open): the
// colony's, else a caravan's at a settlement (game/settlement).
func (g *Game) TradeView() (TradeView, bool) {
	s := g.trade.session
	if s == nil {
		if w := g.worldSt; w != nil && w.settle.Live(settleHost{g}) {
			return w.settle.View(settleHost{g}, travelOf(g))
		}
		return TradeView{}, false
	}
	v := TradeView{Negotiator: s.negotiator, NegotiatorName: g.name(s.negotiator), Reason: s.reason, Orbital: s.trader == 0}
	p, ok := g.party(s)
	if !ok {
		return v, true
	}
	terms := g.termsFor(s.negotiator, p.f)
	v.Trader, v.TraderKind = p.name, lower(trade.DefOf(p.kind).Label)
	v.FactionID = -1
	if p.f != nil {
		v.Faction, v.FactionID, v.Goodwill = p.f.Name, p.f.ID, p.f.Goodwill
	}
	v.HoursLeft = hoursLeft(p.until, uint64(g.clock.Now()))
	v.Social, v.PriceModPct = terms.Social, tradeview.PriceModPct(terms)
	gd := g.colonyGoods(v.Orbital)
	v.ColonySilver, v.TraderSilver = gd.counts[item.Silver], p.stock.Silver
	st := *p.stock
	st.Kind = p.kind // priced as the trader's kind
	v.Rows = tradeview.Rows(g.sk.content.Items, gd.counts, st, terms, itemLabel)
	return v, true
}

// cmdProposeTrade settles the offer encoded in offer (trade.EncodeOffer),
// as a gift when gift is set, against the live counts; a refusal is kept
// as the session's reason.
func (g *Game) cmdProposeTrade(offer string, gift bool) {
	s := g.trade.session
	if s == nil {
		if w := g.worldSt; w != nil && w.settle.Live(settleHost{g}) {
			w.settle.Propose(settleHost{g}, travelOf(g), offer, gift) // game/settlement (#11)
		}
		return
	}
	p, ok := g.party(s)
	if !ok {
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
	res := trade.Settle(g.sk.content.Items, deltas, gd.counts, *p.stock, g.termsFor(s.negotiator, p.f), gift)
	if !res.OK() {
		s.reason = res.Reject.String()
		return
	}
	s.reason = ""
	for _, l := range res.Sell {
		g.takeGoods(gd, l.Kind, l.Qty)
		if l.Kind == item.Silver {
			p.stock.Silver += l.Qty
		} else {
			p.stock.Add(l.Kind, l.Qty)
		}
	}
	now := uint64(g.clock.Now())
	pos, _ := g.positions.Get(s.negotiator)
	if gift {
		faction.Adjust(p.f, res.Goodwill, fmt.Sprintf("Day %d: gifts from the colony (%+d goodwill)", g.relDay(now)+1, res.Goodwill))
		g.message(pos.X, pos.Y, "%s accepted the gift (%+d goodwill).", p.f.Name, res.Goodwill)
		return
	}
	var deliver []trade.Line
	switch {
	case res.Silver < 0:
		g.takeGoods(gd, item.Silver, -res.Silver)
		p.stock.Silver -= res.Silver
	case res.Silver > 0:
		p.stock.Silver -= res.Silver
		deliver = append(deliver, trade.Line{Kind: item.Silver, Qty: res.Silver})
	}
	for _, l := range res.Buy {
		p.stock.AddGoods(l.Goods(), -l.Qty)
		deliver = append(deliver, l)
	}
	if orbital {
		g.sendTradePod(deliver, now) // trade_orbital.go
	} else {
		for _, l := range deliver {
			g.dropGoods(l, pos.X, pos.Y, false)
		}
	}
	g.message(pos.X, pos.Y, "%s traded with %s.", g.name(s.negotiator), p.name)
}

// cmdCloseTrade ends the trade session (the colony's or a settlement's).
func (g *Game) cmdCloseTrade() { g.trade.session = nil; settlementOf(g).Close() }

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
	if s := g.trade.session; s != nil && s.negotiator != id || g.worldSt != nil && g.worldSt.settle.Live(settleHost{g}) { // or a caravan's at a settlement (#11)
		return ReasonTradeOpen
	}
	if c := g.trade.comms; c != nil && c.operator != id {
		return ReasonRadioInUse
	}
	return ""
}

// settlementOf is the settlement markets and session (game/settlement, #11).
func settlementOf(g *Game) *settlement.State {
	if w := g.worldS(); w.settle == nil {
		w.settle = &settlement.State{}
	}
	return g.worldSt.settle
}

// settleHost is game/settlement's Host; Seed and Tiles build the planet (Open only).
type settleHost struct{ g *Game }

func (h settleHost) Now() uint64      { return uint64(h.g.clock.Now()) }
func (h settleHost) Seed() uint64     { return settlement.SeedOf(h.g.worldPlanet()) }
func (h settleHost) Tiles() int       { return settlement.TilesOf(h.g.worldPlanet()) }
func (h settleHost) Items() item.Book { return h.g.sk.content.Items }
func (h settleHost) Day() int         { return h.g.relDay(h.Now()) + 1 }
func (h settleHost) Site(t int) (int, *faction.Faction, string) {
	return settlement.SiteIn(h.g.ng.factions, t)
}
func (h settleHost) Pawn(id uint64) (string, int) {
	return h.g.personName(entity.EntityID(id)), int(h.g.skillOf(entity.EntityID(id), skill.Social).Level)
}
func (h settleHost) Caravan(cid uint64) (settlement.Visit, bool) {
	if c := fleetOf(h.g).At(entity.EntityID(cid)); c != nil {
		return settlement.VisitOf(&c.C, c.Back), true
	}
	return settlement.Visit{}, false
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
	if f := g.factionByID(g.trade.caravan.Faction); f == nil || f.Relation == faction.Hostile {
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
