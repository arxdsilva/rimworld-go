// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"testing"

	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/trade"
	"rimworld-go/internal/domain/sim/entity"
)

// powerBeacon builds a trade beacon at (x, y) on its own grid: a fuelled
// wood generator four tiles west, joined by three conduits, then balances.
// It returns the beacon and its generator (a beacon only reaches while
// powered).
func powerBeacon(t *testing.T, g *Game, x, y int) (beaconID, gen entity.EntityID) {
	t.Helper()
	beaconID = buildNow(t, g, construction.TradeBeacon, item.Steel, construction.South, x, y)
	gen = psFuelled(t, g, x-4, y)
	psConduits(t, g, y, x-3, x-1)
	g.Step(powerInterval)
	if !g.powered(beaconID) {
		t.Fatalf("beacon %d at %d,%d is not powered by its generator", beaconID, x, y)
	}
	return beaconID, gen
}

// reach is the silver the beacons reach, read through the cached cells with
// the self-check on (a stale set panics).
func reach(g *Game) int {
	for range selfCheckEvery {
		beaconCells(g)
	}
	return beaconGoods(g).Counts()[item.Silver]
}

// TestUnpoweredBeaconGivesNoReachCallOrGift: a beacon with no generator
// reaches nothing, the orbital ship can't be called, and gifts are refused.
func TestUnpoweredBeaconGivesNoReachCallOrGift(t *testing.T) {
	t.Parallel()
	g, _, _, pawn := orbitalGame(t)
	g.orbitalArrive(uint64(g.clock.Now()))
	buildNow(t, g, construction.TradeBeacon, item.Steel, construction.South, 22, 14)
	spawn(t, g, item.Silver, 100, 23, 15)
	g.Step(powerInterval)
	if got := reach(g); got != 0 {
		t.Fatalf("unpowered beacon reaches %d silver", got)
	}
	openComms(t, g, pawn)
	cv, _ := g.CommsView()
	if cv.Orbital.CallOK || cv.Orbital.CallReason != ReasonNoBeacon || cv.BeaconSilver != 0 {
		t.Fatalf("comms %+v", cv)
	}
	if ReasonNoBeacon != "needs a powered trade beacon" {
		t.Fatalf("reason %q", ReasonNoBeacon)
	}
	for _, f := range g.ng.factions {
		if row := g.commsFaction(f, false, 0, 0); f.Type.Human() && !f.Defeated && !f.Besieged && (row.Gift.Allowed || row.Gift.Reason != ReasonNoBeacon) {
			t.Fatalf("gift %+v with no powered beacon", row.Gift)
		}
	}
}

// TestBeaconReachFollowsPower: powering the beacon brings the reach and the
// call; emptying the generator takes them away again.
func TestBeaconReachFollowsPower(t *testing.T) {
	t.Parallel()
	g, _, _, pawn := orbitalGame(t)
	g.orbitalArrive(uint64(g.clock.Now()))
	spawn(t, g, item.Silver, 100, 23, 15)
	_, gen := powerBeacon(t, g, 22, 14)
	if got := reach(g); got != 100 {
		t.Fatalf("powered: reach %d, want 100", got)
	}
	openComms(t, g, pawn)
	if cv, _ := g.CommsView(); !cv.Orbital.CallOK || cv.BeaconSilver != 100 {
		t.Fatalf("powered comms %+v", cv)
	}
	g.setFuel(gen, 0)
	g.Step(powerInterval)
	if got := reach(g); got != 0 {
		t.Fatalf("generator dry: reach %d, want 0", got)
	}
	if cv, _ := g.CommsView(); cv.Orbital.CallOK || cv.Orbital.CallReason != ReasonNoBeacon {
		t.Fatalf("dry comms %+v", cv)
	}
	g.setFuel(gen, g.fuelCap(gen))
	g.Step(powerInterval)
	if got := reach(g); got != 100 {
		t.Fatalf("refuelled: reach %d, want 100", got)
	}
}

// TestBeaconCacheSurvivesDropPower: dropPower with the same structVer must
// not leave the powered set behind.
func TestBeaconCacheSurvivesDropPower(t *testing.T) {
	t.Parallel()
	g, _, _, _ := orbitalGame(t)
	spawn(t, g, item.Silver, 100, 23, 15)
	powerBeacon(t, g, 22, 14)
	if got := reach(g); got != 100 {
		t.Fatalf("powered: %d", got)
	}
	g.dropPower()
	if got := reach(g); got != 0 {
		t.Fatalf("power dropped: reach %d, want 0", got)
	}
}

