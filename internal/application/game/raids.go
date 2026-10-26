package game

import (
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/combat/weapon"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/incident"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/temperature"
)

// Threats. A simple storyteller sends a small raid a few days in, then
// every five to nine days, growing slowly with time; in winter, hungry
// predator packs come too, and manhunter packs and hive swarms (hive.go)
// at times (pickThreat has the odds). Raiders are pawns of their own faction: they walk in
// from a map edge, attack the nearest standing colonist, and retreat when
// badly hurt or once half their group is down. A downed raider bleeds out.

type raidMode uint8

const (
	raidArrive  raidMode = iota // walking in towards the colony
	raidAssault                 // fighting colonists
	raidFlee                    // running for the map edge
)

type raiderState struct {
	Group  int
	Mode   raidMode
	Downed uint64 // tick it went down (0 = standing)
	Idle   uint64 // tick it last had no one to fight
	Dead   uint64 // tick it died (removed a little later)
	// RetryAt: no way in was found; don't plan again before this tick.
	RetryAt uint64
	// Arrived is when the raider came onto the map: after raiderGiveUp
	// it gives up and goes home.
	Arrived uint64
	// The colony's prisoners (prisoners.go, M8) stay on this list, out of
	// the raid AI. Leaving: released, walking off the map (since Idle).
	// Hungry: the food band (prisonerFoodLow..prisonerFoodFed) wants a
	// meal brought, not before FoodRetry. Warden: who last visited.
	Leaving   bool
	Hungry    bool
	FoodRetry uint64
	Warden    entity.EntityID
	// KidnapAt: a kidnap failed; no new one before this tick (kidnap.go).
	KidnapAt uint64
}

type raidGroup struct {
	Size int
	Plan raidPlan
	// Siege camp: where it digs in, whether it has, when it started out,
	// when it storms (Until) and whether it has.
	CampX, CampY    int
	Camped, Stormed bool
	Started, Until  uint64
	// Faction is 1 + the index of the faction that sent it (0: an
	// unattributed raid, as every legacy one); Over: it was defeated or
	// left and the faction's goodwill paid for it (factions.go, NG-1).
	Faction int
	Over    bool
}

const (
	firstRaidDay  = 5
	raidGapMin    = 5 // days
	raidGapSpread = 5 // days, added at random
	// raiderLeaveIdle: raiders with no one left to fight for this long go home.
	raiderLeaveIdle = clock.TicksPerHour
	// sightRange: colonists this close pull arriving raiders into the fight.
	sightRange = 18
	// raiderRetry: a raider that found no way in waits this long to re-plan.
	raiderRetry = 120
	// raiderGiveUp: raiders still on the map after this long go home.
	raiderGiveUp = clock.TicksPerDay
)

var raiderNames = []string{"Krell", "Vasha", "Dorn", "Mira", "Tobias", "Ilse", "Grit", "Sable", "Hask", "Rook", "Nessa", "Brann"}

// isRaider reports a pawn of another faction on the map: raiders and the
// colony's prisoners (they path like raiders, so doors hold them), not a
// released prisoner walking out.
func (g *Game) isRaider(id entity.EntityID) bool {
	r, ok := g.raiders[id]
	return ok && !r.Leaving
}

// standingRaider is a hostile raider still on its feet (never a prisoner).
func (g *Game) standingRaider(id entity.EntityID) bool {
	v, ok := g.vital.Get(id)
	return ok && v.Status == health.Alive && g.isRaider(id) && g.prisoners[id] == nil
}

