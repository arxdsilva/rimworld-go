package game

import (
	"cmp"
	"fmt"
	"reflect"
	"slices"

	"rimworld-go/internal/application/game/maps"
	"rimworld-go/internal/application/game/raid"
	"rimworld-go/internal/application/game/ship"
	"rimworld-go/internal/application/game/travel"
	"rimworld-go/internal/application/game/visit"
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/combat/explosion"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/pawn/body"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/pawn/recreation"
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
	"rimworld-go/pkg/randx"
)

// The world beyond the map (ENG-003, M9). The caravans' state machine
// lives in game/maps (forming, the road, visits, coming home) over the
// adapters at the end of this file; members away are stashed off the map
// (world_stash.go). Away, a member's needs are frozen while the caravan
// has food; once it runs out its hunger drains and a member dying of
// malnutrition dies away (dieAway, no corpse on the map).
//
// The planet is not saved: it is rebuilt lazily from the setup's World
// params (deterministic). A legacy colony (no setup) has no world map and
// cannot send caravans.

// worldStep is phaseWorld's cadence for the slow upkeep (caravans,
// allowed areas); caravans advance on the hour.
const worldStep = 250

// worldState is the world beyond the map: caravans, the pawns away on
// them, the visitors on the map and the allowed-area assignments. nil
// until first needed; saved as World. The Game field is worldSt (g.world
// is the World read model).
type worldState struct {
	// planet is rebuilt from the setup's World params on first use (never
	// saved); planetTried: the build was attempted (nil: no world).
	planet      *planet.Planet
	planetTried bool
	caravans    []maps.Caravan // ascending ID (game/maps)
	stashed     []stashRec     // ascending ID (world_stash.go)
	visitors    []visitorRec   // ascending ID (visitors.go)
	// areaOf is each restricted colonist's or tame animal's allowed area
	// ref (areas.go): an area id, or command.AreaHome; absent means
	// unrestricted. Looked up only, never iterated for output unsorted.
	areaOf map[entity.EntityID]int
	// pathFree is the pawn whose job startJob is pathing free of its area
	// right now (areas.go; transient, 0 between calls).
	pathFree entity.EntityID
	// huntOut is, per hunter, the prey of its chase and when it was first
	// seen outside the hunter's area (area_dest.go; saved).
	huntOut map[entity.EntityID]huntOutRec
	// areaVer is each area's version (by id), areaGen the last one handed
	// out (areas.go; not saved: views only).
	areaVer     map[uint16]uint64
	areaGen     uint64
	ship        *ship.State   // story.go: game/ship (AX-8), built on first use
	ax9         *travel.State // game/travel (AX-9): caskets, pods, sites, hitching
	ax9h        travel.Host   // its host (travelOf), built once
	mapsH       maps.Host     // game/maps' host (fleetOf), built once
	maxColonies int           // Options' cap, sent at start and load (never saved; 0 means 1)
	visitH      visit.Host    // game/visit's host (visitsOf), built once
	atlas       maps.Atlas    // game/maps: the open maps beyond the first (MAP-01)
}

// worldSave is worldState as saved.
type worldSave struct {
	Version  uint8 // format of this block; 0 = empty
	Caravans []maps.Caravan
	Stashed  []stashRec
	Visitors []visitorRec
	// AreaOf is the allowed-area assignments: Version 1 saves hold area
	// positions (index i is id i+1 now), Version 2 area refs (PLAY-015).
	AreaOf  []areaAssign
	HuntOut []huntOutRec // area_dest.go
	AX9     *travel.Save // game/travel (AX-9): absent in older saves
	Atlas   *maps.Atlas  // game/maps (MAP-01): absent while one map was ever open
}

// worldSaveVersion is the format exportWorld writes (2: area refs).
const worldSaveVersion = 2

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

// worldPlanet is the colony's planet, rebuilt from its setup on first use;
// nil for a colony without a world map (legacy, or no world params).
func (g *Game) worldPlanet() *planet.Planet {
	w := g.worldS()
	if w.planetTried {
		return w.planet
	}
	w.planetTried = true
	if !g.hasSetup() || g.ng.setup.World.Coverage <= 0 {
		return nil
	}
	p := planet.Build(g.ng.setup.World)
	if g.ng.setup.Tile < 0 || g.ng.setup.Tile >= p.TileCount() {
		return nil
	}
	w.planet = p
	maps.Prune(p, g.ng.factions) // MAP-05: settlements destroyed in this game stay off the rebuilt planet
	return p
}

// colonyTile is the colony's world tile (-1: no world map).
func (g *Game) colonyTile() int {
	if !g.hasSetup() || g.ng.setup.World.Coverage <= 0 {
		return -1
	}
	return g.ng.setup.Tile
}

