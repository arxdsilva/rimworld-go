package game

import (
	"fmt"

	"rimworld-go/internal/application/game/books"
	"rimworld-go/internal/application/game/combat"
	"rimworld-go/internal/application/game/letters"
	"rimworld-go/internal/application/game/mind"
	"rimworld-go/internal/application/game/raid"
	"rimworld-go/internal/application/game/spatial"
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/combat/kit"
	"rimworld-go/internal/domain/combat/weapon"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/trade"
	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/pawn/body"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/pawn/needs"
	raiddef "rimworld-go/internal/domain/raid"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/mapgen"
	"rimworld-go/internal/domain/world/roof"
	"rimworld-go/internal/domain/world/terrain"
	"rimworld-go/pkg/randx"
	"slices"
)

// Threats. The raid package (raid/) runs the storyteller's die, raid
// plans, the raider AI, predator packs and hive swarms; the game keeps
// the raider records and raid groups (prisoners, visitors and factions
// write them too), spawns raids and answers raid's roles (raid_host.go).

const firstRaidDay = 5

var raiderNames = []string{"Krell", "Vasha", "Dorn", "Mira", "Tobias", "Ilse", "Grit", "Sable", "Hask", "Rook", "Nessa", "Brann"}

// isRaider reports a pawn of another faction on the map: raiders and the
// colony's prisoners (they path like raiders, so doors hold them), not a
// released prisoner walking out.
func (g *Game) isRaider(id entity.EntityID) bool {
	r, ok := g.raiders[id]
	return ok && !r.Leaving
}

// standingRaider is a hostile raider still on its feet (never a
// prisoner), or a colonist on its feet in a berserk or murderous break:
// colonists may attack it (SIM-066).
func (g *Game) standingRaider(id entity.EntityID) bool {
	v, ok := g.vital.Get(id)
	return ok && v.Status == health.Alive && (g.isRaider(id) && !g.prison.Is(id) || mind.Hostile(g.breaks[id].kind))
}

// fireStoryteller fires the due threat at the colony it strikes (MAP-12).
// MAP-0: one storyteller clock for the colony, on the first map's pass.
func fireStoryteller(g *Game, now uint64) {
	cur := g.mapState
	g.mapState = threatTarget(g) // world_state.go
	g.fireThreat(now)
	g.mapState = cur
}

// phaseThreats runs every raider, after the setup's story and the
// storyteller (phaseSystems).
func (g *Game) phaseThreats() {
	now := uint64(g.clock.Now())
	g.raid.Pods.Step(now)                          // raid/pods.go (DEF-4): drop pods land
	ids := append(g.raiderBuf[:0], g.raiderIDs...) // the list changes as raiders die or leave
	for _, id := range ids {
		g.raid.Raiders.Think(id, now) // raid/raiders.go
	}
	g.raiderBuf = ids[:0]
	g.combat.Turrets.Phase(now) // combat/turret.go (PLAY-024): after the raiders move
	g.combat.Shots.Step()       // combat/shot.go (DEF-1): projectiles in flight
	g.vaultTick(now)            // vault.go (MAPGEN-3): approach warnings, clearing
}

// scheduleThreats starts the storyteller (colonies, not bare test maps).
func (g *Game) scheduleThreats() {
	if g.hasSetup() {
		g.scheduleStory(uint64(g.clock.Now())) // story.go: the chosen storyteller (NG-1)
		return
	}
	g.threatsOn = true
	g.nextThreat = uint64(firstRaidDay*clock.TicksPerDay + g.rng.IntN(clock.TicksPerDay))
}

func (g *Game) fireThreat(now uint64) {
	if g.hasSetup() {
		g.fireStoryThreat(now) // story.go: storyteller, difficulty, factions (NG-1)
		return
	}
	g.nextThreat = g.raid.Threats.Fire(now) // raid/threats.go
}

// edgeTile is a map-edge tile the colony can be reached from
// (raid/edge.go).
func (g *Game) edgeTile() (int, int, bool) { return g.raid.Pods.EdgeTile() }

// Raid sends n raiders in from one map edge to assault the colony (also
// a dev hook).
func (g *Game) Raid(n int) {
	g.viewGen++ // view cache (indexes.go): this hook changes the state
	g.raidWith(g.kitOf(n), raid.Assault)
}

// raidWith sends one raider per weapon in kit, following plan.
func (g *Game) raidWith(kit []item.Kind, plan raid.Plan) { g.raidFrom(kit, plan, 0) }

