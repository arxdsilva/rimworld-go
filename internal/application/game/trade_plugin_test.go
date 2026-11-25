// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"rimworld-go/internal/application/game/setupview"
	"testing"

	"rimworld-go/internal/application/game/visit"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/trade"
	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// Plugin goods in trade (PLG-026): the trade view lists a plugin good by
// its key, label and category at book prices; deals move it both ways;
// a removed plugin's goods (item.None) neither show nor drop.

// ptBell is the plugin good of ptBook: a copper bell filed under
// components, worth 40, 25 to a tile, stocked by traders that want it.
const ptBell = item.Kind(item.CoreKinds)

// ptBook is core's item kinds plus the copper bell, keyed "frontier.bell".
func ptBook() item.Book {
	core := item.Core()
	defs, keys := make([]item.Def, 0, core.Len()+1), core.Keys()
	for k := range core.Len() {
		defs = append(defs, core.Def(item.Kind(k)))
	}
	defs = append(defs, item.Def{Label: "copper bell", Path: "Manufactured/Components", Mass: 250, Value: 40, Stack: 25, TraderStock: 6})
	return item.NewBook(defs, append(keys, "frontier.bell"))
}

// ptGame is visitorGame run with items as its content, with a trader
// caravan on the map.
func ptGame(t *testing.T, items item.Book) (*Game, *traderCaravan) {
	t.Helper()
	b := NewColonyBuilder(Config{Setup: setupFor(t, "broken_hull", func(s *Setup) { s.MapSize = 60 }), Content: Content{Items: items}})
	var g *Game
	for i := 0; i <= setupview.BuildSteps() && g == nil; i++ {
		if _, _, done := b.Step(1); done {
			g = b.Game()
		}
	}
	if g == nil {
		t.Fatal("the builder never finished")
	}
	stepUntilLanded(t, g)
	first := true
	for i := range g.ng.factions {
		if f := &g.ng.factions[i]; f.Type.Human() {
			f.Defeated, f.Relation = false, faction.Hostile
			if first {
				f.Relation, first = faction.Neutral, false
			}
		}
	}
	if !g.caravanArrive(uint64(g.clock.Now())) {
		t.Fatal("the caravan did not come")
	}
	return g, g.trade.caravan
}

// ptYard designates stockpile cells beside the negotiator and puts each
// of goods (kind, qty) on its own cell.
func ptYard(t *testing.T, g *Game, neg entity.EntityID, goods ...[2]int) {
	t.Helper()
	p, _ := g.positions.Get(neg)
	var cells [][2]int
	for d := 1; d <= 6 && len(cells) < len(goods); d++ {
		for _, o := range [][2]int{{d, 0}, {-d, 0}, {0, d}, {0, -d}} {
			x, y := p.X+o[0], p.Y+o[1]
			if visit.Inside(x, y, g.grid.Width(), g.grid.Height()) && g.dryWalkable(x, y) && g.itemAt(x, y) == 0 && !g.pawnOn(x, y) && len(cells) < len(goods) {
				cells = append(cells, [2]int{x, y})
			}
		}
	}
	if len(cells) < len(goods) {
		t.Fatal("no room for a stockpile")
	}
	for _, c := range cells {
		g.Enqueue(command.Command{Kind: command.DesignateStockpile, X: c[0], Y: c[1], X2: c[0], Y2: c[1]})
	}
	g.ApplyPending()
	for i, gd := range goods {
		spawn(t, g, item.Kind(gd[0]), gd[1], cells[i][0], cells[i][1])
	}
}

// ptRow is the view's row named kind (ok false: none).
func ptRow(v TradeView, kind string) (TradeRowView, bool) {
	for _, r := range v.Rows {
		if r.Kind == kind {
			return r, true
		}
	}
	return TradeRowView{}, false
}

