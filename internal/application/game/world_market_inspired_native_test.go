// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"testing"

	"rimworld-go/internal/application/game/mind"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/trade"
	"rimworld-go/internal/domain/sim/command"
)

func TestWorldMarketInspiredQuoteAndActualSettlementAgree(t *testing.T) {
	g := worldGame(t)
	tile := nearDest(t, g)
	pawn := g.pawnIDs[0]
	cid := visitingMarket(t, g, tile, pawn)
	travelOf(g).Carry(cid, item.Stack{Kind: item.Silver, Qty: 800})
	g.mind.Inspire.Start(pawn, mind.Trade)

	stock, ok := fleetOf(g).Market().At(tile)
	if !ok || marketQty(stock.Stock, item.Cloth) < 1 {
		t.Fatal("visiting settlement has no cloth stock")
	}
	terms := g.termsFor(pawn, g.factionOf(g.settlementFaction(tile)))
	if !terms.Inspired {
		t.Fatal("negotiator lacks trade inspiration")
	}
	terms.ImproveCenti = 200
	wantPrice := trade.Price(item.Book{}, item.Cloth, stock.Stock.Kind, trade.Buy, terms)
	seen := false
	for _, market := range g.WorldMap().Markets {
		if market.CaravanID != cid {
			continue
		}
		for _, row := range market.Rows {
			if row.Kind == item.Cloth.String() {
				seen = true
				if row.BuyC != wantPrice {
					t.Fatalf("world quote %d, authoritative price %d", row.BuyC, wantPrice)
				}
			}
		}
	}
	if !seen {
		t.Fatal("world view omitted an available stock line")
	}
	rejected := trade.EncodeOffer([]trade.Delta{{Kind: item.Cloth, Qty: trade.MaxOfferQty}})
	g.Enqueue(command.Command{Kind: command.WorldCaravanTrade, Target: cid, Pawn: uint64(pawn), Name: rejected})
	g.ApplyPending()
	if !g.mind.Inspire.Has(pawn, mind.Trade) {
		t.Fatal("rejected market offer spent trade inspiration")
	}
	offer := []trade.Delta{{Kind: item.Cloth, Qty: 1}}
	result := trade.Settle(item.Book{}, offer, travelOf(g).TradeCounts(cid), stock.Stock, terms, false)
	if !result.OK() {
		t.Fatalf("inspired market offer refused preflight: %s", result.Reject)
	}
	g.Enqueue(command.Command{Kind: command.WorldCaravanTrade, Target: cid, Pawn: uint64(pawn), Name: trade.EncodeOffer(offer)})
	g.ApplyPending()
	if g.mind.Inspire.Has(pawn, mind.Trade) {
		t.Fatal("successful physical market trade retained inspiration")
	}
	got := travelOf(g).Goods(cid)
	if cargoQty(got, item.Cloth) != 1 || cargoQty(got, item.Silver) != 800+result.Silver {
		t.Fatalf("physical cargo disagrees with inspired quote: %+v, silver delta %d", got, result.Silver)
	}
}