// phaseWorld runs after phaseIncidents: visitors every tick (a handful of
// pawns), caravans and allowed areas every worldStep ticks.
func (g *Game) phaseWorld() {
	w := g.worldSt
	if w == nil {
		return
	}
	now, first := uint64(g.clock.Now()), primary(g) // MAP-01: colony-wide upkeep on the first map's pass
	if len(w.visitors) > 0 && first {
		visitsOf(g).Tick(aidOf(g), now) // game/visit (visitors.go)
	}
	if len(w.caravans) > 0 && now%maps.FormCheck == 0 {
		fleetOf(g).CheckForming(now) // each map's own forming caravans
	}
	travelOf(g).TickPass(now, first) // game/travel (AX-9): each map's caskets and boarders, the colony's flights and sites
	if !first || now%worldStep != 0 {
		return
	}
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
	x, y := g.home()
	g.post(MessageView{Tick: uint64(g.clock.Now()), Text: text, X: x, Y: y, Focus: focus, Letter: true, Title: title, Kind: kind})
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

// settlementFaction is the faction (1 + index) whose settlement stands on
// tile t, 0 when none.
func (g *Game) settlementFaction(t int) int {
	for i, f := range g.ng.factions {
		if slices.Contains(f.Settlements, t) {
			return i + 1
		}
	}
	return 0
}

// tileName names world tile t: its settlement, else "tile N".
func (g *Game) tileName(t int) string {
	if p := g.worldPlanet(); p != nil {
		if n := p.SettlementName(t); n != "" {
			return n
		}
	}
	if f := g.factionOf(g.settlementFaction(t)); f != nil {
		return "a settlement of " + f.Name
	}
	return fmt.Sprintf("tile %d", t)
}

// dieAway kills caravan member id off the map: no corpse, its weapon lost
// with it; the colony grieves as for any death.
func dieAway(g *Game, id entity.EntityID, cause health.Cause) {
	c := fleetOf(g).Of(id)
	delete(g.equip, id)
	g.setWhere(id, whereColony) // the colony's death (its history line), not a stranger's
	g.dieBy(id, cause, 0)
	dropStash(g, id)
	if c != nil {
		c.C.Members = slices.DeleteFunc(c.C.Members, func(m uint64) bool { return m == uint64(id) })
	}
	g.letter(letterCaution, "Death on the road", g.personName(id)+" died on the caravan's journey.", 0)
}

// worldEdge is a walkable map-edge tile from which the colony can be
// reached (walls and doors broken if need be), drawn from the world
// stream: what edgeTile is for raids, without touching g.rng.
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
	if w == nil || len(w.caravans) == 0 && len(w.stashed) == 0 && len(w.visitors) == 0 && len(w.areaOf) == 0 && len(w.huntOut) == 0 && w.ax9.Empty() && w.atlas.Empty() {
		return nil
	}
	s := &worldSave{Version: worldSaveVersion, Stashed: slices.Clone(w.stashed), Visitors: slices.Clone(w.visitors)}
	for _, c := range w.caravans {
		s.Caravans = append(s.Caravans, c.Clone())
	}
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

// importWorld restores exportWorld's state (nil: fresh).
func (g *Game) importWorld(s *worldSave) {
	g.worldSt = nil
	if s == nil {
		return
	}
	w := g.worldS()
	for _, c := range s.Caravans {
		w.caravans = append(w.caravans, c.Clone())
	}
	w.stashed = slices.Clone(s.Stashed)
	w.visitors = slices.Clone(s.Visitors)
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
}

// worldMapView builds the World tab: the colony tile and every caravan.
func (g *Game) worldMapView() WorldMapView {
	v := WorldMapView{Tile: g.colonyTile()}
	if g.worldSt == nil {
		return v
	}
	for i := range g.worldSt.caravans {
		c := &g.worldSt.caravans[i]
		cv := CaravanView{ID: c.C.ID, Tile: c.C.Tile, Dest: c.C.Dest, Progress: c.C.Progress / 10, State: c.C.State.Label()}
		if c.Back {
			cv.Dest = c.Home
		}
		for _, m := range c.C.Members {
			cv.Members = append(cv.Members, g.personName(entity.EntityID(m)))
			cv.Health = append(cv.Health, g.caravanMemberStatus(entity.EntityID(m))) // caravan_medical.go
		}
		v.Caravans = append(v.Caravans, cv)
	}
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
func (h travelHost) RemoveAnimal(id entity.EntityID)           { h.g.removeAnimal(id) }
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
func (h travelHost) Assault(cid uint64, centre bool) { fleetOf(h.g).Assault(cid, centre) }

// MapOpen, LandOn: pods onto an open map (MAP-10, game/maps pod.go).
func (h travelHost) MapOpen(t int) bool { return fleetOf(h.g).MapOpen(t) }
func (h travelHost) LandOn(members, animals []uint64, t int, centre, spot bool, x, y int) uint64 {
	return fleetOf(h.g).LandOn(members, animals, t, centre, spot, x, y)
}

// FoundReason, Found: pods founding a colony (MAP-06).
func (h travelHost) FoundReason(t int) string { return fleetOf(h.g).FoundReason(t) }
func (h travelHost) Found(cid uint64) string  { return fleetOf(h.g).Found(cid) }

// Land: pod passengers become a caravan arrived on t (home by land).
func (h travelHost) Land(members, animals []uint64, t int) uint64 {
	return fleetOf(h.g).Land(members, animals, t)
}

// fleetOf is game/maps' caravans over the game, its state in
// worldSt.caravans.
func fleetOf(g *Game) maps.Fleet {
	w := g.worldS()
	if w.mapsH == nil {
		w.mapsH = mapsHost{mapsCrew{g}, mapsWalk{g}, mapsGround{g}, mapsGlobe{g}, mapsRoad{g}, mapsOpen{g}, mapsPop{g}, mapsSettle{g}, mapsFound{g}, mapsWaylay{g}} // built once
	}
	return maps.Of(&w.caravans, &w.atlas, w.mapsH)
}

// mapsHost is game/maps' roles (maps/deps.go) over small adapters.
type mapsHost struct {
	mapsCrew
	mapsWalk
	mapsGround
	mapsGlobe
	mapsRoad
	mapsOpen
	mapsPop
	mapsSettle
	mapsFound
	mapsWaylay
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
	x, y, ok := h.g.combat.Tiles.WalkableNear(ex, ey, 3)
	return ok && h.g.startJob(id, job.Record{Kind: job.ExitMap, TX: x, TY: y})
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
func (h mapsGround) Letter(caution bool, title, text string, focus entity.EntityID) {
	kind := letterInfo
	if caution {
		kind = letterCaution
	}
	h.g.letter(kind, title, text, focus)
}
func (h mapsGround) Note(text string) { h.g.worldNote("%s", text) }

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
		h.g.joy[id] = min(h.g.joy[id]+joy, recreation.Full)
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
func mapsLabel(g *Game, id uint32, label, kind string) (string, string, int) {
	if g.worldSt == nil || id == 0 {
		return label, kind, 0
	}
	return fleetOf(g).Life().Label(id, label, kind, uint64(g.clock.Now()))
}

// DevOpenMap opens a site map on world tile at once (a dev hook for the
// journeys), registered in the atlas: its id. Without a planet (a legacy
// colony, a test fixture) it opens a blank 60×60 map of the colony's
// biome on tile -1 instead. Like any temporary map it closes after three
// hours with nobody of ours on it.
func DevOpenMap(g *Game, tile int) (MapID, bool) {
	g.viewGen++ // view cache (indexes.go): this hook changes the state
	now := uint64(g.clock.Now())
	if g.worldPlanet() != nil {
		id, ok := fleetOf(g).Life().Open(maps.Site, tile, now)
		return MapID(id), ok
	}
	id := g.worldS().atlas.Add(maps.Site, -1, now)
	m := newMapState(Config{Seed: maps.SeedFor(g.streams.seed, -1, id), Width: 60, Height: 60, Biome: g.maps[0].biome}, id)
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
func (h mapsOpen) Open(id uint32, tile, w, ht int, seed uint64, lump item.Kind, qty int) (maps.Opened, bool) {
	g := h.g
	p := g.worldPlanet()
	if p == nil || tile < 0 || tile >= p.TileCount() || !p.Land(tile) {
		return maps.Opened{}, false
	}
	st := p.Site(tile)
	m := newMapState(Config{Seed: seed, Width: w, Height: ht, Biome: st.Biome}, id)
	if m.tile = tile; st.Climate != (temperature.Climate{}) {
		m.climate = st.Climate
	}
	cur, rng, landing, has := g.mapState, g.rng, g.ng.landing, g.ng.hasLanding
	g.maps = append(g.maps, m)
	g.mapState, g.rng, g.ng.hasLanding = m, randx.New(seed^mapWildSalt), false
	initMapHosts(g)
	res, _ := pipeline.Generate(mapgen.Config{Seed: seed, W: w, H: ht, Site: st, Biome: biome.DefOf(st.Biome)})
	g.applySite(res) // setup_mapgen.go: sets the landing spot it found
	o := maps.Opened{Cells: placeLump(g, lump, qty), Land: g.ng.landing, HasLand: g.ng.hasLanding}
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

// placeLump lays a mineral site's lump near the centre: rock cells
// holding its ore, enough to mine about qty (OreYield a cell); a mineral
// with no vein kind lies dug out in one stack. The ore cells, or nil.
func placeLump(g *Game, k item.Kind, qty int) []int {
	ore := map[item.Kind]geology.Ore{item.Steel: geology.Steel, item.Silver: geology.Silver, item.Gold: geology.Gold, item.Components: geology.Machinery}[k]
	if k == 0 || qty <= 0 {
		return nil
	}
	n, w := 1, g.grid.Width()
	if ore != geology.NoOre {
		n = (qty + OreYield[ore] - 1) / OreYield[ore]
	}
	taken := map[[2]int]bool{}
	for _, sid := range g.itemIDs {
		if s, ok := g.stacks.Get(sid); ok {
			taken[[2]int{s.X, s.Y}] = true
		}
	}
	cells := maps.LumpCells(w, g.grid.Height(), n, func(x, y int) bool {
		return g.dryWalkable(x, y) && g.plantAt(x, y) == 0 && g.layerBlockerAt(x, y) == 0 && !taken[[2]int{x, y}]
	})
	if ore == geology.NoOre {
		if len(cells) > 0 {
			g.dropStack(k, qty, cells[0]%w, cells[0]/w)
		}
		return nil
	}
	for _, c := range cells {
		x, y := c%w, c/w
		g.setTerrain(x, y, terrain.Rock)
		if g.geo.At(x, y) == geology.None {
			g.geo.Set(x, y, geology.Granite)
		}
		g.geo.SetOre(x, y, ore)
	}
	return cells
}

type mapsPop struct{ g *Game }

func (h mapsPop) Seed() uint64 { return h.g.streams.seed }
func (h mapsPop) Raid()        { shipHost{h.g}.Raid(false) } // story.go: the storyteller's raid
func (h mapsPop) Census(cells []int) maps.Census {
	g := h.g
	c := maps.Census{Ours: len(g.pawnIDs)}
	for _, id := range g.raiderIDs {
		if g.prison.Is(id) {
			c.Ours++
		} else if g.isRaider(id) && g.standingPawn(id) {
			c.Hostiles++
		}
	}
	for _, id := range g.animalIDs {
		if a := g.animals.Ref(id); a != nil && a.Tame {
			c.Ours++
		} else if a != nil && a.Manhunter > uint64(g.clock.Now()) {
			c.Hostiles++ // MAP-08: a manhunter ambush is a fight too
		}
	}
	for _, i := range cells {
		if g.geo.OreAt(i%g.grid.Width(), i/g.grid.Width()) != geology.NoOre {
			c.Ore++
		}
	}
	return c
}
func (h mapsPop) Aboard() (people, animals []entity.EntityID) {
	g := h.g
	people = slices.Clone(g.pawnIDs)
	for _, id := range g.raiderIDs {
		if g.prison.Is(id) {
			people = append(people, id)
		}
	}
	slices.Sort(people)
	for _, id := range g.animalIDs {
		if a := g.animals.Ref(id); a != nil && a.Tame {
			animals = append(animals, id)
		}
	}
	return people, animals
}
func (h mapsPop) PackUp(cid uint64) {
	g := h.g
	for _, sid := range slices.Clone(g.itemIDs) {
		if st, ok := g.stacks.Get(sid); ok && !st.Forbidden {
			travelOf(g).Carry(cid, st)
			g.deleteItem(sid)
		}
	}
}

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
	if f := fleetOf(g); f.At(id) != nil {
		f.Waylay(f.At(id), maps.Demand, uint64(g.clock.Now()))
	}
}

// SetMaxColonies is command.SetColonyCap: Options' Max colonies (MAP-06).
func SetMaxColonies(g *Game, n int) { g.worldS().maxColonies = max(1, min(n, maps.ColonyCapLimit)) }

// SettleReason is why caravan id cannot found a colony where it is ("" it
// can; MAP-06).
func SettleReason(g *Game, id uint64) string {
	if g.worldSt == nil {
		return "No such caravan."
	}
	return fleetOf(g).SettleReason(entity.EntityID(id))
}

// threatTarget is the map a threat strikes (MAP-12): the current one in a
// game with one colony, with no draw; else a colony with people on it,
// weighted by wealth, from the story stream. Sites, settlements and
// encounters are never picked.
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
	if p := g.worldPlanet(); p != nil && g.ng.setup != nil {
		g.ng.setup.Tile, g.ng.setup.Site = n.tile, p.Site(n.tile)
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
	for i := range g.letters.list {
		g.letters.list[i].Map = swap(g.letters.list[i].Map)
	}
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
	g.manhunterPack(n)
	g.rng = rng
}
func (h mapsWaylay) DemandLetter(title, text string) uint64 {
	g := h.g
	g.letter(letterThreat, title, text, 0)
	l := &g.letters.list[len(g.letters.list)-1]
	l.Demand = true
	return l.ID
}
func (h mapsWaylay) DropLetter(id uint64) {
	g := h.g
	g.letters.list = slices.DeleteFunc(g.letters.list, func(l savedLetter) bool { return l.ID == id })
}
