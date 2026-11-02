package game

import (
	"cmp"
	"fmt"
	"slices"

	"rimworld-go/internal/application/game/ship"
	"rimworld-go/internal/application/game/travel"
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
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/caravan"
	"rimworld-go/internal/domain/world/planet"
)

// The world beyond the map (ENG-003, M9). A caravan's members walk to the
// nearest map edge (job.ExitMap, drafted so schedules and work leave them
// alone), are stashed off the map (world_stash.go) and travel the planet
// route (planet.Route) at the pace of the slowest Moving capacity. At a
// friendly or neutral settlement they visit for a day (and restock), then
// come back the way they went and walk in from an edge tile.
//
// Away, a member's needs are frozen while the caravan has food (they eat
// and rest on the road); once FoodDays run out its hunger drains at the
// usual rate and a member dying of malnutrition dies away (dieBy, no
// corpse on the map), so the colony grieves. Members talk among themselves every few hours (the
// caravan is one room): a chitchat or a deep talk, with its joy.
//
// The planet is not saved: it is rebuilt lazily from the setup's World
// params (deterministic). A legacy colony (no setup) has no world map and
// cannot send caravans.

const (
	// worldStep is phaseWorld's cadence for the slow upkeep (caravans,
	// allowed areas); caravans advance on the hour.
	worldStep = 250
	// caravanFoodDays is the food a caravan sets out with (and restocks to
	// at a friendly settlement), in milli-days.
	caravanFoodDays int32 = 5000
	// caravanVisitTicks is the stay at a friendly settlement.
	caravanVisitTicks = clock.TicksPerDay
	// caravanTalkPeriod is how often two members talk on the road.
	caravanTalkPeriod = 2 * clock.TicksPerHour
	// caravanTalkJoy is the recreation a talk on the road gives each side.
	caravanTalkJoy = recreation.Full / 25
	// caravanDeepTalkPct is the share of talks on the road that go deep.
	caravanDeepTalkPct = 25
	// caravanFormCheck: forming members are looked at this often; one that
	// is still on the map caravanFormLimit after the order stays behind.
	caravanFormCheck = 60
	caravanFormLimit = clock.TicksPerDay
)

// worldState is the world beyond the map: caravans, the pawns away on
// them, the visitors on the map and the allowed-area assignments. nil
// until first needed; saved as World. The Game field is worldSt (g.world
// is the World read model).
type worldState struct {
	// planet is rebuilt from the setup's World params on first use (never
	// saved); planetTried: the build was attempted (nil: no world).
	planet      *planet.Planet
	planetTried bool
	caravans    []caravanRec // ascending ID
	stashed     []stashRec   // ascending ID (world_stash.go)
	visitors    []visitorRec // ascending ID (visitors.go)
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
	areaVer map[uint16]uint64
	areaGen uint64
	ship    *ship.State   // story.go: game/ship (AX-8), built on first use
	ax9     *travel.State // game/travel (AX-9): caskets, pods, sites, hitching
	ax9h    travel.Host   // its host (travelOf), built once
}

// caravanRec is one caravan with the colony's bookkeeping around it.
type caravanRec struct {
	C caravan.Caravan
	// Home is the colony's tile; Out the outbound route, home first and
	// the destination last (the way back is the travelled part reversed).
	Home int
	Out  []int
	// Back: the caravan is on its way home.
	Back bool
	// Exiting are members still walking to the map edge (Forming).
	Exiting []entity.EntityID
	// Formed is when it was ordered; NextTalk the next talk on the road.
	Formed, NextTalk uint64
}

