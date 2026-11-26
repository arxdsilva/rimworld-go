// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"rimworld-go/internal/application/game/maps"
	"rimworld-go/internal/application/game/raid"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/caravan"
)

// Caravans take their prisoners along and eat what they pack (#14).

// spawnPrisoner makes a standing prisoner of the colony beside captor.
func spawnPrisoner(t *testing.T, g *Game, captor entity.EntityID) entity.EntityID {
	t.Helper()
	pos, _ := g.positions.Get(captor)
	x, y, ok := g.combat.Tiles.WalkableNear(pos.X+2, pos.Y, 8)
	if !ok {
		t.Fatal("no ground for a prisoner")
	}
	g.raidGroups = append(g.raidGroups, raid.Group{Size: 1})
	rid := spawnRaider(g, 0, len(g.raidGroups)-1, item.Revolver, x, y)
	g.makePrisoner(rid, captor)
	return rid
}

// stashRecOf is the stash record of id (nil: none).
func stashRecOf(g *Game, id entity.EntityID) *stashRec {
	if i, ok := slices.BinarySearchFunc(g.worldS().stashed, id, stashCmp); ok {
		return &g.worldS().stashed[i]
	}
	return nil
}

// stepFed steps n ticks keeping the colonists fed.
func stepFeeding(g *Game, n int) {
	for i := 0; i < n; i++ {
		if i%300 == 0 {
			for _, id := range g.pawnIDs {
				g.SetNeeds(id, needs.Full, needs.Full)
			}
		}
		g.Step(1)
	}
}

// formWithFood forms a caravan of ids carrying qty meals to dest and steps
// until it sets out.
func formWithFood(t *testing.T, g *Game, dest, qty int, ids ...entity.EntityID) *maps.Caravan {
	t.Helper()
	pos, _ := g.positions.Get(ids[0])
	x, y, ok := g.combat.Tiles.WalkableNear(pos.X, pos.Y+3, 8)
	if !ok {
		t.Fatal("no ground")
	}
	sid, err := g.SpawnStack(item.Meal, qty, x, y)
	if err != nil {
		t.Fatal(err)
	}
	var parts []string
	for _, id := range ids {
		parts = append(parts, fmt.Sprint(id))
	}
	parts = append(parts, fmt.Sprintf("%d:%d", sid, qty))
	g.Enqueue(command.Command{Kind: command.FormCaravan, Name: strings.Join(parts, ","), Count: dest})
	for i := 0; i < 200; i++ {
		stepFeeding(g, 60)
		if c := fleetOf(g).Of(ids[0]); c != nil && c.C.State != caravan.Forming {
			return c
		}
	}
	t.Fatal("the caravan never set out")
	return nil
}

// farTile is a tile many steps from home.
func farTile(t *testing.T, g *Game) int {
	t.Helper()
	p, home := testPlanet(), g.colonyTile()
	for i := 0; i < p.TileCount(); i += 7 {
		if r := p.Route(home, i); len(r) >= 10 {
			return i
		}
	}
	t.Fatal("no far tile")
	return -1
}

// roadCaravan puts members on a slow caravan on the road far from home
// (no encounters), carrying meals and a reserve of days milli-days.
func roadCaravan(t *testing.T, g *Game, reserve int32, meals int, members ...entity.EntityID) *maps.Caravan {
	t.Helper()
	w, home := g.worldS(), g.colonyTile()
	far := farTile(t, g)
	route := testPlanet().Route(home, far)
	cid := g.entities.Next()
	w.caravans = append(w.caravans, maps.Caravan{C: caravan.Caravan{ID: uint64(cid), Tile: home, Dest: far, Path: slices.Clone(route[1:]),
		State: caravan.Travelling, Slowest: 1, FoodDays: reserve}, Home: home, Out: route, NextEnc: 1 << 62, NextTalk: 1 << 62})
	for _, id := range members {
		if !g.stashPawn(id, cid) {
			t.Fatalf("could not stash %d", id)
		}
		c := fleetOf(g).At(cid)
		c.C.Members = append(c.C.Members, uint64(id))
		g.SetNeeds(id, needs.Full, needs.Full)
	}
	if meals > 0 {
		travelOf(g).Carry(uint64(cid), item.Stack{Kind: item.Meal, Qty: meals})
	}
	return fleetOf(g).At(cid)
}