// raidFrom is raidWith for the faction fac (1 + index into the colony's
// factions; 0: unattributed), settled by raid/threats.go (DEF-4).
func (g *Game) raidFrom(kit []item.Kind, plan raid.Plan, fac int) {
	f := g.factionOf(fac) // factions.go: nil for a legacy raid
	grp, tech := raid.Group{Size: len(kit), Plan: plan}, faction.Industrial
	if f != nil {
		grp.Faction, tech = fac, faction.DefOf(f.Type).Tech
	}
	g.raid.Threats.Settle(&grp, tech, g.raidPoints(int(uint64(g.clock.Now())/clock.TicksPerDay)))
	if grp.Arrival.DropPod() && len(kit) > 0 {
		podRaid(g, grp, kit, f)
		return
	}
	ex, ey, ok := g.edgeTile()
	n := len(kit)
	if !ok || n <= 0 {
		return
	}
	if grp.Plan == raid.Siege {
		grp.CampX, grp.CampY = g.raid.Raiders.Camp(ex, ey)
	}
	g.raidGroups = append(g.raidGroups, grp)
	group := len(g.raidGroups) - 1
	spawned := 0
	// relative_world.go: now and then one raider is a colonist's relative
	wantKin := f != nil && g.relativeRoll(relativeRaidPermille)
	var kin entity.EntityID
	for i := 0; i < n*6 && spawned < n; i++ {
		x, y := ex+g.rng.IntN(5)-2, ey+g.rng.IntN(5)-2
		if !g.grid.InBounds(x, y) || !g.dryWalkable(x, y) {
			continue
		}
		if wantKin {
			wantKin = false
			kin = g.relativeFor(fac, whereRaider, techTag(f))
			spawnRaider(g, kin, group, kit[spawned], x, y)
		} else {
			spawnRaider(g, 0, group, kit[spawned], x, y)
		}
		spawned++
	}
	g.raidGroups[group].Size = spawned
	if spawned == 0 {
		g.raidGroups[group].Over = f != nil // nothing to repel: no goodwill lost
		return
	}
	g.combat.Log.Add(fmt.Sprintf("A raid of %d came: %s.", spawned, grp.Plan)) // combat/log.go (DEF-4)
	if f != nil {
		if kin != 0 { // relative_world.go: before the raid alert, which stays the latest message
			g.relativeLetter(letters.Caution, "Relative among the raiders", kin, "raiders")
		}
		g.raidMessage(ex, ey, spawned, grp.Plan, f) // story.go
		return
	}
	title, text := raid.Warning(grp.Plan, spawned)
	g.threat(title, ex, ey, "%s", text)
}

// spawnRaider puts a raider of group with k on (x, y) (id 0: a new one).
func spawnRaider(g *Game, id entity.EntityID, group int, k item.Kind, x, y int) entity.EntityID {
	fac := g.raidGroups[group].Faction
	f, known := g.factionOf(fac), id != 0
	if known {
		g.raiderIDs = withID(g.raiderIDs, id) // a known person's id may be older
	} else {
		id = g.entities.Next()
		g.raiderIDs = append(g.raiderIDs, id)
	}
	g.positions.Set(id, Position{X: x, Y: y})
	switch {
	case known:
		g.names.Set(id, bioName(g.personOf(id).Bio))
	case f != nil:
		g.names.Set(id, g.raiderName(id, f)) // story.go
	default:
		g.names.Set(id, raiderNames[g.rng.IntN(len(raiderNames))])
	}
	g.jobs.Set(id, job.Record{})
	g.need.Set(id, needs.NewFull())
	g.vital.Set(id, health.Record{Status: health.Alive})
	g.equip[id] = k
	g.apparel[id] = combat.KitOutfit(kit.Armor(k, kit.TechOf(f), uint64(id))) // CBT-3: flak and helmets by tech and points
	g.raiders[id] = raid.Raider{Group: group, Arrived: uint64(g.clock.Now())}
	if !known {
		g.raiderPerson(id, fac) // people.go: a world person (relations, capture)
	}
	return id
}

