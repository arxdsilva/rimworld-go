// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"math"
	"rimworld-go/internal/application/game/wealth"
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/difficulty"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/pawn/pawngen"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
	"testing"
)

func TestWeightedThreatActualResidentsAnimalsAndGoods(t *testing.T) {
	g := bareSetupGame(difficulty.Seasoned)
	id, err := g.SpawnPawn(5, 5)
	if err != nil {
		t.Fatal(err)
	}
	p := g.sk.bios[id]
	p.AgeDays = 18 * pawngen.DaysPerYear
	g.sk.bios[id] = p
	if b := threatBase(g, false); b.PawnWeight != 1 {
		t.Fatalf("adult %+v", b)
	}
	p.AgeDays = 13 * pawngen.DaysPerYear
	g.sk.bios[id] = p
	g.vital.Set(id, health.Record{Status: health.Downed, Injury: 50})
	if b := threatBase(g, false); math.Abs(b.PawnWeight-.3375) > 1e-9 {
		t.Fatalf("injured child %+v", b)
	}
	wolf, err := g.SpawnAnimal("timber_wolf", 6, 5)
	if err != nil {
		t.Fatal(err)
	}
	g.TameAnimal(wolf, "Wolf")
	// A trainable animal counts even before learning any attack orders.
	b := threatBase(g, false)
	if b.AnimalPower != float64(animal.CombatPower(animal.TimberWolf)) || b.AnimalPower == 0 {
		t.Fatalf("tame wolf %+v", b)
	}
	a := g.animals.Ref(wolf)
	a.Downed = true
	if got := threatBase(g, false).AnimalPower; got != 0 {
		t.Fatalf("downed wolf power=%g", got)
	}
	before := threatBase(g, false).Wealth
	if _, err := g.SpawnStack(item.Silver, 200, 7, 5); err != nil {
		t.Fatal(err)
	}
	if got := threatBase(g, false).Wealth - before; got != 200*float64(g.sk.content.Items.Value(item.Silver)) {
		t.Fatalf("goods delta=%g", got)
	}
}

// Actual departure, not a fabricated off-map pawn, shifts ownership to the
// caravan and keeps that pawn's weight in quest sizing alone.
func TestWeightedThreatCaravanAssets(t *testing.T) {
	g := worldGame(t)
	id := g.pawnIDs[0]
	p := g.sk.bios[id]
	p.AgeDays = 30 * pawngen.DaysPerYear
	g.sk.bios[id] = p
	c := formCaravan(t, g, nearDest(t, g), id)
	local, all := threatBase(g, false), threatBase(g, true)
	v, _ := g.vital.Get(id)
	want := difficulty.PawnWeight(30, float64(v.HP())/health.MaxHP, false)
	if math.Abs(all.PawnWeight-local.PawnWeight-want) > 1e-9 {
		t.Fatalf("local=%+v quest=%+v expected away=%g", local, all, want)
	}
	travelOf(g).Carry(c.C.ID, item.Stack{Kind: item.Silver, Qty: 200})
	if got := threatBase(g, true).Wealth - all.Wealth; got != 200*float64(g.sk.content.Items.Value(item.Silver)) {
		t.Fatalf("caravan goods delta=%g", got)
	}
	if got := threatBase(g, false); got != local {
		t.Fatalf("caravan goods changed raid scope: %+v -> %+v", local, got)
	}
}

