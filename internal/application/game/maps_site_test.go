package game

import (
	"reflect"
	"slices"
	"testing"

	"rimworld-go/internal/application/game/maps"
	"rimworld-go/internal/application/game/raid"
	"rimworld-go/internal/application/game/travel"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/biome"
	"rimworld-go/internal/domain/world/caravan"
	"rimworld-go/internal/domain/world/terrain"
)

// Site maps (MAP-01..04): a caravan enters a tile, a map opens there,
// every map ticks, the job done starts the pack-up, the reform takes our
// people and goods home and the map closes leaving nothing behind.

// plainNear is a land tile two steps from home with no settlement.
func plainNear(t *testing.T, g *Game) int {
	t.Helper()
	p, home := testPlanet(), g.colonyTile()
	for _, nb := range p.Neighbours(home) {
		for _, nn := range p.Neighbours(int(nb)) {
			if int(nn) == home || slices.Contains(p.Neighbours(home), nn) {
				continue
			}
			if r := p.Route(home, int(nn)); len(r) == 3 && p.Settlement[nn] < 0 && g.settlementFaction(int(nn)) == 0 {
				return int(nn)
			}
		}
	}
	t.Fatal("no tile two steps from home")
	return -1
}

// arriveAt puts ids on a caravan that has just arrived on tile.
func arriveAt(t *testing.T, g *Game, tile int, ids ...entity.EntityID) entity.EntityID {
	t.Helper()
	w, home := g.worldS(), g.colonyTile()
	cid := g.entities.Next()
	w.caravans = append(w.caravans, maps.Caravan{C: caravan.Caravan{ID: uint64(cid), Tile: tile, Dest: tile, State: caravan.Travelling, FoodDays: maps.FoodDays},
		Home: home, Out: testPlanet().Route(home, tile)})
	for _, id := range ids {
		if !g.stashPawn(id, cid) {
			t.Fatalf("could not stash %d", id)
		}
		c := fleetOf(g).At(cid)
		c.C.Members = append(c.C.Members, uint64(id))
	}
	c := fleetOf(g).At(cid)
	c.C.State = caravan.Arrived
	fleetOf(g).Arrived(c, uint64(g.clock.Now()))
	return cid
}

// onMap is the open map holding creature id (nil: none).
func onMap(g *Game, id entity.EntityID) *mapState {
	for _, m := range g.maps {
		if m.positions.Has(id) {
			return m
		}
	}
	return nil
}

// TestSiteMapOpenStepCloseNoLeak: EnterMap opens a plain site map at a
// waiting caravan's tile; both maps tick; abandoning it closes it, and
// no colony-wide table keyed by entity keeps anything that lived there.
func TestSiteMapOpenStepCloseNoLeak(t *testing.T) {
	t.Parallel()
	g := worldGame(t)
	tile, ada := plainNear(t, g), g.pawnIDs[0]
	cid := arriveAt(t, g, tile, ada)
	if c := fleetOf(g).At(cid); c == nil || c.C.State != caravan.Arrived || len(g.maps) != 1 {
		t.Fatalf("the caravan should wait on a plain tile: %+v", c)
	}
	g.Enqueue(command.Command{Kind: command.EnterMap, Target: uint64(cid)})
	g.Step(1)
	if len(g.maps) != 2 || g.maps[1].tile != tile || g.maps[1].grid.Width() != maps.SiteSide {
		t.Fatalf("no %d-cell site map on tile %d: %d maps", maps.SiteSide, tile, len(g.maps))
	}
	m := g.maps[1]
	if onMap(g, ada) != m || fleetOf(g).At(cid) != nil || g.isStashed(ada) {
		t.Fatal("the caravan did not enter the site map")
	}
	if v := g.Maps(); len(v) != 2 || v[1].Kind != "site" || v[1].Colonists != 1 {
		t.Fatalf("map views %+v", v)
	}
	g.Step(2 * clock.TicksPerHour)
	if len(m.animalIDs) == 0 {
		t.Fatal("the site map has no wildlife")
	}
	gone := slices.Concat(m.pawnIDs, m.raiderIDs, m.animalIDs, m.itemIDs, m.bldgIDs, m.plantIDs, m.frameIDs)
	g.Enqueue(command.Command{Kind: command.AbandonMap, Target: uint64(m.id), Map: m.id, MapSet: true})
	g.Step(1)
	if len(g.maps) != 1 || g.view != g.maps[0] || !g.worldSt.atlas.Empty() && len(g.worldSt.atlas.Maps) != 0 {
		t.Fatalf("the site map did not close: %d maps", len(g.maps))
	}
	assertForgotten(t, g, gone)
	g.Step(60) // the colony goes on
}