// podRaid sends group grp in drop pods (raid/pods.go); raiders spawn as
// the pods land.
func podRaid(g *Game, grp raid.Group, kit []item.Kind, f *faction.Faction) {
	grp.PodFaction, grp.Faction = grp.Faction, 0
	g.raidGroups = append(g.raidGroups, grp)
	gi := len(g.raidGroups) - 1
	x, y, n := g.raid.Pods.Drop(gi, kit, grp.Arrival, uint64(g.clock.Now()))
	g.raidGroups[gi].Size = n
	if n == 0 {
		g.raidGroups[gi].Over = f != nil
		return
	}
	who := ""
	if f != nil {
		who = f.Name
	}
	text := raid.Announce(grp.Arrival, n, who)
	g.combat.Log.Add(text)
	g.threat("Drop pods", x, y, "%s", text)
}

func nearestEdge(p Position, w, h int) (int, int) { return raid.NearestEdge(p.X, p.Y, w, h) }

// dropRaiderGear leaves a dead pawn's (raider's, visitor's or colonist's)
// weapon where it fell: a stranger's is found, not the colony's
// (haul_claim.go); outside the home area it starts forbidden (game/allow).
func (g *Game) dropRaiderGear(id entity.EntityID) {
	pos, _ := g.positions.Get(id)
	out := forbids(g).DropForbidden(pos.X, pos.Y)
	if r := g.raiders[id]; r.LootQty > 0 { // a thief's haul stays where it fell (DEF-4)
		g.dropGoods(trade.Line{Kind: r.Loot, Qty: r.LootQty}, pos.X, pos.Y, out)
		r.Loot, r.LootQty = 0, 0
		g.raiders[id] = r
	}
	if k, ok := g.equip[id]; ok {
		switch {
		case g.isRaider(id):
			g.spawnUnclaimed(k, 1, pos.X, pos.Y, out)
		case g.personOf(id) != nil && g.personOf(id).Where == whereVisitor: // a visitor's gear is not the colony's
			g.spawnUnclaimed(k, 1, pos.X, pos.Y, out)
		default:
			st := g.combat.Gear.Release(id, k, g.combat.Drafts.Drop(id, k)) // a grenade belt whole, the make kept
			st.Forbidden = out
			g.dropStackOf(st, pos.X, pos.Y)
		}
		delete(g.equip, id)
	}
	g.combat.Drafts.DropSpare(id, out) // combat/draft.go (CBT-2): the spare weapon falls too
	if k, ok := books.RaidLoot(g.streams.seed, uint64(id)); ok && g.isRaider(id) {
		g.spawnUnclaimed(k, 1, pos.X, pos.Y, out) // game/books (AX-7): a book a raider carried
	}
}

// removeRaider takes raider (or visitor, prisoner) id off the map. A
// raider who got away is an off-map person again, its relations kept;
// whatever it carried is put down first.
func (g *Game) removeRaider(id entity.EntityID) {
	senseOf(g).Gear.Spill(id) // game/sense (CSE-036)
	if p := g.personOf(id); p != nil && p.Where == whereRaider {
		p.Where = whereOffMap
	}
	g.care.DropPatient(id)
	g.reserve.ReleaseOwner(id)
	g.positions.Delete(id)
	g.names.Delete(id)
	g.jobs.Delete(id)
	g.need.Delete(id)
	g.vital.Delete(id)
	delete(g.equip, id)
	delete(g.raiders, id)
	delete(g.logs, id)
	kept := g.raiderIDs[:0]
	for _, rid := range g.raiderIDs {
		if rid != id {
			kept = append(kept, rid)
		}
	}
	g.raiderIDs = kept
}

// raiderAt is the raider standing (or lying) on a tile, else a colonist
// there in a berserk or murderous break (an attack target, SIM-066).
func (g *Game) raiderAt(x, y int) entity.EntityID {
	for _, id := range g.raiderIDs {
		if p, ok := g.positions.Get(id); ok && p.X == x && p.Y == y {
			return id
		}
	}
	for _, id := range g.pawnIDs {
		if p, ok := g.positions.Get(id); ok && p.X == x && p.Y == y && mind.Hostile(g.breaks[id].kind) {
			return id
		}
	}
	return 0
}

