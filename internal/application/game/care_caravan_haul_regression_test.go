// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"rimworld-go/internal/application/game/maps"
	"rimworld-go/internal/domain/building/zone"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/trade"
	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/caravan"
	"rimworld-go/internal/domain/world/terrain"
	"testing"
)

// Exercise the production admission helper, not a simulated reservation.
func TestGroundRescueAdmissionOwnClaims(t *testing.T) {
	g, patient, carrier, _ := heatwaveColony(t, 320)
	g.ensureRooms()
	x, y, ok := (careReach{g}).HealingSpotFor(patient, carrier)
	if !ok {
		t.Fatal("fixture no thermal spot")
	}
	h := careReach{g}
	if !h.GroundSafe(patient, carrier, x, y) {
		t.Fatal("discovery disagrees with arrival")
	}
	for _, owner := range []entity.EntityID{patient, carrier} {
		if !g.reserve.TryTile(owner, x, y) || !h.GroundSafe(patient, carrier, x, y) {
			t.Fatal("own reservation invalidated safe destination")
		}
		g.reserve.ReleaseTile(x, y)
	}
	other := g.entities.Next()
	if !g.reserve.TryTile(other, x, y) || h.GroundSafe(patient, carrier, x, y) {
		t.Fatal("foreign claim admitted")
	}
	g.reserve.ReleaseTile(x, y)
	if !h.GroundSafe(patient, carrier, x, y) {
		t.Fatal("release did not restore admission")
	}
	if h.GroundSafe(patient, carrier, -1, y) {
		t.Fatal("out-of-map destination admitted")
	}
	g.setTerrain(x, y, terrain.Rock)
	if h.GroundSafe(patient, carrier, x, y) {
		t.Fatal("blocked destination admitted")
	}
}

func TestRoadMutationReplay(t *testing.T) {
	g := worldGame(t)
	for i := range g.ng.factions {
		if f := &g.ng.factions[i]; f.Type.Human() {
			f.Relation, f.Defeated = faction.Hostile, false
		}
	}
	a, b := g.pawnIDs[0], g.pawnIDs[1]
	c := onRoad(t, g, a, b)
	cid := c.C.ID
	travelOf(g).Carry(cid, item.Stack{Kind: item.Silver, Qty: 2000})
	applyNow(g, command.Command{Kind: command.CaravanTravel, Target: cid, Count: c.C.Dest})
	c = fleetOf(g).At(entity.EntityID(cid))
	fleetOf(g).Waylay(c, maps.Demand, uint64(g.clock.Now()))
	letter := c.Ask.Letter
	if letter == 0 {
		t.Fatal("fixture no demand")
	}
	h := saveLoad(t, g)
	h.worldS().planet, h.worldS().planetTried = testPlanet().Fork(), true
	check := func(sim *Game, qty, members int) {
		t.Helper()
		cv := fleetOf(sim).At(entity.EntityID(cid))
		if cv == nil || len(cv.C.Members) != members || cargoQty(travelOf(sim).Goods(cid), item.Silver) != qty {
			t.Fatal("unexpected caravan roster or cargo")
		}
		want := int32(qty * sim.sk.content.Items.MassGrams(item.Silver) / (members * int(caravan.PersonCarryKg)))
		if cv.C.Load != want {
			t.Fatalf("pace load %d want %d", cv.C.Load, want)
		}
	}
	for _, sim := range []*Game{g, h} {
		applyNow(sim, command.Command{Kind: command.EncounterAnswer, Target: letter, Mode: 0})
		check(sim, 1000, 2)
		sim.dieMedical(b, health.CauseInjury)
		check(sim, 1000, 1)
	}
	if g.StateHash() != h.StateHash() {
		t.Fatal("road payment/death save replay diverged")
	}
	for i := 0; i < 24; i++ {
		worldForward(g, 2500)
		worldForward(h, 2500)
		if g.StateHash() != h.StateHash() {
			t.Fatalf("post-mutation movement diverged at hour %d", i)
		}
	}
}

