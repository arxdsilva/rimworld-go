// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"fmt"
	"reflect"
	"rimworld-go/internal/domain/sim/clock"
	"slices"
	"sync"
	"testing"

	"rimworld-go/internal/application/game/maps"
	"rimworld-go/internal/application/game/travel"
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/pawn/mentalbreak"
	"rimworld-go/internal/domain/pawn/mood"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/pawn/social"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/caravan"
	"rimworld-go/internal/domain/world/planet"
)

var (
	worldTestParams = planet.Params{SeedText: "caravan", Coverage: 30, Rain: 2, Temp: 2, Pop: 1, Factions: faction.DefaultSlots()}
	worldTestOnce   sync.Once
	worldTestPlanet *planet.Planet
)

// testPlanet is the caravan tests' planet, built once (read-only: Route
// allocates its own scratch).
func testPlanet() *planet.Planet {
	worldTestOnce.Do(func() { worldTestPlanet = planet.Build(worldTestParams) })
	return worldTestPlanet
}

// TestWorldTestPlanetStaysReadOnly proves testPlanet is safe to share:
// parallel colonies built on it leave it (hop scratch aside, which each
// colony forks) equal to a fresh build. The race detector covers the reads.
func TestWorldTestPlanetStaysReadOnly(t *testing.T) {
	t.Run("colonies", func(t *testing.T) {
		for i := range 4 {
			t.Run(fmt.Sprint(i), func(t *testing.T) {
				t.Parallel()
				g := worldGame(t)
				nearDest(t, g)
				g.Step(30)
			})
		}
	})
	if got, want := testPlanet().Fork(), planet.Build(worldTestParams).Fork(); !reflect.DeepEqual(got, want) {
		t.Fatal("a caravan test mutated the shared planet")
	}
}

// worldGame is a landed Broken Hull colony on testPlanet's first random
// landing tile (60x60), the planet already in its world state.
func worldGame(t *testing.T) *Game {
	t.Helper()
	p := testPlanet()
	tile := p.RandomLanding(0)
	if tile < 0 {
		t.Fatal("no landing tile")
	}
	g := buildSetup(t, setupFor(t, "broken_hull", func(s *Setup) {
		s.MapSize, s.World, s.Tile, s.Site = 60, worldTestParams, tile, p.Site(tile)
		s.Factions = cloneFactionList(p.Factions)
	}))
	stepUntilLanded(t, g)
	w := g.worldS()
	w.planet, w.planetTried = p.Fork(), true // its own hop scratch: tests run in parallel
	return g
}

// nearDest is a tile two steps from home over land, made a settlement of
// a neutral human faction of g.
func nearDest(t *testing.T, g *Game) int {
	t.Helper()
	p, home := testPlanet(), g.colonyTile()
	for _, nb := range p.Neighbours(home) {
		for _, nn := range p.Neighbours(int(nb)) {
			if int(nn) == home || slices.Contains(p.Neighbours(home), nn) {
				continue
			}
			if r := p.Route(home, int(nn)); len(r) == 3 && p.Settlement[nn] < 0 {
				friendlySettlement(t, g, int(nn))
				return int(nn)
			}
		}
	}
	t.Fatal("no tile two steps from home")
	return -1
}

// friendlySettlement gives tile to g's first human faction, made neutral.
func friendlySettlement(t *testing.T, g *Game, tile int) {
	t.Helper()
	for i := range g.ng.factions {
		f := &g.ng.factions[i]
		if f.Type.Human() {
			f.Relation, f.Defeated = faction.Neutral, false
			f.Settlements = append(f.Settlements, tile)
			return
		}
	}
	t.Fatal("no human faction")
}