// raiderViews are the raiders for the read model.
func (g *Game) raiderViews() []PawnView {
	out := make([]PawnView, 0, len(g.raiderIDs))
	for _, id := range g.raiderIDs {
		if g.prison.Is(id) || g.raiders[id].Leaving {
			out = append(out, g.prisonerPawnView(id)) // prisoners.go
			continue
		}
		pos, _ := g.positions.Get(id)
		name, _ := g.names.Get(id)
		j, _ := g.jobs.Get(id)
		v, _ := g.vital.Get(id)
		plan := ""
		if gi := g.raiders[id].Group; gi < len(g.raidGroups) {
			plan = g.raidGroups[gi].Plan.String()
		}
		out = append(out, PawnView{ID: id, Name: name, X: pos.X, Y: pos.Y, Job: j.Line(), RaidPlan: plan,
			Status: v.Status.String(), Reason: v.Cause.String(), Health: v.HP() * 100 / health.MaxHP,
			Weapon: weaponDefOf(g, id).Name, Lying: v.Status != health.Alive, Hostile: true, Apparel: g.Wearing(id), Worn: g.apparel[id].Views(), ApparelArmor: g.apparel[id].Armor(),
			Fleeing: g.raiders[id].Mode == raid.Fleeing})
		g.fillPawnAction(&out[len(out)-1], id, j)
	}
	return out
}

// Raiders counts raiders on the map (standing ones and all), not the
// colony's prisoners.
func (g *Game) Raiders() (standing, total int) {
	for _, id := range g.raiderIDs {
		if g.prison.Is(id) || g.raiders[id].Leaving {
			continue
		}
		total++
		if g.standingRaider(id) {
			standing++
		}
	}
	return
}

// Raid sizes and plans. The storyteller weighs the colony (its wealth,
// its colonists and how long it has stood) into raid points, and spends
// them on raiders: clubs are cheap, rifles dear. Most raids assault head
// on; later ones may lay siege (dig in behind sandbags out of reach and
// shoot at whoever comes close, then storm in) or, against a colony shut
// behind walls and rock, send sappers who tunnel straight through and
// step around traps.

const (
	maxScaledRaiders = 8
	// The colony weapons' raid prices (domain combat/kit keeps them).
	clubCost, knifeCost, spearCost, rifleCost = 60, kit.Cheapest, 70, 90
)

// Wealth is the colony's worth: goods on the map, buildings and tame
// animals.
func (g *Game) Wealth() int {
	w := 0
	for _, id := range g.itemIDs {
		if st, ok := g.stacks.Get(id); ok {
			w += st.Kind.Value() * st.Qty
		}
	}
	for _, id := range g.bldgIDs {
		if b, ok := g.bldgs.Get(id); ok && !notColonyPiece(b) { // ruins and vents are no one's wealth
			v := construction.WoodCost(b.Kind)*2 + construction.WorkTicks(b.Kind)/20
			if item.IsStoneBlocks(b.Stuff) || b.Stuff == item.Stone {
				v = v * 3 / 2
			}
			w += v
		}
	}
	for _, id := range g.animalIDs {
		if a, ok := g.animals.Get(id); ok && a.Tame {
			w += animal.DefOf(a.Species).MaxHP * 2
		}
	}
	w += senseOf(g).Gear.Wealth() // game/sense (CSE-036): what its colonists carry
	return w
}

// raidPoints is what the storyteller may spend on a raid on this day.
func (g *Game) raidPoints(day int) int {
	if g.hasSetup() {
		return g.threatPoints() // story.go: difficulty.ThreatPoints (NG-1 §3.3)
	}
	colonists := 0
	for _, id := range g.pawnIDs {
		if g.standingPawn(id) {
			colonists++
		}
	}
	return g.Wealth()/40 + colonists*30 + day*2
}

// raidKit spends points on raiders' weapons (at least one raider); Of for
// a faction of tech, at most maxRaiders (domain combat/kit, DEF-2).
func (g *Game) raidKit(points int) []item.Kind { return g.raidKitOf(points, faction.Industrial) }

func (g *Game) raidKitOf(points int, tech faction.Tech) []item.Kind {
	return kit.Pick(points, g.maxRaiders(), tech, g.rng.IntN)
}

// scaledRaid is the storyteller's raid: sized by raid points, with a plan
// that suits the colony.
func (g *Game) scaledRaid(day int) {
	kit := g.raidKit(g.raidPoints(day))
	g.raidWith(kit, g.raid.Threats.Plan(day)) // raid/threats.go
}

// enclosed reports that no colonist can be reached from the map edge
// without breaking through something.
func (g *Game) enclosed() bool {
	ex, ey, ok := g.edgeTile()
	if !ok {
		return false
	}
	hm := g.hostileMap()
	for _, id := range g.pawnIDs {
		if !g.standingPawn(id) {
			continue
		}
		p, _ := g.positions.Get(id)
		if _, ok := g.searcher().Find(hm, ex, ey, p.X, p.Y); ok {
			return false
		}
	}
	return true
}

