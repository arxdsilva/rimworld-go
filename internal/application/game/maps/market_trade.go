// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package maps

import (
	"fmt"
	"slices"

	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/trade"
	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/pawn/body"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/sim/statehash"
	"rimworld-go/internal/domain/world/caravan"
)

// PhysicalStock projects the saved aggregate and preserved goods into the
// existing physical quote/settlement contract without a second inventory.
func PhysicalStock(m SettlementStock) (trade.Stock, bool) {
	stock := m.Stock.Clone()
	lots, ok := trade.PhysicalStacks(m.Preserved, true)
	if !ok {
		return trade.Stock{}, false
	}
	totals := make(map[trade.Goods]int)
	for _, p := range lots {
		st := p.Stack
		key := trade.Goods{Kind: st.Kind, Building: st.Building}
		i, found := slices.BinarySearchFunc(stock.Lines, key, func(l trade.Line, g trade.Goods) int { return l.Goods().Compare(g) })
		if !found {
			return trade.Stock{}, false
		}
		l := &stock.Lines[i]
		if totals[key] > l.Qty-st.Qty {
			return trade.Stock{}, false
		}
		totals[key] += st.Qty
		if trade.Detailed(st) {
			l.Lots = append(l.Lots, trade.Lot{ID: p.ID, Stack: st})
		}
	}
	return stock, true
}

// Quote identifies the exact live offer inputs, not a client-supplied price.
func Quote(c *Caravan, negotiator uint64, m SettlementStock, stock trade.Stock, goods []trade.GiftStack, terms trade.Terms, book item.Book) uint64 {
	return max(uint64(1), statehash.Of(struct {
		Caravan, Negotiator, Until, Epoch, Restock uint64
		Tile, Faction                              int
		Stock                                      trade.Stock
		Goods                                      []trade.GiftStack
		Terms                                      trade.Terms
		Book                                       []quoteKind
	}{c.C.ID, negotiator, c.C.VisitUntil, m.Epoch, m.RestockAt, m.Tile, m.FactionID, stock, goods, terms, quoteKinds(book, stock, goods)}))
}

// quoteKind binds the relevant immutable content definitions, not a
// process pointer or the entire plugin catalogue. The pricing owner uses
// Value (and Category for admission); Key prevents ID reuse from
// validating a saved quote for a different good at the same price.
type quoteKind struct {
	Kind          item.Kind
	Key, Category string
	Value         int
}

func quoteKinds(book item.Book, stock trade.Stock, goods []trade.GiftStack) []quoteKind {
	var seen [256]bool
	var out []quoteKind
	add := func(k item.Kind) {
		if !seen[k] {
			seen[k] = true
			out = append(out, quoteKind{k, book.Key(k), book.Category(k), book.Value(k)})
		}
	}
	for _, line := range stock.Lines {
		add(line.Kind)
	}
	for _, g := range goods {
		add(g.Stack.Kind)
	}
	slices.SortFunc(out, func(a, b quoteKind) int { return int(a.Kind) - int(b.Kind) })
	return out
}

// takeStock debits the exact priced lots. Ordinary stock may be generated
// or previously sold; its identical payload is interchangeable.
func takeStock(m *SettlementStock, lines []trade.Line) ([]item.Stack, bool) {
	var bought []item.Stack
	take := func(key item.Stack, qty int) int {
		for i := 0; i < len(m.Preserved) && qty > 0; {
			st := &m.Preserved[i]
			if trade.CarryStack(*st, 1) != key {
				i++
				continue
			}
			n := min(qty, st.Qty)
			bought = append(bought, trade.CarryStack(*st, n))
			st.Qty -= n
			qty -= n
			if st.Qty == 0 {
				m.Preserved = slices.Delete(m.Preserved, i, i+1)
			} else {
				i++
			}
		}
		return qty
	}
	for _, l := range lines {
		for _, lot := range l.Lots {
			if take(trade.CarryStack(lot.Stack, 1), lot.Stack.Qty) != 0 {
				return nil, false
			}
		}
		if remain := take(item.Stack{Kind: l.Kind, Building: l.Building, Qty: 1}, l.OrdinaryQty()); remain > 0 {
			bought = append(bought, item.Stack{Kind: l.Kind, Qty: remain, Building: l.Building, Stuff: trade.CrateStuff(l.Building)})
		}
		m.Stock.AddGoods(l.Goods(), -l.Qty)
	}
	return bought, true
}

// preserveSale joins only stacks with identical physical attributes.
func preserveSale(m *SettlementStock, st item.Stack) {
	st = trade.CarryStack(st, st.Qty)
	for i := range m.Preserved {
		candidate := m.Preserved[i]
		candidate.Qty = st.Qty
		if candidate == st {
			m.Preserved[i].Qty += st.Qty
			return
		}
	}
	m.Preserved = append(m.Preserved, st)
}

