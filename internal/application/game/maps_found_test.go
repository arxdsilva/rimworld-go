package game

import (
	"slices"
	"testing"

	"rimworld-go/internal/application/game/maps"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/biome"
	"rimworld-go/internal/domain/world/caravan"
)

// Founding, abandoning, the storyteller's target and caravan encounters
// (MAP-06, 07, 08, 12).

// landable are free tiles to found on near home, nearest first.
func landable(t *testing.T, g *Game, n int) []int {
	t.Helper()
	p, home := testPlanet(), g.colonyTile()
	var out []int
	for _, a := range p.Neighbours(home) {
		for _, b := range p.Neighbours(int(a)) {
			if ok, _ := p.ValidLanding(int(b)); ok && int(b) != home && !slices.Contains(out, int(b)) && len(p.Route(home, int(b))) >= 2 {
				out = append(out, int(b))
			}
		}
	}
	if len(out) < n {
		t.Fatalf("only %d landable tiles near home", len(out))
	}
	return out[:n]
}

// founded settles ids on tile (cap raised to 5): the new map.
func founded(t *testing.T, g *Game, tile int, ids ...entity.EntityID) *mapState {
	t.Helper()
	SetMaxColonies(g, 5)
	cid := arriveAt(t, g, tile, ids...)
	g.Enqueue(command.Command{Kind: command.SettleHere, Target: uint64(cid)})
	g.Step(1)
	for _, m := range g.maps {
		if m.tile == tile {
			return m
		}
	}
	t.Fatalf("no colony on tile %d: %d maps", tile, len(g.maps))
	return nil
}

// TestSettleSecondColony: the cap refuses at first; raised, a caravan
// founds a colony that ticks and never closes; the cap refuses a third, as
// does a tile next to a settlement.
func TestSettleSecondColony(t *testing.T) {
	t.Parallel()
	g := worldGame(t)
	tiles := landable(t, g, 2)
	ada, bo := g.pawnIDs[0], g.pawnIDs[1]
	cid := arriveAt(t, g, tiles[0], ada)
	if why := SettleReason(g, uint64(cid)); why == "" {
		t.Fatal("the default cap of one colony should refuse a second")
	}
	g.Enqueue(command.Command{Kind: command.SettleHere, Target: uint64(cid)})
	g.Step(1)
	if len(g.maps) != 1 {
		t.Fatal("founded past the cap")
	}
	SetMaxColonies(g, 2)
	if why := SettleReason(g, uint64(cid)); why != "" {
		t.Fatalf("refused under the cap: %s", why)
	}
	g.Enqueue(command.Command{Kind: command.SettleHere, Target: uint64(cid)})
	g.Step(1)
	if len(g.maps) != 2 || fleetOf(g).At(cid) != nil {
		t.Fatalf("no colony: %d maps", len(g.maps))
	}
	m := g.maps[1]
	if m.grid.Width() != g.maps[0].grid.Width() || !m.wild || onMap(g, ada) != m {
		t.Fatalf("colony map %dx, wild %v", m.grid.Width(), m.wild)
	}
	if v := g.Maps()[1]; v.Kind != "colony" || v.Countdown != 0 {
		t.Fatalf("view %+v", v)
	}
	cells := 0
	for _, c := range m.homeArea.cells {
		cells += int(c)
	}
	if cells == 0 {
		t.Fatal("the new colony has no Home area")
	}
	g.Step(5 * clock.TicksPerHour)
	if len(g.maps) != 2 || g.worldSt.atlas.Maps[0].PackBy != 0 {
		t.Fatal("a colony must not pack up or close")
	}
	cid = arriveAt(t, g, tiles[1], bo) // the third
	if why := SettleReason(g, uint64(cid)); why == "" {
		t.Fatal("the cap of two should refuse a third")
	}
	p := testPlanet()
	for i := 0; i < p.TileCount(); i++ {
		if ok, _ := p.ValidLanding(i); ok {
			continue
		}
		if p.Land(i) && p.Settlement[i] < 0 {
			SetMaxColonies(g, 5)
			if why := fleetOf(g).FoundReason(i); why == "" {
				t.Fatalf("tile %d next to a settlement should be refused", i)
			}
			break
		}
	}
}