// Siege sends a siege raid of n raiders (dev hook).
func (g *Game) Siege(n int) {
	g.viewGen++ // view cache (indexes.go): this hook changes the state
	g.raidWith(g.kitOf(n), raid.Siege)
}

// Sappers sends n sappers (dev hook).
func (g *Game) Sappers(n int) {
	g.viewGen++ // view cache (indexes.go): this hook changes the state
	g.raidWith(g.kitOf(n), raid.Sappers)
}

func (g *Game) kitOf(n int) []item.Kind { return kit.Legacy(n, g.rng.IntN) }

// sapperMap is the map as sappers see it when planning a tunnel: walls,
// doors and rock all give way; traps are stepped around.
func sapperBreaks(g *Game) tileSet {
	return g.sapperSet.get(g, g.structVer, func() tileSet {
		set := slices.Clone(g.buildingCells(construction.Wall, construction.Door, construction.Autodoor, construction.OrnateDoor, construction.AnimalFlap))
		w := g.grid.Width()
		for y := 0; y < g.grid.Height(); y++ {
			for x := 0; x < w; x++ {
				if terrain.IsRockWall(g.terr.At(x, y)) {
					set.add(y*w + x)
				}
			}
		}
		return set
	})
}

// sapper reports a raider of a sapper raid.
func (g *Game) sapper(id entity.EntityID) bool {
	r, ok := g.raiders[id]
	return ok && r.Group < len(g.raidGroups) && g.raidGroups[r.Group].Plan == raid.Sappers
}

// sapperMap avoids traps on top of the doors rule.
type sapperMap struct {
	hostileMap
	traps tileSet
}

func (s sapperMap) Walkable(x, y int) bool {
	return s.hostileMap.Walkable(x, y) && !s.traps.has(y*s.g.grid.Width()+x)
}

// HiveSwarm is the dev hook for a hive swarm (raid/hive.go).
func (g *Game) HiveSwarm() {
	g.viewGen++ // view cache (indexes.go): this hook changes the state
	g.raid.Hives.Swarm()
}

// --- DEF-4: the raid package's extra roles (raid/roles.go) ---

type raidArrive struct{ g *Game }

func (h raidArrive) PodSpot(x, y int) bool { return h.g.podFree(x, y, nil) } // arrival.go
func (h raidArrive) Reach(x, y, tx, ty int) bool {
	bm := breachMap{g: h.g, breaks: h.g.buildingCells(construction.Wall, construction.Door, construction.Autodoor, construction.OrnateDoor, construction.AnimalFlap)}
	_, ok := h.g.searcher().Find(bm, x, y, tx, ty)
	return ok
}

func (h raidArrive) Land(gi int, p raid.Pod) entity.EntityID {
	if x, y, ok := h.Crash(p.X, p.Y); ok {
		return spawnRaider(h.g, 0, gi, p.Kit, x, y)
	}
	return 0
}
func (h raidArrive) Crash(px, py int) (int, int, bool) {
	g := h.g
	if c := g.roofAt(px, py); c.Roofed() && !c.Thick() {
		g.setRoof(py*g.grid.Width()+px, c&^(roof.Built|roof.Natural|roofPlans))
		g.roofCollapses = append(g.roofCollapses, roofCollapse{x: px, y: py, tick: uint64(g.clock.Now())})
		for _, s := range (combatRosters{g}).Near(px, py, 1, combat.Everyone, nil) {
			g.hurtAs(s.ID, raiddef.RoofCrush, 0, body.Bruise, health.CauseInjury)
		}
		g.message(px, py, "A drop pod crashed through the roof.")
	}
	g.BuildNow(construction.DefOf(construction.PodShell).Name, item.Steel.String(), "south", px, py)
	return g.freeLandNear(px, py, 3, nil) // arrival.go
}

// raidMischief is the raiders' thieving and burning; wood is the burner's
// building grid, kept across ticks.
type raidMischief struct {
	g    *Game
	wood *woodIndex
}

const stealMin = 50 // silver; original tuning (wiki: rises with raid points)

