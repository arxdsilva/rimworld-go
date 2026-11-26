// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"cmp"
	"fmt"
	"reflect"
	"rimworld-go/internal/application/game/animalstate"
	"slices"

	"rimworld-go/internal/application/game/letters"
	"rimworld-go/internal/application/game/maps"
	"rimworld-go/internal/application/game/outfit"
	"rimworld-go/internal/application/game/quest"
	"rimworld-go/internal/application/game/raid"
	"rimworld-go/internal/application/game/sense"
	"rimworld-go/internal/application/game/ship"
	"rimworld-go/internal/application/game/tabview"
	"rimworld-go/internal/application/game/tradeview"
	"rimworld-go/internal/application/game/travel"
	"rimworld-go/internal/application/game/visit"
	"rimworld-go/internal/application/game/worldlink"
	"rimworld-go/internal/application/game/worldpawn"
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/animal/training"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/combat/explosion"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/pawn/body"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/pawn/social"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/biome"
	"rimworld-go/internal/domain/world/geology"
	"rimworld-go/internal/domain/world/mapgen"
	"rimworld-go/internal/domain/world/mapgen/pipeline"
	"rimworld-go/internal/domain/world/planet"
	"rimworld-go/internal/domain/world/temperature"
	"rimworld-go/internal/domain/world/terrain"
	"rimworld-go/pkg/msgfmt"
	"rimworld-go/pkg/randx"
)

// Slow caravan and allowed-area upkeep cadence.
const worldStep = 250

type worldState struct {
	// Maintenance follows animals across map boundaries and off-map stashes.
	trainingMaintenance map[entity.EntityID]training.Maintenance
	// Planet is lazily rebuilt from setup, never serialized.
	planet              *planet.Planet
	planetTried         bool
	link                *worldlink.Link      // historical standalone world's saved location
	caravans            []maps.Caravan       // ascending ID (game/maps)
	marketReply         tabview.MarketReply  // transient UI receipt
	markets             maps.Markets         // authoritative settlement inventory and old-save recovery
	stashed             []stashRec           // ascending ID (world_stash.go)
	worldPawnMothballed worldpawn.Mothballed // authoritative off-map cadence
	visitors            []visitorRec         // ascending ID (visitors.go)
	visitGroups         []visit.Group
	nextVisitGroup      uint64
	// areaOf maps restricted residents/animals to saved area refs.
	areaOf map[entity.EntityID]int
	// pathFree is the transient area-path exception for the active pawn.
	pathFree entity.EntityID
	// huntOut saves hunter/prey area-crossing observations.
	huntOut map[entity.EntityID]huntOutRec
	// areaVer/areaGen track view-only area updates.
	areaVer     map[uint16]uint64
	areaGen     uint64
	ship        *ship.State   // story.go: game/ship (AX-8), built on first use
	ax9         *travel.State // game/travel (AX-9): caskets, pods, sites, hitching
	ax9h        travel.Host   // its host (travelOf), built once
	mapsH       maps.Host     // game/maps' host (fleetOf), built once
	maxColonies int           // Options' cap, sent at start and load (never saved; 0 means 1)
	visitH      visit.Host    // game/visit's host (visitsOf), built once
	atlas       maps.Atlas    // game/maps: the open maps beyond the first (MAP-01)
	sense       *sense.Sense  // senseOf (sense_host.go): the habit rules (CSE)
	outfits     outfit.Book   // outfitBook (apparel.go): apparel policies (GEAR-3), saved as Outfits
	quests      *quest.Quests // questOf (quest_host.go): the quests root (QST-1), saved as Quests
	questMemo   questMemo     // questViews: the read model's quest list
}

// worldSave is worldState as saved.
type worldSave struct {
	Version          uint8 // format of this block; 0 = empty
	Caravans         []maps.Caravan
	Stashed          []stashRec
	Visitors         []visitorRec
	VisitGroups      []visit.Group
	NextVisitGroup   uint64
	Mothballed       []entity.EntityID // active lightweight world-pawn cadence
	SettlementStocks []maps.SettlementStock
	// AreaOf stores versioned allowed-area references.
	AreaOf  []areaAssign
	HuntOut []huntOutRec    // area_dest.go
	AX9     *travel.Save    // game/travel (AX-9): absent in older saves
	Atlas   *maps.Atlas     // game/maps (MAP-01): absent while one map was ever open
	Quests  *quest.Save     // game/quest (QST-1): absent before the first quest or in a legacy colony
	Link    *worldlink.Link // added historic colony world binding, absent in older saves
}

// worldSaveVersion is the format exportWorld writes (3: visit groups).
const worldSaveVersion = 3

// areaAssign is one pawn's allowed area in a save.
type areaAssign struct {
	Pawn entity.EntityID
	Area int
}

// worldS is the world state, created on first use.
func (g *Game) worldS() *worldState {
	if g.worldSt == nil {
		g.worldSt = &worldState{}
	}
	return g.worldSt
}

// worldPlanet caches the rebuilt world without serializing its geometry.
func (g *Game) worldPlanet() *planet.Planet {
	w := g.worldS()
	if !w.planetTried {
		w.planetTried = true
		if g.hasSetup() {
			w.planet = maps.BuildPlanet(g.ng.setup.World.WithCalendar(g.clock.Calendar()), g.ng.setup.Tile, g.ng.factions)
		} else {
			w.planet = worldlink.Planet(w.link, g.ng.factions)
		}
	}
	return w.planet
}

// colonyTile is the colony's world tile (-1: no world map).
func (g *Game) colonyTile() int {
	if g.hasSetup() && g.ng.setup.World.Coverage > 0 {
		return g.ng.setup.Tile
	}
	if g.worldSt != nil && g.worldSt.link != nil {
		return g.worldSt.link.Tile
	}
	return -1
}

