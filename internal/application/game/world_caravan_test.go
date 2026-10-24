package game

import (
	"fmt"
	"slices"
	"sync"
	"testing"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/pawn/mood"
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
	w.planet, w.planetTried = p, true
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
func formCaravan(t *testing.T, g *Game, dest int, ids ...entity.EntityID) *caravanRec {
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
		if c := g.caravanOf(ids[0]); c != nil && c.C.State != caravan.Forming {
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
		g.phaseWorld()
		ticks--
	}
	for ; ticks > 0; ticks -= 10 {
		g.clock.Advance(10)
		g.phaseWorld()
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
	for i := 0; i < 80 && g.caravanAt(cid) != nil; i++ {
		worldForward(g, 2500)
		if c := g.caravanAt(cid); c != nil && c.C.State == caravan.Visiting {
			visited = true
		}
	}
	if !visited {
		t.Fatal("the caravan never visited the settlement")
	}
	if g.caravanAt(cid) != nil {
		t.Fatalf("the caravan never came home: %+v", g.caravanAt(cid).C)
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
	n.Hunger = 400
	g.need.Set(b, n)
	worldForward(g, 3*2500)
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
	if c := g.caravanAt(cid); c == nil || c.C.State != caravan.Travelling {
		t.Fatal("setup: the caravan should be on the road")
	}
	h, _ := saveLoadInfo(t, g)
	h.worldS().planet, h.worldS().planetTried = testPlanet(), true
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
	if c := g.caravanAt(cid); c != nil && c.C.State != caravan.Returning {
		t.Fatalf("state after Return: %v", c.C.State)
	}
	for i := 0; i < 40 && g.caravanAt(cid) != nil; i++ {
		worldForward(g, 2500)
	}
	if g.caravanAt(cid) != nil || !slices.Contains(g.pawnIDs, a) {
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
	if g.caravanOf(a) != nil || g.isStashed(a) || len(g.worldSt.caravans) != 0 {
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
		g.Enqueue(command.Command{Kind: command.PaintArea, Index: 0, X: 10, Y: 10, X2: 50, Y2: 50, Enabled: true})
		g.Enqueue(command.Command{Kind: command.AssignArea, Pawn: uint64(g.pawnIDs[2]), Index: 0})
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
