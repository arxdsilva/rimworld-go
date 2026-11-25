// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"fmt"
	"reflect"
	"testing"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/recipe"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

func TestNativeOrderSelectedBill(t *testing.T) {
	g, ada, bench := clubGame(t)
	freeTime(g)
	g.setDraft(ada, true)
	applyNow(g, command.Command{Kind: command.AddBill, X: 5, Y: 2, Recipe: uint8(recipe.Club), Mode: uint8(recipe.DoCount), Count: 1})
	stack(t, g, item.Wood, 8, 9, 6)
	applyNow(g, command.Command{Kind: command.Prioritize, Pawn: uint64(ada), Order: command.OrderDoBill, X: 5, Y: 2, Index: 1})
	j, _ := g.jobs.Get(ada)
	if !j.Forced || !j.Active() || j.Dest != bench || j.BillIndex != 1 {
		t.Fatalf("selected bill not started: %+v", j)
	}
	done := func() bool { return g.craft.Bills.Of(bench)[1].Done == 1 }
	runBillTicks(g, ada, 6000, done)
	if !done() || g.craft.Bills.Of(bench)[0].Done != 0 || g.stockOf(item.Club) != 1 {
		t.Fatalf("selected bill not completed: %+v, clubs %d", g.craft.Bills.Of(bench), g.stockOf(item.Club))
	}
}

func TestNativeOrderGiveInterruptAndResumeSave(t *testing.T) {
	g, ada, bo, pile := packGame(t)
	freeTime(g)
	g.setDraft(ada, true)
	g.setDraft(bo, true)
	packed(t, g, ada, pile, 12)
	entry := int(senseOf(g).Gear.Pack(ada).Items[0].ID)
	applyNow(g, command.Command{Kind: command.MovePawn, Pawn: uint64(ada), X: 5, Y: 5})
	applyNow(g, command.Command{Kind: command.MovePawn, Pawn: uint64(ada), X: 7, Y: 8, Queue: true})
	applyNow(g, command.Command{Kind: command.Prioritize, Pawn: uint64(ada), Order: command.OrderDeliverItem,
		Target: uint64(bo), Index: entry, Count: 7, Mode: command.PrioritizeInterrupt})
	j, _ := g.jobs.Get(ada)
	q := g.orderQueue[ada]
	if j.Kind != job.GiveItem || !j.Forced || len(q) != 2 || q[0].cmd.X != 5 || q[1].cmd.X != 7 {
		t.Fatalf("give did not interrupt and preserve queue: %+v %+v", j, q)
	}
	for i := 0; i < 300; i++ {
		g.Step(1)
		j, _ = g.jobs.Get(ada)
		if j.Kind == job.GiveItem && j.Carry != 0 {
			break
		}
	}
	if j.Kind != job.GiveItem || j.Carry == 0 {
		t.Fatalf("give did not take the item: %+v", j)
	}
	h := saveLoad(t, g)
	sameState(t, "mid give after load", g, h)
	reachedFirst := false
	for i := 0; i < 1800; i++ {
		g.Step(1)
		h.Step(1)
		p, _ := g.positions.Get(ada)
		if p == (Position{5, 5}) {
			reachedFirst = true
		}
		if p == (Position{7, 8}) && reachedFirst {
			break
		}
	}
	sameState(t, "give and resume replay", g, h)
	a, b := senseOf(g).Gear.Pack(ada), senseOf(g).Gear.Pack(bo)
	p, _ := g.positions.Get(ada)
	if !reachedFirst || p != (Position{7, 8}) || len(a.Items) != 1 || a.Items[0].Stack.Qty != 5 ||
		len(b.Items) != 1 || b.Items[0].Stack.Kind != item.Steel || b.Items[0].Stack.Qty != 7 || inPlay(g, item.Steel) != 20 {
		t.Fatalf("handover/resume mismatch: Ada %v Bo %v at %v first=%v total=%d", a.Items, b.Items, p, reachedFirst, inPlay(g, item.Steel))
	}
}

func TestNativeOrderRefusalPreservesJobAndQueue(t *testing.T) {
	g, ada, bo, pile := packGame(t)
	freeTime(g)
	packed(t, g, ada, pile, 12)
	applyNow(g, command.Command{Kind: command.MovePawn, Pawn: uint64(ada), X: 5, Y: 5})
	applyNow(g, command.Command{Kind: command.MovePawn, Pawn: uint64(ada), X: 7, Y: 8, Queue: true})
	before, _ := g.jobs.Get(ada)
	queue := append([]queuedOrder(nil), g.orderQueue[ada]...)
	held := stack(t, g, item.Wood, 10, 11, 12)
	g.reserve.TryCount(bo, held, 9, 10)
	for _, c := range []command.Command{
		{Order: command.OrderDoBill, X: 1, Y: 1, Index: 0},
		{Order: command.OrderDeliverItem, Target: uint64(bo), Index: -1, Count: 5},
		{Order: command.OrderHaulMulti, Name: fmt.Sprintf("%d:10@25,25", held)},
		{Order: command.OrderHaulMulti, Name: fmt.Sprintf("%d:1@25,25garbage", held)},
	} {
		c.Kind, c.Pawn, c.Mode = command.Prioritize, uint64(ada), command.PrioritizeInterrupt
		applyNow(g, c)
		j, _ := g.jobs.Get(ada)
		if !reflect.DeepEqual(j, before) || !reflect.DeepEqual(g.orderQueue[ada], queue) {
			t.Fatalf("refused %v changed job or queue: %+v %+v", c.Order, j, g.orderQueue[ada])
		}
	}
	if g.reserve.FreeFor(ada, held, 10) != 1 || inPlay(g, item.Steel) != 20 {
		t.Fatal("refusal changed existing claim or item balance")
	}
}