// AttachPlanet connects a standalone historical colony to a generated world.
// Existing map geometry, residents, simulation streams and history are retained.
func AttachPlanet(g *Game, params planet.Params) (bool, error) {
	if g == nil {
		return false, fmt.Errorf("missing colony")
	}
	w := g.worldSt
	busy := w != nil && (len(w.caravans) != 0 || w.markets.Len() != 0 || !w.atlas.Empty() || !w.ax9.Empty())
	ctx := worldlink.Context{HasWorld: g.colonyTile() >= 0 || w != nil && w.link != nil, Standalone: !g.hasSetup() && len(g.maps) == 1, Active: busy}
	link, factions, changed, err := ctx.Attach(params.WithCalendar(g.clock.Calendar()), g.ng.factions)
	if !changed || err != nil {
		return changed, err
	}
	w = g.worldS()
	w.link, g.ng.factions = link, factions
	w.planet, w.planetTried = nil, false
	g.viewGen++
	return true, nil
}

// phaseWorld runs with a world after phaseIncidents and the colony's
// visitors (phaseSystems): each map's forming caravans and travel.
func (g *Game) phaseWorld() {
	w, now, first := g.worldSt, uint64(g.clock.Now()), primary(g) // MAP-01: the colony's flights and sites on the first map's pass
	if len(w.caravans) > 0 && now%maps.FormCheck == 0 {
		fleetOf(g).CheckForming(now) // each map's own forming caravans
	}
	travelOf(g).TickPass(now, first) // game/travel (AX-9): each map's caskets and boarders, the colony's flights and sites
	if first {
		worldpawn.Step(offMapPawns{g}, &w.worldPawnMothballed, now)
	}
}

// worldUpkeep is the colony's allowed areas, caravans and ship every
// worldStep ticks, after the first map's world pass (phaseSystems).
func worldUpkeep(g *Game, now uint64) {
	w := g.worldSt
	if len(w.areaOf) > 0 {
		g.areasTick(now) // areas.go
	}
	if len(w.caravans) > 0 {
		fleetOf(g).Step(now)
	}
	if w.ship.Busy() {
		w.ship.Tick(shipHost{g}) // story.go: game/ship (AX-8): countdowns, raids, fallen caskets
	}
}

// letter posts a world letter titled title (shown once, jump to the home).
func (g *Game) letter(kind, title, text string, focus entity.EntityID) {
	letterf(g, kind, title, focus, msgfmt.Plain(text))
}

// letterf is letter with its text as a format and args, the catalog's key
// (OPT-001). It returns the text.
func letterf(g *Game, kind, title string, focus entity.EntityID, format string, args ...any) string {
	x, y := g.home()
	return g.post(MessageView{Tick: uint64(g.clock.Now()), Msg: msgfmt.Of(format, args...), X: x, Y: y, Focus: focus, Letter: true, Title: title, Kind: kind})
}

// worldNote posts a plain message at the colony (a refused order).
func (g *Game) worldNote(format string, args ...any) {
	x, y := g.home()
	g.message(x, y, format, args...)
}

// doKidnap: kidnapping is a raider's job, run from the raid AI
// (raid/kidnap.go Step); a pawn on pawnIDs never kidnaps, so one found
// with the job just ends it.
func doKidnap(g *Game, id entity.EntityID, j job.Record) {
	g.endStubJob(id, j)
}

// doExitMap: a pawn at the map edge on job.ExitMap leaves the map with
// its forming caravan (game/maps); anyone else just ends the job.
func doExitMap(g *Game, id entity.EntityID, j job.Record) {
	if !fleetOf(g).ExitMap(id) {
		g.endStubJob(id, j)
	}
}

// LaunchReason is why pod's group would not fly to world tile dest in
// pod arrival mode (command.PodArrive*) from the viewed map ("" it would),
// with the distance in tiles (MAP-10; the world view shows it).
func (g *Game) LaunchReason(pod entity.EntityID, dest int, mode uint8) (string, int) {
	return travelOf(g).LaunchReason(pod, dest, mode)
}

// settlementFaction is the 1-based faction index occupying world tile t.
func (g *Game) settlementFaction(t int) int { return travel.SettlementFaction(g.ng.factions, t) }

// tileName resolves the generated settlement or the fallback faction/tile name.
func (g *Game) tileName(t int) string {
	return travel.TileName(g.worldPlanet(), g.ng.factions, t)
}

// dieAway kills a caravan member, leaving no corpse on the map.
func dieAway(g *Game, id entity.EntityID, cause health.Cause) {
	f := fleetOf(g)
	cid := maps.MemberCaravanID(f, id)
	delete(g.equip, id)
	g.setWhere(id, whereColony) // the colony's death (its history line), not a stranger's
	g.dieBy(id, cause, 0)
	dropStash(g, id)
	maps.RemoveDeadMember(f, cid, id)
	letterf(g, letters.Caution, "Death on the road", 0, "%s died on the caravan's journey.", g.personName(id))
}

// worldEdge picks a reachable edge without consuming g.rng.
func (g *Game) worldEdge() (int, int, bool) {
	hx, hy := g.home()
	if x, y, ok := g.combat.Tiles.WalkableNear(hx, hy, homeReach(g)); ok {
		hx, hy = x, y
	}
	bm := breachMap{g: g, breaks: g.buildingCells(construction.Wall, construction.Door, construction.Autodoor, construction.OrnateDoor, construction.AnimalFlap)}
	return travel.EdgeTile(g.grid.Width(), g.grid.Height(), g.stream(streamWorld).IntN, g.dryWalkable, func(x, y int) bool {
		_, ok := g.searcher().Find(bm, x, y, hx, hy)
		return ok
	})
}

