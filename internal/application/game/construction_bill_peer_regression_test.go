// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/ai/work"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/recipe"
	"rimworld-go/internal/domain/gameplay"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/sim/command"
	"testing"
)

// Real native construction/haul/work/deconstruction path, with no silver supplied.
func TestFloorSwapConservesSecondaryMaterials(t *testing.T) {
	g := New(Config{Seed: 1, Width: 20, Height: 12})
	bareGround(g)
	bid := buildNow(t, g, construction.Floor, item.Wood, construction.South, 5, 4)
	p, _ := g.SpawnNamedPawn(2, 2, "Ada")
	for k := work.Type(0); k < work.NumTypes; k++ {
		g.setPriority(p, k, 0)
	}
	g.setPriority(p, work.Construct, 1)
	g.setPriority(p, work.Haul, 1)
	g.SpawnStack(item.Steel, 3, 3, 4)
	if g.stockOf(item.Silver) != 0 {
		t.Fatal("fixture already has silver")
	}
	applyNow(g, command.Command{Kind: command.DesignateBuild, X: 5, Y: 4, Build: uint8(construction.SterileTile), Stuff: uint8(item.Steel)})
	fid, fr := swapFrameOn(t, g, 5, 4, construction.LayerFloor)
	t.Logf("swap frame=%d primary=%d secondary=%d", fid, fr.Need, fr.ExtraNeed(0))
	for i := 0; i < 10000; i++ {
		g.SetNeeds(p, needs.Full, needs.Full)
		g.Step(1)
		b, _ := g.bldgs.Get(bid)
		if b.Kind == construction.SterileTile {
			g.removeBuilding(bid)
			t.Fatalf("constructed sterile tile with no silver supplied; deconstruction creates %d silver; basis %+v", g.stockOf(item.Silver), b.MaterialBasis)
		}
	}
	if got := g.stockOf(item.Silver); got != 0 {
		t.Fatalf("created silver %d", got)
	}
}

// Both rows have same recipe but different stable identities.
func TestActiveSelectedPackBillKeepsIdentity(t *testing.T) {
	g, ada, bench := clubGame(t)
	senseOf(g).Rules = senseOf(g).Rules.With(gameplay.HaulAllIngredients, true).With(gameplay.StackToStockpile, false)
	g.SpawnStack(item.Wood, 2, 9, 6)
	g.SpawnStack(item.Wood, 1, 11, 8)
	g.SpawnStack(item.Wood, 2, 2, 8)
	applyNow(g, command.Command{Kind: command.AddBill, X: 5, Y: 2, Recipe: uint8(recipe.Club), Mode: uint8(recipe.DoCount), Count: 1})
	g.setDraft(ada, true)
	applyNow(g, command.Command{Kind: command.Prioritize, Pawn: uint64(ada), Order: command.OrderDoBill, X: 5, Y: 2, Index: 1})
	j, _ := g.jobs.Get(ada)
	if j.Kind != job.PackIngredients || j.BillIndex != 1 || !j.Active() {
		t.Fatalf("fixture: expected selected pack gathering, %+v", j)
	}
	selected := g.craft.Bills.Of(bench)[1].Identity
	applyNow(g, command.Command{Kind: command.ReorderBill, X: 5, Y: 2, Index: 1, Count: -1})
	rows := g.craft.Bills.Of(bench)
	j, _ = g.jobs.Get(ada)
	t.Logf("selected=%d row0=%d row1=%d current bill index=%d", selected, rows[0].Identity, rows[1].Identity, j.BillIndex)
	if rows[0].Identity != selected {
		t.Fatal("fixture reorder failed")
	}
	for i := 0; i < 6000; i++ {
		g.SetNeeds(ada, needs.Full, needs.Full)
		g.Step(1)
		rows = g.craft.Bills.Of(bench)
		if rows[0].Done+rows[1].Done > 0 {
			t.Logf("completion selected=%d row0=%+v row1=%+v", selected, rows[0], rows[1])
			if rows[0].Done != 1 || rows[1].Done != 0 {
				t.Fatal("completed different bill from explicitly selected row")
			}
			return
		}
	}
	t.Fatal("selected bill never completed")
}

// A failed speculative peer-only admission cannot hide newly available loose food.
func TestPeerFailureDoesNotHideNewFood(t *testing.T) {
	g := New(Config{Seed: 21, Width: 30, Height: 20})
	bareGround(g)
	donor, _ := g.SpawnPawn(10, 10)
	eater, _ := g.SpawnPawn(11, 10)
	meal := stack(t, g, item.Meal, 2, 10, 10)
	if !senseOf(g).Gear.Stow(donor, meal, 2) {
		t.Fatal("stow failed")
	}
	g.setDraft(donor, true)
	if g.seekFood(eater) {
		t.Fatal("drafted donor eligible")
	}
	g.SpawnStack(item.Meal, 1, 12, 10)
	if !g.hasEdible(false) {
		t.Fatal("new loose meal is not available")
	}
	if !g.seekFood(eater) {
		t.Fatalf("new reachable food hidden by peer search cooldown now=%d retry=%d", g.clock.Now(), g.noFood[eater])
	}
}