// worldSave is worldState as saved.
type worldSave struct {
	Version  uint8 // format of this block; 0 = empty
	Caravans []caravanRec
	Stashed  []stashRec
	Visitors []visitorRec
	// AreaOf is the allowed-area assignments: Version 1 saves hold area
	// positions (index i is id i+1 now), Version 2 area refs (PLAY-015).
	AreaOf  []areaAssign
	HuntOut []huntOutRec // area_dest.go
	AX9     *travel.Save // game/travel (AX-9): absent in older saves
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
	now := uint64(g.clock.Now())
	if len(w.visitors) > 0 {
		g.visitorsTick(now) // visitors.go
	}
	if len(w.caravans) > 0 && now%caravanFormCheck == 0 {
		g.checkForming(now)
	}
	travelOf(g).Tick(now) // game/travel (AX-9)
	if now%worldStep != 0 {
		return
	}
	if len(w.areaOf) > 0 {
		g.areasTick(now) // areas.go
	}
	if len(w.caravans) > 0 {
		g.stepCaravans(now)
	}
	if w.ship.Busy() {
		w.ship.Tick(shipHost{g}) // story.go: game/ship (AX-8): countdowns, raids, fallen caskets
	}
}

// caravanAt is caravan id (nil: none).
func (g *Game) caravanAt(id entity.EntityID) *caravanRec {
	if g.worldSt == nil {
		return nil
	}
	for i := range g.worldSt.caravans {
		if g.worldSt.caravans[i].C.ID == uint64(id) {
			return &g.worldSt.caravans[i]
		}
	}
	return nil
}

