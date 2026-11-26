// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"bytes"
	"rimworld-go/internal/application/game/raid"
	"rimworld-go/internal/application/game/savefmt"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/world/biome"
	"testing"
)

func TestFrozenTravellingPrisoner(t *testing.T) {
	g := worldGame(t)
	tile, ada := plainNear(t, g), g.pawnIDs[0]
	arriveAt(t, g, tile, ada)
	g.Enqueue(command.Command{Kind: command.EnterMap, Target: g.worldS().caravans[0].C.ID})
	g.Step(1)
	m := g.maps[1]
	m.climate = biome.DefOf(biome.TemperateForest).Climate
	g.mapState = m
	pos, _ := g.positions.Get(ada)
	x, y, ok := g.combat.Tiles.WalkableNear(pos.X, pos.Y, 8)
	if !ok {
		t.Fatal("no ground")
	}
	g.raidGroups = append(g.raidGroups, raid.Group{Size: 1})
	rid := spawnRaider(g, 0, 0, item.Revolver, x, y)
	g.makePrisoner(rid, ada)
	g.mapState = g.view
	g.Enqueue(command.Command{Kind: command.CaravanReform, Target: uint64(m.id), Map: m.id, MapSet: true})
	g.Step(1)
	if len(g.maps) != 1 || !g.isStashed(rid) {
		t.Fatal("fixture did not pack prisoner into caravan")
	}
	var old *raid.Raider
	for _, r := range g.worldSt.stashed {
		if r.ID == rid {
			old = r.Raider
		}
	}
	if old == nil {
		t.Fatal("fixture has no carried raider record")
	}
	// The destination already contains earlier raid groups, so the returning
	// prisoner receives a different group index. No artificial live mutation
	// is needed after capture: native caravan arrival performs the assignment.
	for len(g.raidGroups) <= old.Group {
		g.raidGroups = append(g.raidGroups, raid.Group{Over: true})
	}
	auditFrozen(t, g)
	stamp := savefmt.Stamp{Name: "travelling-prisoner"}
	var want bytes.Buffer
	if err := EncodeSaveAs(g, &want, stamp); err != nil {
		t.Fatal(err)
	}
	f, err := FreezeSaveAs(g, stamp)
	if err != nil {
		t.Fatal(err)
	}
	oldGroup := old.Group
	for i := 0; i < 400 && fleetOf(g).Of(ada) != nil; i++ {
		worldForward(g, clock.TicksPerHour)
	}
	if g.isStashed(rid) || !g.isRaider(rid) {
		t.Fatal("prisoner did not arrive")
	}
	if now := g.raiders[rid].Group; now == oldGroup {
		t.Fatal("fixture: destination group unchanged")
	}
	if got := frozenRaw(t, f); !bytes.Equal(got, want.Bytes()) {
		t.Fatalf("native prisoner arrival changed captured bytes; captured group was %d, live record now %d", oldGroup, old.Group)
	}
}