// assertForgotten fails when any colony-wide table of g keyed by entity
// still holds one of ids.
func assertForgotten(t *testing.T, g *Game, ids []entity.EntityID) {
	t.Helper()
	keyT := reflect.TypeOf(entity.EntityID(0))
	v := reflect.ValueOf(g).Elem()
	for i := 0; i < v.NumField(); i++ {
		f := v.Field(i)
		if f.Kind() != reflect.Map || f.Type().Key() != keyT {
			continue
		}
		for _, id := range ids {
			if f.MapIndex(reflect.ValueOf(id)).IsValid() {
				t.Errorf("Game.%s still holds %d from the closed map", v.Type().Field(i).Name, id)
			}
		}
	}
	for _, id := range ids {
		if g.names.Has(id) || g.need.Has(id) || g.vital.Has(id) || g.labor.Has(id) {
			t.Errorf("a component store still holds %d from the closed map", id)
		}
	}
}

// siteRun sends ada to a steel site two steps out, mines the lump out,
// waits for the pack-up and brings her home: the state hash at the end.
func siteRun(t *testing.T) uint64 {
	g := worldGame(t)
	tile, ada := plainNear(t, g), g.pawnIDs[0]
	w := g.worldS()
	w.ax9 = travel.Import(&travel.Save{Sites: []travel.Site{{Tile: tile, Mineral: item.Steel, Qty: 200, Expires: 1 << 40}}})
	arriveAt(t, g, tile, ada)
	g.Step(1)
	if len(g.maps) != 2 || len(w.atlas.Maps) != 1 || len(g.World().AX.Sites) != 0 {
		t.Fatalf("no site map opened (%d maps)", len(g.maps))
	}
	m, meta := g.maps[1], &w.atlas.Maps[0]
	m.climate = biome.DefOf(biome.TemperateForest).Climate // the test planet is all sea ice: keep ada on her feet
	calmSite(g, m)                                         // whatever threat the site rolled (QST-047): this run is about the mining
	if meta.Lump != item.Steel || len(meta.Cells) != 10 {
		t.Fatalf("lump %v in %d cells, want steel in 10", meta.Lump, len(meta.Cells))
	}
	for _, c := range meta.Cells { // mined out, as a miner would leave it
		x, y := c%m.grid.Width(), c/m.grid.Width()
		if m.terr.At(x, y) != terrain.Rock {
			t.Fatalf("lump cell (%d, %d) is not rock", x, y)
		}
		g.mapState = m
		g.setTerrain(x, y, terrain.Stone)
		g.dropStack(item.Steel, 20, x, y)
		g.mapState = g.view
	}
	g.Step(4 * clock.TicksPerHour)
	if meta.PackBy == 0 {
		t.Fatal("the pack-up never started after the lump was mined out")
	}
	if v := g.Maps()[1]; v.Kind != "site" || v.Label != "steel site" || v.Countdown <= 0 || v.Countdown > clock.TicksPerDay {
		t.Fatalf("site view %+v", v)
	}
	packBy := meta.PackBy
	g.mapState = m
	g.Raid(2) // hostiles come and go: the countdown never restarts
	hour := uint64(g.clock.Now()) / clock.TicksPerHour
	for i := uint64(1); i <= 3; i++ { // three censuses with them there, none of them on the move
		fleetOf(g).Life().Flush((hour + i) * clock.TicksPerHour)
	}
	g.mapState = m
	for _, id := range slices.Clone(m.raiderIDs) {
		g.removeRaider(id)
	}
	g.mapState = g.view
	g.Step(3 * clock.TicksPerHour)
	if meta.PackBy != packBy || onMap(g, ada) != m || !g.standingPawn(ada) {
		t.Fatalf("pack-up moved from %d to %d", packBy, meta.PackBy)
	}
	meta.PackBy = uint64(g.clock.Now()) + 1 // skip the rest of the day
	g.Step(clock.TicksPerHour)
	if len(g.maps) != 1 {
		t.Fatal("the site map did not close after the pack-up")
	}
	c := fleetOf(g).Of(ada)
	if c == nil || c.C.State != caravan.Travelling || c.C.Dest != g.colonyTile() {
		t.Fatalf("no caravan home after the pack-up: %+v", c)
	}
	steel := 0
	for _, h := range w.ax9.Export().Cargo {
		for _, st := range h.Hold.Items {
			if st.Kind == item.Steel {
				steel += st.Qty
			}
		}
	}
	if steel < 200 {
		t.Fatalf("the caravan carries %d steel, want the mined 200", steel)
	}
	for i := 0; i < 400 && fleetOf(g).Of(ada) != nil; i++ {
		worldForward(g, clock.TicksPerHour)
	}
	if onMap(g, ada) != g.maps[0] {
		t.Fatalf("ada never came home: stashed %v caravan %+v", g.isStashed(ada), fleetOf(g).Of(ada))
	}
	g.Step(60)
	return g.StateHash()
}

