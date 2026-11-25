// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"testing"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/ai/work"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/recipe"
	"rimworld-go/internal/domain/gameplay"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

func TestFloorRecipeSwapLifecycle(t *testing.T) {
	for _, cancel := range []bool{true, false} {
		g := New(Config{Seed: 1, Width: 20, Height: 12})
		bareGround(g)
		bid := buildNow(t, g, construction.Floor, item.Wood, construction.South, 5, 4)
		p, _ := g.SpawnNamedPawn(2, 2, "Ada")
		onlyWork(g, p, work.Construct, work.Haul)
		g.SpawnStack(item.Steel, 3, 3, 4)
		silver := 12
		if cancel {
			silver = 6
		}
		g.SpawnStack(item.Silver, silver, 4, 3)
		applyNow(g, commandBuild(construction.SterileTile, item.Steel, 5, 4))
		fid, fr := swapFrameOn(t, g, 5, 4, construction.LayerFloor)
		if fr.Swaps != bid || !fr.SwapNewRecipe || fr.Need != 3 || fr.ExtraNeed(0) != 12 {
			t.Fatalf("new recipe not charged %+v", fr)
		}
		for i := 0; i < 10000; i++ {
			g.SetNeeds(p, needs.Full, needs.Full)
			g.Step(1)
			fr, ok := g.frames.Get(fid)
			if cancel && ok && fr.ExtraGot[0] == silver {
				break
			}
			if !cancel && !ok {
				break
			}
		}
		if cancel {
			fr, ok := g.frames.Get(fid)
			if !ok || fr.Wood != 3 || fr.ExtraGot[0] != silver || fr.Ready() {
				t.Fatalf("partial delivery %+v", fr)
			}
			g = saveLoad(t, g)
			h, ok := g.frames.Get(fid)
			if !ok || h != fr {
				t.Fatalf("partial save changed %+v", h)
			}
			g.cancelFrame(fid)
			b, _ := g.bldgs.Get(bid)
			if b.Kind != construction.Floor || inPlay(g, item.Silver) != silver || inPlay(g, item.Steel) != 3 {
				t.Fatalf("cancel lost materials or old floor: %+v steel=%d silver=%d", b, inPlay(g, item.Steel), inPlay(g, item.Silver))
			}
			continue
		}
		b, ok := g.bldgs.Get(bid)
		if !ok || b.Kind != construction.SterileTile || b.MaterialBasis != (item.MaterialBasis{}) || inPlay(g, item.Silver) != 0 || inPlay(g, item.Steel) != 0 {
			t.Fatalf("paid recipe not completed: %+v steel=%d silver=%d", b, inPlay(g, item.Steel), inPlay(g, item.Silver))
		}
		g = saveLoad(t, g)
		// Replacing the paid sterile tile with a wood floor must refund only
		// its old recipe, then not retain its silver provenance on the wood.
		g.SpawnStack(item.Wood, construction.CostFor(construction.Floor, item.Wood), 2, 3)
		applyNow(g, commandBuild(construction.Floor, item.Wood, 5, 4))
		_, fr = swapFrameOn(t, g, 5, 4, construction.LayerFloor)
		if !fr.SwapNewRecipe || fr.ExtraNeed(0) != 0 {
			t.Fatal("reverse recipe")
		}
		for i := 0; i < 10000; i++ {
			g.SetNeeds(p, needs.Full, needs.Full)
			g.Step(1)
			b, _ = g.bldgs.Get(bid)
			if b.Kind == construction.Floor {
				break
			}
		}
		if b.Kind != construction.Floor || inPlay(g, item.Silver) != 6 || inPlay(g, item.Steel) != 1 {
			t.Fatalf("old recipe refund incorrect %+v silver=%d steel=%d", b, inPlay(g, item.Silver), inPlay(g, item.Steel))
		}
		g.removeBuilding(bid)
		if inPlay(g, item.Silver) != 6 {
			t.Fatal("wood floor refunded sterile silver again")
		}
	}
}