// exportWorld is the World save field (nil: nothing yet).
func (g *Game) exportWorld() *worldSave {
	w := g.worldSt
	var qs *quest.Save
	if w != nil && w.quests != nil {
		qs = w.quests.Export() // nil while it holds nothing
	}
	if w == nil || w.link == nil && len(w.caravans) == 0 && w.markets.Len() == 0 && len(w.stashed) == 0 && len(w.visitors) == 0 && len(w.visitGroups) == 0 && w.nextVisitGroup == 0 && len(w.worldPawnMothballed) == 0 && len(w.areaOf) == 0 && len(w.huntOut) == 0 && w.ax9.Empty() && w.atlas.Empty() && qs == nil {
		return nil
	}
	s := &worldSave{Version: worldSaveVersion, Stashed: slices.Clone(w.stashed), Visitors: slices.Clone(w.visitors), NextVisitGroup: w.nextVisitGroup, Quests: qs, Link: w.link.Snapshot(g.ng.factions)}
	for i := range s.Stashed {
		if r := s.Stashed[i].Raider; r != nil {
			copy := *r // arrival mutates the original raider's group
			s.Stashed[i].Raider = &copy
		}
	}
	s.Mothballed = w.worldPawnMothballed.Export()
	s.VisitGroups = visit.CloneGroups(w.visitGroups)
	s.Caravans = maps.CloneCaravans(w.caravans)
	s.SettlementStocks = w.markets.Snapshot()
	for id, a := range w.areaOf {
		s.AreaOf = append(s.AreaOf, areaAssign{Pawn: id, Area: a})
	}
	slices.SortFunc(s.AreaOf, func(a, b areaAssign) int { return cmp.Compare(a.Pawn, b.Pawn) })
	s.HuntOut, s.AX9 = exportHuntOut(w.huntOut), w.ax9.Export() // area_dest.go, game/travel
	if !w.atlas.Empty() {
		s.Atlas = w.atlas.Clone()
	}
	return s
}

// worldAnimalRefs creates private copies of stashed beasts before grouped
// encoding zeroes the legacy world markers. Live stashes remain unchanged.
func worldAnimalRefs(w *worldSave) []animalstate.OffMapRef {
	var refs []animalstate.OffMapRef
	for i := range w.Stashed {
		r := &w.Stashed[i]
		if r.Beast != nil {
			state := *r.Beast
			r.Beast = &state
			refs = append(refs, animalstate.OffMapRef{ID: r.ID, State: r.Beast})
		}
	}
	return refs
}

// importWorld restores the World save (nil: fresh).
func (g *Game) importWorld(s *worldSave) {
	g.worldSt = nil
	if s == nil {
		return
	}
	w := g.worldS()
	w.link, g.ng.factions = worldlink.Restore(s.Link, g.hasSetup(), g.ng.factions)
	w.caravans = maps.CloneCaravans(s.Caravans)
	w.markets.Restore(s.SettlementStocks, len(s.Caravans) > 0)
	w.stashed = slices.Clone(s.Stashed)
	for i := range w.stashed {
		if beast := w.stashed[i].Beast; beast != nil {
			body.MigrateReplacements(beast.Body)
		}
	}
	w.visitors = slices.Clone(s.Visitors)
	w.nextVisitGroup = s.NextVisitGroup
	w.worldPawnMothballed = worldpawn.ImportMothballed(s.Mothballed)
	w.visitGroups = visit.CloneGroups(s.VisitGroups)
	for _, gr := range w.visitGroups {
		if gr.ID > w.nextVisitGroup {
			w.nextVisitGroup = gr.ID
		}
	}
	for _, a := range s.AreaOf {
		if w.areaOf == nil {
			w.areaOf = map[entity.EntityID]int{}
		}
		w.areaOf[a.Pawn] = importAreaRef(s.Version, a.Area) // areas.go
	}
	w.huntOut, w.ax9 = importHuntOut(s.HuntOut), travel.Import(s.AX9) // area_dest.go, game/travel
	if s.Atlas != nil {
		w.atlas = *s.Atlas.Clone()
	}
	if s.Quests != nil && g.hasSetup() {
		questOf(g).Import(s.Quests)
	}
}

// worldMapView builds the World tab from copies of the caravan state.
func (g *Game) worldMapView() WorldMapView {
	if g.worldSt == nil {
		return WorldMapView{Tile: g.colonyTile()}
	}
	v := tabview.BuildWorldMap(g.colonyTile(), uint64(g.clock.Now()), g.worldSt.caravans,
		g.caravanMemberFacts,
		func(id uint64) []item.Stack {
			if g.worldSt.ax9 == nil {
				return nil
			}
			return travelOf(g).Goods(id)
		},
		func(c *maps.Caravan) (tabview.MarketView, bool) {
			return tradeview.Visiting(uint64(g.clock.Now()), c, marketWorldSource{g})
		}, func(c *maps.Caravan, goods []item.Stack) travel.SplitView { return fleetOf(g).SplitView(c, goods) })
	v.MarketReply = g.worldSt.marketReply
	return v
}

// travelOf is game/travel over the game (AX-9): stasis caskets, transport
// pods, the mineral scanner and hitching spots, its state in worldSt.ax9.
func travelOf(g *Game) travel.Ops {
	w := g.worldS()
	if w.ax9h == nil {
		w.ax9h = newTravelHost(g) // built once: views and ticks allocate nothing
	}
	return travel.Of(&w.ax9).Ops(w.ax9h)
}

// travelHost is game/travel's roles (travel/deps.go) over adapters.
type travelHost struct {
	g *Game
	industryHost
	herdHost
	careRoster
	careHealth
	combatSupply
	raidFight
}

func newTravelHost(g *Game) travelHost {
	return travelHost{g, industryHost{g}, herdHost{g}, careRoster{g}, careHealth{g}, combatSupply{g}, raidFight{g}}
}

