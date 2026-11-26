// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package travel

import (
	"testing"

	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/trade"
)

func repeatCaravanDeals(t testing.TB, count int) int {
	t.Helper()
	hold := &Hold{Items: []item.Stack{{Kind: item.HerbalMedicine, Qty: count}}}
	state := &State{cargo: map[uint64]*Hold{7: hold}}
	ops := state.Ops(nil)
	res := trade.Result{
		Sell:   []trade.Line{{Kind: item.HerbalMedicine, Qty: 1}},
		Buy:    []trade.Line{{Kind: item.Cloth, Qty: 1}},
		Silver: 1,
	}
	incoming := []item.Stack{{Kind: item.Cloth, Qty: 1}}
	for i := 0; i < count; i++ {
		plan, why := ops.PlanTrade(7, 1, res, incoming)
		if why != "" {
			t.Fatalf("trade %d failed: %s", i, why)
		}
		// CommitTrade performs the same assignment but also notifies a
		// game Host; this fixture has no world map or host.
		hold.Items = plan.items
	}
	cloth, silver := 0, 0
	for _, st := range hold.Items {
		switch st.Kind {
		case item.Cloth:
			cloth += st.Qty
		case item.Silver:
			silver += st.Qty
		case item.HerbalMedicine:
			t.Fatalf("medicine remained after %d completed sales: %+v", count, st)
		default:
			t.Fatalf("unexpected trade item: %+v", st)
		}
	}
	if cloth != count || silver != count {
		t.Fatalf("physical goods or silver diverged: cloth %d, silver %d, trades %d", cloth, silver, count)
	}
	return len(hold.Items)
}

func TestRepeatedCaravanTradesKeepCargoCompact(t *testing.T) {
	if got := repeatCaravanDeals(t, 40); got != 2 {
		t.Fatalf("40 trades left %d cargo stacks for just two homogeneous goods", got)
	}
}

func TestCaravanTradePreservesDistinctPhysicalStacks(t *testing.T) {
	hold := &Hold{Items: []item.Stack{
		{Kind: item.Silver, Qty: 5, Forbidden: true},
		{Kind: item.Silver, Qty: 3},
		{Kind: item.Cloth, Qty: 2, Quality: 3},
		{Kind: item.Cloth, Qty: 4, Quality: 4},
	}}
	state := &State{cargo: map[uint64]*Hold{7: hold}}
	plan, why := state.Ops(nil).PlanTrade(7, 1,
		trade.Result{Buy: []trade.Line{{Kind: item.Cloth, Qty: 2}}, Silver: 2},
		[]item.Stack{{Kind: item.Cloth, Qty: 2, Quality: 4}})
	if why != "" {
		t.Fatalf("valid trade rejected: %s", why)
	}
	hold.Items = plan.items
	var forbidden, normalSilver, original, bought int
	for _, st := range hold.Items {
		switch {
		case st.Kind == item.Silver && st.Forbidden:
			forbidden += st.Qty
		case st.Kind == item.Silver:
			normalSilver += st.Qty
		case st.Kind == item.Cloth && st.Quality == 3:
			original += st.Qty
		case st.Kind == item.Cloth && st.Quality == 4:
			bought += st.Qty
		default:
			t.Fatalf("unexpected physical stack %+v", st)
		}
	}
	if len(hold.Items) != 4 || forbidden != 5 || normalSilver != 5 || original != 2 || bought != 6 {
		t.Fatalf("a different quality or forbidden flag was merged: %+v", hold.Items)
	}
}

func BenchmarkCaravanRepeatedTrades(b *testing.B) {
	// A benchmark operation is 40 consecutive trades, from a fresh hold.
	// This tests the growth of real saved cargo after successive credits.
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		repeatCaravanDeals(b, 40)
	}
}
