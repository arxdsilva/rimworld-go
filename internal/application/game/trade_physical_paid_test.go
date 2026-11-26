// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"strings"
	"testing"

	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/quality"
	"rimworld-go/internal/domain/economy/trade"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

func paidSwordCaravan(t *testing.T) (*Game, *traderCaravan, entity.EntityID, entity.EntityID, entity.EntityID, item.Stack, item.Stack) {
	t.Helper()
	g, c := caravanGame(t)
	neg := openCaravanTrade(t, g, c)
	tradeYard(t, g, neg, 30000, 80)
	gd := g.colonyGoods(false)
	if len(gd.IDs(item.Components)) == 0 {
		t.Fatal("no stockpile")
	}
	pos, ok := g.stacks.Get(gd.IDs(item.Components)[0])
	if !ok {
		t.Fatal("materials lost")
	}
	poorID, err := g.SpawnStack(item.Longsword, 1, pos.X, pos.Y)
	if err != nil {
		t.Fatal(err)
	}
	fineID, err := g.SpawnStack(item.Longsword, 1, pos.X, pos.Y)
	if err != nil {
		t.Fatal(err)
	}
	poor, _ := g.stacks.Get(poorID)
	poor.Stuff = item.Wood
	poor.Quality = quality.Stored(quality.Poor)
	poor.Wear = poor.MaxHP() / 2
	g.putStack(poorID, poor)
	fine, _ := g.stacks.Get(fineID)
	fine.Stuff = item.Gold
	fine.Quality = quality.Stored(quality.Excellent)
	g.putStack(fineID, fine)
	if len(c.Cargo) == 0 {
		t.Fatal("no caravan cargo")
	}
	// Real stock and physical carrier manifests have the same silver.
	c.Cargo[0].Silver += 30000 - c.Stock.Silver
	c.Stock.Silver = 30000
	return g, c, neg, poorID, fineID, poor, fine
}

func TestPaidTradePhysicalQualityRoundTripThroughSave(t *testing.T) {
	g, c, neg, poorID, fineID, poor, fine := paidSwordCaravan(t)
	v, ok := g.TradeView()
	if !ok || v.Negotiator != neg {
		t.Fatalf("physical trade opened on the wrong negotiator: %+v", v)
	}
	var low, high *TradeRowView
	for i := range v.Rows {
		row := &v.Rows[i]
		switch row.StackID {
		case uint64(poorID):
			low = row
		case uint64(fineID):
			high = row
		}
	}
	if low == nil || high == nil || low.SellC >= high.SellC ||
		!strings.Contains(low.Label, "wood") || !strings.Contains(high.Label, "gold") {
		t.Fatalf("separate paid quality quotes missing: low %+v high %+v", low, high)
	}
	beforeSilver := totalOf(g, item.Silver)
	sell := trade.EncodeOffer([]trade.Delta{{Kind: item.Longsword, Qty: -1, StackID: uint64(fineID)}})
	g.Enqueue(command.Command{Kind: command.ProposeTrade, Name: sell})
	g.ApplyPending()
	after, ok := g.TradeView()
	if !ok || after.Reason != "" {
		t.Fatalf("sale refused: %+v", after)
	}
	if _, ok := g.stacks.Get(fineID); ok {
		t.Fatal("selected excellent sword remained in colony")
	}
	if st, ok := g.stacks.Get(poorID); !ok || st != poor {
		t.Fatalf("unselected worn wooden sword changed: %+v", st)
	}
	if got, want := totalOf(g, item.Silver)-beforeSilver, high.SellC/100; got != want {
		t.Fatalf("cash change %d, previewed seller price %d", got, want)
	}
	var lot trade.Lot
	for _, line := range c.Stock.Lines {
		if line.Kind == item.Longsword {
			if len(line.Lots) != 1 {
				t.Fatalf("sold sword lost physical record: %+v", line)
			}
			lot = line.Lots[0]
		}
	}
	if lot.ID == 0 || lot.Stack.Kind != item.Longsword ||
		lot.Stack.Quality != fine.Quality || lot.Stack.Stuff != fine.Stuff ||
		lot.Stack.Wear != fine.Wear {
		t.Fatalf("trader lost fine sword attributes: %+v", lot)
	}
	g = saveLoad(t, g)
	c = g.trade.caravan
	if c == nil {
		t.Fatal("caravan disappeared on save/load")
	}
	neg = openCaravanTrade(t, g, c)
	v, ok = g.TradeView()
	if !ok {
		t.Fatal("no reopened trade")
	}
	var resale *TradeRowView
	for i := range v.Rows {
		if v.Rows[i].StackID == lot.ID && v.Rows[i].TraderQty > 0 {
			resale = &v.Rows[i]
		}
	}
	if resale == nil || resale.BuyC != trade.PriceMarket(
		trade.Goods{Kind: item.Longsword}, item.StackValue(fine), c.Kind, trade.Buy, g.termsFor(neg, g.factionByID(c.Faction))) {
		t.Fatalf("saved trader variant missing or mispriced: %+v", resale)
	}
	buy := trade.EncodeOffer([]trade.Delta{{Kind: item.Longsword, Qty: 1, StackID: lot.ID}})
	g.Enqueue(command.Command{Kind: command.ProposeTrade, Name: buy})
	g.ApplyPending()
	if v, ok := g.TradeView(); !ok || v.Reason != "" {
		t.Fatalf("repurchase refused: %+v", v)
	}
	found := false
	for _, id := range g.itemIDs {
		st, ok := g.stacks.Get(id)
		if ok && st.Kind == item.Longsword && st.Quality == fine.Quality &&
			st.Stuff == fine.Stuff && st.Wear == fine.Wear {
			found = true
		}
	}
	if !found {
		t.Fatal("repurchase constructed a default sword instead of restoring exact quality and condition")
	}
	for _, line := range c.Stock.Lines {
		if line.Kind == item.Longsword && len(line.Lots) > 0 {
			t.Fatalf("repurchased item still offered by trader: %+v", line)
		}
	}
	if got := saveLoad(t, g).StateHash(); got != g.StateHash() {
		t.Fatal("trade replay/save did not preserve returned item's state")
	}
}

func TestPaidTradeCarrierDeathDropsOriginalPhysicalItemOnce(t *testing.T) {
	g, c, _, _, fineID, _, fine := paidSwordCaravan(t)
	sell := trade.EncodeOffer([]trade.Delta{{Kind: item.Longsword, Qty: -1, StackID: uint64(fineID)}})
	g.Enqueue(command.Command{Kind: command.ProposeTrade, Name: sell})
	g.ApplyPending()
	carrierGone(g, c, c.Trader, true)
	count := func() int {
		n := 0
		for _, id := range g.itemIDs {
			st, ok := g.stacks.Get(id)
			if ok && st.Kind == item.Longsword && st.Quality == fine.Quality &&
				st.Wear == fine.Wear && st.Stuff == fine.Stuff {
				n += st.Qty
			}
		}
		return n
	}
	if got := count(); got != 1 {
		t.Fatalf("trader lost sword instead of dropping it with original state: %d", got)
	}
	carrierGone(g, c, c.Trader, true)
	if got := count(); got != 1 {
		t.Fatalf("carrier death duplicated trader-acquired sword: %d", got)
	}
	for _, line := range c.Stock.Lines {
		if line.Kind == item.Longsword && len(line.Lots) > 0 {
			t.Fatalf("dropped item remained in trader stock: %+v", line)
		}
	}
}