// mealsOf is the meals caravan cid carries.
func mealsOf(g *Game, cid uint64) int {
	n := 0
	for _, s := range travelOf(g).Goods(cid) {
		if s.Kind == item.Meal {
			n += s.Qty
		}
	}
	return n
}

// roadHour runs one road hour of the fleet.
func roadHour(g *Game, h int) { fleetOf(g).Step(uint64(h) * clock.TicksPerHour) }

func hungerOf(g *Game, id entity.EntityID) int {
	n, _ := g.need.Get(id)
	return n.Hunger
}

func TestCaravanTakesPrisoner(t *testing.T) {
	t.Parallel()
	g := worldGame(t)
	ada := g.pawnIDs[0]
	rid := spawnPrisoner(t, g, ada)
	c := formCaravan(t, g, nearDest(t, g), ada, rid)
	if !slices.Contains(c.C.Members, uint64(rid)) || c.Captives != nil {
		t.Fatalf("members %v captives %v", c.C.Members, c.Captives)
	}
	rec := stashRecOf(g, rid)
	if rec == nil || !rec.Prisoner || rec.Raider == nil || !g.isStashed(rid) || g.onMap(rid) {
		t.Fatalf("the prisoner's stash record: %+v", rec)
	}
	v := g.WorldMap().Caravans
	if len(v) != 1 || len(v[0].Prisoner) != len(v[0].Members) || len(v[0].Members) != 2 {
		t.Fatalf("world map view %+v", v)
	}
	for i, m := range c.C.Members {
		if v[0].Prisoner[i] != (entity.EntityID(m) == rid) {
			t.Fatalf("prisoner marks %v for members %v", v[0].Prisoner, c.C.Members)
		}
	}
}

func TestCaravanPrisonerComesBackAPrisoner(t *testing.T) {
	t.Parallel()
	g := worldGame(t)
	ada := g.pawnIDs[0]
	rid := spawnPrisoner(t, g, ada)
	c := formCaravan(t, g, nearDest(t, g), ada, rid)
	cid := entity.EntityID(c.C.ID)
	for i := 0; i < 80 && fleetOf(g).At(cid) != nil; i++ {
		worldForward(g, 2500)
	}
	if fleetOf(g).At(cid) != nil {
		t.Fatal("the caravan never came home")
	}
	if !g.prison.Is(rid) || !slices.Contains(g.raiderIDs, rid) || slices.Contains(g.pawnIDs, rid) || g.isStashed(rid) {
		t.Fatalf("prisoner %v raider %v colonist %v stashed %v", g.prison.Is(rid), slices.Contains(g.raiderIDs, rid), slices.Contains(g.pawnIDs, rid), g.isStashed(rid))
	}
}

func TestFormRejectsPrisonerOnlyAndDownedPrisoner(t *testing.T) {
	t.Parallel()
	g := worldGame(t)
	ada, bo := g.pawnIDs[0], g.pawnIDs[1]
	dest := nearDest(t, g)
	rid, down := spawnPrisoner(t, g, ada), spawnPrisoner(t, g, ada)

	g.Enqueue(command.Command{Kind: command.FormCaravan, Name: fmt.Sprint(rid), Count: dest})
	g.Step(2)
	if len(g.worldS().caravans) != 0 {
		t.Fatal("prisoners alone formed a caravan")
	}

	g.hurt(down, health.DownedInjury+10, 0)
	g.Step(1)
	g.Enqueue(command.Command{Kind: command.FormCaravan, Name: fmt.Sprintf("%d,%d,%d", ada, rid, down), Count: dest})
	g.Step(1)
	first := fleetOf(g).Of(ada)
	if first == nil || !slices.Equal(first.Captives, []entity.EntityID{rid}) {
		t.Fatalf("first caravan %+v: only the standing prisoner may wait to board", first)
	}

	g.Enqueue(command.Command{Kind: command.FormCaravan, Name: fmt.Sprintf("%d,%d", bo, rid), Count: dest})
	g.Step(1)
	second := fleetOf(g).Of(bo)
	if second == nil || len(second.Captives) != 0 {
		t.Fatalf("a prisoner already waiting was taken twice: %+v", second)
	}

	// called off: undraft the colonist before anyone leaves
	g.setDraft(ada, false)
	g.setDraft(bo, false)
	g.Step(200)
	if fleetOf(g).Of(ada) != nil || fleetOf(g).Of(bo) != nil || len(g.worldS().caravans) != 0 {
		t.Fatalf("the caravans were not called off: %+v", g.worldS().caravans)
	}
	if !g.prison.Is(rid) || g.isStashed(rid) || !g.onMap(rid) {
		t.Fatal("the called-off caravan did not leave its prisoner on the map")
	}
}

