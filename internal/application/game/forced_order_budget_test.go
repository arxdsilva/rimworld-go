// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"rimworld-go/internal/domain/ai/job"
	"testing"
)

func TestForcedOrderBudgetRefusalPreservesClaim(t *testing.T) {
	g := New(Config{Seed: 15, Width: 150, Height: 20})
	bareGround(g)
	id, err := g.SpawnNamedPawn(2, 2, "Ada")
	if err != nil {
		t.Fatal(err)
	}
	old := job.Record{Kind: job.Move, State: job.Goto, TX: 4, TY: 2, Forced: true}
	g.jobs.Set(id, old)
	if !g.reserve.TryTile(id, 4, 2) {
		t.Fatal("old claim")
	}
	for range longSearchBudget {
		if !g.regs.budget.Take(uint64(g.clock.Now()), longSearchBudget) {
			t.Fatal("budget fixture")
		}
	}
	if (nativeOrderHost{haulHost{g}}).StartForced(id, job.Record{Kind: job.Move, TX: 140, TY: 2, Forced: true}) {
		t.Fatal("budget should reject")
	}
	got, _ := g.jobs.Get(id)
	owner, held := g.reserve.TileOwner(4, 2)
	if got.Kind != old.Kind || got.State != old.State || got.TX != old.TX || !held || owner != id {
		t.Fatalf("late refusal destroyed old job or claim: %+v owner=%d held=%v", got, owner, held)
	}
}
