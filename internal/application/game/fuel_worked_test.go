// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"testing"

	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/recipe"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// fuelSpecOf is kind k's fuel table row, failing the test when it has none.
func fuelSpecOf(t *testing.T, k construction.Kind) fuelDef {
	t.Helper()
	s, ok := fuelSpec(k)
	if !ok {
		t.Fatalf("%v has no fuel spec", k)
	}
	return s
}

// burnTicks runs g until bill 0 at bench has made one product (or n
// ticks), counting the ticks fuel fell and checking each fell by wb.
func burnTicks(t *testing.T, g *Game, bench, pawn entity.EntityID, wb, n int) int {
	t.Helper()
	worked := 0
	for i := 0; i < n && g.craft.Bills.Of(bench)[0].Done == 0; i++ {
		if i%100 == 0 {
			g.SetNeeds(pawn, needs.Full, needs.Full)
		}
		before := g.fuel[bench]
		g.Step(1)
		if d := before - g.fuel[bench]; d != 0 {
			if d != wb {
				t.Fatalf("a tick burnt %d fuel ticks, want %d", d, wb)
			}
			worked++
		}
	}
	if g.craft.Bills.Of(bench)[0].Done != 1 {
		t.Fatal("nothing made")
	}
	return worked
}

// TestStoveBurnsPerWorkedTick (wiki: Fueled stove): a cook burns exactly
// WorkBurn per worked tick, so the fuel used is ticks x 16; a stove nobody
// works keeps its fuel; the smithy burns 8 per worked tick.
func TestStoveBurnsPerWorkedTick(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 16, Height: 8})
	jumpTo(g, 10, 12)
	cook, _ := g.SpawnNamedPawn(1, 1, "Ada")
	stove := buildNow(t, g, construction.Stove, item.Wood, construction.South, 5, 2)
	g.SpawnStack(item.Rice, 20, 1, 4)
	g.Enqueue(command.Command{Kind: command.AddBill, X: 5, Y: 2, Recipe: uint8(recipe.SimpleMeal), Mode: uint8(recipe.DoCount), Count: 1})
	spec := fuelSpecOf(t, construction.Stove)
	if spec.WorkBurn != 16 {
		t.Fatalf("stove WorkBurn %d", spec.WorkBurn)
	}
	// idle: a full stove with no bill running keeps its fuel
	idle := buildNow(t, g, construction.Stove, item.Wood, construction.South, 9, 5)
	g.setFuel(idle, spec.CapTicks)
	g.Step(1)
	g.setFuel(stove, spec.CapTicks)
	worked := burnTicks(t, g, stove, cook, spec.WorkBurn, 6000)
	if used := spec.CapTicks - g.fuel[stove]; worked == 0 || used != worked*16 {
		t.Fatalf("stove used %d fuel ticks over %d worked ticks", used, worked)
	}
	if g.fuel[idle] != spec.CapTicks {
		t.Fatalf("an unworked stove burnt %d", spec.CapTicks-g.fuel[idle])
	}
	before := g.fuel[stove]
	g.Step(10000)
	if g.fuel[stove] != before {
		t.Fatalf("a stove idle for 10000 ticks burnt %d", before-g.fuel[stove])
	}
}

// TestOldSaveTankRefuels: a stove saved with 5 logs (30000 fuel ticks)
// loads as 10 % of the new 50-log tank, is under Below and gets a refuel
// job.
func TestOldSaveTankRefuels(t *testing.T) {
	t.Parallel()
	g := fuelGame(t)
	id, _ := g.SpawnNamedPawn(2, 2, "Ada")
	g.SetNeeds(id, needs.Full, needs.Full)
	stove := placeFuel(t, g, construction.Stove, 8, 4)
	if _, err := g.SpawnStack(item.Wood, 60, 2, 6); err != nil {
		t.Fatal(err)
	}
	g.setFuel(stove, 5*TicksPerWood)
	b := g.bldgs.Ref(stove)
	if pct, ok := g.fuelPct(stove, *b); !ok || pct != 10 {
		t.Fatalf("FuelPct %d %v, want 10", pct, ok)
	}
	if !g.assignRefuel(id) {
		t.Fatal("an old 5-log stove was not refuelled")
	}
}
