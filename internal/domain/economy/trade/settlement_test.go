// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package trade

import (
	"reflect"
	"testing"

	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/faction"
)

func settlementQty(st Stock, kind item.Kind) int {
	for _, l := range st.Lines {
		if l.Kind == kind {
			return l.Qty
		}
	}
	return 0
}

func BenchmarkSettlementStockGeneration(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		stock := GenerateSettlement(faction.Industrial, uint64(i))
		if len(stock.Lines) == 0 {
			b.Fatal("empty generated stock")
		}
	}
}

// Stock creation may change its allocation strategy without changing seeded
// world inventories. These values were captured before the refactor.
func TestSettlementStockSeedGolden(t *testing.T) {
	cases := []struct {
		tech   faction.Tech
		silver int
		want   []Line
	}{
		{faction.Neolithic, 414, []Line{
			{Kind: item.Wood, Qty: 275}, {Kind: item.Potatoes, Qty: 100},
			{Kind: item.Club, Qty: 3}, {Kind: item.Corn, Qty: 97},
			{Kind: item.Leather, Qty: 79}, {Kind: item.HerbalMedicine, Qty: 33},
			{Kind: item.Spear, Qty: 2}, {Kind: item.Cloth, Qty: 90},
			{Kind: item.Wool, Qty: 32}, {Kind: item.SlateBlocks, Qty: 113},
		}},
		{faction.Industrial, 828, []Line{
			{Kind: item.Wood, Qty: 258}, {Kind: item.Rifle, Qty: 3},
			{Kind: item.Parka, Qty: 6}, {Kind: item.HerbalMedicine, Qty: 33},
			{Kind: item.Cloth, Qty: 259}, {Kind: item.Steel, Qty: 537},
			{Kind: item.Components, Qty: 57}, {Kind: item.IndustrialMedicine, Qty: 49},
			{Kind: item.SurvivalMeal, Qty: 46}, {Kind: item.Novel, Qty: 1},
			{Kind: item.Textbook, Qty: 3},
		}},
		{faction.Spacer, 828, []Line{
			{Kind: item.Rifle, Qty: 5}, {Kind: item.Cloth, Qty: 212},
			{Kind: item.Steel, Qty: 397}, {Kind: item.Gold, Qty: 92},
			{Kind: item.Components, Qty: 61}, {Kind: item.IndustrialMedicine, Qty: 49},
			{Kind: item.NanomedKit, Qty: 4}, {Kind: item.SurvivalMeal, Qty: 66},
			{Kind: item.Textbook, Qty: 2}, {Kind: item.Schematic, Qty: 2},
		}},
	}
	for _, tc := range cases {
		got := GenerateSettlement(tc.tech, 901)
		if got.Silver != tc.silver || !reflect.DeepEqual(got.Lines, tc.want) {
			t.Fatalf("tech %d seed 901 changed stock: silver %d lines %+v", tc.tech, got.Silver, got.Lines)
		}
	}
}

func TestSettlementStocksByTechnologyAndSeed(t *testing.T) {
	for _, tc := range []struct {
		tech faction.Tech
		kind TraderKind
		good item.Kind
	}{
		{faction.Neolithic, TribalSettlement, item.Wood},
		{faction.Industrial, IndustrialSettlement, item.Steel},
		{faction.Spacer, SpacerSettlement, item.Gold},
	} {
		a := GenerateSettlement(tc.tech, 901)
		b := GenerateSettlement(tc.tech, 901)
		if !reflect.DeepEqual(a, b) || a.Kind != tc.kind || a.Silver <= 0 || settlementQty(a, tc.good) <= 0 {
			t.Fatalf("tech %v inventory: %+v", tc.tech, a)
		}
		if !reflect.DeepEqual(a, GenerateSettlement(tc.tech, 901)) {
			t.Fatalf("tech %v rerolled without an epoch change", tc.tech)
		}
	}
	neolithic := GenerateSettlement(faction.Neolithic, 901)
	if settlementQty(neolithic, item.Rifle) != 0 || settlementQty(neolithic, item.Components) != 0 {
		t.Fatal("a tribal settlement stocked industrial weapons or components")
	}
}

func TestSettlementPriceImprovementAndWants(t *testing.T) {
	// Existing trader terms keep their unchanged default when ImproveCenti is zero.

	for _, k := range []TraderKind{TribalSettlement, IndustrialSettlement, SpacerSettlement} {
		if !Wants(item.Book{}, k, item.Wood) || Wants(item.Book{}, k, item.Silver) || Wants(item.Book{}, k, item.MinifiedBuilding) {
			t.Fatalf("settlement %d has inconsistent tradeable goods", k)
		}
		base := Terms{Social: 5, Goodwill: 20}
		settlement := Terms{Social: 5, Goodwill: 20, ImproveCenti: 200}
		if a, b := Price(item.Book{}, item.Cloth, k, Buy, base), Price(item.Book{}, item.Cloth, k, Buy, settlement); a <= b {
			t.Fatalf("base purchase not cheaper: %d -> %d", a, b)
		}
		if a, b := Price(item.Book{}, item.Cloth, k, Sell, base), Price(item.Book{}, item.Cloth, k, Sell, settlement); a >= b {
			t.Fatalf("base resale not more profitable: %d -> %d", a, b)
		}
	}
}