func TestCaravanSetsOutWithoutFixedFood(t *testing.T) {
	t.Parallel()
	g := worldGame(t)
	ada := g.pawnIDs[0]
	c := formWithFood(t, g, nearDest(t, g), 12, ada)
	if c.C.FoodDays != 0 {
		t.Fatalf("FoodDays = %d, want 0: the food is the cargo", c.C.FoodDays)
	}
	days := caravan.Days(caravan.Nutrition(travelOf(g).Goods(c.C.ID)), len(c.C.Members)) / 1000
	if days <= 0 {
		t.Fatalf("setup: %d days", days)
	}
	if !hasLetter(g, fmt.Sprintf("with %d days of food.", days)) {
		t.Fatalf("the letter does not say %d days: %+v", days, g.World().Messages)
	}
	if got := g.WorldMap().Caravans[0].FoodDays; got < int(days)*10 || got > int(days)*10+9 {
		t.Fatalf("view food days %d tenths, want about %d days", got, days)
	}
}

func TestCaravanEatsPackedFood(t *testing.T) {
	t.Parallel()
	g := worldGame(t)
	a, b := g.pawnIDs[0], g.pawnIDs[1]
	c := roadCaravan(t, g, 0, 10, a, b)
	cid := c.C.ID
	unit, need := item.Meal.Nutrition(), caravan.HourNeed(2)
	have := func() int {
		return caravan.Nutrition(travelOf(g).Goods(cid)) + int(fleetOf(g).At(entity.EntityID(cid)).Fed)
	}
	hours := 0
	for have() >= need && hours < 100 {
		roadHour(g, hours+1)
		hours++
		if left, want := mealsOf(g, cid), 10-(hours*need+unit-1)/unit; left != want {
			t.Fatalf("hour %d: %d meals left, want %d", hours, left, want)
		}
		if hungerOf(g, a) != needs.Full || hungerOf(g, b) != needs.Full {
			t.Fatalf("hour %d: somebody starved while the food lasted", hours)
		}
	}
	if hours < 4 {
		t.Fatalf("10 meals fed two people for %d hours", hours)
	}
	roadHour(g, hours+1) // not enough left for an hour: they eat it and starve
	if got := hungerOf(g, b); got != needs.Full-needs.HungerPerHour || mealsOf(g, cid) != 0 {
		t.Fatalf("a short hour: hunger %d (want %d), meals %d (want 0)", got, needs.Full-needs.HungerPerHour, mealsOf(g, cid))
	}
	roadHour(g, hours+2)
	if got := hungerOf(g, b); got != needs.Full-2*needs.HungerPerHour {
		t.Fatalf("with the cargo empty hunger %d, want a further drop of %d", got, needs.HungerPerHour)
	}
}

func TestMealsEatenMatchTheNeed(t *testing.T) {
	t.Parallel()
	g := worldGame(t)
	a, b := g.pawnIDs[0], g.pawnIDs[1]
	c := roadCaravan(t, g, 0, 20, a, b)
	unit, need, hrs := item.Meal.Nutrition(), caravan.HourNeed(2), 6
	for h := 1; h <= hrs; h++ {
		roadHour(g, h)
	}
	if want := 20 - (hrs*need+unit-1)/unit; mealsOf(g, c.C.ID) != want {
		t.Fatalf("meals left %d, want %d", mealsOf(g, c.C.ID), want)
	}
	if hungerOf(g, a) != needs.Full || hungerOf(g, b) != needs.Full {
		t.Fatal("somebody starved while the food lasted")
	}
}

