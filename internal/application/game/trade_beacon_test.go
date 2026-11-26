// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"strings"
	"testing"

	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/world/terrain"
)

// TestBeaconCacheKeyIsNoKind: the beacon reach is cached under key bit
// 254, which no building kind mask can set while there are fewer than 254
// kinds (bit 255 is the shut doors').
func TestBeaconCacheKeyIsNoKind(t *testing.T) {
	t.Parallel()
	if n := len(construction.All()); n >= 254 {
		t.Fatalf("%d building kinds: the beacon cache key (bit 254) collides with a kind mask", n)
	}
}

// TestBeaconWallCutsReach: silver walled in beside a trade beacon is out
// of its reach (the comms silver and the orbital trade view); a door
// doesn't cut the reach, a wall built in its place does, and taking one
// wall down brings the silver back: the reach follows structVer.
func TestBeaconWallCutsReach(t *testing.T) {
	t.Parallel()
	g, _, _, pawn := orbitalGame(t)
	g.orbitalArrive(uint64(g.clock.Now()))
	powerBeacon(t, g, 22, 14)
	spawn(t, g, item.Silver, 100, 26, 14) // to be walled in
	spawn(t, g, item.Silver, 7, 22, 21)   // 7 tiles south: in reach (radius 7.5)
	spawn(t, g, item.Silver, 50, 30, 14)  // 8 tiles east: out of reach
	silver := func() int { return beaconGoods(g).Counts()[item.Silver] }
	if got := silver(); got != 107 {
		t.Fatalf("open ground: beacon silver %d, want 107 (radius 7.5 reaches 22,21, not 30,14)", got)
	}
	for _, c := range [][2]int{{26, 13}, {27, 14}, {26, 15}} {
		buildNow(t, g, construction.Wall, item.Steel, construction.South, c[0], c[1])
	}
	door := buildNow(t, g, construction.Door, item.Steel, construction.South, 25, 14)
	if got := silver(); got != 107 {
		t.Fatalf("walls and a door: beacon silver %d, want 107 (doors don't cut the reach)", got)
	}
	g.removeBuildingAs(door, false) // no refund: nothing lies where the wall goes
	buildNow(t, g, construction.Wall, item.Steel, construction.South, 25, 14)
	if got := silver(); got != 7 {
		t.Fatalf("walled in: beacon silver %d, want 7", got)
	}
	openComms(t, g, pawn)
	if cv, _ := g.CommsView(); cv.BeaconSilver != 7 {
		t.Fatalf("comms beacon silver %d, want 7", cv.BeaconSilver)
	}
	g.Enqueue(command.Command{Kind: command.CommsCallTrader})
	g.ApplyPending()
	if v, ok := g.TradeView(); !ok || !v.Orbital || v.ColonySilver != 7 {
		t.Fatalf("orbital trade view %+v, want 7 silver", v)
	}
	g.removeBuilding(g.layerBldgAt(26, 15, construction.LayerObject))
	if v, _ := g.TradeView(); v.ColonySilver != 107 {
		t.Fatalf("a wall taken down: trade view silver %d, want 107", v.ColonySilver)
	}
	for range 100 * selfCheckEvery { // the self-check rebuilds and compares: no stale panic
		beaconCells(g)
	}
}

// TestBeaconRockCutsReach: natural rock cuts the reach like a wall, and
// mining it (setTerrain bumps structVer) opens it again.
func TestBeaconRockCutsReach(t *testing.T) {
	t.Parallel()
	g, _, _, _ := orbitalGame(t)
	powerBeacon(t, g, 22, 14)
	spawn(t, g, item.Silver, 40, 22, 19)
	if got := beaconGoods(g).Counts()[item.Silver]; got != 40 {
		t.Fatalf("open: %d silver, want 40", got)
	}
	for x := 15; x <= 29; x++ {
		g.setTerrain(x, 17, terrain.Rock)
	}
	if got := beaconGoods(g).Counts()[item.Silver]; got != 0 {
		t.Fatalf("behind a rock line: %d silver, want 0", got)
	}
	g.setTerrain(22, 17, terrain.Soil)
	if got := beaconGoods(g).Counts()[item.Silver]; got != 40 {
		t.Fatalf("a gap mined: %d silver, want 40", got)
	}
}

// TestBeaconGoodsUnionOfBeacons: two beacons' reaches are one set (a
// stack both reach counts once), forbidden and carried stacks stay out,
// and ids come ascending per kind.
func TestBeaconGoodsUnionOfBeacons(t *testing.T) {
	t.Parallel()
	g, _, _, _ := orbitalGame(t)
	if gd := beaconGoods(g); len(gd.Kinds()) != 0 || gd.Counts() == nil {
		t.Fatalf("no beacon: goods %+v", gd)
	}
	powerBeacon(t, g, 20, 14)
	powerBeacon(t, g, 26, 14)
	spawn(t, g, item.Silver, 10, 23, 14) // both reach it
	spawn(t, g, item.Silver, 5, 32, 14)  // the second beacon's only
	spawn(t, g, item.Steel, 3, 14, 15)   // the first's only
	spawn(t, g, item.Steel, 4, 21, 16)
	fid, _ := g.SpawnStack(item.Steel, 9, 22, 13)
	st, _ := g.stacks.Get(fid)
	st.Forbidden = true
	g.putStack(fid, st)
	gd := beaconGoods(g)
	if gd.Counts()[item.Silver] != 15 || gd.Counts()[item.Steel] != 7 {
		t.Fatalf("counts %v, want silver 15, steel 7", gd.Counts())
	}
	for _, k := range gd.Kinds() {
		ids := gd.IDs(k)
		for i := 1; i < len(ids); i++ {
			if ids[i] <= ids[i-1] {
				t.Fatalf("%s ids %v not ascending", k, ids)
			}
		}
	}
}

// TestBeaconCellsSelfCheckCatchesRawWrite: rock written past setTerrain
// (no structVer bump) is caught by the self-check, so a missed bump can't
// leave the reach stale unnoticed.
func TestBeaconCellsSelfCheckCatchesRawWrite(t *testing.T) {
	g, _, _, _ := orbitalGame(t)
	powerBeacon(t, g, 22, 14)
	beaconCells(g)
	g.terr.Set(23, 14, terrain.Rock) // raw: structVer is not bumped
	defer func() {
		r := recover()
		if s, _ := r.(string); !strings.Contains(s, "stale") {
			t.Fatalf("recovered %v, want the stale cell set panic", r)
		}
	}()
	for range 100 * selfCheckEvery {
		beaconCells(g)
	}
	t.Fatal("a raw rock write passed the self-check")
}
