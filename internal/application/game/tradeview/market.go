// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package tradeview

import (
	"cmp"
	"slices"
	"strings"

	"rimworld-go/internal/application/game/maps"
	"rimworld-go/internal/application/game/tabview"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/trade"
	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/caravan"
)

// StockRow is one commodity in the union of a trader's sorted stock and
// the caravan's tradeable cargo. It remains a read-only projection.
type StockRow struct {
	Goods              trade.Goods
	CargoQty, StockQty int
	BuyC, SellC        int
	WillBuy, WillSell  bool
}

func StockRows(stock trade.Stock, cargo map[item.Kind]int, terms trade.Terms) []StockRow {
	return stockRows(nil, stock, cargo, terms)
}

func stockRows(rows []StockRow, stock trade.Stock, cargo map[item.Kind]int, terms trade.Terms) []StockRow {
	var keyBuf [32]trade.Goods
	keys := keyBuf[:0]
	for _, line := range stock.Lines {
		keys = append(keys, line.Goods())
	}
	for k := range cargo {
		if k != item.Silver && k.Value() > 0 {
			keys = append(keys, trade.Goods{Kind: k})
		}
	}
	slices.SortFunc(keys, trade.Goods.Compare)
	keys = slices.Compact(keys)
	stockIdx := 0
	for _, k := range keys {
		for stockIdx < len(stock.Lines) && stock.Lines[stockIdx].Goods().Compare(k) < 0 {
			stockIdx++
		}
		qty := 0
		if stockIdx < len(stock.Lines) && stock.Lines[stockIdx].Goods() == k {
			qty = stock.Lines[stockIdx].Qty
		}
		carry := cargo[k.Kind]
		if k.Building != 0 {
			carry = 0
		}
		rows = append(rows, StockRow{
			Goods: k, CargoQty: carry, StockQty: qty,
			BuyC:     trade.PriceOf(item.Book{}, k, stock.Kind, trade.Buy, terms),
			SellC:    trade.PriceOf(item.Book{}, k, stock.Kind, trade.Sell, terms),
			WillBuy:  qty > 0,
			WillSell: k.Building == 0 && trade.Wants(item.Book{}, stock.Kind, k.Kind),
		})
	}
	return rows
}

// LocalRows is the same price and inventory projection used by the
// existing caravan and orbital trade dialogs.
func LocalRows(stock trade.Stock, cargo map[item.Kind]int, terms trade.Terms, label func(item.Kind) string) []Row {
	parts := StockRows(stock, cargo, terms)
	rows := make([]Row, 0, len(parts))
	for _, r := range parts {
		k := r.Goods
		rows = append(rows, Row{
			Kind: k.Name(item.Book{}), Label: cmp.Or(k.Label(item.Book{}), label(k.Kind)),
			Category: k.Kind.Category(), ColonyQty: r.CargoQty, TraderQty: r.StockQty,
			BuyC: r.BuyC, SellC: r.SellC, WillBuy: r.WillBuy, WillSell: r.WillSell,
		})
	}
	slices.SortStableFunc(rows, func(a, b Row) int {
		if ra, rb := item.CategoryRank(a.Category), item.CategoryRank(b.Category); ra != rb {
			return ra - rb
		}
		return cmp.Compare(a.Label, b.Label)
	})
	return rows
}

// VisitingSource supplies the World tab's existing authoritative lookup
// and negotiation rules without giving presentation code simulation state.
type VisitingSource interface {
	MarketAt(int) (maps.SettlementStock, bool)
	FactionAt(int) *faction.Faction
	Eligible(entity.EntityID) bool
	Terms(entity.EntityID, *faction.Faction) trade.Terms
	Cargo(uint64) []item.Stack
	Items() item.Book
}

// Visiting resolves a live caravan, settlement and negotiator into a
// market snapshot; false means the inspector must not offer trading.
func Visiting(now uint64, c *maps.Caravan, src VisitingSource) (tabview.MarketView, bool) {
	if c == nil || c.Back || c.C.State != caravan.Visiting || now >= c.C.VisitUntil || c.Ask.Letter != 0 {
		return tabview.MarketView{}, false
	}
	m, ok := src.MarketAt(c.C.Tile)
	if !ok {
		return tabview.MarketView{}, false
	}
	fa := src.FactionAt(m.Tile)
	if fa == nil || fa.ID != m.FactionID || fa.Defeated || fa.Relation == faction.Hostile || fa.Besieged || !fa.Type.Human() || !slices.Contains(fa.Settlements, c.C.Tile) || now >= m.RestockAt {
		return tabview.MarketView{}, false
	}
	var negotiator entity.EntityID
	for _, raw := range c.C.Members {
		id := entity.EntityID(raw)
		if src.Eligible(id) {
			negotiator = id
			break
		}
	}
	terms := src.Terms(negotiator, fa)
	terms.ImproveCenti = 200
	goods, valid := trade.PhysicalStacks(src.Cargo(c.C.ID), false)
	stock, ok := maps.PhysicalStock(m)
	if !valid || !ok {
		return tabview.MarketView{}, false
	}
	counts := make(map[item.Kind]int)
	for _, p := range goods {
		counts[p.Stack.Kind] += p.Stack.Qty
	}
	book := src.Items()
	rows := Compose(Trade{}, Market{Kind: stock.Kind, Stock: stock, Colony: counts, GiftStacks: goods, Items: book, Terms: terms, FactionID: m.FactionID})
	v := tabview.MarketView{CaravanID: c.C.ID, Negotiator: uint64(negotiator), Tile: m.Tile, FactionID: m.FactionID, Epoch: m.Epoch, RestockAt: m.RestockAt, CaravanSilver: counts[item.Silver], TraderSilver: stock.Silver, Quote: maps.Quote(c, uint64(negotiator), m, stock, goods, terms, book)}
	convert := func(rows []Row) []tabview.MarketRow {
		out := make([]tabview.MarketRow, 0, len(rows))
		for _, r := range rows {
			name, _, _ := strings.Cut(r.Kind, "@")
			g, ok := trade.ParseGoods(book, name)
			if !ok {
				continue
			}
			out = append(out, tabview.MarketRow{Kind: r.Kind, ItemKind: g.Kind, StackID: r.StackID, Building: g.Building, Label: r.Label, Category: r.Category, CargoQty: r.ColonyQty, StockQty: r.TraderQty, BuyC: r.BuyC, SellC: r.SellC, GiftC: r.GiftC, WillBuy: r.WillBuy, WillSell: r.WillSell})
		}
		return out
	}
	v.Rows, v.GiftRows = convert(rows.Rows), convert(rows.GiftRows)
	return v, true
}
