package game

import (
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/ai/pathing"
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/combat/weapon"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/temperature"
)

// Threats. A simple storyteller sends a small raid a few days in, then
// every five to nine days, growing slowly with time; in winter, hungry
// wolf packs come too. Raiders are pawns of their own faction: they walk in
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
}

type raidGroup struct {
	Size int
	Plan raidPlan
	// Siege camp: where it digs in, whether it has, when it started out,
	// when it storms (Until) and whether it has.
	CampX, CampY    int
	Camped, Stormed bool
	Started, Until  uint64
}

const (
	firstRaidDay  = 5
	raidGapMin    = 5 // days
	raidGapSpread = 5 // days, added at random
	// bleedOut is how long a downed raider lasts.
	bleedOut = 4 * clock.TicksPerHour
	// raiderLeaveIdle: raiders with no one left to fight for this long go home.
	raiderLeaveIdle = clock.TicksPerHour
	// corpseLinger is how long a dead raider's body stays before it is
	// cleared away.
	corpseLinger = clock.TicksPerHour / 2
	// sightRange: colonists this close pull arriving raiders into the fight.
	sightRange = 18
	// raiderRetry: a raider that found no way in waits this long to re-plan.
	raiderRetry = 120
	// raiderGiveUp: raiders still on the map after this long go home.
	raiderGiveUp = clock.TicksPerDay
)

var raiderNames = []string{"Krell", "Vasha", "Dorn", "Mira", "Tobias", "Ilse", "Grit", "Sable", "Hask", "Rook", "Nessa", "Brann"}

func (g *Game) isRaider(id entity.EntityID) bool {
	_, ok := g.raiders[id]
	return ok
}

// standingRaider is a raider still on its feet.
func (g *Game) standingRaider(id entity.EntityID) bool {
	v, ok := g.vital.Get(id)
	return ok && v.Status == health.Alive && g.isRaider(id)
}

// phaseThreats runs the storyteller and every raider.
func (g *Game) phaseThreats() {
	now := uint64(g.clock.Now())
	if g.threatsOn && now >= g.nextThreat {
		g.fireThreat(now)
	}
	for _, id := range append([]entity.EntityID(nil), g.raiderIDs...) {
		g.thinkRaider(id, now)
	}
}

// scheduleThreats starts the storyteller (colonies, not bare test maps).
func (g *Game) scheduleThreats() {
	g.threatsOn = true
	g.nextThreat = uint64(firstRaidDay*clock.TicksPerDay + g.rng.IntN(clock.TicksPerDay))
}

func (g *Game) fireThreat(now uint64) {
	day := int(now / clock.TicksPerDay)
	winter := temperature.Season(clock.Tick(now)) == "Winter"
	switch roll := g.rng.IntN(4); {
	case winter && roll < 2:
		g.wolfPack(2 + g.rng.IntN(2))
	case !winter && roll == 0 && day >= 8:
		g.manhunterPack(3 + g.rng.IntN(3))
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
		if !g.grid.Walkable(x, y) {
			continue
		}
		if !found {
			fx, fy, found = x, y, true // fallback: any walkable edge tile
		}
		if checks++; checks > 12 {
			break
		}
		if _, ok := pathing.Find(bm, x, y, hx, hy); ok {
			return x, y, true
		}
	}
	return fx, fy, found
}

// Raid sends n raiders in from one map edge to assault the colony (also
// a dev hook).
func (g *Game) Raid(n int) { g.raidWith(g.kitOf(n), planAssault) }