// TestPluginTradeSellBuy: at a curio merchant the colony's plugin bells
// are a row with their key, label, category and book prices, beside core
// rows labelled as before; selling 4 and buying 30 move the bells and the
// silver as Settle priced them, and the 30 land in stacks of at most the
// bell's stack limit (25).
func TestPluginTradeSellBuy(t *testing.T) {
	t.Parallel()
	items := ptBook()
	g, c := ptGame(t, items)
	c.Kind, c.Stock.Kind = trade.CurioMerchant, trade.CurioMerchant
	c.Stock.Add(ptBell, 30)
	c.Stock.Silver = 3000
	c.Cargo = visit.AssignCargo(c.Stock, c.Members) // fixture changed stock after caravan arrival
	neg := openCaravanTrade(t, g, c)
	ptYard(t, g, neg, [2]int{int(item.Silver), 3000}, [2]int{int(ptBell), 10})
	v, ok := g.TradeView()
	if !ok {
		t.Fatal("no trade view")
	}
	terms := g.termsFor(neg, g.factionByID(c.Faction))
	r, ok := ptRow(v, "frontier.bell")
	want := TradeRowView{Kind: "frontier.bell", Label: "copper bell", Category: "Manufactured", ColonyQty: 10, TraderQty: 30,
		BuyC:  trade.PriceOf(items, trade.Goods{Kind: ptBell}, trade.CurioMerchant, trade.Buy, terms),
		SellC: trade.PriceOf(items, trade.Goods{Kind: ptBell}, trade.CurioMerchant, trade.Sell, terms), WillBuy: true, WillSell: true}
	if !ok || r != want || r.BuyC <= r.SellC || r.SellC <= 0 {
		t.Fatalf("bell row %+v (%v), want %+v", r, ok, want)
	}
	for _, row := range v.Rows {
		if k, core := item.ParseKind(row.Kind); core && row.Label != itemLabel(k) {
			t.Fatalf("core row %q labelled %q, want %q", row.Kind, row.Label, itemLabel(k))
		}
	}

	sell := []trade.Delta{{Kind: ptBell, Qty: -4}}
	gd := g.colonyGoods(false)
	res := trade.Settle(items, sell, gd.Counts(), c.Stock, terms, false)
	if !res.OK() || res.Silver != 4*want.SellC/100 {
		t.Fatalf("selling 4 bells settles %+v", res)
	}
	silver0 := totalOf(g, item.Silver)
	g.Enqueue(command.Command{Kind: command.ProposeTrade, Name: trade.EncodeOffer(sell)})
	g.ApplyPending()
	if v, _ := g.TradeView(); v.Reason != "" {
		t.Fatalf("selling refused: %s", v.Reason)
	}
	if totalOf(g, ptBell) != 6 || totalOf(g, item.Silver) != silver0+res.Silver || stockQty(c.Stock, ptBell) != 34 || c.Stock.Silver != 3000-res.Silver {
		t.Fatalf("after selling: bells %d, silver %d (was %d, +%d), trader %+v", totalOf(g, ptBell), totalOf(g, item.Silver), silver0, res.Silver, c.Stock)
	}

	buy := []trade.Delta{{Kind: ptBell, Qty: 30}}
	res = trade.Settle(items, buy, g.colonyGoods(false).Counts(), c.Stock, terms, false)
	if !res.OK() || res.Silver >= 0 {
		t.Fatalf("buying 30 bells settles %+v", res)
	}
	silver1, tsilver1 := totalOf(g, item.Silver), c.Stock.Silver
	g.Enqueue(command.Command{Kind: command.ProposeTrade, Name: trade.EncodeOffer(buy)})
	g.ApplyPending()
	if v, _ := g.TradeView(); v.Reason != "" {
		t.Fatalf("buying refused: %s", v.Reason)
	}
	if totalOf(g, ptBell) != 36 || totalOf(g, item.Silver) != silver1+res.Silver || stockQty(c.Stock, ptBell) != 4 || c.Stock.Silver != tsilver1-res.Silver {
		t.Fatalf("after buying: bells %d, silver %d (was %d, %d), trader %+v", totalOf(g, ptBell), totalOf(g, item.Silver), silver1, res.Silver, c.Stock)
	}
	for _, st := range stacksOf(g, ptBell) {
		if st.Qty > 25 {
			t.Fatalf("a bell stack of %d, over the stack limit 25", st.Qty)
		}
	}
}

// TestTradeIgnoresGoneKinds: a removed plugin's goods in a loaded save
// (item.None, on the trader's stock and on the colony's stockpile) show
// no trade row, and a fallen trader drops none of them.
func TestTradeIgnoresGoneKinds(t *testing.T) {
	t.Parallel()
	g, c := ptGame(t, item.Book{})
	c.Stock.Add(item.None, 5)
	neg := openCaravanTrade(t, g, c)
	ptYard(t, g, neg, [2]int{int(item.Silver), 100}, [2]int{int(item.None), 7})
	v, ok := g.TradeView()
	if !ok || len(v.Rows) == 0 {
		t.Fatalf("view %+v %v", v, ok)
	}
	for _, r := range v.Rows {
		if r.Kind == item.None.String() || r.Kind == "" {
			t.Fatalf("a row for a gone kind: %+v", r)
		}
	}
	if len(v.Rows) != len(c.Stock.Lines)-1 {
		t.Fatalf("%d rows for %d stock lines (one gone)", len(v.Rows), len(c.Stock.Lines))
	}
	gone := totalOf(g, item.None)
	g.dropGoods(trade.Line{Kind: item.None, Qty: 5}, 2, 2, false)
	g.traderLost(c)
	if got := totalOf(g, item.None); got != gone {
		t.Fatalf("gone goods on the map %d, want %d", got, gone)
	}
}
