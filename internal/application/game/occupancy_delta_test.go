// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"testing"

	"rimworld-go/internal/domain/ai/job"
)

func assertOccupancyNeighbours(t *testing.T, g *Game) {
	t.Helper()
	o := &g.occ
	for y := range o.h {
		for x := range o.w {
			blockers, partners := 0, 0
			for ny := max(y-1, 0); ny <= min(y+1, o.h-1); ny++ {
				for nx := max(x-1, 0); nx <= min(x+1, o.w-1); nx++ {
					i := ny*o.w + nx
					if o.cells[i] != 0 {
						blockers++
					}
					if o.pfirst[i] != 0 {
						partners++
					}
				}
			}
			i := y*o.w + x
			if int(o.near[i]) != blockers || int(o.pnear[i]) != partners {
				t.Fatalf("occupancy (%d,%d): blockers %d want %d, partners %d want %d",
					x, y, o.near[i], blockers, o.pnear[i], partners)
			}
		}
	}
}

func TestOccupancyBatchRebuildPreservesNearbyCounts(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 11, Width: 20, Height: 20})
	bareGround(g)
	pawn, err := g.SpawnPawn(2, 2)
	if err != nil {
		t.Fatal(err)
	}
	wild, err := g.SpawnAnimal("dustbuck", 7, 7)
	if err != nil {
		t.Fatal(err)
	}
	tame, err := g.SpawnAnimal("dustbuck", 10, 10)
	if err != nil {
		t.Fatal(err)
	}
	g.TameAnimal(tame, "Pet")
	for range 3 {
		g.rebuildOccupancy()
		assertOccupancyNeighbours(t, g)
	}
	from := Position{X: 7, Y: 7}
	to := Position{X: 8, Y: 7}
	g.occ.move(wild, false, from, to)
	g.positions.Set(wild, to)
	assertOccupancyNeighbours(t, g)
	g.rebuildOccupancy()
	assertOccupancyNeighbours(t, g)
	g.jobs.Set(pawn, job.Record{Kind: job.Sleep, State: job.Do})
	g.rebuildOccupancy()
	assertOccupancyNeighbours(t, g)
	if got := g.occ.pfirst[2*20+2]; got != 0 {
		t.Fatalf("sleeping pawn kept its tile: %d", got)
	}
	g.jobs.Set(pawn, job.Record{})
	g.rebuildOccupancy()
	assertOccupancyNeighbours(t, g)
	if got := g.occ.pfirst[2*20+2]; got != pawn {
		t.Fatalf("waking pawn lost its tile: %d", got)
	}
}

func TestOccupancyBatchRebuildKeepsStackedBlocker(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 17, Width: 20, Height: 20})
	bareGround(g)
	first, err := g.SpawnAnimal("dustbuck", 7, 7)
	if err != nil {
		t.Fatal(err)
	}
	second, err := g.SpawnAnimal("dustbuck", 9, 7)
	if err != nil {
		t.Fatal(err)
	}
	g.positions.Set(second, Position{X: 7, Y: 7})
	g.rebuildOccupancy()
	assertOccupancyNeighbours(t, g)
	if got := g.occ.cells[7*20+7]; got != first {
		t.Fatalf("lower ID must own blocker tile, got %d", got)
	}
	// Unstack may order the second animal away; the current cell still
	// contains the lowest-ID blocker until it actually moves.
	g.rebuildOccupancy()
	assertOccupancyNeighbours(t, g)
}
