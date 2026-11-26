// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"bytes"
	"testing"

	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/trade"
	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/pawn/skill"
	"rimworld-go/internal/domain/sim/command"
)

func TestCaravanPaidTradeAccruesGoodwillOnlyForMerchandise(t *testing.T) {
	g, caravan := caravanGame(t)
	neg := openCaravanTrade(t, g, caravan)
	tradeYard(t, g, neg, 10000, 10)
	f := g.factionByID(caravan.Faction)
	f.Goodwill, f.Relation, f.TradeRemainder = 0, faction.Neutral, 499
	goods := caravan.Stock.Lines[0]
	offer := []trade.Delta{{Kind: goods.Kind, Building: goods.Building, Qty: 1}}
	expected := trade.Settle(g.sk.content.Items, offer, g.colonyGoods(false).Counts(),
		caravan.Stock, g.termsFor(neg, f), false)
	if !expected.OK() {
		t.Fatalf("valid merchandise purchase refused: %v", expected.Reject)
	}
	value := expected.TradedValue(g.sk.content.Items)
	g.Enqueue(command.Command{Kind: command.ProposeTrade, Name: trade.EncodeOffer(offer)})
	g.ApplyPending()
	if v, _ := g.TradeView(); v.Reason != "" {
		t.Fatalf("real caravan trade refused: %q", v.Reason)
	}
	if f.Goodwill != 1+value/500 || int(f.TradeRemainder) != (499+value)%500 {
		t.Fatalf("real trade goodwill %d / carry %d, value %d", f.Goodwill, f.TradeRemainder, value)
	}
	gift := []trade.Delta{{Kind: item.Silver, Qty: -1000}}
	beforeRemainder, beforeGoodwill := f.TradeRemainder, f.Goodwill
	g.Enqueue(command.Command{Kind: command.ProposeTrade, Name: trade.EncodeOffer(gift), Enabled: true})
	g.ApplyPending()
	if v, _ := g.TradeView(); v.Reason != "" {
		t.Fatalf("real visitor gift refused: %q", v.Reason)
	}
	giftValue := trade.GiftValue(g.sk.content.Items, gift,
		trade.Terms{Social: int(g.skillOf(neg, skill.Social).Level), FactionType: f.Type})
	// The expected gift is evaluated at the recipient's goodwill before gifting.
	if f.TradeRemainder != beforeRemainder ||
		f.Goodwill != beforeGoodwill+faction.GiftGoodwill(
			faction.Faction{Type: f.Type, Goodwill: beforeGoodwill}, giftValue) {
		t.Fatalf("gift mixed into merchandise trade or credited wrong amount: goodwill %d carry %d", f.Goodwill, f.TradeRemainder)
	}
	var buf bytes.Buffer
	if err := g.Save(&buf, "trade"); err != nil {
		t.Fatal(err)
	}
	recovered, _, err := Load(&buf)
	if err != nil {
		t.Fatal(err)
	}
	other := recovered.factionByID(f.ID)
	if other == nil || other.Goodwill != f.Goodwill || other.TradeRemainder != f.TradeRemainder {
		t.Fatalf("goodwill accumulation lost on save: live %+v recovered %+v", f, other)
	}
}