// formCaravan orders ids to dest and steps until all of them are off the
// map.
func formCaravan(t *testing.T, g *Game, dest int, ids ...entity.EntityID) *maps.Caravan {
	t.Helper()
	list := ""
	for i, id := range ids {
		if i > 0 {
			list += ","
		}
		list += fmt.Sprint(id)
	}
	g.Enqueue(command.Command{Kind: command.FormCaravan, Name: list, Count: dest})
	for i := 0; i < 60; i++ {
		g.Step(60)
		if c := fleetOf(g).Of(ids[0]); c != nil && c.C.State != caravan.Forming {
			return c
		}
	}
	t.Fatalf("the caravan never set out: %+v", g.worldSt)
	return nil
}

// worldForward runs only the world phase for ticks (the map stands still):
// the members are off the map, so nothing else involves them.
func worldForward(g *Game, ticks int) {
	for uint64(g.clock.Now())%10 != 0 {
		g.clock.Advance(1)
		runPhase(g, PhaseWorld)
		ticks--
	}
	for ; ticks > 0; ticks -= 10 {
		g.clock.Advance(10)
		runPhase(g, PhaseWorld)
	}
}

func TestCaravanOutAndBackKeepsRelations(t *testing.T) {
	t.Parallel()
	g := worldGame(t)
	if len(g.pawnIDs) < 3 {
		t.Fatalf("%d colonists", len(g.pawnIDs))
	}
	a, b, home := g.pawnIDs[0], g.pawnIDs[1], g.pawnIDs[2]
	now := uint64(g.clock.Now())
	g.setPair(a, b, social.Pair{Kind: social.PairLover, Since: now, LastChange: now})
	g.addOpinionMemory(a, home, social.MemKindWords, now)
	names := [2]string{g.personName(a), g.personName(b)}
	dest := nearDest(t, g)

	c := formCaravan(t, g, dest, a, b)
	cid := entity.EntityID(c.C.ID)
	for _, id := range []entity.EntityID{a, b} {
		if slices.Contains(g.pawnIDs, id) || !g.isStashed(id) || g.onMap(id) {
			t.Fatalf("member %d is still on the map", id)
		}
		if p := g.personOf(id); p == nil || p.Where != whereCaravan {
			t.Fatalf("member %d registry %+v", id, p)
		}
	}
	if len(g.ng.dead) != 0 {
		t.Fatal("stashed members were counted as lost")
	}
	if v := g.WorldMap(); len(v.Caravans) != 1 || v.Caravans[0].State != "Travelling" || len(v.Caravans[0].Members) != 2 {
		t.Fatalf("world map view %+v", v)
	}

	visited := false
	for i := 0; i < 80 && fleetOf(g).At(cid) != nil; i++ {
		worldForward(g, 2500)
		if c := fleetOf(g).At(cid); c != nil && c.C.State == caravan.Visiting {
			visited = true
		}
	}
	if !visited {
		t.Fatal("the caravan never visited the settlement")
	}
	if fleetOf(g).At(cid) != nil {
		t.Fatalf("the caravan never came home: %+v", fleetOf(g).At(cid).C)
	}
	for i, id := range []entity.EntityID{a, b} {
		if !slices.Contains(g.pawnIDs, id) || g.isStashed(id) || !g.onMap(id) || g.personName(id) != names[i] {
			t.Fatalf("member %d did not come back as itself", id)
		}
		if p := g.personOf(id); p.Where != whereColony {
			t.Fatalf("member %d registry where %d", id, p.Where)
		}
	}
	if !slices.IsSorted(g.pawnIDs) {
		t.Fatalf("pawnIDs out of order: %v", g.pawnIDs)
	}
	if g.pairOf(a, b).Kind != social.PairLover || g.edgeOf(a, home) == nil {
		t.Fatal("relationships changed over the journey")
	}
	g.Step(600) // they walk back in on the live map
	for _, id := range []entity.EntityID{a, b} {
		if !g.alive(id) {
			t.Fatalf("member %d not alive after walking in", id)
		}
	}
}