func (h travelHost) Pos(id entity.EntityID) (int, int)   { return h.industryHost.Pos(id) }
func (h travelHost) Items() item.Book                    { return h.g.sk.content.Items }
func (h travelHost) Work(id entity.EntityID, m bool) int { return h.industryHost.Work(id, m) }
func (h travelHost) Name(id entity.EntityID) string      { return h.g.name(id) }
func (h travelHost) Fuel(id entity.EntityID) int         { return h.g.fuel[id] / chemfuelTicks }
func (h travelHost) SetFuel(id entity.EntityID, n int)   { h.g.setFuel(id, n*chemfuelTicks) }
func (h travelHost) Remove(id entity.EntityID)           { h.g.removeBuildingAs(id, false) }
func (h travelHost) Downed(id entity.EntityID) bool { // AX-9: a downed animal too
	v, ok := h.g.vital.Get(id)
	return v.Status == health.Downed || !ok && h.g.animals.Ref(id) != nil && h.g.animals.Ref(id).Downed
}
func (h travelHost) Stash(id, holder entity.EntityID) bool     { return h.g.stashPawn(id, holder) }
func (h travelHost) Unstash(id entity.EntityID, x, y int) bool { return h.g.unstashPawn(id, x, y) }
func (h travelHost) Pick(c, p entity.EntityID)                 { h.g.care.Carry.Pick(c, p) }
func (h travelHost) PutDown(c entity.EntityID)                 { h.g.care.DropPatient(c) }
func (h travelHost) Move(id entity.EntityID, x, y int)         { h.g.cmdMove(id, x, y) }
func (h travelHost) DropStack(st item.Stack, x, y int)         { h.g.dropStackOf(st, x, y) }
func (h travelHost) MealRule() item.MealRule                   { return item.RuleOf(senseOf(h.g).Rules) }
func (h travelHost) RemoveAnimal(id entity.EntityID)           { h.g.detachAnimal(id) }
func (h travelHost) Planet() *planet.Planet                    { return h.g.worldPlanet() }

// ColonyTile is the viewed-from map's own world tile: pods launch from it
// and measure their range from it (MAP-10).
func (h travelHost) ColonyTile() int {
	if h.g.id != 0 {
		return h.g.tile
	}
	return h.g.colonyTile()
}
func (h travelHost) Home() (int, int)       { return h.g.home() }
func (h travelHost) Caravan(id uint64) bool { return fleetOf(h.g).At(entity.EntityID(id)) != nil }
func (h travelHost) Faction(t int) (*faction.Faction, string) {
	return h.g.factionOf(h.g.settlementFaction(t)), h.g.tileName(t)
}
func (h travelHost) PutAnimal(id entity.EntityID, a animal.State, x, y int) {
	putAnimal(h.g, id, a, x, y)
}
func (h travelHost) Sicken(id entity.EntityID)         { cryptoSicken(h.g, id) }
func (h travelHost) Explode(x, y int, d explosion.Def) { h.g.combat.Blasts.Explode(0, 0, x, y, d) }

// Assault: pod passengers on caravan cid attack the settlement under them.
func (h travelHost) Assault(cid uint64, centre bool) { fleetOf(h.g).Life().Assault(cid, centre) }

// MapOpen, LandOn: pods onto an open map (MAP-10, game/maps pod.go).
func (h travelHost) MapOpen(t int) bool { return fleetOf(h.g).Life().MapOpen(t) }
func (h travelHost) LandOn(members, animals []uint64, t int, centre, spot bool, x, y int) uint64 {
	return fleetOf(h.g).LandOn(members, animals, t, centre, spot, x, y)
}

// FoundReason, Found: pods founding a colony (MAP-06).
func (h travelHost) FoundReason(t int) string { return fleetOf(h.g).Life().FoundReason(t) }
func (h travelHost) Found(cid uint64) string  { return fleetOf(h.g).Life().Found(cid) }

// LandedCargo refreshes pace after the physical pod hold is committed.
func (h travelHost) LandedCargo(cid uint64) {
	maps.RefreshPace(fleetOf(h.g), entity.EntityID(cid))
}

// Land: pod passengers become a caravan arrived on t (home by land).
func (h travelHost) Land(members, animals []uint64, t int) uint64 {
	return fleetOf(h.g).Land(members, animals, t)
}

// fleetOf is game/maps' caravans over the game, its state in
// worldSt.caravans.
func fleetOf(g *Game) maps.Fleet {
	w := g.worldS()
	if w.mapsH == nil {
		w.mapsH = mapsHost{mapsCrew{g}, mapsWalk{g}, mapsGround{g}, mapsGlobe{g}, mapsRoad{g}, mapsMarket{g}, mapsOpen{g}, mapsPop{g}, mapsSettle{g}, mapsFound{g}, mapsWaylay{g}, mapsQuests{g}} // built once
	}
	return maps.Of(&w.caravans, &w.atlas, w.mapsH).WithMarkets(&w.markets)
}

type mapsHost struct {
	mapsCrew
	mapsWalk
	mapsGround
	mapsGlobe
	mapsRoad
	mapsMarket
	mapsOpen
	mapsPop
	mapsSettle
	mapsFound
	mapsWaylay
	mapsQuests // quest_host.go
}

type mapsCrew struct{ g *Game }

func (h mapsCrew) Candidate(id entity.EntityID) bool {
	return slices.Contains(h.g.pawnIDs, id) && h.g.standingPawn(id) && !h.g.breaking(id)
}
func (h mapsCrew) Standing(id entity.EntityID) bool { return h.g.standingPawn(id) }
func (h mapsCrew) Breaking(id entity.EntityID) bool { return h.g.breaking(id) }
func (h mapsCrew) Alive(id entity.EntityID) bool    { return h.g.alive(id) }
func (h mapsCrew) Capacity(id entity.EntityID, c body.Capacity) int32 {
	return h.g.capacity(id, c)
}
func (h mapsCrew) Name(id entity.EntityID) string       { return h.g.personName(id) }
func (h mapsCrew) Drafted(id entity.EntityID) bool      { return h.g.drafted(id) }
func (h mapsCrew) SetDraft(id entity.EntityID, on bool) { h.g.setDraft(id, on) }

type mapsWalk struct{ g *Game }

