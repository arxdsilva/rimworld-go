// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"reflect"
	"testing"

	"rimworld-go/internal/application/game/maps"
	"rimworld-go/internal/domain/economy/trade"
	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/sim/command"
)

func TestVanillaFactionCoreWorldRoles(t *testing.T) {
	if faction.Player != 0 || faction.Settlers != 1 || faction.Clans != 2 ||
		faction.Drifters != 3 || faction.Freebooters != 4 || faction.Swarm != 5 {
		t.Fatal("historic faction type IDs were renumbered")
	}
	g := buildSetup(t, setupFor(t, "broken_hull", nil))
	stepUntilLanded(t, g)
	types := []faction.Type{faction.Settlers, faction.Drifters, faction.Clans,
		faction.FierceClans, faction.SavageClans, faction.Freebooters}
	if len(g.ng.factions) != len(types) {
		t.Fatalf("Core world has %d factions, want %d", len(g.ng.factions), len(types))
	}
	visitors, raiders := 0, 0
	for i, f := range g.ng.factions {
		if f.ID != i || f.Type != types[i] || !f.Type.HasWorldSettlements() {
			t.Fatalf("faction %d has unexpected origin: %+v", i, f)
		}
		if f.CanVisit() {
			visitors++
		}
		if faction.Raider(f) {
			raiders++
		}
	}
	// World roles follow the seeded relation: a standing hostile faction
	// raids, and a neutral or friendly one may visit. Drifters, Warclans,
	// Bloodclans and Freebooters begin in the hostile natural band.
	hostile := 0
	for _, f := range g.ng.factions {
		if f.Relation == faction.Hostile {
			hostile++
		}
	}
	if raiders != hostile || visitors != len(types)-hostile || len(g.visitingFactions()) != visitors || raiders < 3 {
		t.Fatalf("initial eligible visitors=%d raiders=%d hostile=%d, visiting factions=%v",
			visitors, raiders, hostile, g.visitingFactions())
	}
	for _, ty := range []faction.Type{faction.Swarm, faction.AncientQuiet,
		faction.AncientHostile, faction.MechHive} {
		def := faction.DefOf(ty)
		f := faction.Faction{Type: ty, Relation: faction.Hostile}
		if !def.Hidden || ty.HasWorldSettlements() || f.CanVisit() || faction.Raider(f) {
			t.Fatalf("%s leaked into ordinary world factions", ty)
		}
	}
}

func TestVanillaFactionCoreWarIndependentOfPlayerGoodwill(t *testing.T) {
	g := buildSetup(t, setupFor(t, "broken_hull", nil))
	stepUntilLanded(t, g)
	a, b := &g.ng.factions[0], &g.ng.factions[1]
	if !a.AtWar(b.ID) || !b.AtWar(a.ID) || !maps.Enemies(*a, *b) ||
		!((raidMischief{g: g}).Rival(1, 2)) {
		t.Fatal("rough human faction should be at war with the civil faction")
	}
	a.Relation, b.Relation = faction.Neutral, faction.Friendly
	if !maps.Enemies(*a, *b) || !((raidMischief{g: g}).Rival(1, 2)) {
		t.Fatal("player goodwill changed an existing inter-faction war")
	}
	a.Enemies, b.Enemies = nil, nil
	a.Relation, b.Relation = faction.Hostile, faction.Hostile
	if maps.Enemies(*a, *b) || (raidMischief{g: g}).Rival(1, 2) {
		t.Fatal("shared hostility to player fabricated an inter-faction war")
	}
}

func TestVanillaFactionCoreGoodwillCountersSurviveSave(t *testing.T) {
	g := buildSetup(t, setupFor(t, "broken_hull", nil))
	stepUntilLanded(t, g)
	f := &g.ng.factions[0]
	f.Goodwill, f.Relation = 61, faction.Neutral
	f.TradeRemainder = 357
	for range 3 {
		g.factionsDay()
	}
	h, _ := saveLoadInfo(t, g)
	if !reflect.DeepEqual(g.ng.factions, h.ng.factions) || g.StateHash() != h.StateHash() {
		t.Fatal("goodwill and partial trade counters did not survive checkpoint")
	}
	for range 12 {
		g.factionsDay()
		h.factionsDay()
	}
	if g.StateHash() != h.StateHash() || !reflect.DeepEqual(g.ng.factions, h.ng.factions) ||
		g.ng.factions[0].Goodwill != 55 {
		t.Fatalf("goodwill replay mismatch: uninterrupted %+v reloaded %+v",
			g.ng.factions[0], h.ng.factions[0])
	}
}

func TestVanillaFactionCorePhysicalTradeEarnsGoodwill(t *testing.T) {
	g, c := caravanGame(t)
	neg := openCaravanTrade(t, g, c)
	tradeYard(t, g, neg, 300000, 10)
	f := g.factionByID(c.Faction)
	f.Goodwill, f.Relation = 0, faction.Neutral
	var offer []trade.Delta
	value := 0
	for _, ln := range c.Stock.Lines {
		unit := ln.Goods().Value(g.sk.content.Items)
		if unit <= 0 || ln.Qty <= 0 {
			continue
		}
		need := max(1, (500-value+unit-1)/unit)
		n := min(need, ln.Qty)
		offer = append(offer, trade.Delta{Kind: ln.Kind, Building: ln.Building, Qty: n})
		value += unit * n
		if value >= 500 {
			break
		}
	}
	if value < 500 {
		t.Fatalf("real caravan stock is worth only %d silver", value)
	}
	g.Enqueue(command.Command{Kind: command.ProposeTrade, Name: trade.EncodeOffer(offer)})
	g.ApplyPending()
	if view, ok := g.TradeView(); !ok || view.Reason != "" {
		t.Fatalf("physical purchase refused: %+v", view)
	}
	if f.Goodwill != value/500 || int(f.TradeRemainder) != value%500 {
		t.Fatalf("trading value %d yielded goodwill=%d remainder=%d",
			value, f.Goodwill, f.TradeRemainder)
	}
	g.Enqueue(command.Command{Kind: command.CloseTrade})
	g.ApplyPending()
	h, _ := saveLoadInfo(t, g)
	if g.StateHash() != h.StateHash() ||
		g.factionByID(c.Faction).TradeRemainder != h.factionByID(c.Faction).TradeRemainder {
		t.Fatal("physical trade relationship reward diverged after save")
	}
}
