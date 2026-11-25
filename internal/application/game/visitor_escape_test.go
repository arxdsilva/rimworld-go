// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"testing"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/terrain"
	"rimworld-go/pkg/mathx"
)

func enclosedVisitor(t *testing.T, layers int, withDoor bool) (*Game, entity.EntityID, map[entity.EntityID]int) {
	t.Helper()
	g, doc, guest, _ := guestScene(t)
	g.draft[doc] = true
	built := blockVisitorAt(t, g, guest, 20, 6, layers, withDoor)
	g.setVisitorStay(guest, uint64(g.clock.Now()))
	return g, guest, built
}

func blockVisitorAt(t *testing.T, g *Game, guest entity.EntityID, cx, cy, layers int, withDoor bool) map[entity.EntityID]int {
	t.Helper()
	g.positions.Set(guest, Position{X: cx, Y: cy})
	g.invalidateCreatures()
	for y := cy - layers - 1; y <= cy+layers+1; y++ {
		for x := cx - layers - 1; x <= cx+layers+1; x++ {
			g.setTerrain(x, y, terrain.Soil)
		}
	}
	built := make(map[entity.EntityID]int)
	for radius := 2; radius < 2+layers; radius++ {
		for y := cy - radius; y <= cy+radius; y++ {
			for x := cx - radius; x <= cx+radius; x++ {
				if max(mathx.Abs(x-cx), mathx.Abs(y-cy)) != radius {
					continue
				}
				g.setTerrain(x, y, terrain.Soil)
				k := construction.Wall
				if withDoor && radius == 2 && x == cx && y == cy-2 {
					k = construction.Door
				}
				id := buildNow(t, g, k, item.Wood, construction.South, x, y)
				b, _ := g.bldgs.Get(id)
				built[id] = b.HP()
				if k == construction.Door {
					// Model a locked, impassable door. Usable doors stay
					// part of the ordinary visitor walk path.
					g.grid.SetWalkable(x, y, false)
				}
			}
		}
	}
	return built
}

func assertVisitorBreaksOut(t *testing.T, layers int, withDoor bool) {
	t.Helper()
	g, id, built := enclosedVisitor(t, layers, withDoor)
	sawBreach, damaged, destroyed := false, false, false
	for tick := 0; tick < 25000 && g.onMap(id); tick++ {
		g.Step(1)
		j, _ := g.jobs.Get(id)
		if j.Kind == job.Breach {
			sawBreach = true
		}
		for bid, initial := range built {
			if b, ok := g.bldgs.Get(bid); !ok {
				destroyed = true
			} else if b.HP() < initial {
				damaged = true
			}
		}
	}
	if !sawBreach || !damaged || !destroyed {
		t.Fatalf("breach job=%v damage=%v destruction=%v", sawBreach, damaged, destroyed)
	}
	if g.onMap(id) || g.personOf(id).Where != whereOffMap {
		j, _ := g.jobs.Get(id)
		t.Fatalf("visitor failed to walk out; job=%+v pos=%+v", j, g.positions.Ref(id))
	}
}

func TestVisitorBreaksWallAndWalksOut(t *testing.T) {
	t.Parallel()
	assertVisitorBreaksOut(t, 1, false)
}

func TestVisitorBreaksTwoWallsAndWalksOut(t *testing.T) {
	t.Parallel()
	assertVisitorBreaksOut(t, 2, false)
}

func TestVisitorBreaksBlockedDoorAndWalksOut(t *testing.T) {
	t.Parallel()
	assertVisitorBreaksOut(t, 1, true)
}

func TestVisitorUsesOpenExitWithoutBreach(t *testing.T) {
	t.Parallel()
	g, id, built := enclosedVisitor(t, 1, false)
	opening := g.layerBldgAt(20, 8, construction.LayerObject)
	if opening == 0 {
		t.Fatal("exit wall is missing")
	}
	g.removeBuilding(opening)
	for tick := 0; tick < 4000 && g.onMap(id); tick++ {
		g.Step(1)
		j, _ := g.jobs.Get(id)
		if j.Kind == job.Breach {
			t.Fatalf("visitor breached a wall despite an open exit: %+v", j)
		}
	}
	if g.onMap(id) {
		t.Fatal("visitor did not walk through the opening")
	}
	for bid, initial := range built {
		if bid == opening {
			continue
		}
		b, ok := g.bldgs.Get(bid)
		if !ok || b.HP() != initial {
			t.Fatalf("the accessible exit damaged wall %d: initial %d, present %v", bid, initial, ok)
		}
	}
}

