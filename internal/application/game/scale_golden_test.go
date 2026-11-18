// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"testing"
	"time"

	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/sim/entity"
)

// The scale golden (TestScaleGolden, six seeded 20 000-tick runs pinned by
// state hash) lives in ./scalegolden, its own -race test binary.

func TestStateHashStable(t *testing.T) {
	t.Parallel()
	g := NewColony(Config{Seed: 3, Width: 48, Height: 40})
	g.Step(300)
	a, b := g.StateHash(), g.StateHash()
	if a != b {
		t.Fatalf("StateHash not stable: %x vs %x", a, b)
	}
	g.Step(1)
	if g.StateHash() == a {
		t.Fatal("StateHash unchanged after a tick")
	}
}

// TestPhaseObserverDoesNotChangeState: an observed Step is the same Step.
func TestPhaseObserverDoesNotChangeState(t *testing.T) {
	t.Parallel()
	a := NewColony(Config{Seed: 5, Width: 48, Height: 40})
	b := NewColony(Config{Seed: 5, Width: 48, Height: 40})
	seen := map[string]int{}
	b.SetPhaseObserver(func(p string, _ time.Duration) { seen[p]++ })
	a.Step(500)
	b.Step(500)
	if a.StateHash() != b.StateHash() {
		t.Fatal("observed run diverged")
	}
	for _, p := range Phases() {
		if seen[p] != 500 {
			t.Fatalf("phase %s observed %d times, want 500", p, seen[p])
		}
	}
	b.SetPhaseObserver(nil)
	a.Step(10)
	b.Step(10)
	if a.StateHash() != b.StateHash() {
		t.Fatal("diverged after removing the observer")
	}
}

// Test-only hooks for the external scale benchmarks (scale_bench_test.go,
// package game_test, which needs the scenario fixture and so cannot be
// an internal test).

// BenchColonist is the first colonist's id.
func (g *Game) BenchColonist() entity.EntityID { return g.pawnIDs[0] }

// BenchReachable is reachable(id, x, y).
func (g *Game) BenchReachable(id entity.EntityID, x, y int) bool { return g.reachable(id, x, y) }

// BenchPath is the length of pawn id's path to (x, y), or -1.
func (g *Game) BenchPath(id entity.EntityID, x, y int) int {
	p, ok := g.pathTo(id, x, y)
	if !ok {
		return -1
	}
	return len(p)
}

// BenchRebuildRooms toggles a wall beside home (built, then removed on
// the next call) and rebuilds the rooms as that change does; it returns
// the room count.
func (g *Game) BenchRebuildRooms() int {
	if id, ok := benchWalls[g]; ok {
		g.removeBuilding(id)
		delete(benchWalls, g)
	} else {
		hx, hy := g.home()
		for r := 0; r < 60 && !ok; r++ {
			for dy := -r; dy <= r && !ok; dy++ {
				for dx := -r; dx <= r && !ok; dx++ {
					x, y := hx+dx, hy+dy
					if can, _ := g.canPlaceAt(construction.Wall, construction.South, x, y); can && g.BuildNow("wall", "wood", "south", x, y) {
						benchWalls[g], ok = g.layerBldgAt(x, y, construction.LayerObject), true
					}
				}
			}
		}
	}
	g.ensureRooms()
	return len(g.rooms)
}

// benchWalls is the wall BenchRebuildRooms stands, per game.
var benchWalls = map[*Game]entity.EntityID{}

// Every test in this package runs with the per-cell indexes checked
// against the original scans (indexes.go); the external benchmarks turn
// it off.
// Under -race (about ten times slower) one lookup in 32 is checked.
func init() {
	indexSelfCheck = true
	if raceOn {
		selfCheckEvery = 32
	}
}

// SetIndexSelfCheck switches the index self-check (benchmarks: off).
func SetIndexSelfCheck(on bool) { indexSelfCheck = on }