// caravanOf is the caravan pawn id belongs to (nil: none).
func (g *Game) caravanOf(id entity.EntityID) *caravanRec {
	if g.worldSt == nil {
		return nil
	}
	for i := range g.worldSt.caravans {
		if slices.Contains(g.worldSt.caravans[i].C.Members, uint64(id)) {
			return &g.worldSt.caravans[i]
		}
	}
	return nil
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

// cmdFormCaravan sends the listed members (comma-separated entity ids) to
// world tile dest: each standing colonist is drafted and walks to the
// nearest map edge; the caravan sets out once they are all off the map.
func (g *Game) cmdFormCaravan(members string, dest int) {
	p := g.worldPlanet()
	home := g.colonyTile()
	if p == nil || home < 0 {
		g.worldNote("No world map: this colony cannot send caravans.")
		return
	}
	if dest < 0 || dest >= p.TileCount() || dest == home {
		g.worldNote("Pick a destination for the caravan.")
		return
	}
	ids := g.caravanCandidates(members)
	if len(ids) == 0 {
		g.worldNote("Nobody can join the caravan.")
		return
	}
	route := p.Route(home, dest)
	if len(route) < 2 {
		g.worldNote("The caravan has no way to get there.")
		return
	}
	now := uint64(g.clock.Now())
	c := caravanRec{
		C:    caravan.Caravan{ID: uint64(g.entities.Next()), Tile: home, Dest: dest, Path: slices.Clone(route[1:]), State: caravan.Forming},
		Home: home, Out: route, Formed: now,
	}
	travelOf(g).Pack(c.C.ID, members, len(ids)) // game/travel (AX-9): goods and pack animals
	for _, id := range ids {
		g.setDraft(id, true) // schedules and work leave a caravan member alone; AX-9: a hitching spot first
		if !travelOf(g).Hitch(c.C.ID, id, len(c.C.Members)) && !g.startExit(id) {
			g.setDraft(id, false)
			continue
		}
		c.C.Members = append(c.C.Members, uint64(id))
		c.Exiting = append(c.Exiting, id)
	}
	if len(c.C.Members) == 0 {
		g.worldNote("Nobody can reach the map edge.")
		return
	}
	w := g.worldS()
	w.caravans = append(w.caravans, c)
	g.worldNote("The caravan is forming: %d heading for the map edge.", len(c.C.Members))
}

// caravanCandidates parses the member list: standing colonists on the map,
// not breaking and not already in a caravan, each once, ascending.
func (g *Game) caravanCandidates(members string) []entity.EntityID {
	return travel.ParseIDs(members, func(id entity.EntityID) bool {
		return slices.Contains(g.pawnIDs, id) && g.standingPawn(id) && !g.breaking(id) && g.caravanOf(id) == nil
	})
}

// startExit sends pawn id to the nearest map edge to leave (job.ExitMap).
func (g *Game) startExit(id entity.EntityID) bool {
	pos, ok := g.positions.Get(id)
	if !ok {
		return false
	}
	ex, ey := nearestEdge(pos, g.grid.Width(), g.grid.Height())
	x, y, ok := g.combat.Tiles.WalkableNear(ex, ey, 3)
	if !ok {
		return false
	}
	return g.startJob(id, job.Record{Kind: job.ExitMap, TX: x, TY: y})
}

// doKidnap: kidnapping is a raider's job, run from the raid AI
// (raid/kidnap.go Step); a pawn on pawnIDs never kidnaps, so one found
// with the job just ends it.
func (g *Game) doKidnap(id entity.EntityID, j job.Record) {
	g.endStubJob(id, j)
}

// doExitMap: a pawn at the map edge on job.ExitMap leaves the map. A
// caravan member is stashed with its caravan; anyone else just ends the
// job.
func (g *Game) doExitMap(id entity.EntityID, j job.Record) {
	c := g.caravanOf(id)
	if c == nil || c.C.State != caravan.Forming || !slices.Contains(c.Exiting, id) {
		g.endStubJob(id, j)
		return
	}
	cid := entity.EntityID(c.C.ID)
	if !g.stashPawn(id, cid) {
		g.endStubJob(id, j)
		return
	}
	c = g.caravanAt(cid)
	c.Exiting = withoutID(c.Exiting, id)
	if len(c.Exiting) == 0 {
		g.departCaravan(c, uint64(g.clock.Now()))
	}
}

// checkForming drops forming members who can no longer go (down, broken,
// undrafted, or too long on the way), re-sends those whose walk failed for
// want of a search slot, and sets out or cancels caravans with nobody left
// walking.
func (g *Game) checkForming(now uint64) {
	w := g.worldSt
	for i := 0; i < len(w.caravans); i++ {
		c := &w.caravans[i]
		if c.C.State != caravan.Forming || travelOf(g).Gather(c.C.ID, c.Exiting, c.Formed, now) { // AX-9: waiting at the hitching spot
			continue
		}
		for _, id := range slices.Clone(c.Exiting) {
			j, _ := g.jobs.Get(id)
			switch {
			case !g.standingPawn(id) || g.breaking(id) || !g.drafted(id) || now-c.Formed >= caravanFormLimit:
			case j.Kind == job.ExitMap && j.Active():
				continue
			case j.Kind == job.ExitMap && j.State == job.Fail && j.Fail == job.Reserved && g.startExit(id):
				continue // a deferred search: tried again
			case j.Kind != job.ExitMap && g.startExit(id):
				continue // an order or a shot in between: back on the way
			}
			g.dropMember(c, id)
		}
		if len(c.Exiting) > 0 {
			continue
		}
		if len(c.C.Members) == 0 {
			g.worldNote("The caravan was called off: nobody left to go.")
			w.caravans = slices.Delete(w.caravans, i, i+1)
			i--
			continue
		}
		g.departCaravan(c, now)
	}
}

// dropMember takes a forming member out of caravan c (it stays home).
func (g *Game) dropMember(c *caravanRec, id entity.EntityID) {
	c.Exiting = withoutID(c.Exiting, id)
	c.C.Members = slices.DeleteFunc(c.C.Members, func(m uint64) bool { return m == uint64(id) })
	if g.drafted(id) {
		g.setDraft(id, false)
	}
}

// departCaravan sets a formed caravan on its way: speed from the slowest
// member's Moving, its food, the departure letter.
func (g *Game) departCaravan(c *caravanRec, now uint64) {
	slowest := int32(1000)
	var names []string
	for _, m := range c.C.Members {
		id := entity.EntityID(m)
		slowest = min(slowest, g.capacity(id, body.Moving))
		names = append(names, g.personName(id))
	}
	c.C.Slowest = max(slowest*int32(travelOf(g).Pace(c.C.ID, len(c.C.Members)))/1000, 1) // AX-9: slower when heavy
	c.C.FoodDays = caravanFoodDays
	c.C.State = caravan.Travelling
	c.NextTalk = now + caravanTalkPeriod
	g.letter(letterInfo, "Caravan departed", fmt.Sprintf("%s set out for %s with %d days of food.",
		travel.JoinNames(names), g.tileName(c.C.Dest), caravanFoodDays/1000), 0)
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

// stepCaravans advances every travelling caravan on the hour, ends visits,
// lets members talk and starve, and brings the caravans home.
func (g *Game) stepCaravans(now uint64) {
	w := g.worldSt
	hour := now%clock.TicksPerHour == 0
	for i := 0; i < len(w.caravans); i++ {
		c := &w.caravans[i]
		id := entity.EntityID(c.C.ID)
		switch c.C.State {
		case caravan.Forming:
			continue
		case caravan.Travelling, caravan.Returning:
			if !hour {
				break
			}
			caravan.Advance(&c.C, 1)
			if c.C.FoodDays == 0 {
				g.caravanStarve(c)
			}
			if c = g.caravanAt(id); c != nil && caravan.IsArrived(&c.C) {
				g.caravanArrived(c, now)
			}
		case caravan.Visiting:
			if now >= c.C.VisitUntil {
				g.startReturn(c)
			}
		}
		if c = g.caravanAt(id); c == nil {
			i--
			continue
		}
		if len(c.C.Members) == 0 {
			g.letter(letterCaution, "Caravan lost", "Nobody in the caravan survived the journey.", 0)
			w.caravans = slices.Delete(w.caravans, i, i+1)
			i--
			continue
		}
		if c.C.State == caravan.Home {
			w.caravans = slices.Delete(w.caravans, i, i+1)
			i--
			continue
		}
		if hour && c.C.State != caravan.Forming {
			g.caravanCare(c, now) // caravan_medical.go (WS-M): the medic on the road
		}
		if now >= c.NextTalk {
			c.NextTalk = now + caravanTalkPeriod
			g.caravanTalk(c, now)
		}
	}
}

// caravanArrived: c reached the end of its path: home, a friendly
// settlement (a day's visit, food restocked) or anywhere else (it waits
// for the order to return).
func (g *Game) caravanArrived(c *caravanRec, now uint64) {
	if c.Back {
		g.caravanHome(c)
		return
	}
	if until, ok := travelOf(g).Mine(c.C.ID, c.C.Tile, now); ok { // game/travel (AX-9): a mineral site
		c.C.State, c.C.VisitUntil = caravan.Visiting, until
		return
	}
	f := g.factionOf(g.settlementFaction(c.C.Tile))
	if f != nil && f.Type.Human() && f.Relation != faction.Hostile && !f.Defeated {
		c.C.State = caravan.Visiting
		c.C.VisitUntil = now + caravanVisitTicks
		c.C.FoodDays = caravanFoodDays
		g.letter(letterInfo, "Caravan arrived", fmt.Sprintf("The caravan reached %s and stays a day as their guests.", g.tileName(c.C.Tile)), 0)
		return
	}
	g.letter(letterInfo, "Caravan arrived", fmt.Sprintf("The caravan reached %s and waits there for your orders.", g.tileName(c.C.Tile)), 0)
}

// startReturn turns c for home along the way it came.
func (g *Game) startReturn(c *caravanRec) {
	k := slices.Index(c.Out, c.C.Tile)
	if k < 0 {
		k = 0
	}
	back := slices.Clone(c.Out[:k])
	slices.Reverse(back)
	c.C.Path, c.C.Progress, c.Back = back, 0, true
	c.C.State = caravan.Returning
	if len(back) == 0 {
		g.caravanHome(c)
	}
}

// cmdCaravanReturn sends caravan id home: a forming one is called off
// (its members come back in), one on the road turns round.
func (g *Game) cmdCaravanReturn(id entity.EntityID) {
	c := g.caravanAt(id)
	if c == nil {
		return
	}
	switch c.C.State {
	case caravan.Forming:
		for _, m := range slices.Clone(c.Exiting) {
			g.dropMember(c, m)
		}
		if len(c.C.Members) > 0 {
			g.caravanHome(c)
			g.dropCaravan(id)
			return
		}
		g.dropCaravan(id)
	case caravan.Travelling, caravan.Arrived, caravan.Visiting:
		g.startReturn(c)
		if c = g.caravanAt(id); c != nil && c.C.State == caravan.Home {
			g.dropCaravan(id)
		}
	}
}

// dropCaravan removes caravan id from the world.
func (g *Game) dropCaravan(id entity.EntityID) {
	w := g.worldSt
	w.caravans = slices.DeleteFunc(w.caravans, func(c caravanRec) bool { return c.C.ID == uint64(id) })
}

// caravanHome brings c's members back onto the map at an edge tile; they
// walk in towards the colony.
func (g *Game) caravanHome(c *caravanRec) {
	c.C.State = caravan.Home
	ex, ey, ok := g.worldEdge()
	if !ok {
		ex, ey = g.home()
	}
	hx, hy := g.home()
	var used [][2]int
	var names []string
	for i, m := range c.C.Members {
		id := entity.EntityID(m)
		if !g.isStashed(id) {
			continue
		}
		x, y, ok := g.freeLandNear(ex, ey, 4, used)
		if !ok {
			if x, y, ok = g.combat.Tiles.WalkableNear(ex, ey, 6); !ok {
				x, y = hx, hy
			}
		}
		used = append(used, [2]int{x, y})
		if !g.unstashPawn(id, x, y) {
			continue
		}
		names = append(names, g.personName(id))
		o := ringOffsets[i%len(ringOffsets)]
		if tx, ty, ok := g.combat.Tiles.WalkableNear(hx+o[0]/2, hy+o[1]/2, 3); ok {
			g.cmdMove(id, tx, ty)
		}
	}
	travelOf(g).Home(c.C.ID, ex, ey) // game/travel (AX-9): the pods' cargo
	if len(names) > 0 {
		g.letter(letterInfo, "Caravan returned", travel.JoinNames(names)+" came back from the journey.", entity.EntityID(c.C.Members[0]))
	}
}

// caravanStarve: with the food gone, members go hungry at the Fed rate;
// at 0 % malnutrition sets in (stepMalnutrition, FOOD-1).
func (g *Game) caravanStarve(c *caravanRec) {
	for _, m := range c.C.Members {
		n, _ := g.need.Get(entity.EntityID(m))
		n.Hunger = max(n.Hunger-needs.HungerPerHour, 0)
		g.need.Set(entity.EntityID(m), n.Settle())
	}
}

// dieAway kills caravan member id off the map: no corpse, its weapon lost
// with it; the colony grieves as for any death.
func (g *Game) dieAway(id entity.EntityID, cause health.Cause) {
	c := g.caravanOf(id)
	delete(g.equip, id)
	g.setWhere(id, whereColony) // the colony's death (its history line), not a stranger's
	g.dieBy(id, cause, 0)
	g.dropStash(id)
	if c != nil {
		c.C.Members = slices.DeleteFunc(c.C.Members, func(m uint64) bool { return m == uint64(id) })
	}
	g.letter(letterCaution, "Death on the road", g.personName(id)+" died on the caravan's journey.", 0)
}

// caravanTalk pairs two members up for a conversation on the road.
func (g *Game) caravanTalk(c *caravanRec, now uint64) {
	var able []entity.EntityID
	for _, m := range c.C.Members {
		if id := entity.EntityID(m); g.alive(id) {
			able = append(able, id)
		}
	}
	if len(able) < 2 {
		return
	}
	r := g.stream(streamWorld)
	i, k2 := r.IntN(len(able)), r.IntN(len(able)-1)
	if k2 >= i {
		k2++
	}
	a, b := able[i], able[k2]
	if g.capacity(a, body.Talking) <= 0 || g.capacity(b, body.Hearing) <= 0 {
		return
	}
	k := social.Chitchat
	if r.IntN(100) < caravanDeepTalkPct {
		k = social.DeepTalk
	}
	g.socState()
	g.ensurePerson(a)
	g.ensurePerson(b)
	g.applyInteraction(a, b, k, now) // social.go
	for _, id := range [2]entity.EntityID{a, b} {
		g.joy[id] = min(g.joy[id]+caravanTalkJoy, recreation.Full)
	}
}

// worldEdge is a walkable map-edge tile from which the colony can be
// reached (walls and doors broken if need be), drawn from the world
// stream: what edgeTile is for raids, without touching g.rng.
func (g *Game) worldEdge() (int, int, bool) {
	hx, hy := g.home()
	if x, y, ok := g.combat.Tiles.WalkableNear(hx, hy, 6); ok {
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
	if w == nil || len(w.caravans) == 0 && len(w.stashed) == 0 && len(w.visitors) == 0 && len(w.areaOf) == 0 && len(w.huntOut) == 0 && w.ax9.Empty() {
		return nil
	}
	s := &worldSave{Version: worldSaveVersion, Stashed: slices.Clone(w.stashed), Visitors: slices.Clone(w.visitors)}
	for _, c := range w.caravans {
		s.Caravans = append(s.Caravans, c.clone())
	}
	for id, a := range w.areaOf {
		s.AreaOf = append(s.AreaOf, areaAssign{Pawn: id, Area: a})
	}
	slices.SortFunc(s.AreaOf, func(a, b areaAssign) int { return cmp.Compare(a.Pawn, b.Pawn) })
	s.HuntOut, s.AX9 = exportHuntOut(w.huntOut), w.ax9.Export() // area_dest.go, game/travel
	return s
}

// clone is a deep copy of c.
func (c caravanRec) clone() caravanRec {
	c.C.Members = slices.Clone(c.C.Members)
	c.C.Animals = slices.Clone(c.C.Animals)
	c.C.Path = slices.Clone(c.C.Path)
	c.Out = slices.Clone(c.Out)
	c.Exiting = slices.Clone(c.Exiting)
	return c
}

// importWorld restores exportWorld's state (nil: fresh).
func (g *Game) importWorld(s *worldSave) {
	g.worldSt = nil
	if s == nil {
		return
	}
	w := g.worldS()
	for _, c := range s.Caravans {
		w.caravans = append(w.caravans, c.clone())
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
func (h travelHost) ColonyTile() int                           { return h.g.colonyTile() }
func (h travelHost) Home() (int, int)                          { return h.g.home() }
func (h travelHost) Caravan(id uint64) bool                    { return h.g.caravanAt(entity.EntityID(id)) != nil }
func (h travelHost) Faction(t int) (*faction.Faction, string) {
	return h.g.factionOf(h.g.settlementFaction(t)), h.g.tileName(t)
}
func (h travelHost) PutAnimal(id entity.EntityID, a animal.State, x, y int) {
	putAnimal(h.g, id, a, x, y)
}
func (h travelHost) Sicken(id entity.EntityID)         { cryptoSicken(h.g, id) }
func (h travelHost) Explode(x, y int, d explosion.Def) { h.g.combat.Blasts.Explode(0, 0, x, y, d) }

// Land: pod passengers become a caravan arrived on t (home by land).
func (h travelHost) Land(members, animals []uint64, t int) uint64 {
	g, w, home := h.g, h.g.worldS(), h.g.colonyTile()
	c := caravanRec{C: caravan.Caravan{ID: uint64(g.entities.Next()), Members: members, Animals: animals, Tile: t, Dest: t,
		State: caravan.Arrived, FoodDays: caravanFoodDays}, Home: home, Out: g.worldPlanet().Route(home, t)}
	for i, r := range w.stashed {
		if slices.Contains(members, uint64(r.ID)) {
			w.stashed[i].Holder = entity.EntityID(c.C.ID)
			g.setWhere(r.ID, whereCaravan)
		}
	}
	w.caravans = append(w.caravans, c)
	g.caravanArrived(&w.caravans[len(w.caravans)-1], uint64(g.clock.Now()))
	return c.C.ID
}
