// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"testing"

	"rimworld-go/internal/domain/pawn/body"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/world/terrain"
)

func TestFleeingRaiderNeedsAndStarvationThroughNativeMedicalPath(t *testing.T) {
	g, raw := fleeingRaider(t, 20, 10)
	id := uint64ToID(raw)
	for x := 0; x < g.Width(); x++ {
		g.setTerrain(x, 0, terrain.Rock)
		g.setTerrain(x, g.Height()-1, terrain.Rock)
	}
	for y := 0; y < g.Height(); y++ {
		g.setTerrain(0, y, terrain.Rock)
		g.setTerrain(g.Width()-1, y, terrain.Rock)
	}
	initial, _ := g.need.Get(id)
	g.Step(1000)
	after, _ := g.need.Get(id)
	if !(after.Hunger < initial.Hunger && after.Rest < initial.Rest) {
		t.Fatalf("fleeing raider needs were frozen: initial %+v now %+v", initial, after)
	}
	if g.soc == nil {
		t.Fatal("raider has no mood progression")
	}
	if _, ok := g.soc.moodM[id]; !ok {
		t.Fatal("raider mood was never stepped")
	}
	h := saveLoad(t, g)
	saved, _ := h.need.Get(id)
	if saved != after || h.StateHash() != g.StateHash() {
		t.Fatalf("raider needs changed across save/load: %+v vs %+v", after, saved)
	}
	views := g.raiderViews()
	if len(views) != 1 || views[0].Hunger != needs.Percent(after.Hunger) || views[0].Rest != needs.Percent(after.Rest) {
		t.Fatalf("raider read model differs from authoritative needs: %+v, %+v", views, after)
	}
	g.need.Set(id, needs.Pair{Hunger: 0, Rest: needs.Full}.Settle())
	g.Step(clock.TicksPerHour * 2)
	if b := g.bodies[id]; b == nil || body.Malnutrition(b) == 0 {
		t.Fatalf("raider missed the hourly malnutrition path: %+v", b)
	}
	for hour := uint64(0); hour < 100; hour++ {
		if v, _ := g.vital.Get(id); v.Status == health.Dead {
			break
		}
		stepMalnutrition(g, hour*clock.TicksPerHour)
	}
	v, _ := g.vital.Get(id)
	if v.Status != health.Dead || v.Cause != health.CauseNoFood {
		t.Fatalf("unfed trapped raider never starved: %+v malnutrition %d", v, body.Malnutrition(g.bodies[id]))
	}
}

func TestVisitorNeedsProgressWithoutColonistOnlyRecreation(t *testing.T) {
	g, _, vis, _ := guestScene(t)
	before, _ := g.need.Get(vis)
	g.Step(1000)
	after, _ := g.need.Get(vis)
	if after.Hunger >= before.Hunger || after.Rest >= before.Rest {
		t.Fatalf("visitor need progression: before %+v after %+v", before, after)
	}
	if _, ok := g.joy[vis]; ok {
		t.Fatal("visitor acquired a colonist-only recreation meter")
	}
}
