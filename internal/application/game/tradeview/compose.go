// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package tradeview

import (
	"cmp"
	"slices"
	"strconv"

	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/trade"
	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/sim/clock"
)

// Market is the current trade offer's authoritative book, stock and terms.
type Market struct {
	Kind        trade.TraderKind
	Stock       trade.Stock
	Terms       trade.Terms
	Colony      map[item.Kind]int
	GiftStacks  []trade.GiftStack
	Items       item.Book
	Label       func(item.Kind) string
	TraderName  string
	FactionName string
	FactionID   int
	FactionType faction.Type
	HoursLeft   int
}

// Compose builds the trade dialog from the same prices used by settlement.
func Compose(v Trade, m Market) Trade {
	v.Trader, v.TraderKind = m.TraderName, LowerLabel(trade.DefOf(m.Kind).Label)
	v.Faction, v.FactionID, v.FactionType, v.Goodwill = m.FactionName, m.FactionID, m.FactionType, m.Terms.Goodwill
	v.HoursLeft, v.Social = m.HoursLeft, m.Terms.Social
	v.PriceModPct = trade.TradePriceImprovementCenti(m.Terms) / 100
	v.ColonySilver, v.TraderSilver = m.Colony[item.Silver], m.Stock.Silver

	colonyBulk := m.Colony
	if m.GiftStacks != nil {
		colonyBulk = make(map[item.Kind]int)
		for _, physical := range m.GiftStacks {
			if !trade.Detailed(physical.Stack) {
				colonyBulk[physical.Stack.Kind] += physical.Stack.Qty
			}
		}
	}
	theirs := make(map[trade.Goods]int, len(m.Stock.Lines))
	keys := make([]trade.Goods, 0, len(colonyBulk)+len(m.Stock.Lines))
	for _, line := range m.Stock.Lines {
		if int(line.Kind) >= m.Items.Len() {
			continue
		}
		theirs[line.Goods()] += line.OrdinaryQty()
		keys = append(keys, line.Goods())
	}
	for k := range colonyBulk {
		if k != item.Silver && m.Items.Value(k) > 0 && theirs[trade.Goods{Kind: k}] == 0 {
			keys = append(keys, trade.Goods{Kind: k})
		}
	}
	slices.SortFunc(keys, trade.Goods.Compare)
	keys = slices.Compact(keys)
	for _, good := range keys {
		k, qty := good.Kind, colonyBulk[good.Kind]
		if good.Building != 0 {
			qty = 0
		}
		if qty == 0 && theirs[good] == 0 {
			continue
		}
		v.Rows = append(v.Rows, Row{
			Kind: good.Name(m.Items), Label: itemLabel(m.Items, k, m.Label, good),
			Category: m.Items.Category(k), ColonyQty: qty, TraderQty: theirs[good],
			BuyC:    trade.PriceOf(m.Items, good, m.Kind, trade.Buy, m.Terms),
			SellC:   trade.PriceOf(m.Items, good, m.Kind, trade.Sell, m.Terms),
			WillBuy: theirs[good] > 0, WillSell: good.Building == 0 && trade.Wants(m.Items, m.Kind, k),
		})
	}
	for _, physical := range m.GiftStacks {
		st := physical.Stack
		if !trade.Detailed(st) || st.Kind == item.Silver {
			continue
		}
		g := trade.Goods{Kind: st.Kind, Building: st.Building}
		v.Rows = append(v.Rows, Row{
			Kind:  g.Name(m.Items) + "@c" + strconv.FormatUint(physical.ID, 10),
			Label: giftLabel(m.Items, st, m.Label), Category: m.Items.Category(st.Kind),
			ColonyQty: st.Qty, StackID: physical.ID,
			SellC:    trade.PriceMarket(g, stackMarket(m.Items, st), m.Kind, trade.Sell, m.Terms),
			WillSell: g.Building == 0 && trade.Wants(m.Items, m.Kind, st.Kind),
		})
	}
	for _, line := range m.Stock.Lines {
		for _, lot := range line.Lots {
			market := stackMarket(m.Items, lot.Stack)
			if line.Kind == item.MinifiedBuilding {
				market = line.Goods().Value(m.Items)
			}
			v.Rows = append(v.Rows, Row{
				Kind:     line.Goods().Name(m.Items) + "@t" + strconv.FormatUint(lot.ID, 10),
				Label:    giftLabel(m.Items, lot.Stack, m.Label),
				Category: m.Items.Category(line.Kind), TraderQty: lot.Stack.Qty,
				StackID: lot.ID, BuyC: trade.PriceMarket(line.Goods(), market, m.Kind, trade.Buy, m.Terms),
				WillBuy: true,
			})
		}
	}
	if m.FactionID >= 0 {
		method := trade.GiftVisitor
		if m.Terms.ImproveCenti == 200 {
			method = trade.GiftSettlement
		}
		v.GiftRows = giftRows(m.Items, m.GiftStacks, m.Terms.Social, m.Label, method)
	}
	slices.SortStableFunc(v.Rows, func(a, b Row) int {
		if rankA, rankB := item.CategoryRank(a.Category), item.CategoryRank(b.Category); rankA != rankB {
			return rankA - rankB
		}
		return cmp.Compare(a.Label, b.Label)
	})
	return v
}

func LowerLabel(s string) string {
	if s == "" || s[0] < 'A' || s[0] > 'Z' {
		return s
	}
	return string(s[0]+'a'-'A') + s[1:]
}

// HoursLeft rounds a remaining trade or visitor stay up to whole hours.
func HoursLeft(end, now uint64) int {
	if end <= now {
		return 0
	}
	return int((end - now + clock.TicksPerHour - 1) / clock.TicksPerHour)
}

// PriceModPct is the trade-price improvement presented in the dialog.
func PriceModPct(t trade.Terms) int { return trade.TradePriceImprovementCenti(t) / 100 }
