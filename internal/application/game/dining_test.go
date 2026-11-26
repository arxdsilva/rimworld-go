// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"testing"

	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/recipe"
	"rimworld-go/internal/domain/pawn/mood"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/sim/clock"
)

func TestColonistCarriesMealToTable(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 20, Height: 10})
	id, _ := g.SpawnNamedPawn(2, 5, "Ada")
	g.SpawnStack(item.Meal, 5, 3, 5)
	placeBuilt(t, g, construction.Table, construction.South, 12, 5)
	placeBuilt(t, g, construction.Chair, construction.South, 12, 7) // A2: south of the 1x2 table
	g.SetNeeds(id, needs.CriticalHunger-1, needs.Full)
	ate := false
	for i := 0; i < 400+needs.EatWorkTicks && !ate; i++ {
		g.Step(1)
		_, ate = g.lastMeal[id]
	}
	for _, th := range pawn(t, g, id).Thoughts {
		ate = ate && th.Label != "ate without a table" // FOOD-4: a table gives no thought, its lack -3
	}
	if !ate {
		p := pawn(t, g, id)
		t.Fatalf("at (%d,%d) job %s thoughts %+v: should have eaten at the table", p.X, p.Y, p.Job, p.Thoughts)
	}
	p := pawn(t, g, id)
	if p.X != 12 || p.Y != 7 {
		t.Fatalf("ate at (%d,%d), want the chair at (12,7)", p.X, p.Y)
	}
	want := 5 - portionsFor(needs.CriticalHunger-1, item.Stack{Kind: item.Meal})
	if st, ok := stackOf(g, item.Meal); !ok || st.Qty != want || st.X != 3 {
		t.Fatalf("only the portions eaten should be taken (%d left where they were): %+v", want, st)
	}
}

func TestCampfireCooksWithFuelOnly(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 16, Height: 10})
	id, _ := g.SpawnNamedPawn(2, 2, "Ada")
	g.SetNeeds(id, needs.Full, needs.Full)
	if !g.BuildNow("campfire", "wood", "south", 8, 4) {
		t.Fatal("campfire")
	}
	bid := g.layerBldgAt(8, 4, construction.LayerObject)
	g.craft.Bills.Set(bid, []recipe.Bill{{Recipe: recipe.CampfireMeal, Mode: recipe.Forever, Target: 1}})
	g.SpawnStack(item.Potatoes, 10, 3, 6)
	g.Step(clock.TicksPerHour / 2)
	if _, ok := stackOf(g, item.Meal); ok {
		t.Fatal("an unlit campfire cooked a meal")
	}
	if bv := g.craft.Bills.AppendViews(nil, bid); bv[0].Status != BillNoFuel {
		t.Fatalf("bill status %q", bv[0].Status)
	}
	g.FuelCampfire(8, 4)
	for i := 0; i < 20; i++ {
		g.Step(100)
		g.SetNeeds(id, needs.Full, needs.Full)
		if _, ok := stackOf(g, item.Meal); ok {
			return
		}
	}
	t.Fatal("a lit campfire should cook a simple meal")
}

func TestColonyAIVanillaDiagonalTableDoesNotCountAsEatingSurface(t *testing.T) {
	g := New(Config{Seed: 40, Width: 28, Height: 18})
	id, err := g.SpawnNamedPawn(4, 4, "Eater")
	if err != nil {
		t.Fatal(err)
	}
	placeBuilt(t, g, construction.Table, construction.South, 5, 5)
	if g.nextToTable(4, 4) {
		t.Fatal("diagonal corner of a table is not an adjacent cardinal eating surface")
	}
	g.afterEat(id, item.Stack{Kind: item.Meal, Qty: 1}, false)
	found := false
	for _, m := range g.memories[id] {
		if m.Kind == mood.AteWithoutTable {
			found = true
		}
	}
	if !found {
		t.Fatal("diagonally nearby table erased actual ate-without-table thought")
	}
}

// A peer-carried meal reaches a free dining seat 30 tiles from the eater.
func TestColonyAIVanillaPeerPackMealFindsReservedChairBeforeEating(t *testing.T) {
	g := New(Config{Seed: 124, Width: 50, Height: 18})
	owner, _ := g.SpawnNamedPawn(4, 5, "Donor")
	eater, _ := g.SpawnNamedPawn(5, 5, "Hungry")
	placeBuilt(t, g, construction.Table, construction.South, 35, 5)
	placeBuilt(t, g, construction.Chair, construction.South, 35, 7)
	pack := senseOf(g).Gear.Pack(owner)
	pack.Add(item.Stack{Kind: item.Meal, Qty: 2})
	senseOf(g).Packs[owner] = pack
	g.SetNeeds(owner, needs.Full, needs.Full)
	g.SetNeeds(eater, needs.CriticalHunger-1, needs.Full)
	ate := false
	for step := 0; step < 500+needs.EatWorkTicks; step++ {
		g.Step(1)
		if _, ok := g.lastMeal[eater]; ok {
			ate = true
			break
		}
	}
	if !ate {
		j, _ := g.jobs.Get(eater)
		at, _ := g.positions.Get(eater)
		sx, sy, found := g.rest.Seats.AtTable(eater, at.X, at.Y)
		t.Fatalf("peer packed meal unfinished: pawn at %d,%d job=%+v table=%d,%d found=%v lastMeal=%+v", at.X, at.Y, j, sx, sy, found, g.lastMeal[eater])
	}
	p, _ := g.positions.Get(eater)
	if p.X != 35 || p.Y != 7 {
		t.Fatalf("chewed peer packed meal at (%d,%d) instead of reachable chair (35,7)", p.X, p.Y)
	}
	for _, th := range g.thoughts(eater) {
		if th.Kind == mood.AteWithoutTable {
			t.Fatalf("ate without table thought when seated by actual table: %+v", th)
		}
	}
	if len(senseOf(g).Gear.Pack(owner).Items) != 1 ||
		senseOf(g).Gear.Pack(owner).Items[0].Stack.Qty != 1 {
		t.Fatalf("donor's original 2 meals not decremented exactly once: %+v", senseOf(g).Gear.Pack(owner))
	}
}