func TestCaravanDeathAwayGrieves(t *testing.T) {
	t.Parallel()
	g := worldGame(t)
	a, b := g.pawnIDs[0], g.pawnIDs[1]
	now := uint64(g.clock.Now())
	g.setPair(a, b, social.Pair{Kind: social.PairSpouse, Since: now, LastChange: now})
	dest := nearDest(t, g)
	c := formCaravan(t, g, dest, b)
	c.C.FoodDays = 0
	n, _ := g.need.Get(b)
	n.Hunger = 0
	g.need.Set(b, n)
	malnourish(g, b, 990) // food1_test.go: on its last hours (FOOD-1)
	for range 3 {
		worldForward(g, 2500)
		stepMalnutrition(g, uint64(g.clock.Now())) // the hourly medical phase's step
	}
	v, _ := g.vital.Get(b)
	if v.Status != health.Dead || g.isStashed(b) {
		t.Fatalf("the starving member is %v (stashed %v)", v.Status, g.isStashed(b))
	}
	if p := g.personOf(b); p.Where != whereDead {
		t.Fatalf("registry where %d", p.Where)
	}
	if _, st, ok := g.bodyOf(b); ok {
		t.Fatalf("a corpse appeared on the map: %+v", st)
	}
	grief, died := false, false
	for _, m := range g.memories[a] {
		grief = grief || m.Kind == mood.Grief && m.Subject == uint64(b)
		died = died || m.Kind == mood.ColonistDied
	}
	if !grief || !died {
		t.Fatalf("the spouse at home does not grieve: grief %v, colonist died %v", grief, died)
	}
	if g.WorldMap().Caravans != nil {
		t.Fatal("an empty caravan is still on the world map")
	}
}

func TestCaravanSaveLoadInTransit(t *testing.T) {
	t.Parallel()
	g := worldGame(t)
	a := g.pawnIDs[0]
	c := formCaravan(t, g, nearDest(t, g), a)
	cid := entity.EntityID(c.C.ID)
	worldForward(g, 5000)
	if c := fleetOf(g).At(cid); c == nil || c.C.State != caravan.Travelling {
		t.Fatal("setup: the caravan should be on the road")
	}
	h, _ := saveLoadInfo(t, g)
	h.worldS().planet, h.worldS().planetTried = testPlanet().Fork(), true
	if g.StateHash() != h.StateHash() {
		t.Fatal("state hash changed in the round trip")
	}
	for i := 0; i < 6; i++ {
		g.Step(2500)
		h.Step(2500)
		if g.StateHash() != h.StateHash() {
			t.Fatalf("step %d: the loaded game diverged", i)
		}
	}
}

func TestCaravanReturnCommandTurnsBack(t *testing.T) {
	t.Parallel()
	g := worldGame(t)
	a := g.pawnIDs[0]
	c := formCaravan(t, g, nearDest(t, g), a)
	cid := entity.EntityID(c.C.ID)
	worldForward(g, 2500*4)
	g.Enqueue(command.Command{Kind: command.CaravanReturn, Target: uint64(cid)})
	g.ApplyPending()
	if c := fleetOf(g).At(cid); c != nil && c.C.State != caravan.Returning {
		t.Fatalf("state after Return: %v", c.C.State)
	}
	for i := 0; i < 40 && fleetOf(g).At(cid) != nil; i++ {
		worldForward(g, 2500)
	}
	if fleetOf(g).At(cid) != nil || !slices.Contains(g.pawnIDs, a) {
		t.Fatal("the caravan did not come home")
	}
}

func TestCaravanFormingCancelledByUndraft(t *testing.T) {
	t.Parallel()
	g := worldGame(t)
	a := g.pawnIDs[0]
	g.Enqueue(command.Command{Kind: command.FormCaravan, Name: fmt.Sprint(a), Count: nearDest(t, g)})
	g.Step(2)
	if j, _ := g.jobs.Get(a); j.Kind != job.ExitMap || !g.drafted(a) {
		t.Fatalf("member not walking out: %v drafted %v", j.Kind, g.drafted(a))
	}
	g.setDraft(a, false)
	g.Step(120)
	if fleetOf(g).Of(a) != nil || g.isStashed(a) || len(g.worldSt.caravans) != 0 {
		t.Fatal("an undrafted member should stay home and the caravan be called off")
	}
}

