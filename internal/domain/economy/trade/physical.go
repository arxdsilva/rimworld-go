// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package trade

import (
	"slices"

	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/faction"
)

// PlanTraderBuy selects named trader lots before unqualified inventory.
// A single kind-only buy consumes generic stock first, then detailed lots
// by their saved ID; every planned line has the exact properties to drop.
func PlanTraderBuy(offer []Delta, stock Stock) ([]Line, bool) {
	return planTraderBuy(offer, stock, LegacySelection)
}

func planTraderBuy(offer []Delta, stock Stock, selection LotSelection) ([]Line, bool) {
	used := make(map[uint64]int)
	generic := make(map[Goods]int)
	var result []Line
	for _, d := range offer {
		if d.Qty <= 0 || d.StackID == 0 {
			continue
		}
		g := Goods{d.Kind, d.Building}
		line, ok := stockLine(stock.Lines, g)
		if !ok {
			return nil, false
		}
		j := slices.IndexFunc(line.Lots, func(l Lot) bool { return l.ID == d.StackID })
		if j < 0 || d.Qty > line.Lots[j].Stack.Qty-used[d.StackID] {
			return nil, false
		}
		used[d.StackID] += d.Qty
		taken := line.Lots[j]
		taken.Stack.Qty = d.Qty
		result = AddLine(result, Line{Kind: g.Kind, Building: g.Building, Qty: d.Qty, Lots: []Lot{taken}})
	}
	for _, d := range offer {
		if d.Qty <= 0 || d.StackID != 0 {
			continue
		}
		g := Goods{d.Kind, d.Building}
		line, ok := stockLine(stock.Lines, g)
		if !ok {
			return nil, false
		}
		left := d.Qty
		takeOrdinary := func() {
			n := min(left, line.OrdinaryQty()-generic[g])
			if n > 0 {
				generic[g] += n
				result = AddLine(result, Line{Kind: g.Kind, Building: g.Building, Qty: n})
				left -= n
			}
		}
		if selection != PreservedFirst {
			takeOrdinary()
		}
		for _, lot := range line.Lots {
			if left == 0 || selection == ExactSelection {
				break
			}
			n := min(left, lot.Stack.Qty-used[lot.ID])
			if n <= 0 {
				continue
			}
			used[lot.ID] += n
			taken := lot
			taken.Stack.Qty = n
			result = AddLine(result, Line{Kind: g.Kind, Building: g.Building, Qty: n, Lots: []Lot{taken}})
			left -= n
		}
		if selection == PreservedFirst {
			takeOrdinary()
		}
		if left > 0 {
			return nil, false
		}
	}
	return result, true
}

func stockLine(lines []Line, g Goods) (Line, bool) {
	i, ok := slices.BinarySearchFunc(lines, g, func(l Line, g Goods) int {
		return l.Goods().Compare(g)
	})
	if !ok {
		return Line{}, false
	}
	return lines[i], true
}

// stackMarketValue uses the active book for plugin items, and the real
// physical value for built-in items.
func stackMarketValue(b item.Book, st item.Stack) int {
	if b.Plugin(st.Kind) {
		return b.Value(st.Kind)
	}
	return item.StackValue(st)
}

// priceOfLine prices ordinary stock and actual physical lots against the
// same trade terms. Every multiply is checked before committing an offer.
func priceOfLine(b item.Book, l Line, kind TraderKind, side Side, terms Terms) (int, bool) {
	total, ok := addCenti(0, l.OrdinaryQty(), PriceOf(b, l.Goods(), kind, side, terms))
	if !ok {
		return 0, false
	}
	for _, lot := range l.Lots {
		market := stackMarketValue(b, lot.Stack)
		if l.Kind == item.MinifiedBuilding {
			market = l.Goods().Value(b)
		}
		total, ok = addCenti(total, lot.Stack.Qty, PriceMarket(l.Goods(), market, kind, side, terms))
		if !ok {
			return 0, false
		}
	}
	return total, true
}

// LotSelection distinguishes old kind-only offers from UI rows whose
// ordinary and individually valued stock are separate.
type LotSelection uint8

const (
	LegacySelection LotSelection = iota
	PreservedFirst
	ExactSelection
)