// TestSiteMapMinePackUpHome: the whole site loop, twice from the same
// seed with the same hash.
func TestSiteMapMinePackUpHome(t *testing.T) {
	t.Parallel()
	if a, b := siteRun(t), siteRun(t); a != b {
		t.Fatalf("state hash %#x vs %#x", a, b)
	}
}

// calmSite takes site map m's threat away: its raiders leave the map and
// its manhunters calm down.
func calmSite(g *Game, m *mapState) {
	prev := g.mapState
	g.mapState = m
	for _, id := range slices.Clone(m.raiderIDs) {
		g.removeRaider(id)
	}
	for _, id := range m.animalIDs {
		if a := g.animals.Ref(id); a != nil {
			a.Manhunter = 0
		}
	}
	m.raidGroups = nil
	g.mapState = prev
}

// TestMineralSiteRaidDayTen: hostiles come to a mineral site 10 days
// after no hostile stands there (QST-048), once.
func TestMineralSiteRaidDayTen(t *testing.T) {
	t.Parallel()
	g := worldGame(t)
	tile := plainNear(t, g)
	w := g.worldS()
	w.ax9 = travel.Import(&travel.Save{Sites: []travel.Site{{Tile: tile, Mineral: item.Silver, Qty: 2400, Expires: 1 << 40}}})
	arriveAt(t, g, tile, g.pawnIDs[0], g.pawnIDs[1])
	g.Step(1)
	meta := &w.atlas.Maps[0]
	calmSite(g, g.maps[1])
	g.Step(clock.TicksPerHour)
	if meta.Raided || len(g.maps[1].raidGroups) != 0 || meta.Clear == 0 {
		t.Fatalf("a raid before day 10, or the site never counted as clear (clear %d)", meta.Clear)
	}
	meta.Opened -= 20 * clock.TicksPerDay // the day counts from the clear hour, not the open
	g.Step(clock.TicksPerHour)
	if meta.Raided {
		t.Fatal("the raid counted from the open, not from the site being clear")
	}
	meta.Clear -= 10 * clock.TicksPerDay
	g.Step(clock.TicksPerHour)
	if !meta.Raided || len(g.maps[1].raidGroups) == 0 || len(g.maps[0].raidGroups) != 0 {
		t.Fatalf("no raid on the site at day 10 (%d groups there)", len(g.maps[1].raidGroups))
	}
}