// Loot is the nearest stack worth stealing (below, EAI-023).
func (h raidMischief) Loot(x, y, r int) (entity.EntityID, int, int, bool) { return loot(h.g, x, y, r) }
func (h raidMischief) Steal(id, stack entity.EntityID) (item.Kind, int, bool) {
	st, ok := h.g.stacks.Get(stack)
	if !ok || st.Carrier != 0 {
		return 0, 0, false
	}
	h.g.deleteItem(stack)
	return st.Kind, st.Qty, true
}

// Flammable is the nearest unburning wooden colony building
// (below, EAI-023).
func (h raidMischief) Flammable(x, y, r int) (int, int, bool) { return flammable(h.g, h.wood, x, y, r) }
func (h raidMischief) Ignite(x, y int) bool                   { return h.g.ignite(x, y, burstFireSize) } // fire.go
func (h raidMischief) Rival(fa, fb int) bool {
	a, b := h.g.factionOf(fa), h.g.factionOf(fb)
	return fa != fb && a != nil && b != nil && a.Relation == faction.Hostile && b.Relation == faction.Hostile
}

type raidSiege struct{ g *Game }

func (h raidSiege) BuildMortar(x, y int) entity.EntityID {
	if !h.g.BuildNow("mortar", "steel", "south", x, y) {
		return 0
	}
	id := h.g.layerBldgAt(x, y, construction.LayerObject)
	if id != 0 {
		h.g.combat.Mortars.Raise(id) // combat/mortar.go
	}
	return id
}
func (h raidSiege) Crew(m entity.EntityID) (entity.EntityID, bool) { return h.g.combat.Mortars.Crew(m) }
func (h raidSiege) Man(id, m entity.EntityID) bool                 { return h.g.combat.Mortars.Man(id, m) }

type raidChronicle struct{ g *Game }

func (h raidChronicle) Log(text string) { h.g.combat.Log.Add(text) }

// CombatEventView is one combat log line (WorldView.CombatLog, DEF-4).
type CombatEventView struct {
	Tick int
	Text string
}

// fillRaidView adds the combat log and the hostile drop pods to the view.
func fillRaidView(g *Game, w *WorldView) {
	for _, e := range g.combat.Log.Entries() {
		w.CombatLog = append(w.CombatLog, CombatEventView{Tick: int(e.Tick), Text: e.Text})
	}
	for _, p := range g.raid.Pods.Views(nil) {
		w.Arrivals = append(w.Arrivals, ArrivalView{X: p.X, Y: p.Y, Tick: p.Land, Landed: p.Landed, Hostile: true})
	}
}

// sapperTraps: sappers step around every spike trap and the buried
// charges they know of (wiki: Spike trap, about 30 %; fixed per charge).
func sapperTraps(g *Game) tileSet {
	return g.cellSet([4]uint64{3: 1 << 62}).get(g, g.structVer, func() tileSet {
		set := slices.Clone(g.buildingCells(construction.SpikeTrap))
		w := g.grid.Width()
		for _, id := range g.bldgIDs {
			if b, ok := g.bldgs.Get(id); ok && b.Kind.IsCharge() && (uint64(id)*0x9E3779B97F4A7C15)>>32%100 < sapperKnowPct {
				set.add(b.Y*w + b.X)
			}
		}
		return set
	})
}

const sapperKnowPct = 30 // wiki: Spike trap

type combatArms struct{ g *Game }

func (h combatArms) Disarm(id entity.EntityID)             { delete(h.g.equip, id) }
func (h combatArms) Wielded(id entity.EntityID) item.Kind  { return h.g.equip[id] }
func (h combatArms) Wield(id entity.EntityID, k item.Kind) { h.g.equip[id] = k }
func (h combatArms) Undraft(id entity.EntityID)            { h.g.setDraft(id, false) } // CBT-2: combat.Drafting
func (h combatArms) DropAt(st item.Stack)                  { h.g.dropStackOf(st, st.X, st.Y) }
func (h combatArms) Belt(id entity.EntityID) (item.Kind, uint8) { // CBT-3: combat.Belts
	return h.g.apparel[id].On[item.Belt].Kind, h.g.apparel[id].On[item.Belt].Quality
}

// mapsSettle is game/maps' Settler (maps/settle.go, MAP-05).
type mapsSettle struct{ g *Game }

// AttackReason is why caravan id cannot attack its tile's settlement ("" it can).
func (g *Game) AttackReason(id uint64) string {
	return fleetOf(g).Life().AttackReason(entity.EntityID(id))
}