func (h mapsWalk) Job(id entity.EntityID) (job.Record, bool) { return h.g.jobs.Get(id) }
func (h mapsWalk) Stash(id, holder entity.EntityID) bool     { return h.g.stashPawn(id, holder) }
func (h mapsWalk) Unstash(id entity.EntityID, x, y int) bool { return h.g.unstashPawn(id, x, y) }
func (h mapsWalk) IsStashed(id entity.EntityID) bool         { return h.g.isStashed(id) }
func (h mapsWalk) Move(id entity.EntityID, x, y int) { // a colonist: prisoners and animals stay where they are set
	if slices.Contains(h.g.pawnIDs, id) {
		h.g.cmdMove(id, x, y)
	}
}
func (h mapsWalk) StartExit(id entity.EntityID) bool {
	pos, ok := h.g.positions.Get(id)
	if !ok {
		return false
	}
	ex, ey := nearestEdge(pos, h.g.grid.Width(), h.g.grid.Height())
	return maps.TryExit(mapsGround(h), id, ex, ey, h.g.pathDeferred, h.g.startJob)
}
func (h mapsWalk) Rehold(id, holder entity.EntityID) {
	w := h.g.worldS()
	if i, ok := slices.BinarySearchFunc(w.stashed, id, stashCmp); ok {
		w.stashed[i].Holder = holder
		h.g.setWhere(id, whereCaravan)
	}
}

type mapsGround struct{ g *Game }

func (h mapsGround) Edge() (int, int, bool) { return h.g.worldEdge() }
func (h mapsGround) Home() (int, int) {
	x, y := h.g.home()
	if h.g.id == 0 {
		return x, y
	}
	if wx, wy, ok := h.g.combat.Tiles.WalkableNear(x, y, homeReach(h.g)); ok {
		return wx, wy // MAP-01: a temporary map's centre may be rock
	}
	return x, y
}

// homeReach is how far from home a walk-in looks for open ground: a
// temporary map's centre may lie in rock (MAP-01).
func homeReach(g *Game) int {
	if g.id != 0 {
		return 40
	}
	return 6
}
func (h mapsGround) FreeLand(x, y, r int, used [][2]int) (int, int, bool) {
	return h.g.freeLandNear(x, y, r, used)
}
func (h mapsGround) WalkableNear(x, y, r int) (int, int, bool) {
	return h.g.combat.Tiles.WalkableNear(x, y, r)
}

// Landing: the nearest cell to (x, y) a pod can come down on (x < 0: the
// map's centre): dry, walkable and not under overhead mountain (MAP-10).
func (h mapsGround) Landing(x, y int) (int, int, bool) {
	g := h.g
	if x < 0 {
		x, y = g.grid.Width()/2, g.grid.Height()/2
	}
	for r := 0; r <= 14; r++ {
		for dy := -r; dy <= r; dy++ {
			for dx := -r; dx <= r; dx++ {
				if cx, cy := x+dx, y+dy; max(dx, -dx, dy, -dy) == r && g.grid.InBounds(cx, cy) && g.dryWalkable(cx, cy) && !g.roofAt(cx, cy).Thick() {
					return cx, cy, true
				}
			}
		}
	}
	return 0, 0, false
}
func (h mapsGround) HomeSpot(i int) (int, int, bool) {
	hx, hy := h.g.home()
	o := ringOffsets[i%len(ringOffsets)]
	return h.g.combat.Tiles.WalkableNear(hx+o[0]/2, hy+o[1]/2, 3)
}
func (h mapsGround) Letter(caution bool, title string, focus entity.EntityID, format string, args ...any) {
	kind := letters.Info
	if caution {
		kind = letters.Caution
	}
	letterf(h.g, kind, title, focus, format, args...)
}
func (h mapsGround) Note(format string, args ...any) { h.g.worldNote(format, args...) }

type mapsGlobe struct{ g *Game }

func (h mapsGlobe) Planet() *planet.Planet { return h.g.worldPlanet() }
func (h mapsGlobe) ColonyTile() int        { return h.g.colonyTile() }
func (h mapsGlobe) TileName(t int) string  { return h.g.tileName(t) }
func (h mapsGlobe) Faction(t int) *faction.Faction {
	return h.g.factionOf(h.g.settlementFaction(t))
}
func (h mapsGlobe) NewID() uint64       { return uint64(h.g.entities.Next()) }
func (h mapsGlobe) Now() uint64         { return uint64(h.g.clock.Now()) }
func (h mapsGlobe) Cargo() maps.Freight { return travelOf(h.g) }

type mapsRoad struct{ g *Game }

func (h mapsRoad) Starve(id entity.EntityID) { // FOOD-1: at 0 % malnutrition sets in
	n, _ := h.g.need.Get(id)
	n.Hunger = max(n.Hunger-needs.HungerPerHour, 0)
	h.g.need.Set(id, n.Settle())
}
func (h mapsRoad) Talk(a, b entity.EntityID, k social.Interaction, joy int, now uint64) {
	h.g.socState()
	h.g.ensurePerson(a)
	h.g.ensurePerson(b)
	h.g.applyInteraction(a, b, k, now) // social.go
	for _, id := range [2]entity.EntityID{a, b} {
		addRecreation(h.g, id, joy)
	}
}
func (h mapsRoad) Roll(n int) int { return h.g.stream(streamWorld).IntN(n) }
func (h mapsRoad) Species(id entity.EntityID) (animal.Species, bool) {
	w := h.g.worldS()
	if i, ok := slices.BinarySearchFunc(w.stashed, id, stashCmp); ok && w.stashed[i].Beast != nil {
		return w.stashed[i].Beast.Species, true
	}
	return 0, false
}

// TravelEstimate is how long caravan id would take to reach world tile
// t, resting at night (MAP-09, wiki: Caravan), and why it cannot ("" when
// it can).
func (g *Game) TravelEstimate(id uint64, t int) (ticks int, why string) {
	return fleetOf(g).Estimate(entity.EntityID(id), t, uint64(g.clock.Now()))
}

// mapsFlush applies game/maps' queued map requests and the hourly census
// at the end of a tick, after every map's phases (MAP-01..04; game.go
// Step). A single-map game that never opened another returns at once.
func mapsFlush(g *Game) {
	if w := g.worldSt; w != nil && !w.atlas.Empty() {
		fleetOf(g).Life().Flush(uint64(g.clock.Now()))
	}
}

// mapsApply takes a map command (game/maps Lifecycle.Apply).
func mapsApply(g *Game, c command.Command) { fleetOf(g).Life().Apply(c) }

