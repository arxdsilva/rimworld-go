// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"slices"
	"testing"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/caravan"
)

func recoveryGame(t *testing.T) (*Game, command.Command, item.Stack) {
	t.Helper()
	g := worldGame(t)
	tile := plainNear(t, g)
	cid := arriveAt(t, g, tile, g.pawnIDs[0])
	mid, ok := DevOpenMap(g, tile)
	if !ok {
		t.Fatal("site did not open")
	}
	g.ViewMap(mid)
	for y := 0; y < g.grid.Height(); y++ {
		for x := 0; x < g.grid.Width(); x++ {
			if !g.grid.Walkable(x, y) || g.fogged(x, y) {
				continue
			}
			sid, err := g.SpawnStack(item.Cloth, 5, x, y)
			if err != nil {
				t.Fatal(err)
			}
			st, _ := g.stacks.Get(sid)
			st.Wear = 7
			g.putStack(sid, st)
			return g, command.Command{Kind: command.RecoverCargoToCaravan, Map: uint32(mid), MapSet: true, Zone: uint64(cid), Target: uint64(sid)}, st
		}
	}
	t.Fatal("no visible walkable cell")
	return nil, command.Command{}, item.Stack{}
}

func TestNativeCaravanRecoveryConservesCargo(t *testing.T) {
	g, c, source := recoveryGame(t)
	sid := entity.EntityID(c.Target)
	if got, ok := CargoSource(g, MapID(c.Map), sid); !ok || got != source {
		t.Fatalf("source = %+v, %v", got, ok)
	}
	c.Count = 2
	applyNow(g, c)
	part := source
	part.Qty = 3
	if got, ok := CargoSource(g, MapID(c.Map), sid); !ok || got != part {
		t.Fatalf("partial debit = %+v, %v", got, ok)
	}
	part.Qty, part.X, part.Y = 2, 0, 0
	if got := travelOf(g).Goods(c.Zone); !slices.Equal(got, []item.Stack{part}) {
		t.Fatalf("cargo = %+v", got)
	}
	if fleetOf(g).At(entity.EntityID(c.Zone)).C.Load <= 0 {
		t.Fatal("load was not updated")
	}
	g = saveLoad(t, g)
	g.worldS().planet, g.worldS().planetTried = testPlanet().Fork(), true
	g.ViewMap(MapID(c.Map))
	c.Count = 0
	applyNow(g, c)
	if g.stacks.Has(sid) {
		t.Fatal("source still exists")
	}
	part.Qty = 5
	if got := travelOf(g).Goods(c.Zone); !slices.Equal(got, []item.Stack{part}) {
		t.Fatalf("cargo after restore = %+v", got)
	}
	before := g.StateHash()
	applyNow(g, c)
	if g.StateHash() != before {
		t.Fatal("stale command mutated state")
	}
	g.ViewMap(0)
	count := func() int {
		n := 0
		for _, id := range g.itemIDs {
			if st, ok := g.stacks.Get(id); ok && st.Kind == source.Kind && st.Wear == source.Wear {
				n += st.Qty
			}
		}
		return n
	}
	old := count()
	applyNow(g, command.Command{Kind: command.CaravanReturn, Target: c.Zone})
	for i := 0; i < 90 && fleetOf(g).At(entity.EntityID(c.Zone)) != nil; i++ {
		worldForward(g, 2500)
	}
	if fleetOf(g).At(entity.EntityID(c.Zone)) != nil || count() != old+5 {
		t.Fatalf("return did not deliver recovered goods: %d -> %d", old, count())
	}
}

func TestNativeCaravanRecoveryRefusesWrongSource(t *testing.T) {
	g, c, source := recoveryGame(t)
	sid, cid := entity.EntityID(c.Target), entity.EntityID(c.Zone)
	cv := fleetOf(g).At(cid)
	check := func(name string, cmd command.Command) {
		t.Helper()
		// Check the actual source map even if the queued command is for home.
		before, _ := g.stacks.Get(sid)
		g.ViewMap(MapID(cmd.Map))
		applyNow(g, cmd)
		g.ViewMap(MapID(c.Map))
		if after, ok := g.stacks.Get(sid); !ok || after != before || len(travelOf(g).Goods(c.Zone)) != 0 {
			t.Fatalf("%s changed goods: %+v, %v", name, after, ok)
		}
	}
	bad := c
	bad.Map = 0
	check("wrong map", bad)
	bad = c
	bad.MapSet = false
	check("implicit map", bad)
	bad = c
	bad.Pawn = 999
	check("wrong carrier", bad)
	bad = c
	bad.Zone++
	check("missing caravan", bad)
	bad = c
	bad.Count = 6
	check("too many", bad)
	bad.Count = -1
	check("negative", bad)
	for _, flags := range []item.Stack{
		{Unclaimed: true}, {Forbidden: true}, {Carrier: 999},
	} {
		st := source
		st.Unclaimed, st.Forbidden, st.Carrier = flags.Unclaimed, flags.Forbidden, flags.Carrier
		g.putStack(sid, st)
		bad = c
		bad.Pawn = uint64(st.Carrier)
		check("ineligible source", bad)
	}
	g.putStack(sid, source)
	if !g.reserve.TryCount(999, sid, 1, source.Qty) {
		t.Fatal("reserve failed")
	}
	check("reserved", c)
	g.reserve.ReleaseTarget(sid)
	cv.C.State = caravan.Travelling
	check("not arrived", c)
	cv.C.State = caravan.Arrived
	tile := cv.C.Tile
	cv.C.Tile = cv.Home
	check("wrong world tile", c)
	cv.C.Tile = tile
	stash := &g.worldSt.stashed[slices.IndexFunc(g.worldSt.stashed, func(r stashRec) bool { return r.Holder == cid && r.Beast == nil })]
	stash.Holder++
	check("crew not held", c)
	stash.Holder--
	heavy := source
	heavy.Kind, heavy.Qty = item.Steel, int(^uint(0)>>1)
	g.putStack(sid, heavy)
	check("overload", c)
	g.putStack(sid, source)
	// A real resident holding a running job's cargo cannot be debited.
	pid, err := g.SpawnNamedPawn(source.X, source.Y, "Carrier")
	if err != nil {
		t.Fatal(err)
	}
	held := source
	held.Carrier = pid
	g.putStack(sid, held)
	g.jobs.Set(pid, job.Record{Kind: job.Haul, State: job.Goto, Carry: sid})
	bad = c
	bad.Pawn = uint64(pid)
	check("active haul", bad)
	g.jobs.Delete(pid)
	applyNow(g, bad)
	if g.stacks.Has(sid) || len(travelOf(g).Goods(c.Zone)) != 1 {
		t.Fatal("eligible held cargo not transferred")
	}
}
