// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package settlement

import (
	"cmp"
	"fmt"
	"slices"

	"rimworld-go/internal/application/game/setupview"
	"rimworld-go/internal/application/game/tradeview"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/trade"
	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
)

// restockTicks is one restock period: a saved NextAt further than this
// from now is clamped at Open.
const restockTicks = trade.RestockDays * clock.TicksPerDay

// Open opens caravan cid's session with the settlement on its tile and
// returns the refusal ("" on success). It refuses with Reason, a tile
// past the planet with ReasonNoSettlement and, while another caravan's
// session is live, with tradeview.ReasonTradeOpen, changing nothing;
// reopening the live session is a no-op. Otherwise it rolls the tile's
// market (trade.NewMarket) or restocks it when due (trade.Market.Restock)
// at Host.Now with Host.Seed, and the negotiator is the member with the
// highest Social (ties to the lowest id). A market never restocks while
// its session is open: only Open restocks.
func (s *State) Open(h Host, c Cargo, cid uint64, busy string) string {
	if why := s.Reason(h, cid, busy); why != "" {
		return why
	}
	v, _ := h.Caravan(cid)
	if v.Tile < 0 || v.Tile >= h.Tiles() {
		return ReasonNoSettlement
	}
	if s.Live(h) {
		if s.sess.cid == cid {
			return ""
		}
		return tradeview.ReasonTradeOpen
	}
	fi, f, _ := h.Site(v.Tile)
	now, b, seed := h.Now(), h.Items(), h.Seed()
	if i, ok := s.find(v.Tile); ok {
		m := &s.markets[i]
		if m.NextAt > now+restockTicks {
			m.NextAt = now // a hostile save's far restock: due now
		}
		m.Restock(b, f.Type, fi, seed, now)
	} else {
		s.markets = slices.Insert(s.markets, i, trade.NewMarket(b, f.Type, seed, v.Tile, fi, now))
	}
	s.sess = &session{cid: cid, negotiator: negotiator(h, v.Members), tile: v.Tile}
	return ""
}

// negotiator is the member with the highest Social, ties to the lowest
// id (0: no members).
func negotiator(h Host, members []uint64) uint64 {
	best, bestS := uint64(0), 0
	for _, id := range members {
		_, sk := h.Pawn(id)
		if best == 0 || sk > bestS || sk == bestS && id < best {
			best, bestS = id, sk
		}
	}
	return best
}

// Live reports whether a settlement session is open and still valid: the
// caravan can trade where it stands (Reason with no busy) and it still
// stands on the session's tile.
func (s *State) Live(h Host) bool {
	if s == nil || s.sess == nil {
		return false
	}
	v, ok := h.Caravan(s.sess.cid)
	return ok && v.Tile == s.sess.tile && s.Reason(h, s.sess.cid, "") == ""
}

// deal is a live session's parts: its market, the settlement's faction
// and name, the caravan's visit and the price terms.
type deal struct {
	m     *trade.Market
	f     *faction.Faction
	name  string
	visit Visit
	terms trade.Terms
}

// live is the open session's deal (ok false: none, or it is dead).
func (s *State) live(h Host) (deal, bool) {
	if !s.Live(h) {
		return deal{}, false
	}
	i, ok := s.find(s.sess.tile)
	if !ok {
		return deal{}, false
	}
	_, f, name := h.Site(s.sess.tile)
	v, _ := h.Caravan(s.sess.cid)
	_, social := h.Pawn(s.sess.negotiator)
	return deal{m: &s.markets[i], f: f, name: name, visit: v,
		terms: trade.Terms{Social: social, Goodwill: f.Goodwill, Hostile: f.Relation == faction.Hostile}}, true
}