func (h mapsSettle) Factions() []faction.Faction { return h.g.ng.factions }
func (h mapsSettle) Day() int                    { return h.g.relDay(uint64(h.g.clock.Now())) + 1 }
func (h mapsSettle) ThreatScale() int {
	if !h.g.hasSetup() {
		return 100
	}
	return h.g.ng.setup.Difficulty.ThreatScale
}
func (h mapsSettle) Defenders() int { return h.g.raid.Raiders.Defending() }

// Defending is game/orders' count of a settlement's defenders still up
// (Claim, MAP-17).
func (h ordersHost) Defending() int { return h.g.raid.Raiders.Defending() }

// Settle lays compound c as placeRuin lays a ruin (Ancient: nobody's
// wealth, no blueprint), beds forbidden, goods free to pack, with a lane
// from the south gate to the map edge, and spawns the defenders.
func (h mapsSettle) Settle(c mapgen.Compound, fac int, arms []item.Kind, seed uint64) {
	g, rng := h.g, h.g.rng
	g.rng = randx.New(seed)
	defer func() { g.rng = rng }()
	for y := c.Y0 - 2; y <= c.Y1+2; y++ {
		for x := c.X0 - 2; x <= c.X1+2; x++ {
			clearSite(g, x, y)
		}
	}
	for y := c.Y1 + 3; y < g.grid.Height(); y++ {
		clearSite(g, c.Centre.X, y)
	}
	for _, p := range c.Pieces {
		g.placeGenerated(construction.Building{Kind: p.Kind, Stuff: p.Stuff, Facing: p.Facing, X: p.X, Y: p.Y, Ancient: true, Forbidden: p.Kind.Sleeps(), Hostile: p.Kind == construction.SentryTurret || p.Kind.IsDoor()})
	}
	for _, l := range c.Loot {
		g.spawnUnclaimed(l.Kind, l.Qty, l.X, l.Y, true) // like vault loot: found, forbidden; the pack-up takes it (mapsPop.PackUp)
	}
	g.raidGroups = append(g.raidGroups, raid.Group{Plan: raid.Defend, Faction: fac, Over: true, FleeLoss: raid.DefendFleeLoss, CampX: c.Centre.X, CampY: c.Centre.Y, Camped: true})
	gi := len(g.raidGroups) - 1
	for i, k := range arms {
		p := c.Posts[i%len(c.Posts)]
		if x, y, ok := g.combat.Tiles.WalkableNear(p.X, p.Y, 3); ok {
			spawnRaider(g, 0, gi, k, x, y)
			g.raidGroups[gi].Size++
		}
	}
}

// clearSite makes cell (x, y) open dry ground, unroofed, with nothing on it.
func clearSite(g *Game, x, y int) {
	if !g.grid.InBounds(x, y) {
		return
	}
	if t := g.terr.At(x, y); !t.Walkable() || terrain.IsWater(t) || t == terrain.Marsh || t == terrain.Mud {
		g.setTerrain(x, y, terrain.Soil)
	}
	g.clearGrass(x, y)
	if id := g.plantAt(x, y); id != 0 {
		g.removePlant(id)
	}
	for _, l := range [2]construction.Layer{construction.LayerObject, construction.LayerFloor} {
		if id := g.layerBldgAt(x, y, l); id != 0 {
			g.removeBuildingAs(id, false)
		}
	}
	for id := g.itemAt(x, y); id != 0; id = g.itemAt(x, y) {
		g.deleteItem(id)
	}
	if i := y*g.grid.Width() + x; len(g.roofs) > 0 {
		g.setRoof(i, g.roofs[i]&roof.Supported) // derived: the room rebuild keeps it (a load derives it again)
	}
}

// Raider look-ups through bucket grids instead of whole-map scans
// (EAI-021, EAI-023): the nearest standing colonist through the creature
// index, a thief's loot through the food index's item grid, a burner's
// mark through the wooden-building grid below. Each search goes ring by
// ring, a bucket wide and doubling, so a near hit looks at a few buckets;
// ties go to the lower id, as the creature index's nearestWhere.

// ringPawns: with more colonists than this the nearest one is found ring
// by ring; with fewer the list is shorter than the rings (original
// tuning). ringFar: past this radius the list is walked.
const (
	ringPawns = 32
	ringFar   = 64
)