// The QST-025 contract combines all permanent colonies plus the caravan
// once, irrespective of the map selected for display.
func TestWeightedThreatAllColoniesAndCaravans(t *testing.T) {
	g := worldGame(t)
	first := g.maps[0]
	tiles := landable(t, g, 3)
	tile := tiles[0]
	founder := g.pawnIDs[0]
	second := founded(t, g, tile, founder)
	// Put actual assets on the new colony. A normal local raid must see only
	// its map, whereas a quest sees both colonies and the away caravan.
	g.ViewMap(MapID(second.id))
	if _, err := g.SpawnStack(item.Silver, 500000, 0, 0); err != nil {
		t.Fatal(err)
	}
	g.ViewMap(MapID(first.id))
	if len(g.pawnIDs) < 2 {
		t.Fatal("need two residents on first map")
	}
	away := g.pawnIDs[0]
	c := formCaravan(t, g, nearDest(t, g), away)
	travelOf(g).Carry(c.C.ID, item.Stack{Kind: item.Silver, Qty: 2000})
	home := threatBase(g, false)
	g.ViewMap(MapID(second.id))
	next := threatBase(g, false)
	g.ViewMap(MapID(first.id))
	v, _ := g.vital.Get(away)
	p, _ := g.personBio(away)
	aw := difficulty.PawnWeight(float64(p.AgeDays)/pawngen.DaysPerYear, float64(v.HP())/health.MaxHP, false)
	want := difficulty.ThreatBase{
		Wealth:      home.Wealth + next.Wealth,
		PawnWeight:  home.PawnWeight + next.PawnWeight + aw,
		AnimalPower: home.AnimalPower + next.AnimalPower,
	}
	// FormCaravan gives the caravan initial food days rather than a physical
	// silver stack; any other actual cargo must be counted too.
	caravan := difficulty.ThreatBase{PawnWeight: aw}
	for _, st := range travelOf(g).Goods(c.C.ID) {
		v := float64(g.sk.content.Items.Value(st.Kind)) * float64(st.Qty)
		want.Wealth += v
		caravan.Wealth += v
	}
	var points int
	for _, m := range []*mapState{first, second} {
		g.ViewMap(MapID(m.id))
		got := threatBase(g, true)
		if math.Abs(got.Wealth-want.Wealth) > 1e-9 || math.Abs(got.PawnWeight-want.PawnWeight) > 1e-9 || math.Abs(got.AnimalPower-want.AnimalPower) > 1e-9 {
			t.Fatalf("map%d quest base %+v want %+v", m.id, got, want)
		}
		q := questHost{g}
		if got, w := q.Points(), weightedThreat(g, home, g.rel(q.Now()))+weightedThreat(g, next, g.rel(q.Now()))+weightedThreat(g, caravan, g.rel(q.Now())); got != w {
			t.Fatalf("quest points %d want %d", got, w)
		}
		if points != 0 && q.Points() != points {
			t.Fatal("selected map changed quest points")
		}
		points = q.Points()
		if q.Colonists() != len(first.pawnIDs)+len(second.pawnIDs)+1 {
			t.Fatalf("population %d, want all residents and away member", q.Colonists())
		}
		if g.mapState != m || g.ViewedMap() != MapID(m.id) {
			t.Fatal("quest read switched the view")
		}
	}
	g.ViewMap(MapID(first.id))
	before := threatBase(g, true)
	// A temporary encounter/site is not another permanent colony.
	siteTile := tiles[1]
	if siteTile == c.C.Tile || g.settlementFaction(siteTile) != 0 {
		siteTile = tiles[2]
	}
	site, ok := DevOpenMap(g, siteTile)
	if !ok {
		t.Fatal("site")
	}
	g.ViewMap(site)
	if _, err := g.SpawnStack(item.Gold, 1000000, 0, 0); err != nil {
		t.Fatal(err)
	}
	if got := threatBase(g, true); got != before {
		t.Fatalf("site wealth entered colony total %+v vs %+v", got, before)
	}
	if g.mapState.id != uint32(site) {
		t.Fatal("quest read did not restore selected site")
	}
	g.ViewMap(MapID(first.id))
	// Check the actual save/load contract, not just the live projection.
	h := saveLoad(t, g)
	if got := threatBase(h, true); got != before {
		t.Fatalf("loaded quest base %+v want %+v", got, before)
	}
}

func TestWeightedThreatActualStasisResident(t *testing.T) {
	g := bareSetupGame(difficulty.Seasoned)
	id, err := g.SpawnNamedPawn(2, 2, "Ada")
	if err != nil {
		t.Fatal(err)
	}
	p := g.sk.bios[id]
	p.AgeDays = 30 * pawngen.DaysPerYear
	g.sk.bios[id] = p
	ax9Idle(g, id)
	casket := buildNow(t, g, construction.StasisCasket, item.Steel, construction.South, 8, 4)
	g.Enqueue(command.Command{Kind: command.EnterCasket, Pawn: uint64(id), Target: uint64(casket)})
	for i := 0; i < 600 && !g.isStashed(id); i++ {
		g.Step(1)
	}
	if !g.isStashed(id) || travelOf(g).Sleeper(casket) != id {
		t.Fatal("resident did not enter physical casket")
	}
	if got := threatBase(g, false).PawnWeight; math.Abs(got-.3) > 1e-9 {
		t.Fatalf("sleeping pawn weight %g", got)
	}
	if got := (questHost{g}).Colonists(); got != 1 {
		t.Fatalf("stasis resident population %d", got)
	}
	h := saveLoad(t, g)
	if got := threatBase(h, true).PawnWeight; math.Abs(got-.3) > 1e-9 {
		t.Fatalf("loaded sleeper weight %g", got)
	}
	h.Enqueue(command.Command{Kind: command.EjectCasket, Target: uint64(casket)})
	h.Step(1)
	if h.isStashed(id) || threatBase(h, false).PawnWeight != 1 {
		t.Fatal("ejected resident weight did not return")
	}
}

