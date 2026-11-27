// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package craft

import (
	"testing"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/ai/work"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/recipe"
	"rimworld-go/internal/domain/sim/entity"
)

// crafted is one Makers.Crafted call the fake saw.
type crafted struct {
	worker      entity.EntityID
	recipe      recipe.ID
	count, x, y int
}

// TestShopReportsEachFinishedCraft: a finished bill tells the host once,
// with the recipe, the worker, the count made and the bench's cell (the
// plugins' recipe_done); the work ticks before it tell nothing.
func TestShopReportsEachFinishedCraft(t *testing.T) {
	t.Parallel()
	f := newFake()
	c := New(f)
	f.pawns = []entity.EntityID{pawn}
	f.build(1, construction.Workbench, 4, 4)
	f.put(50, item.Stack{Kind: item.Wood, Qty: 10, X: 1, Y: 1})
	c.Bills.Set(1, []recipe.Bill{{Recipe: recipe.Club, Mode: recipe.DoCount, Target: 1}})
	j := runBill(t, f, c, work.Mask(0).Set(work.Craft, true), 100)
	if j.State != job.Complete {
		t.Fatalf("job %v", j.State)
	}
	want := crafted{worker: pawn, recipe: recipe.Club, count: 1, x: 4, y: 4}
	if len(f.crafted) != 1 || f.crafted[0] != want {
		t.Fatalf("crafted %+v, want [%+v]", f.crafted, want)
	}
}