// TestSettleConvertsTemporaryMap: a site map on the tile becomes the colony.
func TestSettleConvertsTemporaryMap(t *testing.T) {
	t.Parallel()
	g := worldGame(t)
	SetMaxColonies(g, 2)
	tile, ada := landable(t, g, 1)[0], g.pawnIDs[0]
	if _, ok := DevOpenMap(g, tile); !ok {
		t.Fatal("no site map")
	}
	site := g.maps[1]
	w, home := g.worldS(), g.colonyTile()
	cid := g.entities.Next()
	w.caravans = append(w.caravans, maps.Caravan{C: caravan.Caravan{ID: uint64(cid), Tile: tile, Dest: tile, State: caravan.Arrived, FoodDays: maps.FoodDays},
		Home: home, Out: testPlanet().Route(home, tile)})
	if !g.stashPawn(ada, cid) {
		t.Fatal("stash")
	}
	fleetOf(g).At(cid).C.Members = []uint64{uint64(ada)}
	if why := fleetOf(g).Found(uint64(cid)); why != "" {
		t.Fatal(why)
	}
	g.Step(1)
	if len(g.maps) != 2 || g.maps[1] != site || onMap(g, ada) != site || w.atlas.Maps[0].Kind != maps.Colony {
		t.Fatalf("not converted: %+v", w.atlas.Maps)
	}
}

// TestAbandonFirstColony: the next colony takes over as map 0, the view
// following; it saves and loads; the last colony's end leaves an empty map.
func TestAbandonFirstColony(t *testing.T) {
	t.Parallel()
	g := worldGame(t)
	tile, ada, bo := landable(t, g, 1)[0], g.pawnIDs[0], g.pawnIDs[1]
	m := founded(t, g, tile, ada)
	if g.ViewedMap() != 0 {
		t.Fatal("setup")
	}
	g.Step(60)
	g.Enqueue(command.Command{Kind: command.AbandonMap, Target: 0, Map: 0, MapSet: true})
	g.Step(1)
	if len(g.maps) != 1 || g.maps[0] != m || m.id != 0 || g.view != m || g.mapState != m {
		t.Fatalf("next colony not first: %d maps, id %d", len(g.maps), g.maps[0].id)
	}
	if g.colonyTile() != tile || g.ViewedMap() != 0 || !m.positions.Has(ada) || len(g.worldSt.atlas.Maps) != 0 {
		t.Fatalf("tile %d, want %d", g.colonyTile(), tile)
	}
	assertForgotten(t, g, []entity.EntityID{bo})
	if x, y := g.landingSpot(); !g.grid.InBounds(x, y) {
		t.Fatal("landing spot off the map")
	}
	g.Step(600)
	h, _ := saveLoadInfo(t, g)
	h.worldS().planet, h.worldS().planetTried = testPlanet(), true
	if g.StateHash() != h.StateHash() {
		t.Fatal("the round trip changed the state")
	}
	g.Step(60)
	h.Step(60)
	if g.StateHash() != h.StateHash() {
		t.Fatal("the loaded game diverged")
	}
	g.Enqueue(command.Command{Kind: command.AbandonMap, Target: 0, Map: 0, MapSet: true})
	g.Step(1)
	if len(g.maps) != 1 || len(g.pawnIDs) != 0 || len(g.itemIDs) != 0 || g.view != g.maps[0] {
		t.Fatalf("last colony: %d maps, %d pawns, %d items", len(g.maps), len(g.pawnIDs), len(g.itemIDs))
	}
	g.Step(600) // the empty map goes on
}

// TestAbandonViewedColony: the view leaves a colony map before it closes.
func TestAbandonViewedColony(t *testing.T) {
	t.Parallel()
	g := worldGame(t)
	m := founded(t, g, landable(t, g, 1)[0], g.pawnIDs[0])
	g.ViewMap(MapID(m.id))
	g.Enqueue(command.Command{Kind: command.AbandonMap, Target: uint64(m.id), Map: m.id, MapSet: true})
	g.Step(1)
	if len(g.maps) != 1 || g.view != g.maps[0] || g.ViewedMap() != 0 {
		t.Fatalf("%d maps, view %d", len(g.maps), g.ViewedMap())
	}
}

