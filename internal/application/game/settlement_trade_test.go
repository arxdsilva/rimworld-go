// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"bytes"
	"encoding/gob"
	"reflect"
	"slices"
	"testing"

	"rimworld-go/internal/application/game/settlement"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/trade"
	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/caravan"
	"rimworld-go/internal/domain/world/planet"
)

// welcomeTile is a settlement tile of a human faction that welcomes the
// colony (made neutral when none does): the tile and the faction index.
func welcomeTile(t *testing.T, g *Game) (int, int) {
	t.Helper()
	for i := range g.ng.factions {
		f := &g.ng.factions[i]
		if f.Type.Human() && !f.Defeated && len(f.Settlements) > 0 && g.worldPlanet().Settlement[f.Settlements[0]] >= 0 {
			if !f.Welcomes() {
				f.Relation, f.Goodwill = faction.Neutral, 0
			}
			return f.Settlements[0], i
		}
	}
	t.Fatal("no human settlement")
	return -1, -1
}

// settleGame is a world game whose first colonist's caravan visits a
// welcoming settlement with silver: the game, the caravan, the tile and
// the faction index. own gives the game its own planet (for a test that
// destroys a settlement: the shared test planet stays read-only).
func settleGame(t *testing.T, own bool) (*Game, entity.EntityID, int, int) {
	t.Helper()
	g := worldGame(t)
	if own {
		g.worldS().planet = planet.Build(worldTestParams)
	}
	tile, fi := welcomeTile(t, g)
	cid := arriveAt(t, g, tile, g.pawnIDs[0])
	if c := fleetOf(g).At(cid); c == nil || c.C.State != caravan.Visiting {
		t.Fatal("the caravan is not visiting")
	}
	travelOf(g).Carry(uint64(cid), item.Stack{Kind: item.Silver, Qty: 5000})
	return g, cid, tile, fi
}

// settleBytes is the settlement block's gob encoding.
func settleBytes(t *testing.T, s *settlement.Save) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(s); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// cargoCount is caravan cid's count of kind k.
func cargoCount(g *Game, cid entity.EntityID, k item.Kind) int {
	n := 0
	for _, st := range travelOf(g).Goods(uint64(cid)) {
		if st.Kind == k {
			n += st.Qty
		}
	}
	return n
}

// TestSettlementTradeCommands: TradeSettlement through Enqueue and Step
// opens the session, TradeView names the settlement, ProposeTrade moves
// goods into the cargo and CloseTrade closes it. A live settlement session
// keeps a colony session from opening, and the reverse.
func TestSettlementTradeCommands(t *testing.T) {
	t.Parallel()
	g, cid, tile, fi := settleGame(t, false)
	g.Enqueue(command.Command{Kind: command.TradeSettlement, Target: uint64(cid)})
	g.Step(1)
	v, ok := g.TradeView()
	_, _, name := settlement.SiteIn(g.ng.factions, tile)
	if !ok || v.Trader != name || name == "" || v.Faction != g.ng.factions[fi].Name || v.ColonySilver != 5000 || v.Orbital {
		t.Fatalf("view %v %+v (want %q)", ok, v, name)
	}
	b := g.sk.content.Items
	i := slices.IndexFunc(v.Rows, func(r TradeRowView) bool { return r.WillBuy && r.BuyC > 0 && r.BuyC <= 500000 })
	if i < 0 {
		t.Fatalf("nothing to buy: %+v", v.Rows)
	}
	gk, _ := trade.ParseGoods(b, v.Rows[i].Kind)
	had := cargoCount(g, cid, gk.Kind)
	g.Enqueue(command.Command{Kind: command.ProposeTrade, Name: trade.EncodeOffer([]trade.Delta{{Kind: gk.Kind, Qty: 1}})})
	g.Step(1)
	if cargoCount(g, cid, gk.Kind) != had+1 || cargoCount(g, cid, item.Silver) >= 5000 {
		t.Fatalf("the buy moved nothing: %v", travelOf(g).Goods(uint64(cid)))
	}
	if why := g.sessionBusy(entity.EntityID(424242)); why != ReasonTradeOpen {
		t.Fatalf("a colony session may open beside it: %q", why)
	}
	if why := tradeWhy(t, g, cid); why != ReasonTradeOpen {
		t.Fatalf("TradeWhy while a session is open: %q", why)
	}
	g.Enqueue(command.Command{Kind: command.CloseTrade})
	g.Step(1)
	if _, ok := g.TradeView(); ok || g.sessionBusy(0) != "" {
		t.Fatal("CloseTrade left the session open")
	}
	g.trade.session = &tradeSession{negotiator: g.pawnIDs[1]}
	g.applyHostCommand(command.Command{Kind: command.TradeSettlement, Target: uint64(cid)})
	if settlementOf(g).Live(settleHost{g}) {
		t.Fatal("a settlement session opened beside a colony one")
	}
	g.trade.session = nil
}

// TestSettlementTradeNoTickWork: with no session open, stepping the game
// leaves the settlement state byte-equal.
func TestSettlementTradeNoTickWork(t *testing.T) {
	t.Parallel()
	g, cid, _, _ := settleGame(t, false)
	g.applyHostCommand(command.Command{Kind: command.TradeSettlement, Target: uint64(cid)})
	g.cmdCloseTrade()
	before := settleBytes(t, settlementOf(g).Export())
	if settlementOf(g).Export() == nil {
		t.Fatal("no market was rolled")
	}
	g.Step(600)
	if !bytes.Equal(before, settleBytes(t, settlementOf(g).Export())) {
		t.Fatal("ticking changed the settlement state")
	}
}