// TestCaravanEtaMatchesTravel: TravelEstimate is the time the caravan
// then takes, and never beats the wiki's 0.06 days per difficulty point
// at its speed, with no travel between 22:00 and 06:00 (MAP-09).
func TestCaravanEtaMatchesTravel(t *testing.T) {
	t.Parallel()
	g := worldGame(t)
	tile, ada, home := plainNear(t, g), g.pawnIDs[0], g.colonyTile()
	route := testPlanet().Route(home, tile)
	cid := g.entities.Next()
	w := g.worldS()
	w.caravans = append(w.caravans, maps.Caravan{C: caravan.Caravan{ID: uint64(cid), Tile: home, Dest: tile, Path: slices.Clone(route[1:]),
		State: caravan.Forming}, Home: home, Out: route})
	if !g.stashPawn(ada, cid) {
		t.Fatal("stash")
	}
	c := fleetOf(g).At(cid)
	c.C.Members = []uint64{uint64(ada)}
	est, why := g.TravelEstimate(uint64(cid), tile)
	if why != "" || est <= 0 {
		t.Fatalf("estimate %d %q", est, why)
	}
	for uint64(g.clock.Now())%clock.TicksPerHour != 0 {
		g.clock.Advance(1)
	}
	c.C.State, c.C.Slowest = caravan.Travelling, 1000 // empty, on foot: speed ×2 (wiki)
	start := uint64(g.clock.Now())
	if est, _ = g.TravelEstimate(uint64(cid), tile); est <= 0 {
		t.Fatal("no estimate")
	}
	day := int(start / clock.TicksPerDay)
	floor := 0
	for _, tt := range route[1:] {
		floor += int(testPlanet().MoveDifficulty(tt, day) * caravan.TicksPerDifficulty / 2)
	}
	if est < floor {
		t.Fatalf("estimate %d ticks beats the wiki floor %d", est, floor)
	}
	for i := 0; i < 400000; i += 10 {
		g.phaseWorld()
		if c := fleetOf(g).At(cid); c == nil || c.C.State == caravan.Arrived {
			break
		}
		g.clock.Advance(10)
	}
	got := int(uint64(g.clock.Now())-start) + clock.TicksPerHour
	if got != est {
		t.Fatalf("took %d ticks, estimated %d", got, est)
	}
	if h := int(uint64(g.clock.Now()) % clock.TicksPerDay / clock.TicksPerHour); !caravan.Moving(h) {
		t.Fatalf("arrived at %02d:00, outside travel hours", h)
	}
}

// TestDevOpenMap: the journeys' hook opens a blank map on a fixture with
// no planet and a site map on a world; an empty temporary map closes
// after three hours with nobody of ours on it (no flapping before).
func TestDevOpenMap(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 3, Width: 24, Height: 16})
	id, ok := DevOpenMap(g, 5)
	if !ok || id != 1 || len(g.maps) != 2 || g.maps[1].grid.Width() != 60 || g.maps[1].tile != -1 {
		t.Fatalf("fixture map %d %v (%d maps)", id, ok, len(g.maps))
	}
	if v := g.Maps(); v[1].Kind != "site" || v[1].ID != id {
		t.Fatalf("views %+v", v)
	}
	g.Step(2 * clock.TicksPerHour)
	if len(g.maps) != 2 {
		t.Fatal("closed before three empty hours")
	}
	g.Step(2 * clock.TicksPerHour)
	if len(g.maps) != 1 {
		t.Fatal("an empty temporary map stayed open")
	}
	w := worldGame(t)
	tile := plainNear(t, w)
	if id, ok := DevOpenMap(w, tile); !ok || len(w.maps) != 2 || w.maps[1].tile != tile || w.maps[1].grid.Width() != maps.SiteSide || id != 1 {
		t.Fatalf("world site map %d %v", id, ok)
	}
}

