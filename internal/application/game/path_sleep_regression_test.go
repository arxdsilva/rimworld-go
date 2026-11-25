// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"rimworld-go/internal/application/game/pathstage"
	"rimworld-go/internal/application/game/savefmt"
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/sim/statehash"
	"rimworld-go/internal/platform/pool"
	"testing"
)

func placePathTestLamp(t *testing.T, g *Game, x, y int) {
	t.Helper()
	g.placeFrameStuff(construction.Lamp, item.Wood, construction.South, x, y)
	fid := g.layerFrameAt(x, y, construction.LayerObject)
	fr, ok := g.frames.Get(fid)
	if !ok {
		t.Fatal("fixture: no lamp frame")
	}
	g.completeFrame(fid, fr)
	if !g.grid.Walkable(x, y) || cellStandable(g, x, y) {
		t.Fatal("fixture: expected traversable nonstandable lamp")
	}
}

func TestLampBatch(t *testing.T) {
	for _, n := range []int{1, 2, 8} {
		lanes := pool.New(pool.Options{Lanes: n, MinTasks: 1})
		defer lanes.Close()
		var p pathstage.Pool = lanes
		g := blockedPawnBurst(t, p)
		placePathTestLamp(t, g, 150, 150)
		id := g.pawnIDs[0]
		j, _ := g.jobs.Get(id)
		serial, sok := g.pathTo(id, j.TX, j.TY)
		preplanPawnPaths(g, g.movers(), uint64(g.clock.Now()))
		parallel, pok, ready := preplannedPawnPath(g, id, j, j.TX, j.TY)
		if !sok || len(serial) == 0 {
			t.Fatal("fixture: serial adjacent path")
		}
		if ready || pok || len(parallel) > 0 {
			t.Fatalf("nonstandable goal incorrectly staged: %v", parallel)
		}
	}
}

func TestLampAfterPlanning(t *testing.T) {
	g := New(Config{Seed: 7, Width: 24, Height: 16})
	bareGround(g)
	id, err := g.SpawnPawn(6, 5)
	if err != nil {
		t.Fatal(err)
	}
	if !g.startJob(id, job.Record{Kind: job.Move, TX: 7, TY: 5}) {
		t.Fatal("fixture: no move")
	}
	placePathTestLamp(t, g, 7, 5)
	g.phaseMovement()
	p, _ := g.positions.Get(id)
	j, _ := g.jobs.Get(id)
	if p.X == 7 && p.Y == 5 {
		t.Fatalf("completed path entered nonstandable lamp; position %v job state %v", p, j.State)
	}
}

func TestRaiderExhaustion(t *testing.T) {
	g, raw := fleeingRaider(t, 20, 10)
	id := uint64ToID(raw)
	n, _ := g.need.Get(id)
	n.Rest = 0
	g.need.Set(id, n)
	g.phaseNeeds()
	v, _ := g.vital.Get(id)
	j, _ := g.jobs.Get(id)
	if v.Status != health.Downed || v.Cause != health.CauseNoBed || j.Kind != job.Sleep || j.State != job.Do {
		t.Fatalf("fixture: vital %v job %v", v, j)
	}
	for i := 0; i < 100; i++ {
		g.clock.Advance(1)
		g.phaseNeeds()
		g.phaseJobs()
		g.phaseThreats()
	}
	n, _ = g.need.Get(id)
	if n.Rest == 0 {
		t.Fatalf("collapsed raider got no sleep progress after 100 ticks: needs %+v", n)
	}
}

func TestVisitorSleepOwner(t *testing.T) {
	g, _, id, _ := guestScene(t)
	now := uint64(g.clock.Now())
	g.clock.Advance(int((uint64(needs.HungerPeriod) - now%uint64(needs.HungerPeriod)) % uint64(needs.HungerPeriod)))
	now = uint64(g.clock.Now())
	g.jobs.Set(id, job.Record{Kind: job.Sleep, State: job.Do, WorkLeft: 1000})
	n := needs.Pair{Hunger: needs.Full, Rest: needs.Full / 4}.Settle()
	g.need.Set(id, n)
	want := needs.SleepTick(n, now)
	g.phaseNeeds()
	g.phaseJobs()
	g.phaseThreats()
	afterThreat, _ := g.need.Get(id)
	if !(visitBodies{g}).RestStep(id, now) {
		t.Fatal("fixture: visitor sleep not stepped")
	}
	got, _ := g.need.Get(id)
	if got.Hunger != want.Hunger {
		t.Fatalf("sleep hunger applied twice: before %d after threats %d after visitor %d single-tick want %d", n.Hunger, afterThreat.Hunger, got.Hunger, want.Hunger)
	}
}