func TestNativeOrderMultiHaulPhysicalLegs(t *testing.T) {
	g := New(Config{Seed: 1, Width: 30, Height: 20})
	ada, _ := g.SpawnNamedPawn(1, 1, "Ada")
	freeTime(g)
	g.setDraft(ada, true)
	a := stack(t, g, item.Wood, 8, 3, 2)
	b := stack(t, g, item.Wood, 5, 4, 2)
	c := stack(t, g, item.Steel, 10, 5, 4)
	applyNow(g, command.Command{Kind: command.Prioritize, Pawn: uint64(ada), Order: command.OrderHaulMulti,
		Name: fmt.Sprintf("%d:8@15,2;%d:5@15,2;%d:4@18,7", a, b, c)})
	j, _ := g.jobs.Get(ada)
	q := g.orderQueue[ada]
	if !j.Forced || j.Kind != job.Haul || j.Target != a || j.Count != 13 || !reflect.DeepEqual(j.Pickups, []entity.EntityID{b, 5}) || len(q) != 1 {
		t.Fatalf("first load or continuation wrong: %+v %+v", j, q)
	}
	if got := g.postponed.orders[ada].cmd.Name; got != fmt.Sprintf("%d:8@15,2;%d:5@15,2", a, b) {
		t.Fatalf("retry command incorrectly includes queued tail: %q", got)
	}
	// Another delivery may merge into the first stack while the hauler
	// travels. The explicit load must still take 8 from a and 5 from b.
	st, _ := g.stacks.Get(a)
	st.Qty += 4
	g.putStack(a, st)
	for i := 0; i < 2500; i++ {
		g.SetNeeds(ada, needs.Full, needs.Full)
		g.Step(1)
		if storedIn(g, "wood", 15, 2, 15, 2) == 13 && storedIn(g, "steel", 18, 7, 18, 7) == 4 {
			break
		}
	}
	if storedIn(g, "wood", 15, 2, 15, 2) != 13 || storedIn(g, "steel", 18, 7, 18, 7) != 4 ||
		storedIn(g, "steel", 5, 4, 5, 4) != 6 || storedIn(g, "wood", 3, 2, 3, 2) != 4 || inPlay(g, item.Wood) != 17 || inPlay(g, item.Steel) != 10 {
		j, _ := g.jobs.Get(ada)
		t.Fatalf("haul lost or misplaced physical goods: wood=%d steel=%d remaining=%d job=%+v",
			storedIn(g, "wood", 15, 2, 15, 2), storedIn(g, "steel", 18, 7, 18, 7), storedIn(g, "steel", 5, 4, 5, 4), j)
	}
}

func TestNativeOrderGiveRecipientGoneConservesGoods(t *testing.T) {
	g, ada, bo, pile := packGame(t)
	freeTime(g)
	g.setDraft(ada, true)
	g.setDraft(bo, true)
	packed(t, g, ada, pile, 12)
	entry := int(senseOf(g).Gear.Pack(ada).Items[0].ID)
	applyNow(g, command.Command{Kind: command.Prioritize, Pawn: uint64(ada), Order: command.OrderDeliverItem,
		Target: uint64(bo), Index: entry, Count: 7})
	for i := 0; i < 300; i++ {
		g.Step(1)
		j, _ := g.jobs.Get(ada)
		if j.Kind == job.GiveItem && j.Carry != 0 {
			break
		}
	}
	j, _ := g.jobs.Get(ada)
	if j.Kind != job.GiveItem || j.Carry == 0 {
		t.Fatal("delivery did not pick up its item")
	}
	g.die(bo, health.CauseInjury)
	for i := 0; i < 1200; i++ {
		g.Step(1)
		j, _ = g.jobs.Get(ada)
		if j.Kind != job.GiveItem || !j.Active() {
			break
		}
	}
	if j.Carry != 0 || inPlay(g, item.Steel) != 20 || senseOf(g).Gear.Has(bo) {
		t.Fatalf("failed handover lost or duplicated goods: %+v total=%d", j, inPlay(g, item.Steel))
	}
}

func TestNativeOrderHaulLargeLegUsesRemainingTrips(t *testing.T) {
	g := New(Config{Seed: 1, Width: 25, Height: 10})
	id, _ := g.SpawnNamedPawn(1, 1, "Ada")
	freeTime(g)
	g.setDraft(id, true)
	limit := min(item.CarryLimit, g.sk.content.Items.StackLimit(item.Wood))
	source := stack(t, g, item.Wood, limit+7, 3, 2)
	applyNow(g, command.Command{Kind: command.Prioritize, Pawn: uint64(id), Order: command.OrderHaulMulti,
		Name: fmt.Sprintf("%d:%d@15,2", source, limit+7)})
	j, _ := g.jobs.Get(id)
	if j.Count != limit || len(g.orderQueue[id]) != 1 || g.orderQueue[id][0].cmd.Name != fmt.Sprintf("%d:7@15,2", source) {
		t.Fatalf("large leg must retain its second trip: %+v %+v", j, g.orderQueue[id])
	}
	for i := 0; i < 2500; i++ {
		g.SetNeeds(id, needs.Full, needs.Full)
		g.Step(1)
		if storedIn(g, "wood", 14, 1, 16, 3) == limit+7 {
			break
		}
	}
	if storedIn(g, "wood", 14, 1, 16, 3) != limit+7 || inPlay(g, item.Wood) != limit+7 {
		t.Fatalf("large leg did not move its whole quantity: %d/%d", storedIn(g, "wood", 14, 1, 16, 3), limit+7)
	}
}
