// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"fmt"
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/recipe"
	"rimworld-go/internal/domain/sim/command"
	"testing"
)

func TestSelectedBillIdentity(t *testing.T) {
	g, ada, bench := clubGame(t)
	applyNow(g, command.Command{Kind: command.AddBill, X: 5, Y: 2, Recipe: uint8(recipe.Club), Mode: uint8(recipe.DoCount), Count: 1})
	g.setDraft(ada, true)
	applyNow(g, command.Command{Kind: command.MovePawn, Pawn: uint64(ada), X: 18, Y: 10})
	applyNow(g, command.Command{Kind: command.Prioritize, Pawn: uint64(ada), Order: command.OrderDoBill, X: 5, Y: 2, Index: 1, Queue: true})
	q := g.orderQueue[ada]
	if len(q) != 1 {
		t.Fatalf("expected selected bill queued: %+v", q)
	}
	c := q[0].cmd
	applyNow(g, command.Command{Kind: command.ReorderBill, X: 5, Y: 2, Index: 1, Count: -1})
	g = saveLoad(t, g)
	bid, index, ok := g.craft.Bills.ResolveOrder(c)
	if !ok || bid != bench || index != 0 {
		t.Fatalf("wrong selected row after reorder/load: %d,%d,%v", bid, index, ok)
	}
	applyNow(g, command.Command{Kind: command.RemoveBill, X: 5, Y: 2, Index: 0})
	applyNow(g, command.Command{Kind: command.AddBill, X: 5, Y: 2, Recipe: uint8(recipe.Club), Count: 1})
	if _, _, ok := g.craft.Bills.ResolveOrder(c); ok {
		t.Fatal("deleted row identity reused")
	}
}
func TestShortExactHaulPreservesJob(t *testing.T) {
	g := New(Config{Seed: 8, Width: 30, Height: 20})
	bareGround(g)
	id, _ := g.SpawnPawn(2, 2)
	st := stack(t, g, item.Wood, 3, 5, 5)
	g.setDraft(id, true)
	applyNow(g, command.Command{Kind: command.MovePawn, Pawn: uint64(id), X: 10, Y: 5})
	old, _ := g.jobs.Get(id)
	applyNow(g, command.Command{Kind: command.Prioritize, Pawn: uint64(id), Order: command.OrderHaulMulti, Name: fmt.Sprintf("%d:5@15,5", st)})
	now, _ := g.jobs.Get(id)
	if now.Kind != old.Kind || now.TX != old.TX || !now.Active() {
		t.Fatalf("short request displaced old job: %+v", now)
	}
}
func TestPeerDoesNotClobberPackExtraction(t *testing.T) {
	g := New(Config{Seed: 21, Width: 30, Height: 20})
	bareGround(g)
	donor, _ := g.SpawnPawn(10, 10)
	eater, _ := g.SpawnPawn(11, 10)
	gear := senseOf(g).Gear
	if len(gear.FoodDonors()) != 0 {
		t.Fatal("fixture has donor")
	}
	meal := stack(t, g, item.Meal, 2, 10, 10)
	wood := stack(t, g, item.Wood, 3, 10, 10)
	if !gear.Stow(donor, meal, 2) || !gear.Stow(donor, wood, 3) {
		t.Fatal("stow failed")
	}
	if len(gear.FoodDonors()) != 1 {
		t.Fatal("food donor cache not invalidated")
	}
	entry, ok := gear.Pack(donor).Find(item.Wood)
	if !ok {
		t.Fatal("wood pack entry")
	}
	carry, ok := gear.TakeOut(donor, entry, 1)
	if !ok {
		t.Fatal("take out")
	}
	g.jobs.Set(donor, job.Record{Kind: job.GiveItem, State: job.Do, Carry: carry})
	if _, _, _, _, ok := g.peerPackedMeal(eater, eater); ok {
		t.Fatal("selected donor with active extraction")
	}
	e, _ := gear.Pack(donor).Find(item.Meal)
	doEatFromPeerPack(g, eater, job.Record{Kind: job.EatFromPeerPack, Target: donor, BillIndex: int(e)})
	if senseOf(g).FromPack[donor] != carry {
		t.Fatal("existing extraction clobbered")
	}
	g.interrupt(donor, job.Cancelled)
	if gear.Pack(donor).Items == nil || senseOf(g).FromPack[donor] != 0 {
		t.Fatal("interruption failed to return pack item")
	}
	if _, _, _, _, ok := g.peerPackedMeal(eater, eater); !ok {
		t.Fatal("returned donor not selectable")
	}
}