// phaseThreats runs the storyteller and every raider.
func (g *Game) phaseThreats() {
	now := uint64(g.clock.Now())
	if g.hasSetup() {
		g.setupTick(now) // story.go: arrival, losses, day rollover (NG-1)
	}
	if g.threatsOn && now >= g.nextThreat {
		g.fireThreat(now)
	}
	ids := append(g.raiderBuf[:0], g.raiderIDs...) // the list changes as raiders die or leave
	for _, id := range ids {
		g.thinkRaider(id, now)
	}
	g.raiderBuf = ids[:0]
	g.vaultTick(now) // vault.go (MAPGEN-3): approach warnings, clearing
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

// threatKind is what the storyteller sends.
type threatKind uint8

const (
	threatRaid threatKind = iota
	threatPack
	threatManhunters
	threatHive
	threatMad // one wild animal near home goes manhunter (SIM-025)
)

// threatRolls is the storyteller's die. Odds (SIM-019): in winter a
// predator pack 4/8 and, from day 12, a hive swarm 1/8; otherwise a
// manhunter pack 2/8 from day 8 and a hive swarm 1/8 from day 12; a raid
// the rest (so raids went from 3/4 to 5/8 outside winter and from 1/2 to
// 3/8 in winter, once hives can come). SIM-025 takes one more raid face
// in every season for a mad animal from day 6 (raids 4/8 outside winter,
// 2/8 in it, from day 12); a mad animal that cannot happen (cooling down,
// no wild animal near home) is a raid after all.
const (
	threatRolls  = 8
	hiveFirstDay = 12
	madRoll      = threatRolls - 2
)

// pickThreat maps a roll of the die (0..threatRolls-1) to a threat.
func pickThreat(roll, day int, winter bool) threatKind {
	switch {
	case winter && roll < 4:
		return threatPack
	case !winter && roll < 2 && day >= 8:
		return threatManhunters
	case roll == threatRolls-1 && day >= hiveFirstDay:
		return threatHive
	case roll == madRoll && day >= incident.DefOf(incident.MadAnimal).MinDay:
		return threatMad
	}
	return threatRaid
}

func (g *Game) fireThreat(now uint64) {
	if g.hasSetup() {
		g.fireStoryThreat(now) // story.go: storyteller, difficulty, factions (NG-1)
		return
	}
	day := int(now / clock.TicksPerDay)
	winter := temperature.Season(clock.Tick(now)) == "Winter"
	switch pickThreat(g.rng.IntN(threatRolls), day, winter) {
	case threatPack:
		g.wolfPack(2 + g.rng.IntN(2))
	case threatManhunters:
		g.manhunterPack(3 + g.rng.IntN(3))
	case threatHive:
		g.hiveSwarm()
	case threatMad:
		if !g.madAnimal(now, false) { // incidents.go
			g.scaledRaid(day)
		}
	default:
		g.scaledRaid(day)
	}
	g.nextThreat = now + uint64((raidGapMin+g.rng.IntN(raidGapSpread))*clock.TicksPerDay+g.rng.IntN(clock.TicksPerDay))
}

// edgeTile is a random walkable tile on the map edge from which the
// colony can be reached (breaking through its walls and doors if need
// be), so nothing arrives stuck in a cut-off pocket.
func (g *Game) edgeTile() (int, int, bool) {
	w, h := g.grid.Width(), g.grid.Height()
	hx, hy := g.home()
	if x, y, ok := g.walkableNear(hx, hy, 6); ok {
		hx, hy = x, y
	}
	bm := breachMap{g: g, breaks: g.buildingCells(construction.Wall, construction.Door)}
	var fx, fy int
	found, checks := false, 0
	for try := 0; try < 200; try++ {
		var x, y int
		switch g.rng.IntN(4) {
		case 0:
			x, y = g.rng.IntN(w), 0
		case 1:
			x, y = g.rng.IntN(w), h-1
		case 2:
			x, y = 0, g.rng.IntN(h)
		default:
			x, y = w-1, g.rng.IntN(h)
		}
		if !g.dryWalkable(x, y) {
			continue // never arrive in the water
		}
		if !found {
			fx, fy, found = x, y, true // fallback: any walkable edge tile
		}
		if checks++; checks > 12 {
			break
		}
		if _, ok := g.searcher().Find(bm, x, y, hx, hy); ok {
			return x, y, true
		}
	}
	return fx, fy, found
}

// Raid sends n raiders in from one map edge to assault the colony (also
// a dev hook).
func (g *Game) Raid(n int) {
	g.viewGen++ // view cache (indexes.go): this hook changes the state
	g.raidWith(g.kitOf(n), planAssault)
}

// raidWith sends one raider per weapon in kit, following plan.
func (g *Game) raidWith(kit []item.Kind, plan raidPlan) { g.raidFrom(kit, plan, 0) }

// raidFrom is raidWith for the faction fac (1 + index into the colony's
// factions; 0: unattributed). A faction's raiders get generated names and
// its message names the faction.
func (g *Game) raidFrom(kit []item.Kind, plan raidPlan, fac int) {
	ex, ey, ok := g.edgeTile()
	n := len(kit)
	if !ok || n <= 0 {
		return
	}
	f := g.factionOf(fac) // factions.go: nil for a legacy raid
	grp := raidGroup{Size: n, Plan: plan}
	if f != nil {
		grp.Faction = fac
	}
	if plan == planSiege {
		grp.CampX, grp.CampY = g.campSpot(ex, ey)
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
		var id entity.EntityID
		if wantKin {
			wantKin = false
			kin = g.relativeFor(fac, whereRaider, techTag(f))
			id = kin
		}
		if id != 0 {
			g.raiderIDs = withID(g.raiderIDs, id) // a known person's id may be older
		} else {
			id = g.entities.Next()
			g.raiderIDs = append(g.raiderIDs, id)
		}
		g.positions.Set(id, Position{X: x, Y: y})
		switch {
		case id == kin:
			g.names.Set(id, bioName(g.personOf(id).Bio))
		case f != nil:
			g.names.Set(id, g.raiderName(id, f)) // story.go
		default:
			g.names.Set(id, raiderNames[g.rng.IntN(len(raiderNames))])
		}
		g.jobs.Set(id, job.Record{})
		g.need.Set(id, needs.NewFull())
		g.vital.Set(id, health.Record{Status: health.Alive})
		g.equip[id] = kit[spawned]
		g.raiders[id] = raiderState{Group: group, Arrived: uint64(g.clock.Now())}
		if id != kin {
			g.raiderPerson(id, fac) // people.go: a world person (relations, capture)
		}
		spawned++
	}
	g.raidGroups[group].Size = spawned
	if spawned == 0 {
		g.raidGroups[group].Over = f != nil // nothing to repel: no goodwill lost
		return
	}
	if f != nil {
		if kin != 0 { // relative_world.go: before the raid alert, which stays the latest message
			g.relativeLetter(letterCaution, "Relative among the raiders", kin, "raiders")
		}
		g.raidMessage(ex, ey, spawned, plan, f) // story.go
		return
	}
	word := "raiders are"
	if spawned == 1 {
		word = "raider is"
	}
	switch plan {
	case planSiege:
		g.threat("Siege", ex, ey, "Raid! %d %s coming. They look ready to dig in for a siege.", spawned, word)
	case planSappers:
		g.threat("Sappers", ex, ey, "Sappers! %d %s tunnelling in through the walls.", spawned, word)
	default:
		g.threat("Raid", ex, ey, "Raid! %d %s coming for the colony.", spawned, word)
	}
}

// wolfPack sends n hungry pack predators of one season-fitting species
// in from an edge.
func (g *Game) wolfPack(n int) {
	s, ok := g.pickSpecies(func(s animal.Species) bool { return animal.DefOf(s).Pack })
	if !ok {
		return
	}
	ex, ey, ok := g.edgeTile()
	if !ok {
		return
	}
	spawned := 0
	for i := 0; i < n; i++ {
		for try := 0; try < 10; try++ {
			x, y := ex+g.rng.IntN(5)-2, ey+g.rng.IntN(5)-2
			if !g.grid.InBounds(x, y) {
				continue
			}
			if id, err := g.spawnAnimal(s, x, y); err == nil {
				a, _ := g.animals.Get(id)
				a.Hunger = animal.DefOf(s).HungerTicks
				g.animals.Set(id, a)
				spawned++
				break
			}
		}
	}
	if spawned > 0 {
		g.threat("Starving predators", ex, ey, "A pack of %d starving %s is prowling near the colony.", spawned, pluralLabel(s))
	}
}

// groupBroken: half or more of a raid group is down, dead or gone.
func (g *Game) groupBroken(group int) bool {
	standing := 0
	for _, id := range g.raiderIDs {
		if r := g.raiders[id]; r.Group == group && g.standingRaider(id) {
			standing++
		}
	}
	return standing*2 <= g.raidGroups[group].Size
}

func (g *Game) thinkRaider(id entity.EntityID, now uint64) {
	r := g.raiders[id]
	if r.Leaving || g.prisoners[id] != nil {
		g.thinkPrisoner(id, now) // prisoner_life.go: out of the raid AI
		return
	}
	v, _ := g.vital.Get(id)
	switch {
	case v.Status == health.Dead:
		if r.Dead == 0 {
			r.Dead = now
			g.dropRaiderGear(id)
			g.raiders[id] = r
		}
		return // its body stays until it is buried or cremated (burial.go)
	case v.Status == health.Downed:
		g.dropPatient(id) // kidnap.go: a downed kidnapper lets go
		if r.Downed == 0 {
			r.Downed = now
			g.raiders[id] = r
			pos, _ := g.positions.Get(id)
			g.message(pos.X, pos.Y, "Raider %s is down.", g.name(id))
		}
		// MED-01: no timer; it bleeds out unless captured and tended, and
		// its wounds close slowly, so one that stops bleeding gets up again
		if !g.beingCaptured(id) {
			g.heal(id, &v, now) // combat.go: stands it up past the band (bleed.go)
		}
		return
	}
	if r.Downed != 0 { // back on its feet after going down: it has had enough
		r.Downed, r.Mode = 0, raidFlee
		g.raiders[id] = r
		g.interrupt(id, job.Cancelled)
	}
	j, _ := g.jobs.Get(id)
	if j.Kind == job.Kidnap {
		g.stepKidnap(id, j, now) // kidnap.go: committed until it ends
		return
	}
	if r.Arrived == 0 {
		r.Arrived = now // raiders from before this field existed
	}
	if r.Mode != raidFlee && (v.Injury >= health.MaxHP/2 || g.groupBroken(r.Group) ||
		r.Idle != 0 && now-r.Idle >= raiderLeaveIdle || now-r.Arrived >= raiderGiveUp) {
		r.Mode = raidFlee
		g.raiders[id] = r
		g.interrupt(id, job.Cancelled)
		j = job.Record{}
	}
	pos, _ := g.positions.Get(id)
	switch r.Mode {
	case raidFlee:
		w, h := g.grid.Width(), g.grid.Height()
		if pos.X == 0 || pos.Y == 0 || pos.X == w-1 || pos.Y == h-1 {
			g.removeRaider(id) // gone home
			return
		}
		if g.tryKidnap(id, pos, now) { // kidnap.go
			return
		}
		if !j.Active() || (now+uint64(id))%120 == 0 && j.Kind != job.Move {
			ex, ey := nearestEdge(pos, w, h)
			if x, y, ok := g.walkableNear(ex, ey, 3); ok {
				g.startJob(id, job.Record{Kind: job.Move, TX: x, TY: y})
			}
		}
	default:
		if g.thinkSiege(id, r, now) {
			return
		}
		if j.Kind == job.Attack && j.Active() {
			g.doCombat(id, j, now)
			return
		}
		if j.Kind == job.Breach && j.Active() {
			// keep bashing unless a colonist comes into reach (re-planning
			// now and then in case a way in opened up)
			_, tp, ok := g.nearestColonist(pos)
			if (now+uint64(id))%300 != 0 && (!ok || !g.canHit(pos, weaponDefOf(g, id), tp)) {
				g.doBreach(id, j)
				return
			}
		}
		if now < r.RetryAt && !j.Active() {
			return // no way in last time: wait before planning again
		}
		target, tp, ok := g.nearestColonist(pos)
		if !ok {
			if r.Idle == 0 {
				r.Idle = now
				g.raiders[id] = r
			}
			g.tryKidnap(id, pos, now) // kidnap.go: no one left standing
			return
		}
		r.Idle = 0
		if r.Mode == raidArrive && weapon.Distance(pos.X, pos.Y, tp.X, tp.Y) > sightRange {
			// walk in towards the colony
			if !j.Active() || (now+uint64(id))%180 == 0 {
				if x, y, ok := g.walkableNear(tp.X, tp.Y, 4); ok &&
					!g.startJob(id, job.Record{Kind: job.Move, TX: x, TY: y}) && g.unreachable(id) && !g.startBreach(id, tp) {
					r.RetryAt = now + raiderRetry
				}
			}
			g.raiders[id] = r
			return
		}
		r.Mode = raidAssault
		g.raiders[id] = r
		d := weaponDefOf(g, id)
		rec := job.Record{Kind: job.Attack, Target: target, TX: tp.X, TY: tp.Y, WorkLeft: aimTicks(d)}
		if g.canHit(pos, d, tp) { // already in reach: no walking needed
			g.reserve.ReleaseOwner(id)
			rec.State = job.Do
			g.jobs.Set(id, rec)
			return
		}
		if !g.startJob(id, rec) && g.unreachable(id) && !g.startBreach(id, tp) {
			r.RetryAt = now + raiderRetry
			g.raiders[id] = r
		}
	}
}

// unreachable reports that a mover's last job failed for want of a path.
func (g *Game) unreachable(id entity.EntityID) bool {
	j, _ := g.jobs.Get(id)
	return j.State == job.Fail && j.Fail == job.Unreachable
}

func nearestEdge(p Position, w, h int) (int, int) {
	best, bx, by := p.X, 0, p.Y
	if d := w - 1 - p.X; d < best {
		best, bx, by = d, w-1, p.Y
	}
	if p.Y < best {
		best, bx, by = p.Y, p.X, 0
	}
	if d := h - 1 - p.Y; d < best {
		bx, by = p.X, h-1
	}
	return bx, by
}

// nearestColonist is the closest standing colonist a raider can reach.
func (g *Game) nearestColonist(p Position) (entity.EntityID, Position, bool) {
	var best entity.EntityID
	var bp Position
	bestD := 1 << 30
	for _, id := range g.pawnIDs {
		if !g.standingPawn(id) {
			continue
		}
		cp, _ := g.positions.Get(id)
		if d := weapon.Distance(p.X, p.Y, cp.X, cp.Y); d < bestD {
			best, bp, bestD = id, cp, d
		}
	}
	return best, bp, best != 0
}

// dropRaiderGear leaves a dead pawn's (raider's, visitor's or colonist's)
// weapon where it fell: a stranger's is found, not the colony's
// (haul_claim.go), and a raider's starts forbidden too.
func (g *Game) dropRaiderGear(id entity.EntityID) {
	pos, _ := g.positions.Get(id)
	if k, ok := g.equip[id]; ok {
		switch {
		case g.isRaider(id):
			g.spawnUnclaimed(k, 1, pos.X, pos.Y, true)
		case g.strangerGear(id):
			g.spawnUnclaimed(k, 1, pos.X, pos.Y, false)
		default:
			g.dropStack(k, 1, pos.X, pos.Y)
		}
		delete(g.equip, id)
	}
}

// strangerGear: id is a visitor (dead or alive), so its gear is not the
// colony's.
func (g *Game) strangerGear(id entity.EntityID) bool {
	p := g.personOf(id)
	return p != nil && p.Where == whereVisitor
}

// removeRaider takes raider (or visitor, prisoner) id off the map. A
// raider who got away is an off-map person again, its relations kept;
// whatever it carried is put down first.
func (g *Game) removeRaider(id entity.EntityID) {
	if p := g.personOf(id); p != nil && p.Where == whereRaider {
		p.Where = whereOffMap
	}
	g.dropPatient(id)
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

// raiderAt is the raider standing (or lying) on a tile.
func (g *Game) raiderAt(x, y int) entity.EntityID {
	for _, id := range g.raiderIDs {
		if p, ok := g.positions.Get(id); ok && p.X == x && p.Y == y {
			return id
		}
	}
	return 0
}

// raiderViews are the raiders for the read model.
func (g *Game) raiderViews() []PawnView {
	out := make([]PawnView, 0, len(g.raiderIDs))
	for _, id := range g.raiderIDs {
		if g.prisoners[id] != nil || g.raiders[id].Leaving {
			out = append(out, g.prisonerPawnView(id)) // prisoner_life.go
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
			Weapon: weaponDefOf(g, id).Name, Lying: v.Status != health.Alive, Hostile: true,
			Fleeing: g.raiders[id].Mode == raidFlee})
		g.fillPawnAction(&out[len(out)-1], id, j)
	}
	return out
}

// Raiders counts raiders on the map (standing ones and all), not the
// colony's prisoners.
func (g *Game) Raiders() (standing, total int) {
	for _, id := range g.raiderIDs {
		if g.prisoners[id] != nil || g.raiders[id].Leaving {
			continue
		}
		total++
		if g.standingRaider(id) {
			standing++
		}
	}
	return
}