// nearestColonist is the closest standing colonist to p (ties: the lower
// id).
func (g *Game) nearestColonist(p Position) (entity.EntityID, Position, bool) {
	if len(g.pawnIDs) > ringPawns {
		return nearestColonistRings(g, p)
	}
	return nearestColonistScan(g, p)
}

// nearestColonistRings is nearestColonist through the creature index.
func nearestColonistRings(g *Game, p Position) (entity.EntityID, Position, bool) {
	for r := spatial.BucketSize; r <= ringFar; r *= 2 {
		if id, cp, ok := g.nearestWhere(p.X, p.Y, r, ckPawn, func(o entity.EntityID, _ Position) bool { return g.standingPawn(o) }); ok {
			return id, cp, true
		}
	}
	return nearestColonistScan(g, p)
}

// nearestColonistScan is nearestColonist over the colonist list.
func nearestColonistScan(g *Game, p Position) (entity.EntityID, Position, bool) {
	var best entity.EntityID
	var bp Position
	bestD := 1 << 30
	for _, id := range g.pawnIDs {
		if !g.standingPawn(id) {
			continue
		}
		cp, _ := g.positions.Get(id)
		if d := weapon.Distance(p.X, p.Y, cp.X, cp.Y); d < bestD || d == bestD && id < best {
			best, bp, bestD = id, cp, d
		}
	}
	return best, bp, best != 0
}

// loot is the nearest stack worth stealing within r of (x, y), through
// the food index's item grid (food: rebuilt once a tick).
func loot(g *Game, x, y, r int) (entity.EntityID, int, int, bool) {
	return spatial.Nearest(&g.food().items, x, y, r, func(id entity.EntityID) (int, int, bool) {
		st, ok := g.stacks.Get(id)
		return st.X, st.Y, ok && worthStealing(st)
	})
}

// worthStealing: a loose stack a thief takes (stealMin silver or more).
func worthStealing(st item.Stack) bool {
	return st.Carrier == 0 && st.Qty > 0 && st.Kind.Value()*st.Qty >= stealMin
}

// woodIndex is the grid of the wooden buildings (what a burner sets
// alight; whether one is the colony's is asked live), rebuilt when the
// structure version, the building store, the map size or the building
// count moved; builds counts rebuilds (tests). Not saved.
type woodIndex struct {
	grid        spatial.Grid[entity.EntityID]
	ok          bool
	ver, builds uint64
	src         any
	w, h, n     int
}

// woods is wi's grid for the map now.
func woods(g *Game, wi *woodIndex) *spatial.Grid[entity.EntityID] {
	w, h := g.grid.Width(), g.grid.Height()
	if !wi.ok || wi.ver != g.structVer || wi.src != any(g.bldgs) || wi.w != w || wi.h != h || wi.n != len(g.bldgIDs) {
		fillWoods(g, &wi.grid, w, h)
		wi.ok, wi.ver, wi.src, wi.w, wi.h, wi.n = true, g.structVer, g.bldgs, w, h, len(g.bldgIDs)
		wi.builds++
	} else if checkIndex(g) {
		var fresh spatial.Grid[entity.EntityID]
		fillWoods(g, &fresh, w, h)
		if !spatial.Same(&fresh, &wi.grid) {
			panic("wood index: a building changed without structVer")
		}
	}
	return &wi.grid
}

// fillWoods buckets every wooden building by its anchor tile, in building
// order.
func fillWoods(g *Game, b *spatial.Grid[entity.EntityID], w, h int) {
	b.Reset(w, h)
	b.Begin(len(g.bldgIDs))
	for i, id := range g.bldgIDs {
		bi := -1
		if bd, ok := g.bldgs.Get(id); ok && bd.Stuff == item.Wood {
			bi = b.Bucket(bd.X, bd.Y)
		}
		b.Add(i, bi)
	}
	b.Seal()
	for i, id := range g.bldgIDs {
		b.Put(i, id)
	}
}

// flammable is the nearest unburning wooden colony building within r of
// (x, y), its anchor tile, through wi.
func flammable(g *Game, wi *woodIndex, x, y, r int) (int, int, bool) {
	if wi == nil {
		wi = &woodIndex{} // a bare adapter (tests)
	}
	_, bx, by, ok := spatial.Nearest(woods(g, wi), x, y, r, func(id entity.EntityID) (int, int, bool) {
		b, ok := g.bldgs.Get(id)
		return b.X, b.Y, ok && !notColonyPiece(b) && g.fireAt(b.X, b.Y) == 0
	})
	return bx, by, ok
}
