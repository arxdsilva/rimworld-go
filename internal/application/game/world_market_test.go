// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"reflect"
	"slices"
	"testing"

	"rimworld-go/internal/application/game/maps"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/trade"
	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/caravan"
)

func visitingMarket(t *testing.T, g *Game, tile int, pawn entity.EntityID) uint64 {
	t.Helper()
	c := formCaravan(t, g, tile, pawn)
	cid := c.C.ID
	maps.DevArrive(fleetOf(g), entity.EntityID(cid), tile, uint64(g.clock.Now()))
	if cv := fleetOf(g).At(entity.EntityID(cid)); cv == nil || cv.C.State != caravan.Visiting {
		t.Fatalf("caravan %d did not visit the settlement: %+v", cid, cv)
	}
	if _, ok := fleetOf(g).Market().At(tile); !ok {
		t.Fatal("visiting failed to open the market")
	}
	return cid
}

func marketQty(stock trade.Stock, k item.Kind) int {
	for _, l := range stock.Lines {
		if l.Kind == k && l.Building == 0 {
			return l.Qty
		}
	}
	return 0
}

func cargoQty(stacks []item.Stack, k item.Kind) int {
	n := 0
	for _, st := range stacks {
		if st.Kind == k {
			n += st.Qty
		}
	}
	return n
}

func TestWorldMarketTradeConservesAndPersists(t *testing.T) {
	g := worldGame(t)
	tile := nearDest(t, g)
	pawn := g.pawnIDs[0]
	cid := visitingMarket(t, g, tile, pawn)
	travelOf(g).Carry(cid, item.Stack{Kind: item.Silver, Qty: 800})
	// A non-default apparel stack must survive both the sale and the
	// purchase from a different caravan in its original physical form.
	sold := item.Stack{Kind: item.Parka, Qty: 1, Stuff: item.Cloth, Quality: 4, Wear: 10}
	travelOf(g).Carry(cid, sold)
	before, _ := fleetOf(g).Market().At(tile)
	world := g.WorldMap()
	if len(world.Markets) != 1 || world.Markets[0].CaravanID != cid ||
		len(world.Markets[0].Rows) == 0 || cargoQty(world.Caravans[0].Cargo, item.Parka) != 1 {
		t.Fatalf("world UI did not report the market and cargo: %+v", world)
	}
	offer := trade.EncodeOffer([]trade.Delta{{Kind: item.Parka, Qty: -1}, {Kind: item.Cloth, Qty: 1}})
	g.Enqueue(command.Command{Kind: command.WorldCaravanTrade, Target: cid, Pawn: uint64(pawn), Name: offer})
	g.ApplyPending()
	after, _ := fleetOf(g).Market().At(tile)
	cargo := travelOf(g).Goods(cid)
	if marketQty(after.Stock, item.Parka) != marketQty(before.Stock, item.Parka)+1 ||
		marketQty(after.Stock, item.Cloth) != marketQty(before.Stock, item.Cloth)-1 ||
		cargoQty(cargo, item.Parka) != 0 || cargoQty(cargo, item.Cloth) != 1 {
		t.Fatalf("trade did not move exact goods: stock %+v cargo %+v", after.Stock, cargo)
	}
	if after.Stock.Silver+cargoQty(cargo, item.Silver) != before.Stock.Silver+800 {
		t.Fatalf("silver created or destroyed: before %d+800 after %d+%d",
			before.Stock.Silver, after.Stock.Silver, cargoQty(cargo, item.Silver))
	}
	if len(after.Preserved) != 1 || after.Preserved[0].Kind != sold.Kind ||
		after.Preserved[0].Quality != sold.Quality || after.Preserved[0].Wear != sold.Wear ||
		after.Preserved[0].Stuff != sold.Stuff {
		t.Fatalf("original apparel properties lost at the settlement: %+v", after.Preserved)
	}
	restored, _ := saveLoadInfo(t, g)
	restored.worldS().planet, restored.worldS().planetTried = testPlanet().Fork(), true
	if restored.StateHash() != g.StateHash() {
		t.Fatal("world stock and caravan cargo diverged after saving")
	}
	if st, ok := fleetOf(restored).Market().At(tile); !ok || !reflect.DeepEqual(st, after) {
		t.Fatalf("the restored market differs: %+v", st)
	}
	// Rejected offers may post a reason, but leave both inventories exact.
	preStock, _ := fleetOf(g).Market().At(tile)
	preCargo := travelOf(g).Goods(cid)
	for _, c := range []command.Command{
		{Kind: command.WorldCaravanTrade, Target: cid, Pawn: 999, Name: trade.EncodeOffer([]trade.Delta{{Kind: item.Cloth, Qty: 1}})},
		{Kind: command.WorldCaravanTrade, Target: cid, Pawn: uint64(pawn), Name: trade.EncodeOffer([]trade.Delta{{Kind: item.Cloth, Qty: 100000}})},
		{Kind: command.WorldCaravanTrade, Target: cid, Pawn: uint64(pawn), Name: "malformed"},
	} {
		g.Enqueue(c)
		g.ApplyPending()
		curr, _ := fleetOf(g).Market().At(tile)
		if !reflect.DeepEqual(curr, preStock) || !reflect.DeepEqual(travelOf(g).Goods(cid), preCargo) {
			t.Fatal("a rejected trade mutated inventory")
		}
	}
	heavyKind := item.Wood
	if after.Stock.Kind == trade.SpacerSettlement {
		heavyKind = item.Steel
	}
	if marketQty(after.Stock, heavyKind) < 150 {
		t.Fatalf("fixture lacks large cargo %v: %+v", heavyKind, after.Stock)
	}
	_, why := fleetOf(g).Market().Trade(cid, pawn, trade.EncodeOffer([]trade.Delta{{Kind: heavyKind, Qty: 150}}), false)
	if why == "" {
		t.Fatal("the caravan accepted more than its capacity")
	}
	curr, _ := fleetOf(g).Market().At(tile)
	if !reflect.DeepEqual(curr, preStock) || !reflect.DeepEqual(travelOf(g).Goods(cid), preCargo) {
		t.Fatal("failed capacity preflight committed a partial trade")
	}
	second := g.pawnIDs[1]
	secondCID := visitingMarket(t, g, tile, second)
	travelOf(g).Carry(secondCID, item.Stack{Kind: item.Silver, Qty: 600})
	g.Enqueue(command.Command{Kind: command.WorldCaravanTrade, Target: secondCID, Pawn: uint64(second),
		Name: trade.EncodeOffer([]trade.Delta{{Kind: item.Parka, Qty: 1}})})
	g.ApplyPending()
	acquired := travelOf(g).Goods(secondCID)
	found := false
	for _, st := range acquired {
		if st.Kind == sold.Kind && st.Quality == sold.Quality && st.Wear == sold.Wear &&
			st.Stuff == sold.Stuff {
			found = true
		}
	}
	if !found {
		t.Fatalf("a second caravan lost the original traded item's metadata: %+v", acquired)
	}
	g.Enqueue(command.Command{Kind: command.CaravanReturn, Target: secondCID})
	g.ApplyPending()
	for i := 0; i < 90 && fleetOf(g).At(entity.EntityID(secondCID)) != nil; i++ {
		worldForward(g, 2500)
	}
	if fleetOf(g).At(entity.EntityID(secondCID)) != nil {
		t.Fatal("the second caravan did not return")
	}
	landed := false
	for _, id := range g.itemIDs {
		if st := g.stacks.Ref(id); st != nil && st.Kind == sold.Kind &&
			st.Quality == sold.Quality && st.Wear == sold.Wear && st.Stuff == sold.Stuff {
			landed = true
			break
		}
	}
	if !landed {
		t.Fatal("the purchased apparel did not reach the home map intact")
	}
}