// SettlePhysical validates and prices a barter or gift against the live
// eligible colony stacks and the trader's persistent lot inventory. It
// returns an exact removal plan; all mutations are performed by the
// application only after visit carrier settlement also succeeds.
func SettlePhysical(b item.Book, offer []Delta, colony map[item.Kind]int, trader Stock,
	terms Terms, gift bool, eligible []GiftStack, selection ...LotSelection) (Result, []GiftAllocation, bool) {
	mode := LegacySelection
	if len(selection) != 0 {
		mode = selection[0]
	}
	if terms.Hostile {
		return Result{Reject: RejectHostile}, nil, true
	}
	if len(offer) == 0 {
		return Result{Reject: RejectEmpty}, nil, true
	}
	var selling, buying []Delta
	offered := make(map[item.Kind]int)
	wanted := make(map[Goods]int)
	for _, d := range offer {
		g := Goods{d.Kind, d.Building}
		if d.Qty == 0 || d.Qty < -MaxOfferQty || d.Qty > MaxOfferQty ||
			int(d.Kind) >= b.Len() {
			return Result{Reject: RejectBadQty}, nil, true
		}
		if g.Building != 0 && d.Kind != item.MinifiedBuilding {
			return Result{}, nil, false
		}
		switch {
		case d.Qty < 0:
			if gift {
				if g.Value(b) <= 0 {
					return Result{Reject: RejectUnwanted}, nil, true
				}
			} else if !Wants(b, trader.Kind, d.Kind) || d.Building != 0 {
				return Result{Reject: RejectUnwanted}, nil, true
			}
			offered[d.Kind] -= d.Qty
			if offered[d.Kind] > MaxOfferQty {
				return Result{Reject: RejectBadQty}, nil, true
			}
			selling = append(selling, d)
		case gift || d.Kind == item.Silver:
			return Result{Reject: RejectUnwanted}, nil, true
		default:
			wanted[g] += d.Qty
			if wanted[g] > MaxOfferQty {
				return Result{Reject: RejectBadQty}, nil, true
			}
			buying = append(buying, d)
		}
	}
	// Explicit IDs are a stricter contract than aggregate stock counts:
	// report stale or wrong-kind item selection before quantity shortages.
	hasNamed := slices.ContainsFunc(selling, func(d Delta) bool { return d.StackID != 0 })
	var plan []GiftAllocation
	if hasNamed {
		var ok bool
		plan, ok = planSales(selling, eligible, gift, mode == ExactSelection)
		if !ok {
			return Result{}, nil, false
		}
	}
	for kind, qty := range offered {
		if qty > colony[kind] {
			return Result{Reject: RejectOverStock}, nil, true
		}
	}
	for _, d := range buying {
		if d.StackID == 0 {
			continue
		}
		line, ok := stockLine(trader.Lines, Goods{d.Kind, d.Building})
		if !ok {
			return Result{}, nil, false
		}
		j := slices.IndexFunc(line.Lots, func(l Lot) bool { return l.ID == d.StackID })
		if j < 0 || d.Qty > line.Lots[j].Stack.Qty {
			return Result{}, nil, false
		}
	}
	for g, qty := range wanted {
		line, ok := stockLine(trader.Lines, g)
		if !ok || qty > line.Qty {
			return Result{Reject: RejectOverStock}, nil, true
		}
	}
	if !hasNamed {
		var ok bool
		plan, ok = planSales(selling, eligible, gift, mode == ExactSelection)
		if !ok {
			return Result{}, nil, false
		}
	}
	// Unquoted legacy settlement commands historically bought preserved
	// goods before generated stock; a quoted row instead buys ordinary stock first.
	buys, ok := planTraderBuy(buying, trader, mode)
	if !ok {
		return Result{}, nil, false
	}
	res := Result{Buy: buys}
	next := trader.NextLotID
	for _, a := range plan {
		s := CarryStack(a.Stack, a.Qty)
		line := Line{Kind: s.Kind, Building: s.Building, Qty: a.Qty}
		if Detailed(s) {
			next++
			line.Lots = []Lot{{ID: next, Stack: s}}
		}
		res.Sell = AddLine(res.Sell, line)
	}
	if gift {
		value, ok := GiftValueAllocations(b, plan, terms)
		if !ok {
			return Result{Reject: RejectOverStock}, nil, true
		}
		res.Goodwill = faction.GiftGoodwill(faction.Faction{
			Type: terms.FactionType, Goodwill: terms.Goodwill,
		}, value)
		return res, plan, true
	}
	var buyC, sellC int
	for _, l := range res.Buy {
		value, ok := priceOfLine(b, l, trader.Kind, Buy, terms)
		if !ok {
			return Result{Reject: RejectOverStock}, nil, true
		}
		buyC, ok = addCenti(buyC, 1, value)
		if !ok {
			return Result{Reject: RejectOverStock}, nil, true
		}
	}
	for _, l := range res.Sell {
		value, ok := priceOfLine(b, l, trader.Kind, Sell, terms)
		if !ok {
			return Result{Reject: RejectOverStock}, nil, true
		}
		sellC, ok = addCenti(sellC, 1, value)
		if !ok {
			return Result{Reject: RejectOverStock}, nil, true
		}
	}
	res.SilverCenti = sellC - buyC
	res.Silver = RoundedSilver(res.SilverCenti)
	if res.Silver < 0 && colony[item.Silver] < -res.Silver ||
		res.Silver > 0 && trader.Silver < res.Silver {
		return Result{Reject: RejectUnaffordable}, nil, true
	}
	return res, plan, true
}
