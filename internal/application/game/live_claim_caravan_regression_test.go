// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"rimworld-go/internal/application/game/herd"
	"rimworld-go/internal/application/game/maps"
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/animal/training"
	"rimworld-go/internal/domain/building/zone"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/caravan"
	"rimworld-go/internal/domain/world/temperature"
	"testing"
)

func TestHaulNoWorkUnderUnrelatedMoveClaims(t *testing.T) {
	g := feedField(64, 64)
	h := animalHaulHost{haulHost{g}}
	var dogs []entity.EntityID
	for i := 0; i < 40; i++ {
		dog, err := g.SpawnAnimal("husky", 2+i, 2)
		if err != nil {
			t.Fatal(err)
		}
		g.TameAnimal(dog, "carrier")
		setAnimal(g, dog, func(a *animal.State) { a.NextThink = 1 << 40; a.NextDecide = 1 << 40 })
		saveTraining(g, dog, training.Maintenance{DecayAt: 1 << 40, HaulProgress: training.Needed(training.Haul) | herd.TrainToggled})
		dogs = append(dogs, dog)
	}
	for i := 0; i < 512; i++ {
		stack(t, g, item.Wood, 5, 2+i%32, 8+i/32)
	}
	zid := g.zones.Paint(zone.KindStockpile, zone.NewStockpile(), zone.RectOf(40, 8, 42, 10), nil)
	g.zones.Update(zid, func(z *zone.Zone) { z.Custom = true; z.Filter = (zone.Filter{}).With(item.Steel, true) })
	pawn, err := g.SpawnNamedPawn(50, 50, "Mover")
	if err != nil {
		t.Fatal(err)
	}
	applyNow(g, command.Command{Kind: command.DraftPawn, Pawn: uint64(pawn), Enabled: true})
	a, _ := g.animals.Get(dogs[0])
	if len(h.HaulOffers(dogs[0], a)) != 0 {
		t.Fatal("fixture has animal work")
	}
	s0, c0, p0 := g.idx.animalHaul.Counts()
	ver0 := g.stacks.Ver()
	changed := 0
	for i := 0; i < 60; i++ {
		// Human moves nowhere near the items, animals, or storage.
		r := g.reserve.Ver()
		applyNow(g, command.Command{Kind: command.MovePawn, Pawn: uint64(pawn), X: 60, Y: 50 + i%2})
		if g.reserve.Ver() != r {
			changed++
		}
		g.Step(1)
		fed(g, pawn)
	}
	s1, c1, p1 := g.idx.animalHaul.Counts()
	t.Logf("unrelated move claims=%d, source scans=%d->%d, candidate checks=%d->%d, destination searches=%d->%d, stack revision=%d->%d", changed, s0, s1, c0, c1, p0, p1, ver0, g.stacks.Ver())
	if changed < 40 || g.stacks.Ver() != ver0 {
		t.Fatal("fixture must isolate claim churn with stable item availability")
	}
	if c1-c0 >= uint64(len(dogs)*512/2) {
		t.Fatalf("unrelated reservations caused repeated complete candidate scans: %d extra checks in 60 ticks for %d dogs and 512 candidates", c1-c0, len(dogs))
	}
}

func TestExposureSpotRevalidatesLiveFire(t *testing.T) {
	g, ada, doc, _ := heatwaveColony(t, 320)
	pin := func() {
		g.ensureRooms()
		for _, xy := range [][2]int{{4, 3}, {16, 3}} {
			g.thermo.Temps[g.roomAt(xy[0], xy[1])] = 320
		}
	}
	pin()
	x, y := tileOf(g, ada)
	g.Enqueue(prioritize(command.OrderRescue, doc, ada, x, y, false))
	for i := 0; i < 3000 && g.care.Carry.Carrier(ada) != doc; i++ {
		g.Step(1)
		pin()
		fed(g, doc)
	}
	j, _ := g.jobs.Get(doc)
	if g.care.Carry.Carrier(ada) != doc || j.Dest != 0 || !g.grid.InBounds(j.TX, j.TY) {
		t.Fatalf("fixture: no carry to ground: %+v", j)
	}
	tx, ty := j.TX, j.TY
	if g.roomAt(tx, ty) >= 0 {
		t.Fatalf("fixture: not outdoor destination %d,%d", tx, ty)
	}
	if g.thermo.Residual == nil {
		g.thermo.Residual = map[int]temperature.Temp{}
	}
	g.thermo.Residual[ty*g.grid.Width()+tx] = 220
	fx, fy := -1, -1
	for _, d := range [][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
		px, py := tx+d[0], ty+d[1]
		if g.grid.InBounds(px, py) && g.grid.Walkable(px, py) {
			fx, fy = px, py
			break
		}
	}
	if fx < 0 {
		t.Fatal("fixture: no fire neighbour")
	}
	ignited := false
	for i := 0; i < 3000 && g.care.Carry.Carrier(ada) == doc; i++ {
		pin()
		g.thermo.Residual[ty*g.grid.Width()+tx] = 220
		p := g.positionOf(doc)
		dx, dy := p.X-tx, p.Y-ty
		if dx*dx+dy*dy <= 9 && !g.fireNear(tx, ty, 1) {
			g.devStartFire(fx, fy)
			ignited = true
			if !g.fireNear(tx, ty, 1) || !(careReach{g}).AirHeals(ada, tx, ty, 0) || safeThermalCell(g, ada, tx, ty) {
				t.Fatal("fixture: destination must be thermal but unsafe")
			}
		}
		g.Step(1)
		fed(g, doc)
	}
	if !ignited {
		t.Fatal("fixture: no in-flight ignition")
	}
	end, _ := g.jobs.Get(doc)
	pos := g.positionOf(ada)
	t.Logf("initial destination=(%d,%d), patient=%+v, fire near=%t, job=%+v, cooldown=%d now=%d", tx, ty, pos, g.fireNear(tx, ty, 1), end, (careReach{g}).ExposureWait(ada), g.clock.Now())
	if !g.fireNear(tx, ty, 1) {
		t.Fatal("fixture: fire died before arrival")
	}
	if end.Kind == job.Rescue && end.State == job.Complete && pos.X == tx && pos.Y == ty {
		t.Fatal("rescue declared success at a ground destination made unsafe by fire in transit")
	}
	if g.care.Carry.Carrier(ada) != 0 {
		t.Fatal("carrier did not finish")
	}
}