// View is the open session's dialog model (ok false while none is open
// or it is no longer live). It changes nothing.
func (s *State) View(h Host, c Cargo) (tradeview.Trade, bool) {
	d, ok := s.live(h)
	if !ok {
		return tradeview.Trade{}, false
	}
	b, now := h.Items(), h.Now()
	mine := counts(c.Goods(s.sess.cid))
	nname, _ := h.Pawn(s.sess.negotiator)
	v := tradeview.Trade{Trader: d.name, TraderKind: lowerFirst(trade.DefOf(d.m.Stock.Kind).Label), Faction: d.f.Name, FactionID: d.f.ID,
		Negotiator: entity.EntityID(s.sess.negotiator), NegotiatorName: nname, Social: d.terms.Social, PriceModPct: tradeview.PriceModPct(d.terms),
		Goodwill: d.f.Goodwill, ColonySilver: mine[item.Silver], TraderSilver: d.m.Stock.Silver, Reason: s.sess.reason,
		Rows: tradeview.Rows(b, mine, d.m.Stock, d.terms, setupview.ItemLabel)}
	if d.visit.Until > now {
		v.HoursLeft = int((d.visit.Until - now + clock.TicksPerHour - 1) / clock.TicksPerHour)
	}
	return v, true
}

// counts sums the caravan's goods by kind (silver included).
func counts(goods []item.Stack) map[item.Kind]int {
	m := make(map[item.Kind]int, len(goods))
	for _, st := range goods {
		m[st.Kind] += st.Qty
	}
	return m
}

// addSilver is a + n silver (n >= 0), saturating at maxSilver.
func addSilver(a, n int) int {
	if n > maxSilver-a {
		return maxSilver
	}
	return a + n
}

// lowerFirst lower-cases s's first letter ("Bulk hauler" -> "bulk
// hauler").
func lowerFirst(s string) string {
	if s == "" || s[0] < 'A' || s[0] > 'Z' {
		return s
	}
	return string(s[0]+'a'-'A') + s[1:]
}

// Propose settles offer (trade.EncodeOffer; a gift when gift) against the
// open session and returns its note: the refusal, or "" when accepted.
// It settles against the cargo's live counts and the market's stock, with
// the negotiator's Social and the faction's goodwill. Sales (and the
// silver the caravan pays) come out of the cargo first; if any Yield
// fails what was taken goes back and nothing changes
// (tradeview.ReasonOfferBroken). A dead session changes nothing.
func (s *State) Propose(h Host, c Cargo, offer string, gift bool) (note string) {
	d, ok := s.live(h)
	if !ok {
		if s == nil || s.sess == nil {
			return tradeview.ReasonNoTrade
		}
		return cmp.Or(s.Reason(h, s.sess.cid, ""), ReasonNoSettlement)
	}
	defer func() { s.sess.reason = note }()
	b, cid := h.Items(), s.sess.cid
	deltas, err := trade.DecodeOffer(b, offer)
	if err != nil {
		return tradeview.ReasonOfferBroken
	}
	res := trade.Settle(b, deltas, counts(c.Goods(cid)), d.m.Stock, d.terms, gift)
	if !res.OK() {
		return res.Reject.String()
	}
	take := slices.Clone(res.Sell)
	if !gift && res.Silver < 0 {
		take = append(take, trade.Line{Kind: item.Silver, Qty: -res.Silver})
	}
	for i, l := range take {
		if !c.Yield(cid, l.Kind, l.Qty) {
			c.CarryLines(cid, take[:i])
			return tradeview.ReasonOfferBroken
		}
	}
	st := &d.m.Stock
	for _, l := range take {
		if l.Kind == item.Silver {
			st.Silver = addSilver(st.Silver, l.Qty) // saturates: no overflow from a huge stock
		} else {
			st.Add(l.Kind, l.Qty)
		}
	}
	if gift {
		faction.Adjust(d.f, res.Goodwill, fmt.Sprintf("Day %d: gifts from the colony (%+d goodwill)", h.Day(), res.Goodwill))
		return ""
	}
	var deliver []trade.Line
	if res.Silver > 0 {
		st.Silver -= res.Silver
		deliver = append(deliver, trade.Line{Kind: item.Silver, Qty: res.Silver})
	}
	for _, l := range res.Buy {
		st.AddGoods(l.Goods(), -l.Qty)
		deliver = append(deliver, l)
	}
	c.CarryLines(cid, deliver)
	return ""
}

// Close ends the open session, if any.
func (s *State) Close() {
	if s != nil {
		s.sess = nil
	}
}
