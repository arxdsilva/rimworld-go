// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"testing"

	"rimworld-go/internal/application/game/maps"
	"rimworld-go/internal/application/game/tabview"
	"rimworld-go/internal/application/game/tradeview"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/trade"
	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/world/caravan"
)

func BenchmarkWorldMarketView(b *testing.B) {
	g := NewColony(Config{Seed: 2, Width: 40, Height: 40})
	g.ng.factions = []faction.Faction{{
		ID: 1, Type: faction.Settlers, Relation: faction.Neutral,
		Settlements: []int{7},
	}}
	pawn := g.pawnIDs[0]
	now := uint64(g.clock.Now())
	w := g.worldS()
	w.caravans = []maps.Caravan{{C: caravan.Caravan{
		ID: 777, Tile: 7, State: caravan.Visiting,
		Members: []uint64{uint64(pawn)}, VisitUntil: now + clock.TicksPerDay,
	}}}
	w.markets.Restore([]maps.SettlementStock{{
		Tile: 7, FactionID: 1, RestockAt: now + 30*clock.TicksPerDay,
		Stock: trade.GenerateSettlement(faction.Industrial, 901),
	}}, false)
	travelOf(g).Carry(777, item.Stack{Kind: item.Silver, Qty: 500})
	travelOf(g).Carry(777, item.Stack{Kind: item.Plasteel, Qty: 2})
	b.ReportAllocs()
	b.ResetTimer()
	var view tabview.MarketView
	for i := 0; i < b.N; i++ {
		var ok bool
		view, ok = tradeview.Visiting(uint64(g.clock.Now()), &w.caravans[0], marketWorldSource{g})
		if !ok {
			b.Fatal("market became unavailable")
		}
	}
	if len(view.Rows) < 2 {
		b.Fatal("market view lost goods")
	}
}