func TestReachLampCluster(t *testing.T) {
	g := New(Config{Seed: 7, Width: 24, Height: 24})
	bareGround(g)
	id, err := g.SpawnPawn(2, 2)
	if err != nil {
		t.Fatal(err)
	}
	for y := 9; y <= 11; y++ {
		for x := 9; x <= 11; x++ {
			placePathTestLamp(t, g, x, y)
		}
	}
	_, ok := g.pathTo(id, 10, 10)
	indexed := g.reachRegions(id, 10, 10)
	if indexed != ok {
		t.Fatalf("region reach=%t, pathTo=%t for lamp surrounded by eight nonstandable cells", indexed, ok)
	}
}

func TestVisitorFullPhase(t *testing.T) {
	g, _, id, _ := guestScene(t)
	now := uint64(g.clock.Now())
	g.clock.Advance(int((uint64(needs.HungerPeriod) - now%uint64(needs.HungerPeriod)) % uint64(needs.HungerPeriod)))
	now = uint64(g.clock.Now())
	g.jobs.Set(id, job.Record{Kind: job.Sleep, State: job.Do, WorkLeft: 1000})
	n := needs.Pair{Hunger: needs.Full, Rest: needs.Full / 4}.Settle()
	g.need.Set(id, n)
	want := needs.SleepTick(n, now)
	g.Step(1)
	got, _ := g.need.Get(id)
	if got.Hunger != want.Hunger {
		t.Fatalf("full Step(1) sleep hunger=%d, want %d from one sleep tick", got.Hunger, want.Hunger)
	}
}

func TestLampBatchStateParity(t *testing.T) {
	serial := blockedPawnBurst(t, nil)
	placePathTestLamp(t, serial, 150, 150)
	for _, n := range []int{1, 2, 8} {
		lanes := pool.New(pool.Options{Lanes: n, MinTasks: 1, Shuffle: 2})
		parallel := blockedPawnBurst(t, lanes)
		reference := blockedPawnBurst(t, nil)
		placePathTestLamp(t, parallel, 150, 150)
		placePathTestLamp(t, reference, 150, 150)
		if parallel.StateHash() != serial.StateHash() {
			t.Fatal("fixture state")
		}
		for tick := 0; tick < 30; tick++ {
			reference.phaseMovement()
			parallel.phaseMovement()
			if got, want := parallel.StateHash(), reference.StateHash(); got != want {
				lanes.Close()
				t.Fatalf("lanes=%d tick=%d state=%x want=%x", n, tick, got, want)
			}
		}
		lanes.Close()
	}
}

func TestExhaustedRaiderWakeAfterSave(t *testing.T) {
	g, raw := fleeingRaider(t, 20, 10)
	id := uint64ToID(raw)
	// Complete ordinary room/roof initialization before the save boundary.
	g.Step(1)
	g.SetNeeds(id, needs.Full, 0)
	g.phaseNeeds()
	v, _ := g.vital.Get(id)
	if v.Cause != health.CauseNoBed || v.Status != health.Downed {
		t.Fatal("collapse fixture")
	}
	n, _ := g.need.Get(id)
	n.Rest = needs.WakeRest - needs.SleepGain
	g.need.Set(id, n)
	h := saveLoad(t, g)
	assertWakeHash(t, "initial load", g, h)
	// Ground sleep is scaled by actual rest effectiveness, not the unscaled SleepGain.
	for i := 0; i < 20*needs.SleepPeriod; i++ {
		for _, scene := range []*Game{g, h} {
			scene.Step(1)
		}
		assertWakeHash(t, "wake replay", g, h)
		v, _ := g.vital.Get(id)
		if v.Status == health.Alive {
			break
		}
	}
	v, _ = g.vital.Get(id)
	if v.Status != health.Alive {
		n, _ := g.need.Get(id)
		j, _ := g.jobs.Get(id)
		t.Fatalf("exhausted raider never stood: %+v needs=%+v job=%+v", v, n, j)
	}
}

func assertWakeHash(t *testing.T, label string, a, b *Game) {
	t.Helper()
	if a.StateHash() == b.StateHash() {
		return
	}
	ai, as, err := snapshotForSave(a, savefmt.Stamp{Name: "statehash"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	bi, bs, err := snapshotForSave(b, savefmt.Stamp{Name: "statehash"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	af, bf := normalisedFields(ai, as), normalisedFields(bi, bs)
	for k, v := range af {
		if statehash.OfFields(map[string]any{k: v}) != statehash.OfFields(map[string]any{k: bf[k]}) {
			t.Logf("%s: %s original hash=%x restored hash=%x", label, k, statehash.OfFields(map[string]any{k: v}), statehash.OfFields(map[string]any{k: bf[k]}))
		}
	}
	t.Fatalf("%s: %x vs %x", label, a.StateHash(), b.StateHash())
}
