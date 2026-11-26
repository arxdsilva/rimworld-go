// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"testing"

	"reflect"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/trade"
	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/caravan"
)

func TestWorldMarketViewOffersOnlyPurchasableStock(t *testing.T) {
	g := worldGame(t)
	tile := nearDest(t, g)
	id := g.pawnIDs[0]
	cid := visitingMarket(t, g, tile, id)
	travelOf(g).Carry(cid, item.Stack{Kind: item.Plasteel, Qty: 2})
	v := g.WorldMap()
	if len(v.Markets) != 1 {
		t.Fatalf("expected a visited market: %+v", v.Markets)
	}
	var seen bool
	for _, row := range v.Markets[0].Rows {
		if row.Kind == item.Plasteel.String() {
			seen = true
			if row.StockQty != 0 || row.CargoQty != 2 || row.WillBuy || !row.WillSell {
				t.Fatalf("the world trade flags contradict the trader session conventions: %+v", row)
			}
		}
	}
	if !seen {
		t.Fatal("the caravan's plasteel was missing from the trade view")
	}
	fa := g.factionOf(g.settlementFaction(tile))
	fa.Relation = faction.Hostile
	if len(g.WorldMap().Markets) != 0 {
		t.Fatal("a newly hostile settlement still offers a trade view")
	}
}

func TestWorldMarketRepeatedSalesKeepSilverAndAttributesAcrossSave(t *testing.T) {
	g := worldGame(t)
	tile := nearDest(t, g)
	pawn := g.pawnIDs[0]
	cid := visitingMarket(t, g, tile, pawn)
	travelOf(g).Carry(cid, item.Stack{Kind: item.Silver, Qty: 10})
	travelOf(g).Carry(cid, item.Stack{Kind: item.HerbalMedicine, Qty: 10})
	before, _ := fleetOf(g).Market().At(tile)
	for i := 0; i < 10; i++ {
		g.Enqueue(command.Command{Kind: command.WorldCaravanTrade, Target: cid, Pawn: uint64(pawn),
			Name: trade.EncodeOffer([]trade.Delta{{Kind: item.HerbalMedicine, Qty: -1}})})
		g.ApplyPending()
	}
	after, _ := fleetOf(g).Market().At(tile)
	if marketQty(after.Stock, item.HerbalMedicine) != marketQty(before.Stock, item.HerbalMedicine)+10 {
		t.Fatalf("repeated sales failed to transfer goods: before %+v, after %+v", before.Stock, after.Stock)
	}
	physical, silverStacks, silver := travelOf(g).Goods(cid), 0, 0
	for _, st := range physical {
		if st.Kind == item.Silver {
			silverStacks++
			silver += st.Qty
		}
	}
	if silverStacks != 1 || silver <= 10 || after.Stock.Silver+silver != before.Stock.Silver+10 {
		t.Fatalf("repeated trades fragmented or created silver: %+v", physical)
	}
	if len(after.Preserved) != 1 || after.Preserved[0].Qty != 10 {
		t.Fatalf("repeated sales fragmented the settlement inventory: %+v", after.Preserved)
	}
	restored, _ := saveLoadInfo(t, g)
	restored.worldS().planet, restored.worldS().planetTried = testPlanet().Fork(), true
	if restored.StateHash() != g.StateHash() {
		t.Fatal("repeated trade settlement state changed across save/load")
	}
	loaded, _ := fleetOf(restored).Market().At(tile)
	if !reflect.DeepEqual(loaded, after) {
		t.Fatalf("saved stock differs: got %+v want %+v", loaded, after)
	}
}

func TestWorldMarketPurchaseRefreshesReturnSpeed(t *testing.T) {
	g := worldGame(t)
	tile := nearDest(t, g)
	pawn := g.pawnIDs[0]
	cid := visitingMarket(t, g, tile, pawn)
	travelOf(g).Carry(cid, item.Stack{Kind: item.Silver, Qty: 900})
	market, _ := fleetOf(g).Market().At(tile)
	kind := item.Wood
	if market.Stock.Kind == trade.SpacerSettlement {
		kind = item.Steel
	}
	c := fleetOf(g).At(entity.EntityID(cid))
	before := c.C.Load
	g.Enqueue(command.Command{Kind: command.WorldCaravanTrade, Target: cid, Pawn: uint64(pawn),
		Name: trade.EncodeOffer([]trade.Delta{{Kind: kind, Qty: 20}})})
	g.ApplyPending()
	if cargoQty(travelOf(g).Goods(cid), kind) != 20 {
		t.Fatal("setup failed to purchase goods for return")
	}
	c = fleetOf(g).At(entity.EntityID(cid))
	grams := 0
	for _, st := range travelOf(g).Goods(cid) {
		grams += st.Qty * st.Kind.MassGrams()
	}
	want := int32(grams * 1000 / (len(c.C.Members) * caravan.PersonCarryKg * 1000))
	if c.C.Load != want || c.C.Load <= before {
		t.Fatalf("traded cargo did not affect return speed: old %d new %d expected %d", before, c.C.Load, want)
	}
}