// TestBeaconCacheFollowsPoweredFlipAlone: only the powered flag flips
// (structVer unchanged); the key's pwc.vers term must rebuild the reach.
// Red first: with the key reduced to structVer this reads a stale 100.
func TestBeaconCacheFollowsPoweredFlipAlone(t *testing.T) {
	t.Parallel()
	g, _, _, _ := orbitalGame(t)
	spawn(t, g, item.Silver, 100, 23, 15)
	b, _ := powerBeacon(t, g, 22, 14)
	if got := reach(g); got != 100 {
		t.Fatalf("powered: %d", got)
	}
	sv := g.structVer
	g.setPowered(b, false)
	if g.structVer != sv {
		t.Fatalf("structVer moved %d -> %d", sv, g.structVer)
	}
	if got := beaconGoods(g).Counts()[item.Silver]; got != 0 {
		t.Fatalf("powered flag off: reach %d, want 0", got)
	}
	g.setPowered(b, true)
	if got := beaconGoods(g).Counts()[item.Silver]; got != 100 {
		t.Fatalf("powered flag on: reach %d, want 100", got)
	}
}

// TestBeaconCacheSurvivesSwitchFlick: a real flick of a switch between the
// beacon and its generator drops the reach. pwc.topo is not in the key: a
// flick (power_flick.go:188) and a reconnect bump vers with it, and the
// topo-only bumps (prunePower after a removal, importPower on load) come with
// a structVer move or an empty cache.
func TestBeaconCacheSurvivesSwitchFlick(t *testing.T) {
	t.Parallel()
	g, _, _, _ := orbitalGame(t)
	spawn(t, g, item.Silver, 100, 23, 15)
	beaconID := buildNow(t, g, construction.TradeBeacon, item.Steel, construction.South, 22, 14)
	psFuelled(t, g, 15, 14)
	psConduits(t, g, 14, 16, 17)
	sw := buildNow(t, g, construction.PowerSwitch, item.Steel, construction.South, 18, 14)
	psConduits(t, g, 14, 19, 21)
	g.Step(powerInterval)
	if !g.powered(beaconID) || reach(g) != 100 {
		t.Fatal("closed switch: the beacon should reach")
	}
	g.pw.wantOff[sw] = true
	g.flick(sw)
	if got := reach(g); got != 0 {
		t.Fatalf("switch open: reach %d, want 0", got)
	}
}

// TestTradePodLandsAtPoweredBeaconElseHome: the drop spot is the powered
// beacon, or home() when none is powered.
func TestTradePodLandsAtPoweredBeaconElseHome(t *testing.T) {
	t.Parallel()
	g, _, _, _ := orbitalGame(t)
	line := []trade.Line{{Kind: item.Steel, Qty: 1}}
	far := func(p tradePod, x, y int) bool {
		dx, dy := p.X-x, p.Y-y
		return dx*dx+dy*dy > podRadius*podRadius*2
	}
	buildNow(t, g, construction.TradeBeacon, item.Steel, construction.South, 30, 20)
	g.Step(powerInterval)
	hx, hy := g.home()
	g.sendTradePod(line, 0)
	if p := g.trade.pods[0]; far(p, hx, hy) {
		t.Fatalf("unpowered beacon: pod at %d,%d, want near home %d,%d", p.X, p.Y, hx, hy)
	}
	g.trade.pods = nil
	powerBeacon(t, g, 22, 14)
	g.sendTradePod(line, 0)
	if p := g.trade.pods[0]; far(p, 22, 14) {
		t.Fatalf("powered beacon: pod at %d,%d, want near 22,14", p.X, p.Y)
	}
}

// TestUnpoweredBeaconShowsNoPowerReason: the inspector names why.
func TestUnpoweredBeaconShowsNoPowerReason(t *testing.T) {
	t.Parallel()
	g, _, _, _ := orbitalGame(t)
	id := buildNow(t, g, construction.TradeBeacon, item.Steel, construction.South, 30, 20)
	g.Step(powerInterval)
	if v := psView(t, g, id); v.Powered || v.NoPowerReason == "" {
		t.Fatalf("unpowered beacon view %+v", v)
	}
	b, _ := powerBeacon(t, g, 22, 14)
	if v := psView(t, g, b); !v.Powered || v.NoPowerReason != "" {
		t.Fatalf("powered beacon view %+v", v)
	}
}
