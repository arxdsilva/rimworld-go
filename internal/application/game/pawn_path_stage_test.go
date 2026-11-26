// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"testing"

	"rimworld-go/internal/application/game/pathstage"
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/ai/pathing"
	"rimworld-go/internal/platform/pool"
)

const pawnRepathCount = 12

func blockedPawnBurst(tb testing.TB, p pathstage.Pool) *Game {
	tb.Helper()
	g := New(Config{Seed: 7, Width: 160, Height: 160})
	bareGround(g)
	for i := range pawnRepathCount {
		x, y := 10+i*3, 10
		id, err := g.SpawnPawn(x, y)
		if err != nil {
			tb.Fatal(err)
		}
		g.grid.SetWalkable(x+1, y, false)
		g.jobs.Set(id, job.Record{Kind: job.Move, State: job.Goto,
			TX: 150, TY: 150, Path: []pathing.Point{{X: x + 1, Y: y}}})
	}
	g.regs.paths.SetPool(p)
	return g
}

func TestPawnObstructedPathBatchEqualsSerial(t *testing.T) {
	t.Parallel()
	serial := blockedPawnBurst(t, nil)
	lanes := pool.New(pool.Options{Lanes: 4, MinTasks: 1, Shuffle: 2})
	defer lanes.Close()
	parallel := blockedPawnBurst(t, lanes)
	for tick := range 30 {
		serial.phaseMovement()
		parallel.phaseMovement()
		if got, want := parallel.StateHash(), serial.StateHash(); got != want {
			t.Fatalf("tick %d: staged move changed state: got %x want %x", tick, got, want)
		}
	}
	runs, searched := parallel.regs.pawnPaths.Stats()
	if runs != 1 || searched != pawnRepathCount {
		t.Fatalf("staged pawn searches: %d runs %d searches", runs, searched)
	}
}

func TestPawnObstructedPathBatchRejectsJobChange(t *testing.T) {
	t.Parallel()
	lanes := pool.New(pool.Options{Lanes: 4, MinTasks: 1})
	defer lanes.Close()
	g := blockedPawnBurst(t, lanes)
	id := g.pawnIDs[0]
	preplanPawnPaths(g, g.movers(), uint64(g.clock.Now()))
	j, _ := g.jobs.Get(id)
	if _, _, ok := preplannedPawnPath(g, id, j, j.TX, j.TY); !ok {
		t.Fatal("missing staged result for unchanged job")
	}
	j.Forced = true
	g.jobs.Set(id, j)
	if _, _, ok := preplannedPawnPath(g, id, j, j.TX, j.TY); ok {
		t.Fatal("a forced order used a path planned for a regular job")
	}
}

func TestPawnObstructedPathBatchRejectsGridChange(t *testing.T) {
	t.Parallel()
	lanes := pool.New(pool.Options{Lanes: 4, MinTasks: 1})
	defer lanes.Close()
	g := blockedPawnBurst(t, lanes)
	id := g.pawnIDs[0]
	preplanPawnPaths(g, g.movers(), uint64(g.clock.Now()))
	j, _ := g.jobs.Get(id)
	if _, _, ok := preplannedPawnPath(g, id, j, j.TX, j.TY); !ok {
		t.Fatal("missing preplanned route")
	}
	g.grid.SetWalkable(12, 10, false)
	if _, _, ok := preplannedPawnPath(g, id, j, j.TX, j.TY); ok {
		t.Fatal("stale route was used after walkability changed")
	}
}

func BenchmarkPawnObstructedRepath160(b *testing.B) {
	for _, c := range []struct {
		name  string
		lanes int
	}{{"serial", 0}, {"pool4", 4}} {
		b.Run(c.name, func(b *testing.B) {
			var p pathstage.Pool
			if c.lanes > 0 {
				pl := pool.New(pool.Options{Lanes: c.lanes})
				b.Cleanup(pl.Close)
				p = pl
			}
			g := blockedPawnBurst(b, p)
			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				b.StopTimer()
				for i, id := range g.pawnIDs {
					j, _ := g.jobs.Get(id)
					j.Path, j.PathI, j.State = []pathing.Point{{X: 11 + i*3, Y: 10}}, 0, job.Goto
					g.jobs.Set(id, j)
				}
				b.StartTimer()
				g.phaseMovement()
			}
		})
	}
}

// The common disabled and enabled-but-idle paths must not allocate a
// per-tick callback bundle. Disabling also invalidates the last batch.
func TestPawnObstructedPathBatchIdleAllocsAndReset(t *testing.T) {
	lanes := pool.New(pool.Options{Lanes: 4, MinTasks: 1})
	defer lanes.Close()
	g := blockedPawnBurst(t, lanes)
	movers := g.movers()
	now := uint64(g.clock.Now())
	preplanPawnPaths(g, movers, now)
	id := g.pawnIDs[0]
	j, _ := g.jobs.Get(id)
	if _, _, ok := preplannedPawnPath(g, id, j, j.TX, j.TY); !ok {
		t.Fatal("no prepared result before disabling the pool")
	}
	g.regs.paths.SetPool(nil)
	if n := testing.AllocsPerRun(100, func() { preplanPawnPaths(g, movers, now) }); n != 0 {
		t.Fatalf("disabled batch allocated %g objects per tick", n)
	}
	if _, _, ok := preplannedPawnPath(g, id, j, j.TX, j.TY); ok {
		t.Fatal("disabled batch retained a stale result")
	}
	g.regs.paths.SetPool(lanes)
	for _, id := range g.pawnIDs {
		j, _ := g.jobs.Get(id)
		j.State = job.Complete
		g.jobs.Set(id, j)
	}
	if n := testing.AllocsPerRun(100, func() { preplanPawnPaths(g, movers, now) }); n != 0 {
		t.Fatalf("idle enabled batch allocated %g objects per tick", n)
	}
	if n := len(g.regs.pawnPaths.Requests()); n != 0 {
		t.Fatalf("idle batch has %d requests", n)
	}
}