// TestSettlementTradeSaveRoundTrip: save, load and save give the same
// Settle bytes; a save without Settle loads and its first trade rolls a
// fresh stock; hostile Settle data is bounded on load.
func TestSettlementTradeSaveRoundTrip(t *testing.T) {
	t.Parallel()
	g, cid, tile, fi := settleGame(t, false)
	if g.exportWorld().Settle != nil {
		t.Fatal("a game with no trade saved markets")
	}
	plain := saveLoad(t, g)
	if plain.worldSt.settle != nil {
		t.Fatal("a save without Settle loaded markets")
	}
	plain.worldS().planet, plain.worldS().planetTried = testPlanet().Fork(), true
	plain.applyHostCommand(command.Command{Kind: command.TradeSettlement, Target: uint64(cid)})
	sv := plain.exportWorld().Settle
	want := trade.NewMarket(plain.sk.content.Items, plain.ng.factions[fi].Type, testPlanet().Seed, tile, fi, uint64(plain.clock.Now()))
	if sv == nil || len(sv.Markets) != 1 || !reflect.DeepEqual(sv.Markets[0], want) {
		t.Fatalf("first trade after load: %+v", sv)
	}

	g.applyHostCommand(command.Command{Kind: command.TradeSettlement, Target: uint64(cid)})
	a := settleBytes(t, g.exportWorld().Settle)
	h := saveLoad(t, g)
	b := settleBytes(t, h.exportWorld().Settle)
	c := settleBytes(t, saveLoad(t, h).exportWorld().Settle)
	if !bytes.Equal(a, b) || !bytes.Equal(b, c) {
		t.Fatal("a save round trip changed the Settle block")
	}

	ws := g.exportWorld()
	ws.Settle = &settlement.Save{Version: 1}
	for i := range 5000 {
		ws.Settle.Markets = append(ws.Settle.Markets, trade.Market{Tile: int32(i%4500 - 3), Stock: trade.Stock{Silver: -9, Lines: []trade.Line{{Kind: item.Steel, Qty: -4}}}})
	}
	g.importWorld(ws)
	got := g.exportWorld().Settle
	if len(got.Markets) != 4096 || got.Markets[0].Tile != 0 {
		t.Fatalf("%d markets from 5000 hostile ones, first tile %d", len(got.Markets), got.Markets[0].Tile)
	}
	for _, m := range got.Markets {
		if m.Stock.Silver < 0 || len(m.Stock.Lines) != 0 {
			t.Fatalf("hostile stock kept: %+v", m.Stock)
		}
	}
}

// TestSettlementTradeDestroy: a settlement destroyed in a raid leaves the
// game's own planet through planet.Unsettle (its territory version moves),
// its market goes, and a caravan arriving there reads "No settlement
// here.".
func TestSettlementTradeDestroy(t *testing.T) {
	t.Parallel()
	g, cid, tile, _ := settleGame(t, true)
	if len(g.pawnIDs) < 2 {
		t.Fatal("too few colonists")
	}
	p := g.worldPlanet()
	g.applyHostCommand(command.Command{Kind: command.TradeSettlement, Target: uint64(cid)})
	g.cmdCloseTrade()
	if settlementOf(g).Export() == nil {
		t.Fatal("no market")
	}
	v0 := planet.TerritoryVersion(p)
	g.Enqueue(command.Command{Kind: command.AttackSettlement, Target: uint64(cid)})
	g.Step(1)
	if len(g.maps) != 2 {
		t.Fatalf("no settlement map: %d maps", len(g.maps))
	}
	m := g.maps[1]
	fell(g, defenders(m)...)
	nextHours(g, 4)
	if !settleMeta(g, m).Defeated {
		t.Fatal("not destroyed")
	}
	if p.Settlement[tile] >= 0 || planet.TerritoryVersion(p) == v0 {
		t.Fatalf("settlement %d, territory version %d (was %d)", p.Settlement[tile], planet.TerritoryVersion(p), v0)
	}
	if sv := settlementOf(g).Export(); sv != nil {
		t.Fatalf("the market outlived its settlement: %+v", sv.Markets)
	}
	other := arriveAt(t, g, tile, g.pawnIDs[1])
	if why := tradeWhy(t, g, other); why != settlement.ReasonNoSettlement {
		t.Fatalf("TradeWhy %q", why)
	}
}

// tradeWhy is caravan cid's TradeWhy on the World tab (fails when it is
// not listed).
func tradeWhy(t *testing.T, g *Game, cid entity.EntityID) string {
	t.Helper()
	for _, cv := range g.WorldMap().Caravans {
		if cv.ID == uint64(cid) {
			return cv.TradeWhy
		}
	}
	t.Fatalf("caravan %d is not on the World tab", cid)
	return ""
}

// TestSettlementTradeWhyNoPlanetBuild: the World tab fills TradeWhy for
// every caravan without building the planet.
func TestSettlementTradeWhyNoPlanetBuild(t *testing.T) {
	t.Parallel()
	g, cid, _, _ := settleGame(t, false)
	w := g.worldS()
	w.planet, w.planetTried = nil, false
	found := false
	for _, cv := range g.WorldMap().Caravans {
		if cv.ID == uint64(cid) {
			found = true
			if cv.TradeWhy != "" {
				t.Fatalf("a visiting caravan can't trade: %q", cv.TradeWhy)
			}
		}
	}
	if !found || w.planet != nil || w.planetTried {
		t.Fatalf("found %v, planet built %v", found, w.planetTried)
	}
	fleetOf(g).At(cid).Back = true
	for _, cv := range g.WorldMap().Caravans {
		if cv.ID == uint64(cid) && cv.TradeWhy != settlement.ReasonNotVisiting {
			t.Fatalf("a caravan heading home: %q", cv.TradeWhy)
		}
	}
}
