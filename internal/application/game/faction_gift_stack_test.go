// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"testing"

	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/quality"
	"rimworld-go/internal/domain/economy/trade"
	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

func TestFactionGiftSelectsExactQualityConditionMaterialStack(t *testing.T) {
	g, caravan := caravanGame(t)
	neg := openCaravanTrade(t, g, caravan)
	tradeYard(t, g, neg, 900, 80)
	stock := g.colonyGoods(false)
	lowSpot, ok := g.stacks.Get(stock.IDs(item.Silver)[0])
	if !ok {
		t.Fatal("silver stockpile missing")
	}
	highSpot, ok := g.stacks.Get(stock.IDs(item.Components)[0])
	if !ok {
		t.Fatal("materials stockpile missing")
	}
	poorID, err := g.SpawnStack(item.Longsword, 1, lowSpot.X, lowSpot.Y)
	if err != nil {
		t.Fatal(err)
	}
	fineID, err := g.SpawnStack(item.Longsword, 1, highSpot.X, highSpot.Y)
	if err != nil {
		t.Fatal(err)
	}
	poor, _ := g.stacks.Get(poorID)
	poor.Quality, poor.Stuff = quality.Stored(quality.Poor), item.Wood
	poor.Wear = poor.MaxHP() / 2
	g.putStack(poorID, poor)
	fine, _ := g.stacks.Get(fineID)
	fine.Quality, fine.Stuff = quality.Stored(quality.Excellent), item.Gold
	g.putStack(fineID, fine)

	f := g.factionByID(caravan.Faction)
	if f == nil {
		t.Fatal("caravan faction missing")
	}
	f.Goodwill = 0
	faction.Update(f)
	v, open := g.TradeView()
	if !open {
		t.Fatal("trade session closed")
	}
	var poorRow, fineRow *TradeRowView
	paid := 0
	for _, row := range v.Rows {
		if row.StackID == uint64(poorID) || row.StackID == uint64(fineID) {
			paid++
		}
	}
	if paid != 2 {
		t.Fatalf("paid trade must distinguish physical swords: %+v", v.Rows)
	}
	for i := range v.GiftRows {
		row := &v.GiftRows[i]
		switch row.StackID {
		case uint64(poorID):
			poorRow = row
		case uint64(fineID):
			fineRow = row
		}
	}
	if poorRow == nil || fineRow == nil ||
		poorRow.GiftC != trade.GiftStackCenti(g.sk.content.Items, poor, trade.GiftVisitor, v.Social) ||
		fineRow.GiftC != trade.GiftStackCenti(g.sk.content.Items, fine, trade.GiftVisitor, v.Social) ||
		fineRow.GiftC <= poorRow.GiftC || fineRow.Label == poorRow.Label {
		t.Fatalf("physical gifts differ but view does not: poor=%+v fine=%+v", poorRow, fineRow)
	}
	value := fineRow.GiftC / 100
	want := faction.GiftGoodwill(*f, value)
	if want <= 0 {
		t.Fatalf("high-value material was not valuable enough for an observable gift: %d", value)
	}
	offer := trade.EncodeOffer([]trade.Delta{{Kind: item.Longsword, Qty: -1, StackID: uint64(fineID)}})
	g.Enqueue(command.Command{Kind: command.ProposeTrade, Name: offer, Enabled: true})
	g.ApplyPending()
	if _, ok := g.stacks.Get(fineID); ok {
		t.Fatal("the selected excellent sword remained in stock")
	}
	back, ok := g.stacks.Get(poorID)
	if !ok || back != poor || f.Goodwill != want {
		t.Fatalf("different sword consumed or wrong goodwill: poor %+v/%t, goodwill %d want %d", back, ok, f.Goodwill, want)
	}
	check := func(name, offer string) {
		t.Helper()
		before := f.Goodwill
		g.Enqueue(command.Command{Kind: command.ProposeTrade, Name: offer, Enabled: true})
		g.ApplyPending()
		view, ok := g.TradeView()
		if !ok || view.Reason != ReasonOfferBroken || f.Goodwill != before {
			t.Fatalf("%s accepted stale gift: view %+v, goodwill %d", name, view, f.Goodwill)
		}
		if actual, ok := g.stacks.Get(poorID); !ok || actual != poor {
			t.Fatalf("%s consumed unrelated physical item", name)
		}
	}
	check("stale", offer)
	check("wrong kind", trade.EncodeOffer([]trade.Delta{{Kind: item.Club, Qty: -1, StackID: uint64(poorID)}}))
	poor.Forbidden = true
	g.putStack(poorID, poor)
	check("forbidden", trade.EncodeOffer([]trade.Delta{{Kind: item.Longsword, Qty: -1, StackID: uint64(poorID)}}))
	poor.Forbidden = false
	g.putStack(poorID, poor)
	stable := saveLoad(t, g)
	loaded, ok := stable.stacks.Get(poorID)
	if !ok || loaded != poor {
		t.Fatalf("save/load lost physical gift material, quality or condition: %+v", loaded)
	}
}

func TestFactionGiftOrdinaryGoodsRemainGrouped(t *testing.T) {
	g, caravan := caravanGame(t)
	neg := openCaravanTrade(t, g, caravan)
	tradeYard(t, g, neg, 900, 80)
	gd := g.colonyGoods(false)
	p, _ := g.stacks.Get(gd.IDs(item.Components)[0])
	first, _ := g.SpawnStack(item.Wood, 35, p.X, p.Y)
	second, _ := g.SpawnStack(item.Wood, 45, p.X, p.Y)
	v, open := g.TradeView()
	if !open {
		t.Fatal("no gift trade view")
	}
	woodRows, silverRows := 0, 0
	for _, r := range v.GiftRows {
		switch r.Kind {
		case item.Wood.String():
			woodRows++
			if r.ColonyQty != 80 || r.StackID != 0 || r.GiftC != trade.GiftStackCenti(g.sk.content.Items, item.Stack{Kind: item.Wood}, trade.GiftVisitor, v.Social) {
				t.Fatalf("ordinary wood should be one priced row: %+v", r)
			}
		case item.Silver.String():
			silverRows++
			if r.ColonyQty != 900 || r.StackID != 0 {
				t.Fatalf("silver gift is grouped at face value: %+v", r)
			}
		}
	}
	if woodRows != 1 || silverRows != 1 {
		t.Fatalf("gift UI split ordinary stacks: %d wood, %d silver", woodRows, silverRows)
	}
	f := g.factionByID(caravan.Faction)
	before := f.Goodwill
	offer := trade.EncodeOffer([]trade.Delta{{Kind: item.Wood, Qty: -65}})
	want := faction.GiftGoodwill(*f, 65*trade.GiftStackCenti(g.sk.content.Items, item.Stack{Kind: item.Wood}, trade.GiftVisitor, 0)/100)
	g.Enqueue(command.Command{Kind: command.ProposeTrade, Name: offer, Enabled: true})
	g.ApplyPending()
	v, _ = g.TradeView()
	if v.Reason != "" || f.Goodwill-before != want {
		t.Fatalf("bulk gift rejected or misvalued: %q, goodwill %d want %d", v.Reason, f.Goodwill-before, want)
	}
	qty := 0
	for _, id := range []entity.EntityID{first, second} {
		if st, ok := g.stacks.Get(id); ok {
			qty += st.Qty
		}
	}
	if qty != 15 {
		t.Fatalf("bulk gift removed %d, want leave 15", 80-qty)
	}
}
