// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"testing"

	"rimworld-go/internal/application/game/care"
	"rimworld-go/internal/application/game/hit"
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/difficulty"
	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/pawn/body"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/sim/command"
)

func TestTransportPodCrashActualRescue(t *testing.T) {
	g := New(Config{Seed: 3, Width: 24, Height: 18})
	doc := worker(t, g, 8, 8)
	placeBuilt(t, g, construction.SleepingSpot, construction.South, 15, 7)
	if !g.FireIncident("transport_pod_crash") {
		t.Fatal("crash failed")
	}
	visitors := g.visitorIDs()
	if len(visitors) != 1 {
		t.Fatalf("crash visitors: %v", visitors)
	}
	id := visitors[0]
	v, _ := g.vital.Get(id)
	if v.Status != health.Downed || g.standsUp(id, v) || g.personOf(id).Faction != 0 {
		t.Fatalf("crash victim: %+v", v)
	}
	shells := 0
	for _, bid := range g.bldgIDs {
		b, _ := g.bldgs.Get(bid)
		if b.Kind == construction.PodShell {
			shells++
		}
	}
	if shells != 1 {
		t.Fatalf("crash shells=%d", shells)
	}
	g = saveLoad(t, g)
	if why := g.care.Force(doc, command.OrderRescue, id); why != "" {
		t.Fatalf("crash rescue refused: %s", why)
	}
	if !runCare(g, 1500, func() bool { return g.inBed(id) }, doc) {
		t.Fatal("crash survivor never reached bed")
	}
}

func TestNativeDownedCrawlBedAndClaim(t *testing.T) {
	g := New(Config{Seed: 6, Width: 24, Height: 14})
	id := worker(t, g, 3, 5)
	placeBuilt(t, g, construction.SleepingSpot, construction.South, 8, 5)
	bed := g.bldgIDs[len(g.bldgIDs)-1]
	for _, part := range []body.Part{body.LegL, body.LegR} {
		g.harm(id, hit.Blow{Dmg: 22, Kind: body.Cut, Part: part, Placed: true}, 0, health.CauseInjury)
	}
	if g.alive(id) {
		g.down(id, health.CauseInjury)
	}
	start := g.positionOf(id)
	g.clock.Advance(care.CrawlDelay - 1)
	g.phaseMovement()
	if g.positionOf(id) != start {
		t.Fatal("crawled before delay")
	}
	g.clock.Advance(1)
	g.phaseMovement()
	if g.positionOf(id) == start || g.inBed(id) {
		t.Fatalf("not one physical crawl step: %v", g.positionOf(id))
	}
	g = saveLoad(t, g)
	for range 10 {
		g.clock.Advance(care.CrawlStepTicks)
		g.phaseMovement()
		if g.inBed(id) {
			break
		}
	}
	b, _ := g.bldgs.Get(bed)
	if !g.inBed(id) || construction.BedSlot(b, uint64(id)) < 0 {
		t.Fatalf("crawl did not claim destination: pos=%v bed=%+v", g.positionOf(id), b)
	}
}

func TestNativeEnemyDownedDeathOnlyCombat(t *testing.T) {
	deaths, downs := 0, 0
	for i := range 32 {
		g := bareSetupGame(difficulty.Seasoned)
		g.stream(streamBody).IntN(1000)
		for range i {
			g.stream(streamBody).IntN(1000)
		}
		attacker := worker(t, g, 4, 4)
		enemy := spawnRaiderAt(t, g, 8, 4, 0)
		control := spawnRaiderAt(t, g, 9, 4, 0)
		g.hurt(enemy, health.DownedInjury+10, attacker)
		g.hurt(control, health.DownedInjury+10, 0)
		a, _ := g.vital.Get(enemy)
		c, _ := g.vital.Get(control)
		if c.Status != health.Downed {
			t.Fatalf("environmental control %+v", c)
		}
		switch a.Status {
		case health.Dead:
			deaths++
		case health.Downed:
			downs++
		default:
			t.Fatalf("combat outcome %+v", a)
		}
	}
	if deaths == 0 || downs == 0 {
		t.Fatalf("downed combat chance: dead=%d down=%d", deaths, downs)
	}
}

func TestForbidBedReleasesClaimAndRejectsOwnership(t *testing.T) {
	g := New(Config{Seed: 4, Width: 18, Height: 12})
	id := worker(t, g, 2, 2)
	placeBuilt(t, g, construction.SleepingSpot, construction.South, 6, 5)
	bed := g.bldgIDs[len(g.bldgIDs)-1]
	assignOwner(g, bed, id)
	g.jobs.Set(id, job.Record{Kind: job.Sleep, Target: bed, State: job.Do})
	g.setForbidden(bed, true)
	assignOwner(g, bed, id)
	if b := bedOf(g, bed); construction.BedSlot(b, uint64(id)) >= 0 || !b.Forbidden {
		t.Fatalf("forbidden bed claim %+v", b)
	}
	if j, _ := g.jobs.Get(id); j.Kind == job.Sleep && j.Active() {
		t.Fatalf("forbidden bed still used: %+v", j)
	}
}

func TestHiddenFactionObjectsResolveActualOrigins(t *testing.T) {
	g := New(Config{Seed: 3, Width: 18, Height: 12})
	if got := g.SpecialFactions(); len(got) != 4 {
		t.Fatalf("hidden factions %+v", got)
	}
	id := spawnRaiderAt(t, g, 8, 4, 0)
	g.personOf(id).Origin = faction.AncientHostile
	a, ok := g.ActorFaction(id)
	if !ok || !a.Hidden || a.ID >= 0 || a.Relation != "hostile" {
		t.Fatalf("ancient faction %+v", a)
	}
	g = saveLoad(t, g)
	b, ok := g.ActorFaction(id)
	if !ok || b.ID != a.ID || b.Name != a.Name || len(g.Factions()) != 0 {
		t.Fatalf("hidden identity changed %+v", b)
	}
}

func TestHostilePodCrashRequiresCapture(t *testing.T) {
	for seed := 1; seed <= 32; seed++ {
		g := New(Config{Seed: uint64(seed), Width: 24, Height: 18})
		doc := worker(t, g, 8, 8)
		g.ng.factions = []faction.Faction{{ID: 0, Type: faction.Drifters, Name: "Hostile", Relation: faction.Hostile, Goodwill: -100}}
		hutWalls(t, g, 1, 1, 5, 5)
		placeBuilt(t, g, construction.SleepingSpot, construction.South, 3, 3)
		bed := g.bldgIDs[len(g.bldgIDs)-1]
		g.Enqueue(command.Command{Kind: command.SetBedForPrisoners, Target: uint64(bed), Enabled: true})
		g.Step(1)
		if !g.FireIncident("transport_pod_crash") {
			t.Fatal("crash failed")
		}
		if len(g.raiderIDs) == 0 || !g.isRaider(g.raiderIDs[0]) {
			continue // visitors share the map roster but not the enemy record
		}
		id := g.raiderIDs[0]
		v, _ := g.vital.Get(id)
		if v.Status != health.Downed || g.standsUp(id, v) || len(g.visitorIDs()) != 0 {
			t.Fatalf("hostile crash %+v", v)
		}
		if why := g.care.Force(doc, command.OrderRescue, id); why == "" {
			t.Fatal("hostile rescue accepted")
		}
		capture(t, g, doc, doc, id)
		if !g.inBed(id) {
			t.Fatal("captured victim not in prison bed")
		}
		return
	}
	t.Fatal("no hostile victim among deterministic seeds")
}