func TestWeightedThreatMixedStashOwners(t *testing.T) {
	g := worldGame(t)
	ids := append([]entity.EntityID(nil), g.pawnIDs...)
	if len(ids) < 3 {
		t.Fatal("need three living starting residents")
	}
	for _, id := range ids {
		p := g.sk.bios[id]
		p.AgeDays = 30 * pawngen.DaysPerYear
		g.sk.bios[id] = p
		ax9Idle(g, id)
	}
	// Select a legal footprint near the actual sleeper.
	sleeper := ids[0]
	pos, _ := g.positions.Get(sleeper)
	var casket entity.EntityID
	for d := 1; d < 16 && casket == 0; d++ {
		for y := pos.Y - d; y <= pos.Y+d && casket == 0; y++ {
			for x := pos.X - d; x <= pos.X+d; x++ {
				if ok, _ := g.canPlaceStuffAt(construction.StasisCasket, item.Steel, construction.South, x, y); ok {
					casket = buildNow(t, g, construction.StasisCasket, item.Steel, construction.South, x, y)
					break
				}
			}
		}
	}
	if casket == 0 {
		t.Fatal("no legal casket footprint")
	}
	g.Enqueue(command.Command{Kind: command.EnterCasket, Pawn: uint64(sleeper), Target: uint64(casket)})
	for i := 0; i < 1200 && !g.isStashed(sleeper); i++ {
		g.Step(1)
	}
	if g.worldSt.ax9.Sleeper(casket) != sleeper || !g.isStashed(sleeper) {
		t.Fatal("actual casket entry failed")
	}
	dest := nearDest(t, g)
	a := formCaravan(t, g, dest, ids[1]).C.ID
	b := formCaravan(t, g, dest, ids[2]).C.ID
	if a == b {
		t.Fatal("caravans must be distinct holders")
	}
	travelOf(g).Carry(a, item.Stack{Kind: item.Silver, Qty: 7})
	travelOf(g).Carry(b, item.Stack{Kind: item.Gold, Qty: 11})
	check := func(g *Game) {
		t.Helper()
		var scopes []wealth.Census
		eachThreatScope(g, true, func(c wealth.Census) { scopes = append(scopes, c) })
		if len(scopes) != 3 {
			t.Fatalf("expected colony plus two actual caravans: %+v", scopes)
		}
		wantHome := float64(len(ids)-3) + .3
		if math.Abs(scopes[0].PawnWeight-wantHome) > 1e-9 || scopes[0].Residents != len(ids)-2 {
			t.Fatalf("home must count stasis only once: %+v", scopes[0])
		}
		for _, c := range scopes[1:] {
			if c.Residents != 1 || math.Abs(c.PawnWeight-1) > 1e-9 {
				t.Fatalf("road scope mixed owners: %+v", c)
			}
		}
		if c := threatCensusOf(g, true); c.Residents != len(ids) || math.Abs(c.PawnWeight-(wantHome+2)) > 1e-9 {
			t.Fatalf("combined census duplicated or omitted a resident: %+v", c)
		}
	}
	check(g)
	h := saveLoad(t, g)
	check(h)
	// A true off-map death uses the pre-event roster, including the
	// sleeping and other travelling member; it must not duplicate anyone.
	want := h.ng.adapt
	want.Lose(len(ids), true, h.relDay(uint64(h.clock.Now())))
	dieAway(h, ids[1], health.CauseInjury)
	if h.ng.adapt != want {
		t.Fatalf("adaptation event census: %+v want %+v", h.ng.adapt, want)
	}
}
