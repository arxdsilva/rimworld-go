// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"reflect"
	"testing"

	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/trade"
	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/command"
)

// A former World v2 block has caravans and no SettlementStocks field.
// Re-encoding a real saved game with that field absent exercises the gob
// and Load path used by saves created before settlement markets existed.
func legacyVisitingWorld(t *testing.T, g *Game) *Game {
	t.Helper()
	h := loadAsVersionWith(t, g, SaveVersion, func(s *saveState) {
		if s.World == nil || s.World.Version != worldSaveVersion {
			t.Fatalf("fixture has no v%d World block", worldSaveVersion)
		}
		s.World.SettlementStocks = nil
	})
	h.worldS().planet, h.worldS().planetTried = testPlanet().Fork(), true
	return h
}

func TestOldWorldV2VisitingMarketRestoredOnFirstCommandPass(t *testing.T) {
	g := worldGame(t)
	tile := nearDest(t, g)
	pawn := g.pawnIDs[0]
	cid := visitingMarket(t, g, tile, pawn)
	travelOf(g).Carry(cid, item.Stack{Kind: item.Silver, Qty: 700})
	want, _ := fleetOf(g).Market().At(tile)

	loaded := legacyVisitingWorld(t, g)
	if _, ok := fleetOf(loaded).Market().At(tile); ok {
		t.Fatal("old World v2 fixture unexpectedly contains a market")
	}
	preMigration := loaded.StateHash()
	if len(loaded.WorldMap().Markets) != 0 || loaded.StateHash() != preMigration {
		t.Fatal("reading the old World map performed a migration")
	}
	loaded.Step(1)
	g.Step(1)
	after, ok := fleetOf(loaded).Market().At(tile)
	if !ok || !reflect.DeepEqual(after, want) {
		t.Fatalf("first authoritative Step lost original arrival stock: got %+v want %+v", after, want)
	}
	if loaded.StateHash() != g.StateHash() {
		t.Fatal("seeded migration diverges from an uninterrupted visiting caravan")
	}

	buy := trade.EncodeOffer([]trade.Delta{{Kind: item.Cloth, Qty: 2}})
	cmd := command.Command{Kind: command.WorldCaravanTrade, Pawn: uint64(pawn), Target: cid, Name: buy}
	loaded.Enqueue(cmd)
	g.Enqueue(cmd)
	loaded.Step(1)
	g.Step(1)
	if loaded.StateHash() != g.StateHash() || cargoQty(travelOf(loaded).Goods(cid), item.Cloth) != 2 {
		t.Fatal("post-migration trade diverges from the original session")
	}
	saved, _ := saveLoadInfo(t, loaded)
	saved.worldS().planet, saved.worldS().planetTried = testPlanet().Fork(), true
	if saved.StateHash() != loaded.StateHash() {
		t.Fatal("migrated market and purchased cargo did not survive resave/load")
	}
	wantAfterTrade, _ := fleetOf(loaded).Market().At(tile)
	if check, ok := fleetOf(saved).Market().At(tile); !ok || !reflect.DeepEqual(check, wantAfterTrade) {
		t.Fatal("resaved market no longer matches purchased stock")
	}
}

func TestOldWorldV2MigrationRejectsUnavailableBases(t *testing.T) {
	g := worldGame(t)
	tile := nearDest(t, g)
	visitingMarket(t, g, tile, g.pawnIDs[0])
	cases := []struct {
		name   string
		change func(*Game)
	}{
		{"hostile", func(h *Game) { h.factionOf(h.settlementFaction(tile)).Relation = faction.Hostile }},
		{"defeated", func(h *Game) { h.factionOf(h.settlementFaction(tile)).Defeated = true }},
		{"expired", func(h *Game) {
			h.worldS().caravans[0].C.VisitUntil = uint64(h.clock.Now())
		}},
		{"destroyed", func(h *Game) {
			fa := h.factionOf(h.settlementFaction(tile))
			fa.Settlements = []int{}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := legacyVisitingWorld(t, g)
			tc.change(h)
			before := h.StateHash()
			if len(h.WorldMap().Markets) != 0 || h.StateHash() != before {
				t.Fatal("read-side migration modified an unavailable base")
			}
			h.Step(1)
			if _, ok := fleetOf(h).Market().At(tile); ok {
				t.Fatal("migration created an unavailable base's market")
			}
		})
	}
}

func TestRestoredMarketIsNotRestockedByMigration(t *testing.T) {
	g := worldGame(t)
	tile := nearDest(t, g)
	pawn := g.pawnIDs[0]
	cid := visitingMarket(t, g, tile, pawn)
	travelOf(g).Carry(cid, item.Stack{Kind: item.Wood, Qty: 3})
	g.Enqueue(command.Command{Kind: command.WorldCaravanTrade, Pawn: uint64(pawn),
		Target: cid, Name: trade.EncodeOffer([]trade.Delta{{Kind: item.Wood, Qty: -3}})})
	g.ApplyPending()
	want, _ := fleetOf(g).Market().At(tile)
	h, _ := saveLoadInfo(t, g)
	h.worldS().planet, h.worldS().planetTried = testPlanet().Fork(), true
	h.Step(1)
	after, _ := fleetOf(h).Market().At(tile)
	if !reflect.DeepEqual(after, want) || h.worldS().caravans[0].C.VisitUntil < uint64(clock.TicksPerDay) {
		t.Fatalf("existing market or visit timer changed during load: %+v", after)
	}
}
