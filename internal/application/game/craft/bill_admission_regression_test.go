// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package craft

import (
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/ai/work"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/recipe"
	"rimworld-go/internal/domain/sim/entity"
	"testing"
)

func TestRotDuringCooking(t *testing.T) {
	f := newFake()
	c := New(f)
	f.pawns = []entity.EntityID{pawn}
	f.build(1, construction.Stove, 4, 4)
	f.put(50, item.Stack{Kind: item.Berries, Qty: 20})
	c.Bills.Set(1, []recipe.Bill{{Recipe: recipe.SimpleMeal, Mode: recipe.DoCount, Target: 1}})
	if !c.Shop.Assign(pawn, work.Mask(0).Set(work.Cook, true)) {
		t.Fatal("assign")
	}
	c.Shop.Do(pawn, f.jobs[pawn])
	j := f.jobs[pawn]
	if j.Carry == 0 {
		t.Fatalf("did not fetch: %+v", j)
	}
	st := f.stacks[j.Carry]
	st.Rot = st.Shelf()
	f.stacks[j.Carry] = st
	c.Condition.Rot(250)
	if _, ok := f.stacks[j.Carry]; ok {
		t.Fatal("rot did not remove")
	}
	for i := 0; i < 1000 && f.jobs[pawn].State != job.Complete && f.jobs[pawn].State != job.Fail; i++ {
		c.Shop.Do(pawn, f.jobs[pawn])
	}
	if f.meals > 0 || c.Bills.Of(1)[0].Done != 0 {
		t.Fatalf("output without ingredient: meals=%d bill Done=%d state=%v", f.meals, c.Bills.Of(1)[0].Done, f.jobs[pawn].State)
	}
}

func TestOldBillWrongBench(t *testing.T) {
	f := newFake()
	c := New(f)
	f.pawns = []entity.EntityID{pawn}
	f.build(1, construction.Workbench, 4, 4)
	f.put(50, item.Stack{Kind: item.Steel, Qty: 40})
	f.put(51, item.Stack{Kind: item.Components, Qty: 4})
	f.skills[pawn] = 20
	c.Bills.Import(map[entity.EntityID][]recipe.Bill{1: {{Recipe: recipe.MakeProstheticLeg, Mode: recipe.Forever, Target: 1}}})
	c.Bills.UpgradeLegacy()
	d := c.Bills.book.DefOf(recipe.MakeProstheticLeg)
	if d.At(construction.Workbench) {
		t.Fatal("fixture: recipe still valid on workbench")
	}
	if c.Shop.Assign(pawn, work.Mask(0).Set(work.Craft, true)) {
		t.Fatalf("legacy bill ran on incompatible bench: %+v", f.jobs[pawn])
	}
}

func TestRetiredPegLegBillCannotProduce(t *testing.T) {
	f := newFake()
	c := New(f)
	f.pawns = []entity.EntityID{pawn}
	f.build(1, construction.Workbench, 4, 4)
	f.put(50, item.Stack{Kind: item.Wood, Qty: 20})
	c.Bills.Set(1, []recipe.Bill{{Recipe: recipe.MakePegLeg, Mode: recipe.DoCount, Target: 1}})
	if c.Shop.Assign(pawn, work.Mask(0).Set(work.Craft, true)) {
		t.Fatal("retired recipe assigned")
	}
	load := f.Carry(pawn, 50, 10)
	j := job.Record{Kind: job.Bill, State: job.Do, Dest: 1, Carry: load, WorkLeft: 1}
	c.Shop.Do(pawn, j)
	if f.jobs[pawn].State != job.Fail || f.inPlay(item.Wood) != 20 || c.Bills.Of(1)[0].Done != 0 || f.Stock(item.PegLeg) != 0 {
		t.Fatal("retired active bill consumed inputs or produced output")
	}
}
