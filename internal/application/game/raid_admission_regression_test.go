// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"math"
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/difficulty"
	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/world/roof"
	"testing"
)

func TestRefusedRaidNoHook(t *testing.T) {
	for _, seed := range []uint64{9} {
		g := New(Config{Seed: seed, Width: 30, Height: 20})
		if _, err := g.SpawnNamedPawn(15, 10, "Ada"); err != nil {
			t.Fatal(err)
		}
		for i := range g.roofs {
			g.roofs[i] = roof.Natural | roof.NaturalThick
		}
		f := &fakeScripts{}
		UseScripts(g, f)
		if ok := g.FireIncident("rival_raid"); ok || len(f.events) > 0 {
			t.Fatalf("failed raid reported success or event: %v %+v", ok, f.events)
		}
	}

}
func TestRaiderDropsDownedForeignAnimal(t *testing.T) {
	g, _ := def4Game(t, 40, 30, 30, 20)
	rid := eaiRaid(t, g, Position{X: 10, Y: 10})[0]
	hive, err := g.SpawnAnimal("hive_crawler", 12, 10)
	if err != nil {
		t.Fatal(err)
	}
	now := uint64(g.clock.Now())
	g.raid.Raiders.Think(rid, now)
	j, _ := g.jobs.Get(rid)
	if j.Kind != job.Attack || j.Target != hive {
		t.Fatalf("not acquired: %+v hive=%d", j, hive)
	}
	a, _ := g.animals.Get(hive)
	a.Downed = true
	g.animals.Set(hive, a)
	g.raid.Raiders.Think(rid, now+1)
	j, _ = g.jobs.Get(rid)
	t.Logf("downed hive=%d job=%+v", hive, j)
	if j.Kind == job.Attack && j.Target == hive && j.Active() {
		t.Fatal("kept downed foreign target")
	}
}

func TestRaidBudgetBound(t *testing.T) {
	g := New(Config{Seed: 7, Width: 40, Height: 40})
	for _, n := range []int{difficulty.PointsCap, math.MaxInt} {
		k := g.raidKitOf(n, faction.Neolithic)
		if len(k) <= 24 || len(k) > difficulty.PointsCap/40 {
			t.Fatalf("bounded points not bounded kit: %d", len(k))
		}
	}
	for _, tc := range []struct{ points, pct, want int }{
		{math.MaxInt, math.MaxInt, difficulty.PointsCap},
		{math.MaxInt, 50, difficulty.PointsCap},
		{20000, 50, difficulty.PointsCap},
		{15000, 50, 7500},
		{101, 33, 33},
		{0, 100, 0},
		{-1, 100, 0},
		{1, -1, 0},
	} {
		if got := difficulty.ScaleRaidPoints(tc.points, tc.pct); got != tc.want {
			t.Fatalf("scale %d * %d%% = %d, want %d", tc.points, tc.pct, got, tc.want)
		}
	}
}