// mapsLabel is map id's switcher label, kind and pack-up countdown.
func mapsLabel(g *Game, id uint32, label, kind string) (string, string, int, int) {
	if g.worldSt == nil || id == 0 {
		return label, kind, 0, 0
	}
	return fleetOf(g).Life().Label(id, label, kind, uint64(g.clock.Now()))
}

// DevOpenMap opens a site map on world tile at once for the journeys: its
// id. Without a planet (a legacy colony, a fixture) it opens a blank 60×60
// map of the colony's biome on tile -1; empty, it closes after three hours.
func DevOpenMap(g *Game, tile int) (MapID, bool) {
	g.viewGen++ // view cache (indexes.go): this hook changes the state
	now := uint64(g.clock.Now())
	if g.worldPlanet() != nil {
		id, ok := fleetOf(g).Life().Open(maps.Site, tile, now)
		return MapID(id), ok
	}
	id := g.worldS().atlas.Add(maps.Site, -1, now)
	m := newMapState(Config{Seed: maps.SeedFor(g.streams.seed, -1, id), Width: 60, Height: 60, Biome: g.maps[0].biome, Calendar: g.clock.Calendar()}, id)
	cur := g.mapState
	g.maps, g.mapState = append(g.maps, m), m
	initMapHosts(g)
	g.warmIndexes(true)
	g.mapState = cur
	return MapID(id), true
}

// closeMap closes map id (never the first): our people left on it are
// lost to the colony, and every creature, item, plant and building on it
// is forgotten in the colony-wide tables (forget).
func closeMap(g *Game, id uint32) {
	m := mapByID(g, id)
	if m == nil || m == g.maps[0] {
		return
	}
	cur := g.mapState
	g.mapState = m
	for _, pid := range slices.Clone(g.raiderIDs) {
		g.removeRaider(pid) // raids.go: a raider is an off-map person again
	}
	gone := slices.Concat(g.pawnIDs, g.animalIDs, g.itemIDs, g.bldgIDs, g.plantIDs, g.frameIDs)
	for _, pid := range g.pawnIDs {
		g.setWhere(pid, whereOffMap) // left behind: lost to the colony
		senseOf(g).Gear.Spill(pid)   // game/sense (CSE-036): its pack goes with what is on the map
	}
	for _, e := range gone {
		forget(g, e)
	}
	g.mapState = cur
	if cur == m {
		g.mapState = g.maps[0]
	}
	g.maps = slices.DeleteFunc(g.maps, func(x *mapState) bool { return x == m })
	if g.view == m {
		g.view = g.maps[0]
		g.viewGen++
	}
}

// forget drops entity id from every colony-wide table keyed by entity
// (its map's own stores close with the map).
func forget(g *Game, id entity.EntityID) {
	g.craft.Bills.Forget(id)
	g.names.Delete(id)
	g.need.Delete(id)
	g.vital.Delete(id)
	g.labor.Delete(id)
	for _, m := range []any{g.lastMeal, g.fuel, g.graves, g.exposure, g.noComfort, g.troughs, g.noFood, g.calm, g.banked,
		g.orderQueue, g.apparel, g.prio, g.scheds, g.joy, g.breaks, g.memories, g.ateAt, g.logs, g.draft, g.sleptAt,
		g.equip, g.lastStrike, g.lastHit, g.bodies, g.drugs} {
		reflect.ValueOf(m).SetMapIndex(reflect.ValueOf(id), reflect.Value{})
	}
	g.care.Carry.Forget(id)
	g.rest.Unmark(id)
	g.care.Rest.Clear(id)
}

type mapsOpen struct{ g *Game }

func (h mapsOpen) Current() uint32 { return h.g.id }
func (h mapsOpen) Close(id uint32) { closeMap(h.g, id) }
func (h mapsOpen) Switch(id uint32) bool {
	m := mapByID(h.g, id)
	if m != nil {
		h.g.mapState = m
	}
	return m != nil
}

// Open builds map id on world tile (MAP-01): the generator over the
// tile's site, the lump, its wildlife from the map's own stream (map 0's
// never moves), fog and warm indexes. The colony's landing spot is kept.
func (h mapsOpen) Open(id uint32, tile, w, ht int, seed uint64, lump item.Kind, qty, cells int) (maps.Opened, bool) {
	g := h.g
	p := g.worldPlanet()
	if p == nil || tile < 0 || tile >= p.TileCount() || !p.Land(tile) {
		return maps.Opened{}, false
	}
	st := p.Site(tile)
	m := newMapState(Config{Seed: seed, Width: w, Height: ht, Biome: st.Biome, Calendar: g.clock.Calendar()}, id)
	if m.tile = tile; st.Climate != (temperature.Climate{}) {
		m.climate = st.Climate.WithCalendar(g.clock.Calendar())
	}
	cur, rng, landing, has := g.mapState, g.rng, g.ng.landing, g.ng.hasLanding
	g.maps = append(g.maps, m)
	g.mapState, g.rng, g.ng.hasLanding = m, randx.New(seed^mapWildSalt), false
	initMapHosts(g)
	res, _ := pipeline.Generate(mapgen.Config{Seed: seed, W: w, H: ht, Site: st, Biome: biome.DefOf(st.Biome)})
	g.applySite(res) // setup_mapgen.go: sets the landing spot it found
	o := maps.Opened{Cells: maps.PlaceLump(mapsPop{g}, lump, qty, cells), Land: g.ng.landing, HasLand: g.ng.hasLanding}
	g.spawnWildlife()
	if o.HasLand {
		g.fogFromStart(o.Land[0], o.Land[1])
	} else {
		g.fogFromStart(w/2, ht/2)
	}
	g.warmIndexes(true)
	g.mapState, g.rng, g.ng.landing, g.ng.hasLanding = cur, rng, landing, has
	return o, true
}

// mapWildSalt keeps a new map's wildlife stream apart from its generator.
const mapWildSalt = 0x5717E_3A9

type mapsPop struct{ g *Game }

