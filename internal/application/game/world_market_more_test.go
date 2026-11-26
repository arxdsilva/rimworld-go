// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"fmt"
	"reflect"
	"testing"

	"rimworld-go/internal/application/game/maps"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/trade"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/caravan"
)

func TestWorldMarketSharedStockLastUnitAndSave(t *testing.T) {
	g := worldGame(t)
	tile := nearDest(t, g)
	a, b := g.pawnIDs[0], g.pawnIDs[1]
	ac := visitingMarket(t, g, tile, a)
	bc := visitingMarket(t, g, tile, b)
	m, _ := fleetOf(g).Market().At(tile)
	kind := item.IndustrialMedicine
	if m.Stock.Kind == trade.TribalSettlement {
		kind = item.HerbalMedicine
	}
	n := marketQty(m.Stock, kind)
	if n <= 0 {
		t.Fatalf("no guaranteed medicine at the base: %+v", m.Stock)
	}
	travelOf(g).Carry(ac, item.Stack{Kind: item.Silver, Qty: 2000})
	travelOf(g).Carry(bc, item.Stack{Kind: item.Silver, Qty: 2000})
	g.Enqueue(command.Command{Kind: command.WorldCaravanTrade, Pawn: uint64(a), Target: ac,
		Name: trade.EncodeOffer([]trade.Delta{{Kind: kind, Qty: n}})})
	g.Step(1)
	after, _ := fleetOf(g).Market().At(tile)
	if marketQty(after.Stock, kind) != 0 || cargoQty(travelOf(g).Goods(ac), kind) != n {
		t.Fatalf("first caravan did not purchase the last of %v: %v, %+v", kind, n, after.Stock)
	}
	cargoBefore := travelOf(g).Goods(bc)
	g.Enqueue(command.Command{Kind: command.WorldCaravanTrade, Pawn: uint64(b), Target: bc,
		Name: trade.EncodeOffer([]trade.Delta{{Kind: kind, Qty: 1}})})
	g.ApplyPending()
	recheck, _ := fleetOf(g).Market().At(tile)
	if !reflect.DeepEqual(recheck, after) || !reflect.DeepEqual(travelOf(g).Goods(bc), cargoBefore) {
		t.Fatal("second caravan purchased a depleted settlement inventory")
	}
	loaded, _ := saveLoadInfo(t, g)
	loaded.worldS().planet, loaded.worldS().planetTried = testPlanet().Fork(), true
	if loaded.StateHash() != g.StateHash() {
		t.Fatal("shared depleted settlement stock changed in save/load")
	}
}

func TestWorldMarketNativePackedCargoRoundTrip(t *testing.T) {
	g := worldGame(t)
	dest := nearDest(t, g)
	pawn := g.pawnIDs[0]
	hx, hy := g.home()
	silver, err := g.SpawnStack(item.Silver, 350, hx, hy)
	if err != nil {
		t.Fatal(err)
	}
	wood, err := g.SpawnStack(item.Wood, 6, hx+1, hy)
	if err != nil {
		t.Fatal(err)
	}
	silverBefore, woodBefore := g.stockOf(item.Silver), g.stockOf(item.Wood)
	g.Enqueue(command.Command{Kind: command.FormCaravan,
		Name: fmt.Sprintf("%d,%d:350,%d:6", pawn, silver, wood), Count: dest})
	var cid uint64
	for i := 0; i < 90; i++ {
		g.Step(400)
		c := fleetOf(g).Of(pawn)
		if c != nil && c.C.State != caravan.Forming {
			cid = c.C.ID
			break
		}
	}
	if cid == 0 {
		t.Fatal("native caravan packing never departed")
	}
	loaded := travelOf(g).Goods(cid)
	if cargoQty(loaded, item.Silver) != 350 || cargoQty(loaded, item.Wood) != 6 {
		t.Fatalf("goods were not physically loaded from the home map: %+v", loaded)
	}
	if left := g.stockOf(item.Silver); left != silverBefore-350 {
		t.Fatalf("silver conservation: ground %d, before %d, loaded 350", left, silverBefore)
	}
	if left := g.stockOf(item.Wood); left != woodBefore-6 {
		t.Fatalf("wood conservation: ground %d, before %d, loaded 6", left, woodBefore)
	}
	maps.DevArrive(fleetOf(g), entity.EntityID(cid), dest, uint64(g.clock.Now()))
	m, ok := fleetOf(g).Market().At(dest)
	if !ok {
		t.Fatal("visiting lost the settlement")
	}
	offer := []trade.Delta{{Kind: item.Wood, Qty: -6}, {Kind: item.Cloth, Qty: 2}}
	term := g.termsFor(pawn, g.factionOf(g.settlementFaction(dest)))
	term.ImproveCenti = 200
	expected := trade.Settle(item.Book{}, offer, travelOf(g).TradeCounts(cid), m.Stock, term, false)
	if !expected.OK() {
		t.Fatalf("normal packed goods could not settle: %v", expected.Reject)
	}
	g.Enqueue(command.Command{Kind: command.WorldCaravanTrade, Pawn: uint64(pawn), Target: cid, Name: trade.EncodeOffer(offer)})
	g.Step(1)
	bought := travelOf(g).Goods(cid)
	if cargoQty(bought, item.Wood) != 0 || cargoQty(bought, item.Cloth) != 2 ||
		cargoQty(bought, item.Silver) != 350+expected.Silver {
		t.Fatalf("physical settlement trade disagrees with trade.Settle: %+v want silver %d", bought, 350+expected.Silver)
	}
	g.Enqueue(command.Command{Kind: command.CaravanReturn, Target: cid})
	g.Step(1)
	for i := 0; i < 90 && fleetOf(g).At(entity.EntityID(cid)) != nil; i++ {
		worldForward(g, 2500)
	}
	if fleetOf(g).At(entity.EntityID(cid)) != nil {
		t.Fatal("trading caravan failed to return home")
	}
	var backSilver, backCloth int
	for _, id := range g.itemIDs {
		st, ok := g.stacks.Get(id)
		if !ok {
			continue
		}
		if st.Kind == item.Silver {
			backSilver += st.Qty
		}
		if st.Kind == item.Cloth {
			backCloth += st.Qty
		}
	}
	if backSilver < 350+expected.Silver || backCloth < 2 {
		t.Fatalf("returned purchased cargo did not reach the map: silver %d cloth %d", backSilver, backCloth)
	}
}