// TestSiteMapSaveLoad: a save taken while a caravan works a site map
// (the extra map, the atlas, the lump) loads to the same state, and both
// games step on identically through the pack-up.
func TestSiteMapSaveLoad(t *testing.T) {
	t.Parallel()
	g := worldGame(t)
	tile, ada := plainNear(t, g), g.pawnIDs[0]
	w := g.worldS()
	w.ax9 = travel.Import(&travel.Save{Sites: []travel.Site{{Tile: tile, Mineral: item.Steel, Qty: 200, Expires: 1 << 40}}})
	arriveAt(t, g, tile, ada)
	g.Step(clock.TicksPerHour + 1)
	if len(g.maps) != 2 {
		t.Fatal("no site map")
	}
	h, _ := saveLoadInfo(t, g)
	h.worldS().planet, h.worldS().planetTried = testPlanet().Fork(), true
	if len(h.maps) != 2 || len(h.worldSt.atlas.Maps) != 1 || len(h.worldSt.atlas.Maps[0].Cells) != 10 || onMap(h, ada) != h.maps[1] {
		t.Fatalf("the loaded game lost the site map: %d maps", len(h.maps))
	}
	if g.StateHash() != h.StateHash() {
		t.Fatal("state hash changed in the round trip")
	}
	w.atlas.Maps[0].PackBy = uint64(g.clock.Now()) + 2*clock.TicksPerHour
	h.worldSt.atlas.Maps[0].PackBy = w.atlas.Maps[0].PackBy
	for i := 0; i < 4; i++ {
		g.Step(clock.TicksPerHour)
		h.Step(clock.TicksPerHour)
		if g.StateHash() != h.StateHash() {
			t.Fatalf("hour %d: the loaded game diverged", i)
		}
	}
	if len(g.maps) != 1 || len(h.maps) != 1 {
		t.Fatal("the pack-up did not close the site map")
	}
}

// TestReformCarriesPrisonerAndAnimal: CaravanReform takes a prisoner (still
// one on arrival, with its raider record) and a tame animal off a site map;
// nobody is left behind and they come home with the colonist.
func TestReformCarriesPrisonerAndAnimal(t *testing.T) {
	t.Parallel()
	g := worldGame(t)
	tile, ada := plainNear(t, g), g.pawnIDs[0]
	arriveAt(t, g, tile, ada)
	g.Enqueue(command.Command{Kind: command.EnterMap, Target: g.worldS().caravans[0].C.ID})
	g.Step(1)
	m := g.maps[1]
	m.climate = biome.DefOf(biome.TemperateForest).Climate // the test planet is all sea ice
	g.mapState = m
	pos, _ := g.positions.Get(ada)
	x, y, ok := g.combat.Tiles.WalkableNear(pos.X, pos.Y, 8)
	if !ok {
		t.Fatal("no ground")
	}
	g.raidGroups = append(g.raidGroups, raid.Group{Size: 1})
	rid := spawnRaider(g, 0, 0, item.Revolver, x, y)
	g.makePrisoner(rid, ada)
	aid, err := g.SpawnAnimal("donkey", x, y)
	if err != nil {
		t.Fatal(err)
	}
	g.animals.Ref(aid).Tame = true
	g.mapState = g.view
	g.Enqueue(command.Command{Kind: command.CaravanReform, Target: uint64(m.id), Map: m.id, MapSet: true})
	g.Step(1)
	if len(g.maps) != 1 {
		t.Fatal("the map did not close")
	}
	c := fleetOf(g).Of(ada)
	if c == nil || !slices.Contains(c.C.Members, uint64(rid)) || !slices.Contains(c.C.Animals, uint64(aid)) {
		t.Fatalf("caravan %+v", c)
	}
	if p := g.personOf(ada); p == nil || p.Where != whereCaravan {
		t.Fatal("the colonist is not on the caravan")
	}
	for i := 0; i < 400 && fleetOf(g).Of(ada) != nil; i++ {
		worldForward(g, clock.TicksPerHour)
	}
	h := g.maps[0]
	if !h.positions.Has(ada) || !h.positions.Has(rid) || !h.animals.Has(aid) {
		t.Fatalf("not all home: stashed %v %v %v", g.isStashed(ada), g.isStashed(rid), g.isStashed(aid))
	}
	if !g.prison.Is(rid) || !g.isRaider(rid) || !slices.Contains(h.raiderIDs, rid) || !h.animals.Ref(aid).Tame {
		t.Fatal("the prisoner or the animal changed on the way")
	}
	g.Step(60)
}