// raidWith sends one raider per weapon in kit, following plan.
func (g *Game) raidWith(kit []item.Kind, plan raidPlan) {
	ex, ey, ok := g.edgeTile()
	n := len(kit)
	if !ok || n <= 0 {
		return
	}
	grp := raidGroup{Size: n, Plan: plan}
	if plan == planSiege {
		grp.CampX, grp.CampY = g.campSpot(ex, ey)
	}
	g.raidGroups = append(g.raidGroups, grp)
	group := len(g.raidGroups) - 1
	spawned := 0
	for i := 0; i < n*6 && spawned < n; i++ {
		x, y := ex+g.rng.IntN(5)-2, ey+g.rng.IntN(5)-2
		if !g.grid.InBounds(x, y) || !g.grid.Walkable(x, y) {
			continue
		}
		id := g.entities.Next()
		g.raiderIDs = append(g.raiderIDs, id)
		g.positions.Set(id, Position{X: x, Y: y})
		g.names.Set(id, raiderNames[g.rng.IntN(len(raiderNames))])
		g.jobs.Set(id, job.Record{})
		g.need.Set(id, needs.NewFull())
		g.vital.Set(id, health.Record{Status: health.Alive})
		g.equip[id] = kit[spawned]
		g.raiders[id] = raiderState{Group: group, Arrived: uint64(g.clock.Now())}
		spawned++
	}
	g.raidGroups[group].Size = spawned
	if spawned == 0 {
		return
	}
	word := "raiders are"
	if spawned == 1 {
		word = "raider is"
	}
	switch plan {
	case planSiege:
		g.threat(ex, ey, "Raid! %d %s coming. They look ready to dig in for a siege.", spawned, word)
	case planSappers:
		g.threat(ex, ey, "Sappers! %d %s tunnelling in through the walls.", spawned, word)
	default:
		g.threat(ex, ey, "Raid! %d %s coming for the colony.", spawned, word)
	}
}

// wolfPack sends hungry ridgewolves in from an edge.
func (g *Game) wolfPack(n int) {
	ex, ey, ok := g.edgeTile()
	if !ok {
		return
	}
	for i := 0; i < n; i++ {
		for try := 0; try < 10; try++ {
			x, y := ex+g.rng.IntN(5)-2, ey+g.rng.IntN(5)-2
			if !g.grid.InBounds(x, y) {
				continue
			}
			if id, err := g.spawnAnimal(animal.Ridgewolf, x, y); err == nil {
				a, _ := g.animals.Get(id)
				a.Hunger = animal.DefOf(animal.Ridgewolf).HungerTicks
				g.animals.Set(id, a)
				break
			}
		}
	}
	g.threat(ex, ey, "A pack of %d starving ridgewolves is prowling near the colony.", n)
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
	v, _ := g.vital.Get(id)
	switch {
	case v.Status == health.Dead:
		if r.Dead == 0 {
			r.Dead = now
			g.dropRaiderGear(id)
			g.raiders[id] = r
		}
		if now-r.Dead >= corpseLinger {
			g.removeRaider(id)
		}
		return
	case v.Status == health.Downed:
		if r.Downed == 0 {
			r.Downed = now
			g.raiders[id] = r
			pos, _ := g.positions.Get(id)
			g.message(pos.X, pos.Y, "Raider %s is down.", g.name(id))
		}
		if now-r.Downed >= bleedOut {
			g.die(id, health.CauseInjury)
		}
		return
	}
	j, _ := g.jobs.Get(id)
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

// dropRaiderGear leaves a dead raider's weapon where it fell.
func (g *Game) dropRaiderGear(id entity.EntityID) {
	pos, _ := g.positions.Get(id)
	if k, ok := g.equip[id]; ok {
		g.dropStack(k, 1, pos.X, pos.Y)
		delete(g.equip, id)
	}
}

func (g *Game) removeRaider(id entity.EntityID) {
	g.reserve.ReleaseOwner(id)
	g.positions.Delete(id)
	g.names.Delete(id)
	g.jobs.Delete(id)
	g.need.Delete(id)
	g.vital.Delete(id)
	delete(g.equip, id)
	delete(g.raiders, id)
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

// Raiders counts raiders on the map (standing ones and all).
func (g *Game) Raiders() (standing, total int) {
	for _, id := range g.raiderIDs {
		total++
		if g.standingRaider(id) {
			standing++
		}
	}
	return
}
