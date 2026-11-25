// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"bytes"
	"os"

	"rimworld-go/internal/application/game/visit"
	"testing"

	"rimworld-go/internal/domain/pawn/health"
)

func TestSecondaryCaravanOpensIndependentTradeAndPromotesOnLoss(t *testing.T) {
	g := visitorGame(t)
	now := uint64(g.clock.Now())
	for i := 0; i < 2; i++ {
		if !g.caravanArrive(now) {
			t.Fatalf("independent caravan %d did not arrive", i+1)
		}
	}
	first, second := g.trade.caravan, g.trade.others[0]
	party, ok := g.party(&tradeSession{trader: second.Trader})
	if !ok || party.stock != &second.Stock || party.f.ID != second.Faction {
		t.Fatalf("second caravan's actual trade ledger was not selected: %+v", party)
	}
	if !g.traderOpen(second.Trader) {
		t.Fatal("second caravan is on the map but cannot trade")
	}
	p, _ := g.positions.Get(second.Trader)
	if g.traderAt(p.X, p.Y) != second.Trader {
		t.Fatal("second caravan's trader is absent from the public order target")
	}
	g.die(first.Trader, health.CauseInjury)
	g.Step(1)
	if g.trade.caravan != second || g.trade.retired != first {
		t.Fatalf("after primary trader death: active %p (want %p), retired %p (want %p), extras %d", g.trade.caravan, second, g.trade.retired, first, len(g.trade.others))
	}
	if party, ok = g.party(&tradeSession{trader: second.Trader}); !ok || party.stock != &second.Stock {
		t.Fatal("second caravan trade closed when first trader died")
	}
}

func TestLegacyVisitorSaveRebuildsNativeGroups(t *testing.T) {
	g, c := caravanGame(t)
	legacy := g.exportWorld()
	legacy.Version = 2
	legacy.VisitGroups = nil
	legacy.NextVisitGroup = 0
	for i := range legacy.Visitors {
		legacy.Visitors[i].GroupID = 0
	}
	g.importWorld(legacy)
	c.GroupID = 0
	rebuildVisitGroups(g)
	if len(g.worldSt.visitGroups) != 1 || g.worldSt.visitGroups[0].Leader != c.Trader ||
		c.GroupID != g.worldSt.visitGroups[0].ID || c.GroupID == 0 {
		t.Fatal("legacy caravan did not recover stable group membership")
	}
	for _, r := range g.worldSt.visitors {
		if r.GroupID != c.GroupID {
			t.Fatalf("legacy visitor %d not attached to caravan", r.ID)
		}
	}
}

// The frozen v12 fixture predates durable group IDs. Load must rebuild its
// existing human visitors and trading caravan without changing the fixture.
func TestV12VisitorCaravanMigratesWithAllMembers(t *testing.T) {
	raw, err := os.ReadFile("testdata/v12.sav")
	if err != nil {
		t.Fatal(err)
	}
	g, info, err := Load(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	if info.Version != 12 || g.worldSt == nil || len(g.worldSt.visitors) != 4 ||
		len(g.worldSt.visitGroups) != 1 || g.trade.caravan == nil {
		t.Fatalf("v12 group migration: version %d visitors %d groups %d caravan %v", info.Version, len(g.worldSt.visitors), len(g.worldSt.visitGroups), g.trade.caravan)
	}
	gr := g.worldSt.visitGroups[0]
	if gr.ID == 0 || gr.Kind != visit.KindCaravan || gr.Leader != g.trade.caravan.Trader ||
		g.trade.caravan.GroupID != gr.ID || len(gr.Members) < len(g.worldSt.visitors) {
		t.Fatalf("v12 caravan group and saved traders diverged: %+v", gr)
	}
	for _, guest := range g.worldSt.visitors {
		if guest.GroupID != gr.ID {
			t.Fatalf("saved visitor %d has group %d, wanted %d", guest.ID, guest.GroupID, gr.ID)
		}
	}
	h, _ := saveLoadInfo(t, g)
	if g.StateHash() != h.StateHash() || len(h.worldSt.visitGroups) != 1 ||
		h.trade.caravan.GroupID != gr.ID {
		t.Fatal("v12 migrated caravan failed a fresh checkpoint round-trip")
	}
}

// WorldView must publish the authoritative visiting parties, and callers
// must be unable to edit the saved group membership through a read model.
func TestWorldViewVisitorPartiesRemainDetachedFromSavedState(t *testing.T) {
	g := visitorGame(t)
	if !g.visitorsArrive(uint64(g.clock.Now())) {
		t.Fatal("visitor group did not arrive")
	}
	one := g.World().VisitorGroups
	if len(one) != 1 || one[0].ID == 0 || len(one[0].Members) < 1 || one[0].Kind != "visitor" {
		t.Fatalf("public world has no live visitor party: %+v", one)
	}
	want := one[0].Members[0]
	one[0].Members[0] = 0
	again := g.World().VisitorGroups
	if len(again) != 1 || again[0].Members[0] != want {
		t.Fatal("mutable public visitor roster leaked into the authoritative game")
	}
	h, _ := saveLoadInfo(t, g)
	if len(h.World().VisitorGroups) != 1 || h.World().VisitorGroups[0].ID != one[0].ID {
		t.Fatal("visitor party vanished from public WorldView after checkpoint")
	}
}