// TestThreatTargetsColonies: one colony draws nothing; two share threats by
// wealth; a site with people on it is never picked.
func TestThreatTargetsColonies(t *testing.T) {
	t.Parallel()
	g := worldGame(t)
	if threatTarget(g) != g.maps[0] || g.streams.m[streamStory] != nil {
		t.Fatal("one colony: no draw")
	}
	tiles := landable(t, g, 2)
	m := founded(t, g, tiles[0], g.pawnIDs[0])
	site := tiles[1]
	cid := arriveAt(t, g, site, g.pawnIDs[0+1])
	g.Enqueue(command.Command{Kind: command.EnterMap, Target: uint64(cid)})
	g.Step(1)
	if len(g.maps) != 3 || len(g.maps[2].pawnIDs) == 0 {
		t.Fatal("setup: no populated site")
	}
	g.mapState = m
	g.dropStack(item.Silver, 500, 3, 3)
	g.mapState = g.view
	seen := map[*mapState]int{}
	for range 400 {
		seen[threatTarget(g)]++
	}
	if seen[g.maps[0]] == 0 || seen[m] == 0 || seen[g.maps[2]] != 0 {
		t.Fatalf("first %d, founded %d, site %d", seen[g.maps[0]], seen[m], seen[g.maps[2]])
	}
	if g.mapState != g.view {
		t.Fatal("the current map moved")
	}
	struck := map[MapID]int{}
	before := len(g.messages)
	for range 60 { // the real path: the storyteller's clock fires on the first map's pass
		g.threatsOn, g.nextThreat = true, 0
		g.phaseThreats()
	}
	for _, msg := range g.messages[min(before, len(g.messages)):] {
		if msg.Danger {
			struck[msg.Map]++
		}
	}
	if g.mapState != g.view || struck[MapID(g.maps[2].id)] != 0 || struck[0] == 0 && struck[MapID(m.id)] == 0 {
		t.Fatalf("threats struck %+v", struck)
	}
}

// onRoad puts ids on a caravan under way to a tile two steps out with its
// food: the caravan.
func onRoad(t *testing.T, g *Game, ids ...entity.EntityID) *maps.Caravan {
	t.Helper()
	w, home := g.worldS(), g.colonyTile()
	dest := plainNear(t, g)
	route := testPlanet().Route(home, dest)
	cid := g.entities.Next()
	w.caravans = append(w.caravans, maps.Caravan{C: caravan.Caravan{ID: uint64(cid), Tile: route[1], Dest: dest, Path: slices.Clone(route[2:]), State: caravan.Travelling, FoodDays: maps.FoodDays, Slowest: 1000},
		Home: home, Out: route})
	for _, id := range ids {
		if !g.stashPawn(id, cid) {
			t.Fatalf("could not stash %d", id)
		}
		c := fleetOf(g).At(cid)
		c.C.Members = append(c.C.Members, uint64(id))
	}
	return fleetOf(g).At(cid)
}

// clearHostiles removes the raiders on m.
func clearHostiles(g *Game, m *mapState) {
	g.mapState = m
	for _, id := range slices.Clone(m.raiderIDs) {
		g.removeRaider(id)
	}
	g.mapState = g.view
}

// ambushRun: ambush, encounter map, win, pack up, resume: the final hash.
func ambushRun(t *testing.T) uint64 {
	g := worldGame(t)
	ada := g.pawnIDs[0]
	c := onRoad(t, g, ada)
	dest := c.C.Dest
	fleetOf(g).Waylay(c, maps.Ambush, uint64(g.clock.Now()))
	if c.C.State != caravan.Arrived || c.NextEnc == 0 {
		t.Fatalf("the caravan did not stop: %+v", c.C)
	}
	g.Step(1)
	if len(g.maps) != 2 {
		t.Fatalf("no encounter map: %d maps", len(g.maps))
	}
	m := g.maps[1]
	row := g.worldSt.atlas.Maps[0]
	if m.grid.Width() != maps.EncounterSide || row.Kind != maps.Encounter || row.ResumeDest != dest || onMap(g, ada) != m || !g.drafted(ada) || len(m.raidGroups) == 0 {
		t.Fatalf("encounter map %d, row %+v", m.grid.Width(), row)
	}
	if v := g.Maps()[1]; v.Kind != "encounter" {
		t.Fatalf("view %+v", v)
	}
	if !slices.ContainsFunc(g.letters.list, func(l savedLetter) bool { return l.Title == "Ambush!" && l.Map == MapID(m.id) }) {
		t.Fatal("no letter jumping to the encounter")
	}
	m.climate = biome.DefOf(biome.TemperateForest).Climate
	g.Step(clock.TicksPerHour)
	clearHostiles(g, m)
	g.Step(4 * clock.TicksPerHour)
	meta := &g.worldSt.atlas.Maps[0]
	if !meta.Hostile || meta.PackBy == 0 {
		t.Fatalf("the won fight did not start the pack-up: %+v", *meta)
	}
	meta.PackBy = uint64(g.clock.Now()) + 1
	g.Step(clock.TicksPerHour)
	r := fleetOf(g).Of(ada)
	if len(g.maps) != 1 || r == nil || r.C.State != caravan.Travelling || r.C.Dest != dest || r.NextEnc <= uint64(g.clock.Now()) {
		t.Fatalf("the caravan did not resume: %d maps, %+v", len(g.maps), r)
	}
	for i := 0; i < 400 && r != nil && r.C.State == caravan.Travelling; i++ {
		worldForward(g, clock.TicksPerHour)
		r = fleetOf(g).Of(ada)
	}
	if r == nil || r.C.Tile != dest {
		t.Fatalf("never reached its destination: %+v", r)
	}
	return g.StateHash()
}