func activeDuplicatePackBill(t *testing.T) (*Game, entity.EntityID, entity.EntityID, uint64) {
	t.Helper()
	g, ada, bench := clubGame(t)
	senseOf(g).Rules = senseOf(g).Rules.With(gameplay.HaulAllIngredients, true).With(gameplay.StackToStockpile, false)
	g.SpawnStack(item.Wood, 2, 9, 6)
	g.SpawnStack(item.Wood, 1, 11, 8)
	g.SpawnStack(item.Wood, 2, 2, 8)
	applyNow(g, command.Command{Kind: command.AddBill, X: 5, Y: 2, Recipe: uint8(recipe.Club), Mode: uint8(recipe.DoCount), Count: 1})
	applyNow(g, command.Command{Kind: command.ConfigureBill, X: 5, Y: 2, Index: 1, Store: uint8(recipe.DropOnFloor)})
	g.setDraft(ada, true)
	applyNow(g, command.Command{Kind: command.Prioritize, Pawn: uint64(ada), Order: command.OrderDoBill, X: 5, Y: 2, Index: 1})
	selected := g.craft.Bills.Of(bench)[1].Identity
	for i := 0; i < 1500; i++ {
		g.SetNeeds(ada, needs.Full, needs.Full)
		g.Step(1)
		j, _ := g.jobs.Get(ada)
		if j.Kind == job.PackIngredients && j.RouteI > 0 && j.RouteI < j.Count && senseOf(g).Gear.Has(ada) {
			if j.BillIdentity != selected {
				t.Fatalf("wrong active identity %+v", j)
			}
			return g, ada, bench, selected
		}
	}
	t.Fatal("native gathering never stowed a partial load")
	return nil, 0, 0, 0
}

func TestActivePackRowEditAfterLoad(t *testing.T) {
	for _, remove := range []bool{true, false} {
		g, ada, bench, selected := activeDuplicatePackBill(t)
		g = saveLoad(t, g)
		j, _ := g.jobs.Get(ada)
		if j.BillIdentity != selected || !senseOf(g).Gear.Has(ada) {
			t.Fatal("save lost active row or pack")
		}
		applyNow(g, command.Command{Kind: command.ReorderBill, X: 5, Y: 2, Index: 1, Count: -1})
		j, _ = g.jobs.Get(ada)
		if j.BillIndex != 0 || j.BillIdentity != selected {
			t.Fatalf("active row switched %+v", j)
		}
		if remove {
			applyNow(g, command.Command{Kind: command.RemoveBill, X: 5, Y: 2, Index: 0})
			j, _ = g.jobs.Get(ada)
			if j.State != job.Fail || j.Fail != job.TargetGone {
				t.Fatalf("deleted row still active %+v", j)
			}
			for i := 0; i < 60; i++ {
				g.SetNeeds(ada, needs.Full, needs.Full)
				g.Step(1)
			}
			if rows := g.craft.Bills.Of(bench); len(rows) != 1 || rows[0].Identity == selected || rows[0].Done != 0 || inPlay(g, item.Wood) != 5 || inPlay(g, item.Club) != 0 {
				t.Fatalf("removed row substituted or lost goods: %+v wood=%d club=%d", rows, inPlay(g, item.Wood), inPlay(g, item.Club))
			}
			if _, held := g.reserve.EntityOwner(bench); held {
				t.Fatal("removed bill retained bench claim")
			}
			continue
		}
		h := saveLoad(t, g)
		for i := 0; i < 6000; i++ {
			for _, scene := range []*Game{g, h} {
				scene.SetNeeds(ada, needs.Full, needs.Full)
				scene.Step(1)
			}
			if i%100 == 0 && g.StateHash() != h.StateHash() {
				t.Fatalf("active row replay diverged tick %d", i)
			}
			rows := g.craft.Bills.Of(bench)
			if rows[0].Done+rows[1].Done > 0 {
				if rows[0].Identity != selected || rows[0].Done != 1 || rows[0].Store != recipe.DropOnFloor || rows[1].Done != 0 {
					t.Fatalf("different row finished %+v", rows)
				}
				if h.StateHash() != g.StateHash() {
					t.Fatal("completion replay diverged")
				}
				break
			}
			if i == 5999 {
				t.Fatal("selected row never completed")
			}
		}
	}
}