func (h mapsPop) Seed() uint64                                { return h.g.streams.seed }
func (h mapsPop) Raid()                                       { shipHost(h).Raid(false) }
func (h mapsPop) Census(cells []int) maps.Census              { return maps.CountSite(h, cells) }
func (h mapsPop) Aboard() (people, animals []entity.EntityID) { return maps.Aboard(h) }

func (h mapsPop) Colonists() []entity.EntityID     { return h.g.pawnIDs }
func (h mapsPop) RaiderIDs() []entity.EntityID     { return h.g.raiderIDs }
func (h mapsPop) AnimalIDs() []entity.EntityID     { return h.g.animalIDs }
func (h mapsPop) Prisoner(id entity.EntityID) bool { return h.g.prison.Is(id) }
func (h mapsPop) Threat(id entity.EntityID) bool   { return h.g.isRaider(id) && h.g.standingPawn(id) }
func (h mapsPop) Tame(id entity.EntityID) bool     { a := h.g.animals.Ref(id); return a != nil && a.Tame }
func (h mapsPop) Manhunter(id entity.EntityID) bool {
	a := h.g.animals.Ref(id)
	return a != nil && a.Manhunter > uint64(h.g.clock.Now())
}
func (h mapsPop) OreCell(i int) bool {
	w := h.g.grid.Width()
	return h.g.geo.OreAt(i%w, i/w) != geology.NoOre
}

func (h mapsPop) LumpSize() (int, int) { return h.g.grid.Width(), h.g.grid.Height() }
func (h mapsPop) LumpTaken() map[[2]int]bool {
	taken := map[[2]int]bool{}
	for _, id := range h.g.itemIDs {
		if s, ok := h.g.stacks.Get(id); ok {
			taken[[2]int{s.X, s.Y}] = true
		}
	}
	return taken
}
func (h mapsPop) LumpFree(x, y int) bool {
	g := h.g
	return g.dryWalkable(x, y) && g.plantAt(x, y) == 0 && g.layerBlockerAt(x, y) == 0
}
func (h mapsPop) LumpVein(x, y int, ore geology.Ore) {
	g := h.g
	g.setTerrain(x, y, terrain.Rock)
	if g.geo.At(x, y) == geology.None {
		g.geo.Set(x, y, geology.Granite)
	}
	g.geo.SetOre(x, y, ore)
}
func (h mapsPop) LumpStack(k item.Kind, qty, x, y int) { h.g.dropStack(k, qty, x, y) }

// PackUp: a settlement's goods start forbidden like any found loot, and
// taking them is what the pack-up is for, so there it allows what was found
// lying (mapsSettle.Settle).
func (h mapsPop) PackUp(cid uint64) {
	g := h.g
	m := atlasRow(g)
	loot := m != nil && (m.Kind == maps.Settlement || m.Defeated) // a fallen settlement is a Site
	travel.PackMapGoods(travelOf(g), haulHost{g}, cid, loot)
}

// CargoSource reads a detached eligible source stack on the specified map.
// The map and native entity ID identify the source; no external source
// label or actor authority is inferred.
func CargoSource(g *Game, mapID MapID, id entity.EntityID) (item.Stack, bool) {
	m := mapByID(g, uint32(mapID))
	if m == nil {
		return item.Stack{}, false
	}
	prev := g.mapState
	g.mapState = m
	defer func() { g.mapState = prev }()
	return cargoSource(g, id)
}

func cargoSource(g *Game, id entity.EntityID) (item.Stack, bool) {
	st, ok := g.stacks.Get(id)
	if !ok || st.Qty <= 0 || st.Unclaimed || st.Forbidden || g.reserve.Free(id, st.Qty) != st.Qty {
		return item.Stack{}, false
	}
	x, y := st.X, st.Y
	if st.Carrier != 0 {
		p, here := g.positions.Get(st.Carrier)
		j, _ := g.jobs.Get(st.Carrier)
		if !here || !slices.Contains(g.pawnIDs, st.Carrier) || !g.alive(st.Carrier) || g.isStashed(st.Carrier) || j.Active() && j.Carry == id {
			return item.Stack{}, false
		}
		x, y = p.X, p.Y
	}
	return st, g.grid.InBounds(x, y) && !g.fogged(x, y)
}

// recoverCargo delegates command admission and committed pace to the fleet.
func recoverCargo(g *Game, c command.Command) {
	if g.worldSt != nil {
		maps.RecoverCargo(fleetOf(g), recoverySource{g}, c)
	}
}

type recoverySource struct{ g *Game }

func (h recoverySource) Source(id entity.EntityID) (item.Stack, bool) { return cargoSource(h.g, id) }
func (h recoverySource) CrewEligible(id, cid entity.EntityID) bool {
	g := h.g
	i, ok := slices.BinarySearchFunc(g.worldSt.stashed, id, stashCmp)
	p := g.personOf(id)
	return ok && g.worldSt.stashed[i].Holder == cid && !g.worldSt.stashed[i].Prisoner && g.worldSt.stashed[i].Beast == nil && p != nil && p.Faction == 0 && p.Where == whereCaravan && g.alive(id)
}
func (h recoverySource) RecoverFreight() maps.RecoveryFreight             { return travelOf(h.g) }
func (h recoverySource) Debit(id entity.EntityID, st item.Stack, qty int) { h.g.takeItems(id, st, qty) }

// atlasRow is the current map's atlas row (nil: the first map or none).
func atlasRow(g *Game) *maps.Meta {
	if g.id == 0 || g.worldSt == nil {
		return nil
	}
	return g.worldSt.atlas.Meta(g.id)
}

// DevDemand stages a road demand on caravan id, carrying st (MAP-08 rolls
// one at random), for the journeys.
func DevDemand(g *Game, id entity.EntityID, st item.Stack) {
	g.viewGen++
	travelOf(g).Carry(uint64(id), st)
	maps.RefreshPace(fleetOf(g), id)
	maps.DevDemand(fleetOf(g), id, uint64(g.clock.Now()))
}