// TestAmbushEncounterResumes: the whole loop, the same twice from one seed.
func TestAmbushEncounterResumes(t *testing.T) {
	t.Parallel()
	if a, b := ambushRun(t), ambushRun(t); a != b {
		t.Fatalf("state hash %#x vs %#x", a, b)
	}
}

// TestManhunterEncounter: enraged animals count as the fight on the map.
func TestManhunterEncounter(t *testing.T) {
	t.Parallel()
	g := worldGame(t)
	c := onRoad(t, g, g.pawnIDs[0])
	fleetOf(g).Waylay(c, maps.Manhunters, uint64(g.clock.Now()))
	g.Step(1)
	if len(g.maps) != 2 || g.worldSt.atlas.Maps[0].Kind != maps.Encounter {
		t.Fatal("no encounter map")
	}
	m := g.maps[1]
	g.mapState = m
	x, y, _ := g.combat.Tiles.WalkableNear(10, 10, 30)
	aid, err := g.SpawnAnimal("donkey", x, y)
	if err != nil {
		t.Fatal(err)
	}
	g.animals.Ref(aid).Manhunter = uint64(g.clock.Now()) + clock.TicksPerDay
	census := mapsPop{g}.Census(nil)
	g.mapState = g.view
	if census.Hostiles == 0 {
		t.Fatal("a manhunter does not count as a hostile")
	}
}

// demandGame is a caravan carrying 100 silver with a hostile faction about,
// asked for half of it.
func demandGame(t *testing.T) (*Game, *maps.Caravan, savedLetter) {
	t.Helper()
	g := worldGame(t)
	for i := range g.ng.factions {
		if f := &g.ng.factions[i]; f.Type.Human() {
			f.Relation, f.Defeated = faction.Hostile, false
		}
	}
	c := onRoad(t, g, g.pawnIDs[0])
	travelOf(g).Carry(c.C.ID, item.Stack{Kind: item.Silver, Qty: 100})
	fleetOf(g).Waylay(c, maps.Demand, uint64(g.clock.Now()))
	l := g.letters.list[len(g.letters.list)-1]
	if !l.Demand || c.Ask.Qty != 50 || c.Ask.Kind != item.Silver || c.C.State != caravan.Travelling {
		t.Fatalf("no demand: %+v, %+v", l, c.Ask)
	}
	if !slices.ContainsFunc(g.letterViews(), func(v LetterView) bool { return v.Demand && v.ID == l.ID }) {
		t.Fatal("the read model does not show the demand")
	}
	return g, c, l
}

// TestDemandGive: met, the goods go and the caravan moves on with no map.
func TestDemandGive(t *testing.T) {
	t.Parallel()
	g, c, l := demandGame(t)
	cid := c.C.ID
	g.Enqueue(command.Command{Kind: command.EncounterAnswer, Target: l.ID, Mode: 0})
	g.Step(1)
	c = fleetOf(g).At(entity.EntityID(cid))
	goods := travelOf(g).Goods(cid)
	if len(g.maps) != 1 || c.Ask.Letter != 0 || len(goods) != 1 || goods[0].Qty != 50 || c.C.State != caravan.Travelling {
		t.Fatalf("give: %d maps, goods %+v, %+v", len(g.maps), goods, c.Ask)
	}
	if slices.ContainsFunc(g.letters.list, func(s savedLetter) bool { return s.ID == l.ID }) {
		t.Fatal("the letter stayed")
	}
}

// TestDemandRefuse: refused (or left unanswered for a day), it is a fight.
func TestDemandRefuse(t *testing.T) {
	t.Parallel()
	g, _, l := demandGame(t)
	g.Enqueue(command.Command{Kind: command.EncounterAnswer, Target: l.ID, Mode: 1})
	g.Step(1)
	if len(g.maps) != 2 || g.worldSt.atlas.Maps[0].Kind != maps.Encounter {
		t.Fatalf("refused: %d maps", len(g.maps))
	}
	h, c, _ := demandGame(t)
	tile := c.C.Tile
	for i := 0; i < 26 && len(h.worldSt.atlas.Maps) == 0; i++ {
		worldForward(h, clock.TicksPerHour)
		h.Step(1)
	}
	if len(h.maps) != 2 || h.maps[1].tile != tile {
		t.Fatalf("an unanswered demand should turn to a fight: %d maps", len(h.maps))
	}
}