func TestQuestCargoPaceAndLastMember(t *testing.T) {
	g := worldGame(t)
	p := g.pawnIDs[0]
	c := onRoad(t, g, p)
	cid := c.C.ID
	h := questHost{g}
	check := func(qty int) {
		t.Helper()
		v := fleetOf(g).At(entity.EntityID(cid))
		want := int32(qty * g.sk.content.Items.MassGrams(item.Silver) / int(caravan.PersonCarryKg))
		if v == nil || v.C.Load != want || cargoQty(travelOf(g).Goods(cid), item.Silver) != qty {
			t.Fatalf("quest cargo did not refresh load to %d: %+v", want, v)
		}
	}
	if !h.IntoCaravan(cid, []trade.Line{{Kind: item.Silver, Qty: 2000}}) {
		t.Fatal("quest reward failed")
	}
	check(2000)
	if h.Take(cid, []trade.Line{{Kind: item.Silver, Qty: 2001}}) {
		t.Fatal("unavailable goods accepted")
	}
	check(2000)
	if !h.Take(cid, []trade.Line{{Kind: item.Silver, Qty: 1000}}) {
		t.Fatal("quest delivery failed")
	}
	check(1000)
	g.dieMedical(p, health.CauseInjury)
	c = fleetOf(g).At(entity.EntityID(cid))
	if c == nil || len(c.C.Members) != 0 {
		t.Fatal("last member not removed")
	}
	fleetOf(g).Step(uint64(g.clock.Now()))
	if fleetOf(g).At(entity.EntityID(cid)) != nil {
		t.Fatal("empty caravan retained")
	}
}

// Structural no-work is independent of even relevant claim churn, including
// a change journal overflow. An actual filter change must still restore work.
func TestHaulStaticNegativeSurvivesClaimOverflow(t *testing.T) {
	g := feedField(64, 64)
	h := animalHaulHost{haulHost{g}}
	dog, err := g.SpawnAnimal("husky", 2, 2)
	if err != nil {
		t.Fatal(err)
	}
	g.TameAnimal(dog, "carrier")
	a, _ := g.animals.Get(dog)
	for i := 0; i < 512; i++ {
		stack(t, g, item.Wood, 5, 2+i%32, 8+i/32)
	}
	zid := g.zones.Paint(zone.KindStockpile, zone.NewStockpile(), zone.RectOf(40, 8, 42, 10), nil)
	g.zones.Update(zid, func(z *zone.Zone) { z.Custom = true; z.Filter = (zone.Filter{}).With(item.Steel, true) })
	owner := g.entities.Next()
	if len(h.HaulOffers(dog, a)) != 0 {
		t.Fatal("fixture has work")
	}
	s0, c0, p0 := g.idx.animalHaul.Counts()
	for i := 0; i < 600; i++ {
		x, y := 40+i%3, 8+i%3
		if !g.reserve.TryTile(owner, x, y) {
			t.Fatal("claim failed")
		}
		if i%30 == 0 && len(h.HaulOffers(dog, a)) != 0 {
			t.Fatal("incompatible reserved storage offered")
		}
		g.reserve.ReleaseTile(x, y)
	}
	for i := 0; i < 300; i++ {
		if !g.reserve.TryTile(owner, 60, 50) {
			t.Fatal("move claim failed")
		}
		g.reserve.ReleaseTile(60, 50)
	}
	if len(h.HaulOffers(dog, a)) != 0 {
		t.Fatal("incompatible storage offered after overflow")
	}
	s1, c1, p1 := g.idx.animalHaul.Counts()
	if s0 != s1 || c0 != c1 || p0 != p1 {
		t.Fatalf("static negative rescanned: %d/%d/%d -> %d/%d/%d", s0, c0, p0, s1, c1, p1)
	}
	g.zones.Update(zid, func(z *zone.Zone) { z.Filter = (zone.Filter{}).With(item.Wood, true) })
	if len(h.HaulOffers(dog, a)) == 0 {
		t.Fatal("filter change failed to restore work")
	}
}