// DevArrive sets caravan id down at tile as if it had walked, for the journeys.
func DevArrive(g *Game, id entity.EntityID, tile int) {
	g.viewGen++
	maps.DevArrive(fleetOf(g), id, tile, uint64(g.clock.Now()))
}

// DevClearHostiles removes map id's raiders but not its guests (a quest
// camp's guards around its prisoner), for the journeys.
func DevClearHostiles(g *Game, id MapID) {
	if m := mapByID(g, uint32(id)); m != nil {
		g.viewGen++
		cur := g.mapState
		g.mapState = m
		for _, r := range slices.Clone(m.raiderIDs) {
			if !g.visitor(r) {
				g.removeRaider(r)
			}
		}
		g.mapState = cur
	}
}

// SetMaxColonies is command.SetColonyCap: Options' Max colonies (MAP-06).
func SetMaxColonies(g *Game, n int) { g.worldS().maxColonies = max(1, min(n, maps.ColonyCapLimit)) }

// SettleReason is why caravan id cannot found a colony here ("" it can; MAP-06).
func SettleReason(g *Game, id uint64) string {
	if g.worldSt == nil {
		return "No such caravan."
	}
	return fleetOf(g).Life().SettleReason(entity.EntityID(id))
}

// threatTarget is the map a threat strikes (MAP-12): the current one with one
// colony, with no draw; else a colony with people on it, weighted by wealth,
// from the story stream. Sites, settlements and encounters are never picked.
func threatTarget(g *Game) *mapState {
	cur := g.mapState
	if g.worldSt == nil || len(g.worldSt.atlas.Maps) == 0 {
		return cur
	}
	var to []*mapState
	var weight []int
	for _, m := range g.maps {
		g.mapState = m
		if r := atlasRow(g); len(m.pawnIDs) > 0 && (r == nil || r.Kind == maps.Colony) {
			to, weight = append(to, m), append(weight, g.Wealth()+1)
		}
	}
	g.mapState = cur
	if i := maps.PickWeighted(weight, g.stream(streamStory).IntN); i >= 0 {
		return to[i]
	}
	return cur
}

type mapsFound struct{ g *Game }

func (h mapsFound) MaxColonies() int { return h.g.worldS().maxColonies }
func (h mapsFound) ColonySide() int  { return h.g.maps[0].grid.Width() }
func (h mapsFound) StartHome()       { h.g.wild = true; h.g.startHome() }

// Promote makes colony map next the first map (MAP-07): view first, so it
// always points at an open map; then the two swap ids, and the setup's tile
// and landing are the new first colony's.
func (h mapsFound) Promote(next uint32, land [2]int, hasLand bool) {
	g := h.g
	n, old := mapByID(g, next), g.maps[0]
	if n == nil {
		return
	}
	if g.view == old {
		g.view = n
		g.viewGen++
	}
	if g.mapState == old {
		g.mapState = n
	}
	n.id, old.id = 0, next
	g.maps[slices.Index(g.maps, n)], g.maps[0] = old, n
	g.ng.landing, g.ng.hasLanding = land, hasLand
	if p := g.worldPlanet(); p != nil {
		if g.ng.setup != nil {
			g.ng.setup.Tile, g.ng.setup.Site = n.tile, p.Site(n.tile)
		} else if g.worldSt.link != nil {
			g.worldSt.link.Tile = n.tile
		}
	}
	swap := func(id MapID) MapID {
		switch id {
		case 0:
			return MapID(next)
		case MapID(next):
			return 0
		}
		return id
	}
	for i := range g.messages {
		g.messages[i].Map = swap(g.messages[i].Map)
	}
	g.letters.EditAll(func(l *letters.Letter) { l.Map = uint32(swap(MapID(l.Map))) })
}

// Wipe removes the current map's colonists, prisoners and items (MAP-07):
// the last colony's end. The map itself stays.
func (h mapsFound) Wipe() {
	g := h.g
	for _, id := range slices.Clone(g.raiderIDs) {
		if g.prison.Is(id) {
			g.removeRaider(id)
		}
	}
	for _, id := range g.pawnIDs {
		senseOf(g).Gear.Spill(id) // game/sense (CSE-036): the items go below, with the rest
		g.reserve.ReleaseOwner(id)
		g.setWhere(id, whereOffMap)
		g.positions.Delete(id)
		g.jobs.Delete(id)
		g.motion.Delete(id)
		forget(g, id)
	}
	g.pawnIDs = nil
	for _, id := range slices.Clone(g.itemIDs) {
		g.deleteItem(id)
	}
	g.invalidateCreatures()
}

type mapsWaylay struct{ g *Game }

func (h mapsWaylay) RoadRoll(n int) int { return h.g.stream(streamRoad).IntN(n) }
func (h mapsWaylay) Items() item.Book   { return h.g.sk.content.Items }

// Ambush sends raiders of faction fac in from an edge of the current map;
// its draws come from seed, so g.rng does not move.
func (h mapsWaylay) Ambush(seed uint64, fac int) {
	g, rng := h.g, h.g.rng
	g.rng = randx.New(seed)
	tech := faction.Industrial
	if f := g.factionOf(fac); f != nil {
		tech = faction.DefOf(f.Type).Tech
	}
	day := int(uint64(g.clock.Now()) / clock.TicksPerDay)
	g.raidFrom(g.raidKitOf(g.raidPoints(day), tech), raid.Assault, fac)
	g.rng = rng
}

// Hunt sends a manhunter pack of n in from an edge of the current map.
func (h mapsWaylay) Hunt(seed uint64, n int) {
	g, rng := h.g, h.g.rng
	g.rng = randx.New(seed)
	rageOf(g).Pack(n)
	g.rng = rng
}
func (h mapsWaylay) DemandLetter(title, format string, args ...any) uint64 {
	g := h.g
	letterf(g, letters.Threat, title, 0, format, args...)
	var id uint64
	g.letters.EditLast(func(l *letters.Letter) { l.Demand, id = true, l.ID })
	return id
}
func (h mapsWaylay) DropLetter(id uint64) {
	g := h.g
	g.letters.Remove(id)
}