// TestDemandWithoutGoodsIsAmbush: nothing to demand, they attack.
func TestDemandWithoutGoodsIsAmbush(t *testing.T) {
	t.Parallel()
	g := worldGame(t)
	c := onRoad(t, g, g.pawnIDs[0])
	fleetOf(g).Waylay(c, maps.Demand, uint64(g.clock.Now()))
	g.Step(1)
	if len(g.maps) != 2 {
		t.Fatal("no fight")
	}
}

// TestEncounterCooldownNoFlap: the hourly roll draws nothing for two days
// after an encounter, and rolls again after; left walking, it hits in time.
func TestEncounterCooldownNoFlap(t *testing.T) {
	t.Parallel()
	g := worldGame(t)
	c := onRoad(t, g, g.pawnIDs[0])
	cid := entity.EntityID(c.C.ID)
	path := slices.Repeat(slices.Clone(c.C.Path), 40000)
	c.C.Path, c.C.Slowest = path, 1
	now := uint64(g.clock.Now())
	c.NextEnc = now + 2*clock.TicksPerDay
	fleet := fleetOf(g)
	step := func(hours int) {
		for range hours {
			g.clock.Advance(clock.TicksPerHour - int(uint64(g.clock.Now())%clock.TicksPerHour))
			fleet.Step(uint64(g.clock.Now()))
			if c := fleet.At(cid); c != nil {
				c.C.FoodDays = maps.FoodDays
			}
		}
	}
	step(47)
	if g.streams.m[streamRoad] != nil || len(g.worldSt.atlas.Queue) != 0 {
		t.Fatal("drew during the cooldown")
	}
	for range 60000 {
		step(1)
		if len(g.worldSt.atlas.Queue) > 0 {
			break
		}
		if m := g.letters.list; len(m) > 0 && m[len(m)-1].Demand {
			break
		}
	}
	if g.streams.m[streamRoad] == nil || len(g.worldSt.atlas.Queue) == 0 && !g.letters.list[len(g.letters.list)-1].Demand {
		t.Fatal("no encounter in 60 000 hours on the road")
	}
	if fleet.At(cid).NextEnc <= uint64(g.clock.Now()) {
		t.Fatal("no cooldown after the encounter")
	}
}

// TestEncounterSaveLoad: a save mid-fight and a save with a demand waiting
// both load to the same state and step on identically.
func TestEncounterSaveLoad(t *testing.T) {
	t.Parallel()
	g := worldGame(t)
	c := onRoad(t, g, g.pawnIDs[0])
	fleetOf(g).Waylay(c, maps.Ambush, uint64(g.clock.Now()))
	g.Step(61)
	round := func(g *Game) {
		t.Helper()
		h, _ := saveLoadInfo(t, g)
		h.worldS().planet, h.worldS().planetTried = testPlanet(), true
		if g.StateHash() != h.StateHash() || len(g.maps) != len(h.maps) {
			t.Fatal("the round trip changed the state")
		}
		g.Step(600)
		h.Step(600)
		if g.StateHash() != h.StateHash() {
			t.Fatal("the loaded game diverged")
		}
	}
	round(g)
	d, cv, _ := demandGame(t)
	round(d)
	if fleetOf(d).At(entity.EntityID(cv.C.ID)) == nil {
		t.Fatal("the caravan vanished")
	}
}

// TestCaravanTravelReroutes: a new destination from where it stands, and it
// can still get home the way it came.
func TestCaravanTravelReroutes(t *testing.T) {
	t.Parallel()
	g := worldGame(t)
	c := onRoad(t, g, g.pawnIDs[0])
	cid, to := c.C.ID, landable(t, g, 1)[0]
	if to == c.C.Tile {
		t.Skip("already there")
	}
	g.Enqueue(command.Command{Kind: command.CaravanTravel, Target: cid, Count: to})
	g.Step(1)
	c = fleetOf(g).At(entity.EntityID(cid))
	if c.C.Dest != to || c.C.State != caravan.Travelling || c.Back || c.C.Path[len(c.C.Path)-1] != to || c.Out[0] != g.colonyTile() {
		t.Fatalf("not rerouted: %+v", c)
	}
}