func TestLegacyCaravanKeepsFoodDays(t *testing.T) {
	t.Parallel()
	g := worldGame(t)
	a := g.pawnIDs[0]
	c := roadCaravan(t, g, 5000, 0, a)
	prev := c.C.FoodDays
	for h := 1; h <= 200; h++ {
		h0 := hungerOf(g, a)
		roadHour(g, h)
		c = fleetOf(g).At(entity.EntityID(c.C.ID))
		got := hungerOf(g, a)
		if c.C.FoodDays > 0 && got != h0 {
			t.Fatalf("hour %d: starved with %d days left", h, c.C.FoodDays)
		}
		if c.C.FoodDays == 0 {
			if prev == 0 || got != h0-needs.HungerPerHour {
				t.Fatalf("hour %d: reserve %d -> 0, hunger %d -> %d, want the first starving hour here", h, prev, h0, got)
			}
			if h < 100 || h > 130 {
				t.Fatalf("5000 milli-days lasted %d hours", h)
			}
			break
		}
		if prev-c.C.FoodDays != 42 && prev-c.C.FoodDays != 41 {
			t.Fatalf("hour %d: the reserve fell by %d", h, prev-c.C.FoodDays)
		}
		prev = c.C.FoodDays
	}
	if c.C.FoodDays != 0 {
		t.Fatal("the reserve never ran out")
	}

	// the hour the reserve reaches 0, the cargo feeds a full hour
	g2 := worldGame(t)
	b := g2.pawnIDs[0]
	c2 := roadCaravan(t, g2, 30, 4, b)
	roadHour(g2, 1)
	if c2 = fleetOf(g2).At(entity.EntityID(c2.C.ID)); c2.C.FoodDays != 0 {
		t.Fatalf("reserve %d after the hour, want 0", c2.C.FoodDays)
	}
	if hungerOf(g2, b) != needs.Full || mealsOf(g2, c2.C.ID) >= 4 {
		t.Fatalf("hunger %d meals %d: the cargo should have fed the hour the reserve ran out", hungerOf(g2, b), mealsOf(g2, c2.C.ID))
	}
}

func TestCaravanFoodSaveLoad(t *testing.T) {
	t.Parallel()
	reload := func(t *testing.T, g *Game) *Game {
		t.Helper()
		h, _ := saveLoadInfo(t, g)
		h.worldS().planet, h.worldS().planetTried = testPlanet().Fork(), true
		if g.StateHash() != h.StateHash() {
			t.Fatal("state hash changed in the round trip")
		}
		return h
	}
	t.Run("forming with a captive", func(t *testing.T) {
		t.Parallel()
		g := worldGame(t)
		ada := g.pawnIDs[0]
		rid := spawnPrisoner(t, g, ada)
		g.Enqueue(command.Command{Kind: command.FormCaravan, Name: fmt.Sprintf("%d,%d", ada, rid), Count: nearDest(t, g)})
		g.Step(2)
		c := fleetOf(g).Of(ada)
		if c == nil || c.C.State != caravan.Forming || !slices.Equal(c.Captives, []entity.EntityID{rid}) {
			t.Fatalf("setup: %+v", c)
		}
		h := reload(t, g)
		if hc := fleetOf(h).Of(ada); hc == nil || !slices.Equal(hc.Captives, c.Captives) {
			t.Fatalf("loaded caravan %+v", hc)
		}
		stepFeeding(g, 1500)
		stepFeeding(h, 1500)
		if g.StateHash() != h.StateHash() {
			t.Fatal("the loaded game diverged")
		}
	})
	t.Run("travelling with part-eaten cargo", func(t *testing.T) {
		t.Parallel()
		g := worldGame(t)
		a := g.pawnIDs[0]
		c := roadCaravan(t, g, 0, 5, a)
		cid := entity.EntityID(c.C.ID)
		for h := 1; h <= 3; h++ {
			roadHour(g, h)
		}
		if c = fleetOf(g).At(cid); c.Fed <= 0 || mealsOf(g, c.C.ID) >= 5 {
			t.Fatalf("setup: fed %d meals %d", c.Fed, mealsOf(g, c.C.ID))
		}
		h := reload(t, g)
		hc := fleetOf(h).At(cid)
		if hc == nil || hc.Fed != c.Fed || mealsOf(h, hc.C.ID) != mealsOf(g, c.C.ID) {
			t.Fatalf("loaded caravan %+v", hc)
		}
	})
	t.Run("old format", func(t *testing.T) {
		t.Parallel()
		g := worldGame(t)
		a := g.pawnIDs[0]
		roadCaravan(t, g, maps.FoodDays, 0, a) // Captives nil, Fed 0, a reserve
		h := reload(t, g)
		if hc := fleetOf(h).Of(a); hc == nil || hc.Fed != 0 || hc.Captives != nil || hc.C.FoodDays != maps.FoodDays {
			t.Fatalf("loaded caravan %+v", hc)
		}
		for i := 1; i <= 5; i++ {
			roadHour(h, i)
		}
		if hc := fleetOf(h).Of(a); hc == nil || hc.C.FoodDays >= maps.FoodDays {
			t.Fatalf("the loaded caravan did not travel: %+v", hc)
		}
	})
}