func TestVisitorEscapeSaveRestoreAndRepeat(t *testing.T) {
	t.Parallel()
	g, id, _ := enclosedVisitor(t, 2, false)
	for tick := 0; tick < 1500; tick++ {
		g.Step(1)
		j, _ := g.jobs.Get(id)
		if j.Kind == job.Breach && j.State == job.Do && j.WorkLeft < 50 {
			break
		}
		if tick == 1499 {
			t.Fatalf("visitor never started breaching: %+v", j)
		}
	}
	h := saveLoad(t, g)
	if g.StateHash() != h.StateHash() {
		t.Fatal("breach state differs immediately after restore")
	}
	for tick := 0; tick < 2500; tick += 25 {
		g.Step(25)
		h.Step(25)
		if a, b := g.StateHash(), h.StateHash(); a != b {
			t.Fatalf("escape diverged at tick %d: %x vs %x", tick+25, a, b)
		}
	}
}

func TestTraderAndGuardBreakOutAndCloseCaravan(t *testing.T) {
	t.Parallel()
	g, caravan := caravanGame(t)
	if len(caravan.Members) < 2 {
		t.Fatal("the caravan has no guard")
	}
	trader, guard := caravan.Trader, caravan.Members[0]
	if guard == trader {
		guard = caravan.Members[1]
	}
	g.draft[g.pawnIDs[0]] = true
	traderWalls := blockVisitorAt(t, g, trader, 15, 18, 1, false)
	guardWalls := blockVisitorAt(t, g, guard, 42, 18, 1, false)
	now, goodwill := uint64(g.clock.Now()), g.factionByID(caravan.Faction).Goodwill
	caravan.LeaveAt = now
	for i := range g.worldSt.visitors {
		g.worldSt.visitors[i].LeaveAt = now
	}
	for tick := 0; tick < 25000 && (g.onMap(trader) || g.onMap(guard) || g.trade.caravan != nil); tick++ {
		g.Step(1)
	}
	if g.onMap(trader) || g.onMap(guard) || g.trade.caravan != nil {
		t.Fatal("trader or guard remained, or the caravan remained open")
	}
	for _, walls := range []map[entity.EntityID]int{traderWalls, guardWalls} {
		destroyed := false
		for bid := range walls {
			if _, ok := g.bldgs.Get(bid); !ok {
				destroyed = true
			}
		}
		if !destroyed {
			t.Fatal("trader or guard left without breaching its enclosure")
		}
	}
	if got := g.factionByID(caravan.Faction).Goodwill; got != goodwill {
		t.Fatalf("peaceful caravan departure changed goodwill: %d -> %d", goodwill, got)
	}
}

func TestVisitorReplansWhenWallBlocksCurrentMove(t *testing.T) {
	t.Parallel()
	g, doc, guest, _ := guestScene(t)
	g.draft[doc] = true
	g.positions.Set(guest, Position{X: 20, Y: 6})
	g.invalidateCreatures()
	g.setVisitorStay(guest, uint64(g.clock.Now()))
	g.Step(1)
	j, _ := g.jobs.Get(guest)
	if j.Kind != job.Move || j.PathI >= len(j.Path) {
		t.Fatalf("no departure path to obstruct: %+v", j)
	}
	point := j.Path[j.PathI]
	buildNow(t, g, construction.Wall, item.Wood, construction.South, point.X, point.Y)
	for tick := 0; tick < 6000 && g.onMap(guest); tick++ {
		g.Step(1)
	}
	if g.onMap(guest) {
		t.Fatal("visitor did not replan its departure after the new wall")
	}
}

func TestVisitorCannotTunnelThroughNaturalRock(t *testing.T) {
	t.Parallel()
	g, doc, guest, _ := guestScene(t)
	g.draft[doc] = true
	g.positions.Set(guest, Position{X: 20, Y: 6})
	g.invalidateCreatures()
	for y := 4; y <= 8; y++ {
		for x := 18; x <= 22; x++ {
			if max(mathx.Abs(x-20), mathx.Abs(y-6)) == 2 {
				g.setTerrain(x, y, terrain.Rock)
			}
		}
	}
	r := &g.worldSt.visitors[0]
	r.Leaving, r.LeaveAt = true, 0
	visitsOf(g).Tick(aidOf(g), 1000)
	if !g.onMap(guest) {
		t.Fatal("rock trapped visitor vanished before the terminal timeout")
	}
	if j, _ := g.jobs.Get(guest); j.Kind == job.Breach {
		t.Fatal("ordinary visitor started a sapper's rock breach")
	}
	visitsOf(g).Tick(aidOf(g), uint64(clock.TicksPerDay))
	if g.onMap(guest) {
		t.Fatal("irrecoverable rock enclosure retained the visitor past terminal cleanup")
	}
}