func TestLegacyColonyCannotFormCaravan(t *testing.T) {
	t.Parallel()
	g := NewColony(Config{Seed: 2, Width: 40, Height: 40})
	g.Enqueue(command.Command{Kind: command.FormCaravan, Name: fmt.Sprint(g.pawnIDs[0]), Count: 3})
	g.Step(5)
	if g.worldSt != nil && len(g.worldSt.caravans) != 0 {
		t.Fatal("a colony without a world formed a caravan")
	}
	if v := g.WorldMap(); v.Tile != -1 || len(v.Caravans) != 0 {
		t.Fatalf("legacy world map view %+v", v)
	}
}

func TestStashRestoresComponents(t *testing.T) {
	t.Parallel()
	g := NewColony(Config{Seed: 2, Width: 40, Height: 40})
	id := g.pawnIDs[1]
	name := g.personName(id)
	g.setDraft(id, true)
	if !g.stashPawn(id, 99) {
		t.Fatal("stash failed")
	}
	if _, ok := g.positions.Get(id); ok || slices.Contains(g.pawnIDs, id) || g.drafted(id) {
		t.Fatal("stashed pawn keeps map components")
	}
	if _, ok := g.vital.Get(id); !ok || g.personName(id) != name {
		t.Fatal("stashed pawn lost its identity")
	}
	g.Step(50) // the colony runs on without it
	if !g.unstashPawn(id, 5, 5) {
		t.Fatal("unstash failed")
	}
	if p, ok := g.positions.Get(id); !ok || p != (Position{5, 5}) || !slices.IsSorted(g.pawnIDs) {
		t.Fatal("unstashed pawn not back at (5, 5) in order")
	}
	g.Step(50)
}

// TestWorldDeterminism: two runs of the same colony with a caravan, a
// visitor group and a restricted pawn stay identical.
func TestWorldDeterminism(t *testing.T) {
	t.Parallel()
	run := func() uint64 {
		g := worldGame(t)
		dest := nearDest(t, g)
		g.Enqueue(command.Command{Kind: command.CreateArea}) // PLAY-015: area id 1
		g.Enqueue(command.Command{Kind: command.PaintArea, Index: 1, X: 10, Y: 10, X2: 50, Y2: 50, Enabled: true})
		g.Enqueue(command.Command{Kind: command.AssignArea, Pawn: uint64(g.pawnIDs[2]), Index: 1})
		formCaravan(t, g, dest, g.pawnIDs[0])
		g.visitorsArrive(uint64(g.clock.Now()))
		g.Step(4000)
		worldForward(g, 2500*3)
		g.Step(500)
		return g.StateHash()
	}
	if a, b := run(), run(); a != b {
		t.Fatalf("state hash %#x vs %#x", a, b)
	}
}

// TestEveryoneGone: the ending waits for the last living colonist under our
// control (MAP-14, wiki: Ending). Someone due to land or on the road holds it
// off; the dead, the kidnapped and prisoners don't.
func TestEveryoneGone(t *testing.T) {
	t.Parallel()
	g := worldGame(t)
	if everyoneGone(g) {
		t.Fatal("a colony with colonists is gone")
	}
	ada, tile := g.pawnIDs[0], plainNear(t, g)
	arriveAt(t, g, tile, ada) // on the road
	for _, id := range slices.Clone(g.pawnIDs) {
		kill(g, id)
	}
	if everyoneGone(g) {
		t.Fatal("gone with a colonist on the road")
	}
	g.setWhere(ada, whereKidnapped)
	if !everyoneGone(g) {
		t.Fatal("the kidnapped still count")
	}
	g.setWhere(ada, whereCaravan)
	kill(g, ada)
	if !everyoneGone(g) {
		t.Fatal("a dead caravan member still counts")
	}
	g.ng.pods = []podState{{Pawns: []int{0}}}
	if everyoneGone(g) {
		t.Fatal("gone with a pod still to land")
	}
	g.ng.pods[0].Landed = true
	if !everyoneGone(g) {
		t.Fatal("a landed pod still counts")
	}
}

