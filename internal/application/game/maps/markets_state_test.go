// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package maps

import (
	"testing"

	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/trade"
)

func TestMarketsOwnTheirStockAndPhysicalLots(t *testing.T) {
	input := []SettlementStock{{
		Tile: 9, FactionID: 3, Stock: trade.Stock{
			Lines: []trade.Line{{Kind: item.Parka, Qty: 2}},
		},
		Preserved: []item.Stack{{Kind: item.Parka, Qty: 1, Quality: 4}},
	}}
	var markets Markets
	markets.Restore(input, false)
	input[0].Stock.Lines[0].Qty = 90
	input[0].Preserved[0].Quality = 1
	input[0].Tile = 15

	check := func() {
		t.Helper()
		got, ok := markets.At(9)
		if !ok || markets.Len() != 1 || got.Stock.Lines[0].Qty != 2 ||
			got.Preserved[0].Quality != 4 {
			t.Fatalf("market ownership was violated: %+v", got)
		}
	}
	check()

	first := markets.Snapshot()
	first[0].Stock.Lines[0].Qty = 17
	first[0].Preserved[0].Quality = 2
	check()

	read, ok := markets.At(9)
	if !ok {
		t.Fatal("market lookup failed")
	}
	read.Stock.Lines[0].Qty = 91
	read.Preserved[0].Quality = 0
	check()
}

func TestMarketsRestoreMarksOnlyLegacyVisitorsForRecovery(t *testing.T) {
	var markets Markets
	markets.Restore(nil, true)
	if !markets.RestorationPending() {
		t.Fatal("legacy caravan stock was not scheduled for reconstruction")
	}
	saved := []SettlementStock{{Tile: 5}}
	markets.Restore(saved, true)
	if markets.RestorationPending() || markets.Len() != 1 {
		t.Fatal("saved markets unexpectedly require reconstruction")
	}
	markets.Restore(nil, false)
	if markets.RestorationPending() || markets.Len() != 0 {
		t.Fatal("fresh world unexpectedly scheduled market reconstruction")
	}
}
