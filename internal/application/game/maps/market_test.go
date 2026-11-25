// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package maps

import (
	"testing"

	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/trade"
	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/world/planet"
)

type marketDestroyHost struct {
	Host
	fs   []faction.Faction
	p    *planet.Planet
	lost []int
}

func (h *marketDestroyHost) Factions() []faction.Faction { return h.fs }
func (h *marketDestroyHost) Planet() *planet.Planet      { return h.p }
func (h *marketDestroyHost) SettlementLost(_ int, tile int, _ bool) {
	h.lost = append(h.lost, tile)
}

func BenchmarkMarketLookup(b *testing.B) {
	list := make([]SettlementStock, 256)
	for i := range list {
		list[i] = SettlementStock{Tile: i * 3, Stock: trade.GenerateSettlement(faction.Industrial, uint64(i))}
	}
	var caravans []Caravan
	var atlas Atlas
	var markets Markets
	markets.Restore(list, false)
	fleet := Of(&caravans, &atlas, nil).WithMarkets(&markets)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		m, found := fleet.Market().At((i % 256) * 3)
		if !found || m.Tile < 0 || len(m.Stock.Lines) == 0 {
			b.Fatal("market lookup failed")
		}
	}
}

func BenchmarkMarketIndex(b *testing.B) {
	list := make([]SettlementStock, 256)
	for i := range list {
		list[i].Tile = i * 3
	}
	var markets Markets
	markets.Restore(list, false)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		index, found := markets.index((i % 256) * 3)
		if !found || list[index].Tile < 0 {
			b.Fatal("authoritative market missing")
		}
	}
}

func TestPreserveSaleCoalescesIdenticalStacks(t *testing.T) {
	var stock SettlementStock
	for i := 0; i < 150; i++ {
		preserveSale(&stock, item.Stack{Kind: item.Wood, Qty: 1, X: i})
	}
	if len(stock.Preserved) != 1 || stock.Preserved[0].Qty != 150 {
		t.Fatalf("same goods fragmented the persistent market: %+v", stock.Preserved)
	}
	// Quality, book identity and incubation must stay as separate lots.
	preserveSale(&stock, item.Stack{Kind: item.Wood, Qty: 2, Quality: 3})
	preserveSale(&stock, item.Stack{Kind: item.Wood, Qty: 1, Quality: 4})
	if len(stock.Preserved) != 3 || stock.Preserved[1].Qty != 2 {
		t.Fatalf("distinct item attributes were merged: %+v", stock.Preserved)
	}
}

func TestMarketClearedOnlyBySettlementDestruction(t *testing.T) {
	h := &marketDestroyHost{
		fs: []faction.Faction{{ID: 1, Name: "Settlement", Type: faction.Settlers, Settlements: []int{7, 9}}},
		p:  &planet.Planet{Settlement: make([]int32, 12)},
	}
	h.p.Settlement[7], h.p.Settlement[9] = 0, 1
	stock := []SettlementStock{
		{Tile: 7, FactionID: 1, Stock: trade.Stock{Silver: 100}},
		{Tile: 9, FactionID: 1, Stock: trade.Stock{Silver: 200}},
	}
	var list []Caravan
	var atlas Atlas
	var markets Markets
	markets.Restore(stock, false)
	f := Of(&list, &atlas, h).WithMarkets(&markets)
	f.Life().destroy(0, 7)
	stock = markets.Snapshot()
	if len(stock) != 1 || stock[0].Tile != 9 || stock[0].Stock.Silver != 200 ||
		h.p.Settlement[7] != -1 || len(h.fs[0].Settlements) != 1 || h.fs[0].Settlements[0] != 9 {
		t.Fatalf("destroy did not remove only the defeated settlement's market: %+v", stock)
	}
	f.Life().destroy(0, 9)
	stock = markets.Snapshot()
	if len(stock) != 0 || !h.fs[0].Defeated || len(h.lost) != 2 {
		t.Fatalf("last base kept a trade inventory: %+v, lost %v", stock, h.lost)
	}
}