func TestPeerEligibilityAndRetryAfterLoad(t *testing.T) {
	g := New(Config{Seed: 21, Width: 30, Height: 20})
	bareGround(g)
	donor, _ := g.SpawnPawn(10, 10)
	eater, _ := g.SpawnPawn(11, 10)
	meal := stack(t, g, item.Meal, 2, 10, 10)
	packed(t, g, donor, meal, 2)
	g.setDraft(donor, true)
	g.Step(1)
	if g.seekFood(eater) || g.noFood[eater] != 0 {
		t.Fatal("speculative miss set ordinary retry")
	}
	h := saveLoad(t, g)
	for _, scene := range []*Game{g, h} {
		scene.setDraft(donor, false)
		if !scene.seekFood(eater) {
			t.Fatal("newly eligible donor ignored")
		}
		scene.assignEat(eater)
		j, _ := scene.jobs.Get(eater)
		if j.Kind != job.EatFromPeerPack || j.Target != donor {
			t.Fatalf("wrong peer assignment %+v", j)
		}
		scene.setDraft(donor, true)
		doEatFromPeerPack(scene, eater, j)
		if scene.noFood[eater] != 0 {
			t.Fatal("failed peer transfer set ordinary retry")
		}
		scene.SpawnStack(item.Meal, 1, 12, 10)
		if !scene.seekFood(eater) {
			t.Fatal("new meal suppressed")
		}
	}
	if g.StateHash() != h.StateHash() {
		t.Fatal("peer eligibility replay diverged")
	}
	// Historical ordinary retry remains authoritative and survives saving.
	g.noFood[eater] = uint64(g.clock.Now()) + foodRetry
	h = saveLoad(t, g)
	if h.noFood[eater] != g.noFood[eater] || h.seekFood(eater) {
		t.Fatal("ordinary retry lost on load")
	}
	h.clock.Advance(foodRetry)
	if !h.seekFood(eater) {
		t.Fatal("ordinary retry never expired")
	}
}

func TestPeerMissDoesNotSuppressPaste(t *testing.T) {
	g, _, _, eater := pasteKitchen(t, 0)
	donor, _ := g.SpawnPawn(10, 10)
	meal := stack(t, g, item.Meal, 2, 10, 10)
	packed(t, g, donor, meal, 2)
	g.setDraft(donor, true)
	if g.seekFood(eater) {
		t.Fatal("unexpected food")
	}
	g.SpawnStack(item.Potatoes, 75, 9, 4)
	if !g.haul.store.Paste.Ready(haulHost{g}, haulHost{g}) || !g.seekFood(eater) {
		t.Fatal("newly ready paste suppressed")
	}
}

func TestSuspendedBillRowEditAfterLoad(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		g, id := tidyGame(t)
		dirty(g, [2]int{7, 4})
		bench := buildNow(t, g, construction.Workbench, item.Wood, construction.South, 5, 4)
		for range 2 {
			applyNow(g, command.Command{Kind: command.AddBill, X: 5, Y: 4, Recipe: uint8(recipe.Club), Mode: uint8(recipe.DoCount), Count: 1})
		}
		rows := g.craft.Bills.Of(bench)
		if len(rows) != 2 || rows[1].Identity == 0 {
			t.Fatalf("fixture rows: %+v", rows)
		}
		load := stack(t, g, item.Wood, 5, 3, 6)
		st, _ := g.stacks.Get(load)
		st.Carrier = id
		g.stacks.Set(load, st)
		j := billRecord(bench, load)
		j.BillIndex, j.BillIdentity = 1, rows[1].Identity
		if legacy {
			j.BillIdentity = 0
		}
		if !senseOf(g).Tidy.TidyBench(id, bench, j) {
			t.Fatal("fixture: no suspended bill")
		}
		g = saveLoad(t, g)
		applyNow(g, command.Command{Kind: command.ReorderBill, X: 5, Y: 4, Index: 1, Count: -1})
		resume, ok := senseOf(g).Resume[id]
		if !ok || resume.BillIndex != 0 || resume.BillIdentity != j.BillIdentity {
			t.Fatalf("legacy=%v wrong suspended bill %+v", legacy, resume)
		}
		applyNow(g, command.Command{Kind: command.RemoveBill, X: 5, Y: 4, Index: 0})
		if _, ok := senseOf(g).Resume[id]; ok {
			t.Fatal("removed bill still waiting to resume")
		}
		if _, held := g.reserve.EntityOwner(bench); held {
			t.Fatal("removed suspended bill retained bench claim")
		}
		if st, ok := g.stacks.Get(load); ok && st.Carrier != 0 {
			t.Fatal("removed suspended bill still carrying ingredient")
		}
		if inPlay(g, item.Wood) != 5 || inPlay(g, item.Club) != 0 || g.craft.Bills.Of(bench)[0].Done != 0 {
			t.Fatal("removed suspended bill changed physical ingredients or wrong row")
		}
	}
}