func TestFormEstimateMatchesTravelEstimate(t *testing.T) {
	t.Parallel()
	g := worldGame(t)
	ada := g.pawnIDs[0]
	dest := nearDest(t, g)
	g.Enqueue(command.Command{Kind: command.FormCaravan, Name: fmt.Sprint(ada), Count: dest})
	g.Step(1)
	c := fleetOf(g).Of(ada)
	if c == nil || c.C.State != caravan.Forming {
		t.Fatal("setup: the caravan is not forming")
	}
	want, why := g.TravelEstimate(c.C.ID, dest) // taken at the same tick as the plan
	got, gwhy := g.FormEstimate([]entity.EntityID{ada}, dest, c.C.Load)
	if why != "" || gwhy != "" || got != want || got <= 0 {
		t.Fatalf("FormEstimate %d %q, TravelEstimate %d %q", got, gwhy, want, why)
	}
	for _, e := range []struct {
		dest int
		want string
	}{{-1, "Pick a destination for the caravan."}, {g.colonyTile(), "Pick a destination for the caravan."}} {
		if _, why := g.FormEstimate([]entity.EntityID{ada}, e.dest, 0); why != e.want {
			t.Fatalf("dest %d: %q, want %q", e.dest, why, e.want)
		}
	}
	legacy := NewColony(Config{Seed: 2, Width: 40, Height: 40})
	if _, why := legacy.FormEstimate([]entity.EntityID{legacy.pawnIDs[0]}, 3, 0); why != "No world map." {
		t.Fatalf("no world: %q", why)
	}
}

func TestReformKeepsCargoFood(t *testing.T) {
	t.Parallel()
	g := worldGame(t)
	tile, ada := plainNear(t, g), g.pawnIDs[0]
	cid := arriveAt(t, g, tile, ada)
	fleetOf(g).At(cid).C.FoodDays = 0
	travelOf(g).Carry(uint64(cid), item.Stack{Kind: item.Meal, Qty: 20})
	g.Enqueue(command.Command{Kind: command.EnterMap, Target: uint64(cid)})
	g.Step(1)
	m := g.maps[1]
	g.mapState = g.view
	g.Enqueue(command.Command{Kind: command.CaravanReform, Target: uint64(m.id), Map: m.id, MapSet: true})
	g.Step(1)
	c := fleetOf(g).Of(ada)
	if c == nil || c.C.State == caravan.Forming {
		t.Fatalf("the site did not reform: %+v", c)
	}
	if c.C.FoodDays != 0 {
		t.Fatalf("FoodDays = %d: reform grants no reserve", c.C.FoodDays)
	}
	if got := mealsOf(g, c.C.ID); got < 20 {
		t.Fatalf("the reformed caravan carries %d meals, want at least the 20 it dropped", got)
	}
	days := caravan.Days(caravan.Nutrition(travelOf(g).Goods(c.C.ID)), len(c.C.Members)) / 1000
	if !hasLetter(g, fmt.Sprintf("with %d days of food.", days)) {
		t.Fatalf("the departure letter does not say %d days", days)
	}
}