func TestWorldMarketRestockAndHostility(t *testing.T) {
	g := worldGame(t)
	tile := nearDest(t, g)
	pawn := g.pawnIDs[0]
	cid := visitingMarket(t, g, tile, pawn)
	f := fleetOf(g)
	now := uint64(g.clock.Now())
	initial, _ := f.Market().At(tile)
	fa := g.factionOf(g.settlementFaction(tile))
	f.Market().Visit(tile, fa, now+29*clock.TicksPerDay)
	before, _ := f.Market().At(tile)
	if !reflect.DeepEqual(initial, before) {
		t.Fatal("revisit before day 30 rerolled the settlement")
	}
	f.Market().Visit(tile, fa, initial.RestockAt)
	restocked, _ := f.Market().At(tile)
	if restocked.Epoch != initial.Epoch+1 || restocked.RestockAt != initial.RestockAt+30*clock.TicksPerDay {
		t.Fatalf("restock schedule drifted: initial %+v, new %+v", initial, restocked)
	}
	beforeHostile := restocked.Clone()
	fa.Relation = faction.Hostile
	if res, why := f.Market().Trade(cid, pawn, trade.EncodeOffer([]trade.Delta{{Kind: item.Cloth, Qty: 1}}), false); res.OK() || why == "" {
		t.Fatal("a hostile settlement accepted the trade")
	}
	after, _ := f.Market().At(tile)
	if !reflect.DeepEqual(after, beforeHostile) {
		t.Fatal("a hostile trade mutated the settlement")
	}
	// World views are copies; callers cannot modify authoritative stock.
	view, ok := f.Market().At(tile)
	if !ok {
		t.Fatal("lost market")
	}
	view.Stock.Lines[0].Qty++
	if current, _ := f.Market().At(tile); reflect.DeepEqual(current, view) {
		t.Fatal("a caller mutated the authoritative market through the read view")
	}
}

func TestWorldMarketDeterministicSaveReplay(t *testing.T) {
	g := worldGame(t)
	tile := nearDest(t, g)
	pawn := g.pawnIDs[0]
	cid := visitingMarket(t, g, tile, pawn)
	travelOf(g).Carry(cid, item.Stack{Kind: item.Silver, Qty: 400})
	h, _ := saveLoadInfo(t, g)
	h.worldS().planet, h.worldS().planetTried = testPlanet().Fork(), true
	if h.StateHash() != g.StateHash() {
		t.Fatal("initial state differs across save/load")
	}
	action := command.Command{Kind: command.WorldCaravanTrade, Target: cid, Pawn: uint64(pawn),
		Name: trade.EncodeOffer([]trade.Delta{{Kind: item.Cloth, Qty: 2}})}
	for _, sim := range []*Game{g, h} {
		sim.Enqueue(action)
		sim.ApplyPending()
	}
	if g.StateHash() != h.StateHash() ||
		!reflect.DeepEqual(g.WorldMap().Markets, h.WorldMap().Markets) ||
		!slices.EqualFunc(travelOf(g).Goods(cid), travelOf(h).Goods(cid), func(a, b item.Stack) bool { return a == b }) {
		t.Fatal("settlement trade diverged across uninterrupted/save-loaded replay")
	}
}