// Trade validates a caravan at its actual visited tile and performs an
// all-or-nothing exchange against the settlement's shared inventory.
// Target tile and prices are always server-derived.
func (s Market) Trade(cid uint64, negotiator entity.EntityID, encoded string, gift bool, expected ...uint64) (trade.Result, string) {
	refuse := func(reason string, reject trade.Reject) (trade.Result, string) {
		s.fleet.h.Note("%s", reason)
		return trade.Result{Reject: reject}, reason
	}
	c := s.fleet.At(entity.EntityID(cid))
	if c == nil || c.Back || c.C.State != caravan.Visiting || s.fleet.h.Now() >= c.C.VisitUntil || c.Ask.Letter != 0 {
		return refuse("The caravan is not visiting a settlement.", trade.RejectUnwanted)
	}
	fa := s.fleet.h.Faction(c.C.Tile)
	if fa == nil || fa.Defeated || !fa.Type.Human() || fa.Relation == faction.Hostile || fa.Besieged || !slices.Contains(fa.Settlements, c.C.Tile) {
		return refuse("The settlement cannot trade.", trade.RejectHostile)
	}
	if !slices.Contains(c.C.Members, uint64(negotiator)) || !s.fleet.h.Alive(negotiator) || !s.fleet.h.Standing(negotiator) ||
		s.fleet.h.Breaking(negotiator) || s.fleet.h.Capacity(negotiator, body.Talking) <= 0 {
		return refuse("No caravan member can negotiate this trade.", trade.RejectUnwanted)
	}
	if s.fleet.market == nil {
		return refuse("The settlement market is unavailable.", trade.RejectUnwanted)
	}
	i, ok := s.fleet.market.index(c.C.Tile)
	if !ok || s.fleet.market.entries[i].FactionID != fa.ID {
		return refuse("The settlement has no stock.", trade.RejectUnwanted)
	}
	if s.fleet.h.Now() >= s.fleet.market.entries[i].RestockAt {
		s.Visit(c.C.Tile, fa, s.fleet.h.Now())
	}
	book := s.fleet.h.MarketItems()
	deltas, err := trade.DecodeOffer(book, encoded)
	if err != nil {
		return refuse("The trade offer is invalid.", trade.RejectUnwanted)
	}
	if !gift {
		for _, d := range deltas {
			if d.Kind == item.Silver {
				return refuse("Silver is settled by the trader.", trade.RejectUnwanted)
			}
		}
	}
	m := &s.fleet.market.entries[i]
	cargo := s.fleet.h.MarketCargo()
	terms := s.fleet.h.MarketTerms(negotiator, fa)
	terms.ImproveCenti = settlementPriceCenti
	stock, ok := PhysicalStock(*m)
	goods, valid := trade.PhysicalStacks(s.fleet.h.Cargo().Goods(cid), false)
	if !ok || !valid {
		return refuse("The settlement cargo is inconsistent.", trade.RejectUnwanted)
	}
	if len(expected) > 0 && expected[0] != 0 && expected[0] != Quote(c, uint64(negotiator), *m, stock, goods, terms, book) {
		return refuse("The trade quote changed. Review the current offer.", trade.RejectUnwanted)
	}
	if gift {
		terms.Inspired = false
	}
	selection := trade.ExactSelection
	if len(expected) == 0 || expected[0] == 0 {
		selection = trade.PreservedFirst
	}
	res, _, valid := trade.SettlePhysical(book, deltas, cargo.TradeCounts(cid), stock, terms, gift, goods, selection)
	if !valid {
		return refuse("The trade offer is invalid.", trade.RejectUnwanted)
	}
	if !res.OK() {
		return refuse(res.Reject.String(), res.Reject)
	}
	next := m.Clone()
	incoming, ok := takeStock(&next, res.Buy)
	if !ok {
		return refuse("The settlement cargo is inconsistent.", trade.RejectUnwanted)
	}
	members := CarrierCount(c.C.Members, s.fleet.h.Alive)
	plan, why := cargo.PlanTrade(cid, members, res, incoming, s.fleet.h.MarketCarriers(c))
	if why != "" {
		return refuse(why, trade.RejectOverStock)
	}
	next.Stock.Silver -= res.Silver
	for _, st := range plan.Sold() {
		if st.Kind == item.Silver {
			next.Stock.Silver += st.Qty
			continue
		}
		next.Stock.AddGoods(trade.Goods{Kind: st.Kind, Building: st.Building}, st.Qty)
		preserveSale(&next, st)
	}
	cargo.CommitTrade(cid, plan)
	s.fleet.market.entries[i] = next
	if !gift {
		s.fleet.h.UseTradeInspiration(negotiator)
	}
	s.fleet.pace(c) // purchased cargo changes the return speed
	if gift && res.Goodwill != 0 {
		faction.Adjust(fa, res.Goodwill, fmt.Sprintf("Day %d: caravan gifts (%+d goodwill)", s.fleet.h.Day(), res.Goodwill))
	}
	s.fleet.h.Note("The caravan traded with %s.", s.fleet.h.TileName(c.C.Tile))
	return res, ""
}