func TestCaravanRansomRecomputesLoad(t *testing.T) {
	g := worldGame(t)
	for i := range g.ng.factions {
		if f := &g.ng.factions[i]; f.Type.Human() {
			f.Relation, f.Defeated = faction.Hostile, false
		}
	}
	c := onRoad(t, g, g.pawnIDs[0])
	cid := c.C.ID
	travelOf(g).Carry(cid, item.Stack{Kind: item.Silver, Qty: 2000})
	applyNow(g, command.Command{Kind: command.CaravanTravel, Target: cid, Count: c.C.Dest})
	c = fleetOf(g).At(entity.EntityID(cid))
	want := func(qty int) int32 {
		return int32(qty * g.sk.content.Items.MassGrams(item.Silver) / int(caravan.PersonCarryKg))
	}
	if c.C.Load != want(2000) || c.C.Load <= 0 || c.C.Load >= 1000 {
		t.Fatalf("fixture: before demand load %d want %d", c.C.Load, want(2000))
	}
	before := c.C.Load
	fleetOf(g).Waylay(c, maps.Demand, uint64(g.clock.Now()))
	if c.Ask.Letter == 0 || c.Ask.Qty != 1000 {
		t.Fatalf("fixture no demand %+v", c.Ask)
	}
	applyNow(g, command.Command{Kind: command.EncounterAnswer, Target: c.Ask.Letter, Mode: 0})
	c = fleetOf(g).At(entity.EntityID(cid))
	if qty := cargoQty(travelOf(g).Goods(cid), item.Silver); qty != 1000 || c.Ask.Letter != 0 || c.C.State != caravan.Travelling {
		t.Fatalf("fixture failed to pay: qty=%d, ask=%+v", qty, c.Ask)
	}
	t.Logf("paid native demand: cargo 2000->1000 silver, load %d->%d expected %d, speed=%d", before, c.C.Load, want(1000), c.C.Speed())
	if c.C.Load != want(1000) {
		t.Fatal("post-commit road movement retains stale cargo load")
	}
}

func TestCaravanMedicalDeathRecomputesLoad(t *testing.T) {
	g := worldGame(t)
	a, b := g.pawnIDs[0], g.pawnIDs[1]
	c := onRoad(t, g, a, b)
	cid := c.C.ID
	travelOf(g).Carry(cid, item.Stack{Kind: item.Silver, Qty: 2000})
	applyNow(g, command.Command{Kind: command.CaravanTravel, Target: cid, Count: c.C.Dest})
	c = fleetOf(g).At(entity.EntityID(cid))
	grams := 2000 * g.sk.content.Items.MassGrams(item.Silver)
	before, want := int32(grams/(2*int(caravan.PersonCarryKg))), int32(grams/int(caravan.PersonCarryKg))
	if c.C.Load != before || before <= 0 || want >= 1000 {
		t.Fatalf("fixture load %d want %d", c.C.Load, before)
	}
	g.dieMedical(b, health.CauseInjury)
	c = fleetOf(g).At(entity.EntityID(cid))
	if c == nil || len(c.C.Members) != 1 || c.C.Members[0] != uint64(a) {
		t.Fatal("medical death did not remove caravan member")
	}
	t.Logf("native medical death: members 2->1, load %d->%d expected %d, speed=%d", before, c.C.Load, want, c.C.Speed())
	if c.C.Load != want {
		t.Fatal("post-death road movement retains lost carrier capacity")
	}
}