func TestCaravanMemoryExpirySurvivesSaveLoad(t *testing.T) {
	t.Parallel()
	g := worldGame(t)
	id := g.pawnIDs[0]
	formCaravan(t, g, nearDest(t, g), id)
	if !g.isStashed(id) {
		t.Fatal("caravan member not stashed")
	}
	day := uint64(clock.TicksPerDay)
	due := (uint64(g.clock.Now())/day+1)*day + 875
	live := mood.Memory{Kind: mood.SleptInNiceRoom, Expires: due + day}
	g.memories[id] = mood.Memories{{Kind: mood.AteRaw, Expires: due}, live}
	g.clock.Advance(int(due - uint64(g.clock.Now())))
	runPhase(g, PhaseSocial)
	if m := g.memories[id]; len(m) != 1 || m[0] != live {
		t.Fatalf("caravan member kept expired memory: %+v", m)
	}
	h := saveAndLoad(t, g)
	if m := h.memories[id]; len(m) != 1 || m[0] != live {
		t.Fatalf("loaded caravan memory %+v", m)
	}
	if g.StateHash() != h.StateHash() {
		t.Fatal("caravan state changed during save/load")
	}
	g.Step(250)
	h.Step(250)
	if g.StateHash() != h.StateHash() {
		t.Fatal("caravan state diverged after resuming")
	}
}

// A nearby map border can be fully blocked while a different reachable
// border still permits the same pawn to leave.
func TestCaravanExitFallsBackFromFullyBlockedNearEdge(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 31, Width: 36, Height: 22})
	id, err := g.SpawnNamedPawn(5, 11, "Ada")
	if err != nil {
		t.Fatal(err)
	}
	for y := 0; y < g.Height(); y++ {
		for x := 0; x < 4; x++ {
			g.grid.SetWalkable(x, y, false)
		}
	}
	if _, _, ok := g.combat.Tiles.WalkableNear(0, 11, 3); ok {
		t.Fatal("fixture has an open tile at the near border")
	}
	if !(mapsWalk{g}).StartExit(id) {
		t.Fatal("alternate reachable border was not tried")
	}
	j, ok := g.jobs.Get(id)
	if !ok || j.Kind != job.ExitMap || !j.Active() ||
		j.TX != 0 && j.TX != g.Width()-1 && j.TY != 0 && j.TY != g.Height()-1 {
		t.Fatalf("wrong exit job after fallback: %+v", j)
	}
}

func TestCaravanHeadlessReadModelRoundTrip(t *testing.T) {
	g := worldGame(t)
	a, b := g.pawnIDs[0], g.pawnIDs[1]
	c := formCaravan(t, g, nearDest(t, g), a, b)
	c.C.FoodDays = 1250
	travelOf(g).Carry(c.C.ID, item.Stack{Kind: item.Wood, Qty: 7})
	g.socState().moodM[a] = 1000
	delete(g.socState().moodM, b)
	n, _ := g.need.Get(a)
	n.Hunger = 0
	g.need.Set(a, n.Settle())
	base := g.StateHash()
	v := g.WorldMap().Caravans[0]
	if v.FoodDays != 1250 || v.SpeedPermil != c.C.Speed() || len(v.Cargo) == 0 || len(v.MemberFacts) != 2 {
		t.Fatalf("native caravan facts missing: %+v", v)
	}
	m := v.MemberFacts[0]
	if m.ID != a || !m.MoodKnown || m.Mood != 1 || m.BreakTier != mentalbreak.TierAt(1, mentalbreak.ShiftThresholds(mentalbreak.DefaultThresholds(), g.breakOffset(a))) || !m.HungerKnown || m.Hunger != 0 || m.HungerBand != needs.Malnourished || v.MemberFacts[1].MoodKnown {
		t.Fatalf("native member facts wrong: %+v", v.MemberFacts)
	}
	if g.StateHash() != base {
		t.Fatal("reading the view changed authoritative state")
	}
	expected := g.WorldMap()
	v.Cargo[0].Qty = 9999
	v.MemberFacts[0].Mood = 99
	v.Members[0] = "mutated"
	if !reflect.DeepEqual(g.WorldMap(), expected) {
		t.Fatal("caravan read model aliases live state")
	}
	h, _ := saveLoadInfo(t, g)
	h.worldS().planet, h.worldS().planetTried = testPlanet().Fork(), true
	if h.StateHash() != base || !reflect.DeepEqual(h.WorldMap(), expected) {
		t.Fatal("caravan read model or authoritative state changed on load")
	}
}

