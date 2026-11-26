// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"testing"

	"rimworld-go/internal/application/game/worldpawn"
	"rimworld-go/internal/domain/pawn/body"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/sim/entity"
)

// heldGame is Ada hurt, then held off the map by faction 1 as kidnapped.
func heldGame(t *testing.T) (*Game, entity.EntityID) {
	t.Helper()
	g := New(Config{Seed: 3, Width: 30, Height: 20})
	ada, _ := g.SpawnNamedPawn(5, 10, "Ada")
	fed(g, ada)
	g.ensurePerson(ada)
	g.hurt(ada, 30, 0)
	if b := g.bodies[ada]; b == nil || !b.TendDue(uint64(g.clock.Now())) {
		t.Fatalf("Ada should have an untended wound: %+v", g.bodies[ada])
	}
	if !g.stashPawn(ada, 1) {
		t.Fatal("stash failed")
	}
	g.setWhere(ada, whereKidnapped)
	return g, ada
}

func freshWorldPawn(t *testing.T) (*Game, entity.EntityID) {
	t.Helper()
	g := New(Config{Seed: 4, Width: 30, Height: 20})
	id, _ := g.SpawnNamedPawn(5, 10, "Away")
	fed(g, id)
	g.ensurePerson(id)
	if !g.stashPawn(id, 1) {
		t.Fatal("stash failed")
	}
	g.setWhere(id, whereOffMap)
	return g, id
}

func TestKidnappedPawnIsTendedAndHealsWhileHeld(t *testing.T) {
	t.Parallel()
	g, ada := heldGame(t)
	start := g.bodies[ada].Damage()
	g.Step(3 * 2500 * 2) // two auto-tend periods
	b := g.bodies[ada]
	if b == nil {
		t.Fatal("body lost")
	}
	if b.TendDue(uint64(g.clock.Now())) {
		t.Fatalf("held pawn never auto-tended: %+v", b.Wounds)
	}
	if b.BleedPerDay() != 0 {
		t.Fatalf("still bleeding: %d", b.BleedPerDay())
	}
	if !g.isStashed(ada) {
		t.Fatal("should remain held")
	}
	g.Step(60000 - 3*2500*2)
	b = g.bodies[ada]
	if v, _ := g.vital.Get(ada); v.Status == health.Dead {
		t.Fatal("died while held")
	}
	if b.Damage() >= start {
		t.Fatalf("held pawn did not heal: %d -> %d", start, b.Damage())
	}
}

func TestKidnappedPawnReturnsAsElapsedHealingLeftIt(t *testing.T) {
	t.Parallel()
	g, ada := heldGame(t)
	start := g.bodies[ada].Damage()
	g.Step(2500 * 6) // two tend periods: tended, only partly healed
	damage := g.bodies[ada].Damage()
	if damage >= start || damage == 0 {
		t.Fatalf("expected partial healing: %d -> %d", start, damage)
	}
	if !g.unstashPawn(ada, 5, 10) {
		t.Fatal("unstash failed")
	}
	if got := g.bodies[ada].Damage(); got != damage {
		t.Fatalf("return changed the body: %d -> %d", damage, got)
	}
}

func TestCaravanPawnIsNotAutoTendedAsKidnapped(t *testing.T) {
	t.Parallel()
	g, ada := heldGame(t)
	g.setWhere(ada, whereCaravan)
	g.Step(2500 * 6)
	if !g.bodies[ada].TendDue(uint64(g.clock.Now())) {
		t.Fatal("a caravan pawn must not get the world-pawn auto-tend")
	}
}

func TestWorldPawnPrisonerUsesWorldAutoTend(t *testing.T) {
	t.Parallel()
	g, id := heldGame(t)
	g.setWhere(id, wherePrisoner)
	g.Step(15000)
	if g.bodies[id].TendDue(uint64(g.clock.Now())) {
		t.Fatal("stashed prisoners must use the world-pawn auto-tend owner")
	}
}

func TestWorldPawnNeedsAdvanceWhileOffMap(t *testing.T) {
	t.Parallel()
	g, id := freshWorldPawn(t)
	n, _ := g.need.Get(id)
	n.Hunger = needs.Full
	g.need.Set(id, n)
	g.Step(200)
	after, _ := g.need.Get(id)
	if after.Hunger >= needs.Full {
		t.Fatalf("off-map hunger stayed frozen: %+v", after)
	}
}

func TestWorldPawnMothballsAtBoundary(t *testing.T) {
	t.Parallel()
	g, id := freshWorldPawn(t)
	g.Step(15001) // includes simulation tick 15000
	if !g.worldSt.worldPawnMothballed[id] {
		t.Fatal("eligible world pawn was not mothballed at 15000 ticks")
	}
}

func TestWorldPawnMothballPreventionMatchesHealthState(t *testing.T) {
	t.Parallel()
	g, id := freshWorldPawn(t)
	v, _ := g.vital.Get(id)
	b := g.ensureBody(id, &v)
	if !worldpawn.ShouldMothball(offMapPawns{g}, id) {
		t.Fatal("healthy world pawn should be mothball-eligible")
	}
	b.Wounds = []body.Wound{{Part: body.ArmL, Dmg: 10, Kind: body.Cut}}
	if worldpawn.ShouldMothball(offMapPawns{g}, id) {
		t.Fatal("open wound must prevent mothball")
	}
	b.Wounds[0].Permanent = true
	if !worldpawn.ShouldMothball(offMapPawns{g}, id) {
		t.Fatal("permanent wound should not prevent mothball")
	}
	b.Wounds = nil
	if !b.Develop(body.CondAsthma, body.LungL, 0) {
		t.Fatal("failed to add chronic asthma")
	}
	if !worldpawn.ShouldMothball(offMapPawns{g}, id) {
		t.Fatal("low-priority chronic condition should allow mothball")
	}
}
