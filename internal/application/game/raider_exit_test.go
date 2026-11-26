// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"testing"

	"rimworld-go/internal/application/game/raid"
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/world/terrain"
)

func fleeingRaider(t *testing.T, x, y int) (*Game, uint64) {
	t.Helper()
	g, ada := def4Game(t, 50, 30, 40, 15)
	g.draft[ada] = true
	g.combat.Drafts.SetFireAtWill(ada, false)
	id := turretRaider(t, g, x, y)
	r := g.raiders[id]
	r.Mode = raid.Fleeing
	r.KidnapAt = ^uint64(0)
	g.raiders[id] = r
	return g, uint64(id)
}

func TestFleeingRaiderUsesAnotherEdgeWhenNearestIsRock(t *testing.T) {
	g, rid := fleeingRaider(t, 20, 7)
	for x := 0; x < g.Width(); x++ {
		g.setTerrain(x, 0, terrain.Rock)
	}
	g.Step(1)
	j, _ := g.jobs.Get(uint64ToID(rid))
	if j.Kind != job.Move || j.TY == 0 || j.State == job.Fail {
		t.Fatalf("selected unavailable north edge: %+v", j)
	}
	for tick := 0; tick < 3000 && g.onMap(uint64ToID(rid)); tick++ {
		g.Step(1)
	}
	if g.onMap(uint64ToID(rid)) {
		j, _ = g.jobs.Get(uint64ToID(rid))
		t.Fatalf("raider stayed on map; job %+v pos %+v", j, g.positions.Ref(uint64ToID(rid)))
	}
}

func TestFleeingRaiderBreachesEnclosureAndLeaves(t *testing.T) {
	for _, prisoner := range []bool{false, true} {
		t.Run(map[bool]string{false: "raid", true: "escape-prisoner"}[prisoner], func(t *testing.T) {
			g, rid := fleeingRaider(t, 20, 10)
			id := uint64ToID(rid)
			r := g.raiders[id]
			r.Escaping = prisoner
			g.raiders[id] = r
			built := blockVisitorAt(t, g, id, 20, 10, 1, false)
			sawBreach, broke := false, false
			for tick := 0; tick < 25000 && g.onMap(id); tick++ {
				g.Step(1)
				if j, _ := g.jobs.Get(id); j.Kind == job.Breach {
					sawBreach = true
				}
				for bid := range built {
					if _, ok := g.bldgs.Get(bid); !ok {
						broke = true
					}
				}
			}
			if !sawBreach || !broke || g.onMap(id) {
				j, _ := g.jobs.Get(id)
				t.Fatalf("escape=%v breach=%v broke=%v onMap=%v job=%+v", prisoner, sawBreach, broke, g.onMap(id), j)
			}
		})
	}
}