func TestCaravanSplitCommandConservesOwnersAndReplay(t *testing.T) {
	g := worldGame(t)
	a, b := g.pawnIDs[0], g.pawnIDs[1]
	c := formCaravan(t, g, nearDest(t, g), a, b)
	cid := c.C.ID
	c.C.FoodDays = 1732
	c.C.Progress = 313
	x, y := ax9Free(t, g)
	packed, err := g.SpawnAnimal("donkey", x, y)
	if err != nil {
		t.Fatal(err)
	}
	stashed, err := g.SpawnAnimal("horse", x, y)
	if err != nil {
		t.Fatal(err)
	}
	g.animals.Ref(packed).Tame = true
	g.animals.Ref(stashed).Tame = true
	packedState := g.animals.Ref(packed).State()
	g.detachAnimal(packed)
	if !g.stashPawn(stashed, entity.EntityID(cid)) {
		t.Fatal("stash pack animal")
	}
	c.C.Animals = append(c.C.Animals, uint64(stashed))
	// Exact physical lots, one hold-owned animal and one native stashed animal.
	travelOf(g).Carry(cid, item.Stack{Kind: item.Steel, Qty: 30, Forbidden: true})
	travelOf(g).Carry(cid, item.Stack{Kind: item.Cloth, Qty: 5, Quality: 3, Wear: 4})
	saved := g.worldSt.ax9.Export()
	for i := range saved.Cargo {
		if saved.Cargo[i].Caravan == cid {
			saved.Cargo[i].Hold.Animals = append(saved.Cargo[i].Hold.Animals, travel.Beast{ID: packed, State: packedState})
		}
	}
	g.worldSt.ax9 = travel.Import(saved)
	view := g.WorldMap().Caravans[0]
	if view.Split.Reason != "" || view.Split.Quote == 0 || len(view.Split.Animals) != 2 {
		t.Fatalf("split view: %+v", view.Split)
	}
	before := c.Clone()
	initialGoods := travelOf(g).Goods(cid)
	// Duplicate and all-member selections must not move state or spend an ID.
	for _, cmd := range []command.Command{
		{Kind: command.CaravanSplit, Target: cid, Quote: view.Split.Quote + 1, Name: travel.EncodeSplit(travel.SplitSelection{Members: []uint64{uint64(b)}})},
		{Kind: command.CaravanSplit, Target: cid, Quote: view.Split.Quote, Name: travel.EncodeSplit(travel.SplitSelection{Members: []uint64{uint64(b), uint64(b)}})},
		{Kind: command.CaravanSplit, Target: cid, Quote: view.Split.Quote, Name: travel.EncodeSplit(travel.SplitSelection{Members: []uint64{uint64(a), uint64(b)}})},
		{Kind: command.CaravanSplit, Target: cid, Quote: view.Split.Quote, Name: travel.EncodeSplit(travel.SplitSelection{Members: []uint64{uint64(b)}, Lots: []travel.SplitLot{{Index: 0, Qty: 99999}}})},
	} {
		g.Enqueue(cmd)
		g.ApplyPending()
		if len(g.worldSt.caravans) != 1 || !reflect.DeepEqual(g.worldSt.caravans[0], before) || !reflect.DeepEqual(travelOf(g).Goods(cid), initialGoods) {
			t.Fatal("rejected split mutated owner")
		}
	}
	// Save the actual pending command, then execute on both native owners.
	cmd := command.Command{Kind: command.CaravanSplit, Target: cid, Quote: view.Split.Quote, Name: travel.EncodeSplit(travel.SplitSelection{Members: []uint64{uint64(b)}, Animals: []uint64{uint64(packed), uint64(stashed)}, Lots: []travel.SplitLot{{Index: 0, Qty: 13}, {Index: 1, Qty: 5}}})}
	g.Enqueue(cmd)
	h, _ := saveLoadInfo(t, g)
	h.worldS().planet, h.worldS().planetTried = testPlanet().Fork(), true
	for _, gg := range []*Game{g, h} {
		gg.ApplyPending()
		if len(gg.worldSt.caravans) != 2 {
			t.Fatalf("split rejected: %+v", gg.WorldMap())
		}
		left, right := gg.worldSt.caravans[0], gg.worldSt.caravans[1]
		if left.C.ID != cid || right.C.ID <= cid || !slices.Equal(left.C.Members, []uint64{uint64(a)}) || !slices.Equal(right.C.Members, []uint64{uint64(b)}) || left.C.FoodDays != 1732 || right.C.FoodDays != 1732 || left.C.Progress != 313 || right.C.Progress != 313 || !slices.Equal(left.C.Path, right.C.Path) || left.NextEnc != before.NextEnc || right.NextTalk != before.NextTalk {
			t.Fatalf("split changed route or members: %+v %+v", left, right)
		}
		for _, id := range []entity.EntityID{b, stashed} {
			i, ok := slices.BinarySearchFunc(gg.worldSt.stashed, id, stashCmp)
			if !ok || gg.worldSt.stashed[i].Holder != entity.EntityID(right.C.ID) {
				t.Fatalf("wrong holder %d", id)
			}
		}
		v := gg.WorldMap()
		if len(v.Caravans[0].Split.Animals) != 0 || len(travelOf(gg).SplitAnimals(right.C.ID)) != 1 || len(right.C.Animals) != 1 || right.C.Animals[0] != uint64(stashed) {
			t.Fatal("animal owners changed or duplicated")
		}
		l, r := travelOf(gg).Goods(cid), travelOf(gg).Goods(right.C.ID)
		wantLeft, wantRight := initialGoods[0], initialGoods[0]
		wantLeft.Qty, wantRight.Qty = 17, 13
		if !reflect.DeepEqual(l, []item.Stack{wantLeft}) || !reflect.DeepEqual(r, []item.Stack{wantRight, initialGoods[1]}) {
			t.Fatalf("cargo conservation %v %v", l, r)
		}
		if left.C.Load <= 0 || right.C.Load <= 0 || right.C.Mount <= 0 {
			t.Fatal("pace not recomputed with both animal owners")
		}
	}
	if g.StateHash() != h.StateHash() {
		t.Fatal("split pending-command replay diverged")
	}
	rightID := g.worldSt.caravans[1].C.ID
	// Divergent orders cannot alias sibling routes. Both can go home independently.
	for _, gg := range []*Game{g, h} {
		gg.Enqueue(command.Command{Kind: command.CaravanReturn, Target: rightID})
		gg.ApplyPending()
		if other := fleetOf(gg).At(entity.EntityID(cid)); other == nil || !slices.Equal(other.C.Path, before.C.Path) || other.C.Progress != 313 {
			t.Fatal("sibling route aliased")
		}
		gg.Enqueue(command.Command{Kind: command.CaravanReturn, Target: cid})
		gg.ApplyPending()
	}
	if g.StateHash() != h.StateHash() {
		t.Fatal("return replay diverged")
	}
	for i := 0; i < 20 && len(g.worldSt.caravans) > 0; i++ {
		worldForward(g, 2500)
		worldForward(h, 2500)
	}
	if g.StateHash() != h.StateHash() || len(g.worldSt.caravans) != 0 || !g.animals.Has(packed) || !g.animals.Has(stashed) || !g.positions.Has(a) || !g.positions.Has(b) {
		t.Fatal("split owners failed independent return")
	}
}
