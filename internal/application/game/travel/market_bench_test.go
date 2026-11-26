// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package travel

import (
	"testing"

	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/trade"
)

func BenchmarkCaravanTradePreflight(b *testing.B) {
	s := &State{cargo: map[uint64]*Hold{
		7: {Items: []item.Stack{
			{Kind: item.Silver, Qty: 800},
			{Kind: item.Steel, Qty: 8},
			{Kind: item.Wood, Qty: 10},
			{Kind: item.Cloth, Qty: 15},
			{Kind: item.HerbalMedicine, Qty: 2},
			{Kind: item.Parka, Qty: 1, Quality: 4, Wear: 15},
			{Kind: item.Novel, Qty: 1, Book: 42, Quality: 3},
			{Kind: item.Components, Qty: 2},
		}},
	}}
	ops := s.Ops(nil)
	result := trade.Result{
		Sell:   []trade.Line{{Kind: item.Wood, Qty: 3}},
		Buy:    []trade.Line{{Kind: item.Components, Qty: 2}},
		Silver: -25,
	}
	bought := []item.Stack{{Kind: item.Components, Qty: 2}}
	b.ReportAllocs()
	var last TradePlan
	for i := 0; i < b.N; i++ {
		var why string
		last, why = ops.PlanTrade(7, 1, result, bought)
		if why != "" {
			b.Fatal(why)
		}
	}
	components := 0
	for _, st := range last.items {
		if st.Kind == item.Components {
			components += st.Qty
		}
	}
	if len(last.items) != 8 || len(last.sold) != 1 || components != 4 {
		b.Fatalf("preflight lost physical goods or failed to merge bought components: %+v", last)
	}
}
