// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"testing"

	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/trade"
)

func TestPurchasedGoodsMergeWithOrdinaryStorage(t *testing.T) {
	g := NewColony(Config{Seed: 2, Width: 40, Height: 40})
	x, y := g.home()
	g.dropStackOf(item.Stack{Kind: item.Cloth, Qty: 3}, x, y)
	plan, why := travelOf(g).PlanTrade(7, 1, trade.Result{
		Buy: []trade.Line{{Kind: item.Cloth, Qty: 2}},
	}, []item.Stack{{Kind: item.Cloth, Qty: 2}})
	if why != "" {
		t.Fatal(why)
	}
	travelOf(g).CommitTrade(7, plan)
	bought := travelOf(g).Goods(7)
	if len(bought) != 1 || bought[0].Kind != item.Cloth || bought[0].Qty != 2 {
		t.Fatalf("purchased cargo changed: %+v", bought)
	}
	g.dropStackOf(bought[0], x, y)
	stacks, count := 0, 0
	for _, id := range g.itemIDs {
		st, ok := g.stacks.Get(id)
		if ok && st.Kind == item.Cloth {
			stacks++
			count += st.Qty
		}
	}
	if stacks != 1 || count != 5 {
		t.Fatalf("identical purchased and ordinary cloth occupied %d stacks (%d pieces)", stacks, count)
	}
}

func TestReturningCaravanPreservesBookAndCrateIdentities(t *testing.T) {
	g := NewColony(Config{Seed: 2, Width: 40, Height: 40})
	x, y := g.home()
	samples := []item.Stack{
		{Kind: item.Novel, Qty: 1, Book: 1234567, Quality: 4},
		{Kind: item.MinifiedBuilding, Qty: 1, Building: uint8(construction.Urn), Stuff: item.Wood},
		{Kind: item.FertileEgg, Qty: 1, Animal: 3, Tame: true, Hatch: 1200},
	}
	for _, st := range samples {
		g.dropStackOf(st, x, y)
		found := false
		for _, id := range g.itemIDs {
			got := g.stacks.Ref(id)
			if got == nil || got.Kind != st.Kind {
				continue
			}
			found = true
			if got.Book != st.Book || got.Building != st.Building ||
				got.Hatch != st.Hatch || got.Tame != st.Tame ||
				got.Animal != st.Animal || got.Stuff != st.Stuff ||
				got.Quality != st.Quality {
				t.Fatalf("returning %v lost physical attributes: got %+v, want %+v", st.Kind, *got, st)
			}
		}
		if !found {
			t.Fatalf("returning %v was missing from the map", st.Kind)
		}
	}
}
