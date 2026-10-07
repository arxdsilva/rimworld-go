package game

import (
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/ai/pathing"
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/combat/projectile"
	"rimworld-go/internal/domain/combat/weapon"
	"rimworld-go/internal/domain/sim/entity"
)

// Defences. Buildings take damage and break. Raiders and wild animals
// cannot open the colony's doors, so a closed-off colony makes raiders
// bash through the nearest door or wall instead. Sandbags (and wall
// corners) give cover to whoever stands right behind them. Spike traps
// spring on raiders and predators, then are re-laid as blueprints.

const (
	// breachCooldown is the ticks between blows on a door or wall.
	breachCooldown = 60
	// trapDamage is the injury a sprung spike trap deals.
	trapDamage = 32
	// trapChance is the percent chance a raider or predator springs a trap
	// it walks over.
	trapChance = 75
	// repairPerTick is the hit points a builder mends per work tick.
	repairPerTick = 1
)

// tileSet is a set of map cells (index y*w+x) built once per query.
type tileSet map[int]bool

// buildingCells indexes object-layer buildings of the given kinds.
func (g *Game) buildingCells(kinds ...construction.Kind) tileSet {
	set := tileSet{}
	w := g.grid.Width()
	for _, id := range g.bldgIDs {
		b, ok := g.bldgs.Get(id)
		if !ok {
			continue
		}
		for _, k := range kinds {
			if b.Kind != k {
				continue
			}
			cells, _, _ := construction.Footprint(b.Kind, b.Facing, b.X, b.Y)
			for _, c := range cells {
				set[c[1]*w+c[0]] = true
			}
		}
	}
	return set
}

// hostileMap is the map as raiders and wild animals see it: the colony's
// doors are shut to them.
type hostileMap struct {
	g     *Game
	doors tileSet
}

func (g *Game) hostileMap() hostileMap {
	return hostileMap{g: g, doors: g.buildingCells(construction.Door)}
}

func (h hostileMap) Width() int  { return h.g.grid.Width() }
func (h hostileMap) Height() int { return h.g.grid.Height() }
func (h hostileMap) Walkable(x, y int) bool {
	return h.g.grid.Walkable(x, y) && !h.doors[y*h.g.grid.Width()+x]
}

// breachMap is the map with every door and wall knocked down: raiders
// shut out plan their way in through it.
type breachMap struct {
	g      *Game
	breaks tileSet
}

func (b breachMap) Width() int  { return b.g.grid.Width() }
func (b breachMap) Height() int { return b.g.grid.Height() }
func (b breachMap) Walkable(x, y int) bool {
	if !b.g.grid.InBounds(x, y) {
		return false
	}
	return b.g.grid.Walkable(x, y) || b.breaks[y*b.g.grid.Width()+x]
}

// pathQuery is the walkability a mover plans with.
func (g *Game) pathQuery(id entity.EntityID) pathing.Query {
	if g.isRaider(id) {
		return g.hostileMap()
	}
	return g.grid
}

// doorAt reports a built door on (x, y).
func (g *Game) doorAt(x, y int) bool {
	bid := g.layerBldgAt(x, y, construction.LayerObject)
	if bid == 0 {
		return false
	}
	b, _ := g.bldgs.Get(bid)
	return b.Kind == construction.Door
}

// startBreach sends a raider that cannot reach tp to break in: it plans
// a route as if walls and doors were gone and attacks the first one on it.
func (g *Game) startBreach(id entity.EntityID, tp Position) bool {
	pos, _ := g.positions.Get(id)
	bm := breachMap{g: g, breaks: g.buildingCells(construction.Wall, construction.Door)}
	path, ok := pathing.Find(bm, pos.X, pos.Y, tp.X, tp.Y)
	if !ok {
		return false
	}
	hm := g.hostileMap()
	sx, sy := pos.X, pos.Y
	for _, p := range path {
		if !hm.Walkable(p.X, p.Y) {
			bid := g.layerBldgAt(p.X, p.Y, construction.LayerObject)
			if bid == 0 {
				return false
			}
			rec := job.Record{Kind: job.Breach, Target: bid, TX: sx, TY: sy, WorkLeft: breachCooldown}
			return g.startJob(id, rec)
		}
		sx, sy = p.X, p.Y
	}
	return false
}

// doBreach: a raider beside a door or wall hits it on cooldown.
func (g *Game) doBreach(id entity.EntityID, j job.Record) {
	b, ok := g.bldgs.Get(j.Target)
	if !ok {
		j.State = job.Complete
		g.jobs.Set(id, j)
		return
	}
	if j.State == job.Goto {
		return // still walking up (movement advances it)
	}
	pos, _ := g.positions.Get(id)
	if !g.touches(pos, b) {
		j.State, j.Fail = job.Fail, job.Unreachable
		g.jobs.Set(id, j)
		return
	}
	if j.WorkLeft > 0 {
		j.WorkLeft--
		g.jobs.Set(id, j)
		return
	}
	j.WorkLeft = breachCooldown
	g.jobs.Set(id, j)
	dmg := weapon.DefOf(g.weaponOf(id)).Damage
	if g.hasRanged(id) {
		dmg = 7 // rifle butt
	}
	g.traces = append(g.traces, projectile.Trace{FromX: pos.X, FromY: pos.Y, ToX: b.X, ToY: b.Y,
		Tick: uint64(g.clock.Now()), Hit: true, Melee: true})
	g.damageBuilding(j.Target, dmg)
}

// touches reports p standing next to (or on) a building's footprint.
func (g *Game) touches(p Position, b construction.Building) bool {
	cells, _, _ := construction.Footprint(b.Kind, b.Facing, b.X, b.Y)
	for _, c := range cells {
		if abs(c[0]-p.X) <= 1 && abs(c[1]-p.Y) <= 1 {
			return true
		}
	}
	return false
}

// damageBuilding harms a building, destroying it at zero hit points.
func (g *Game) damageBuilding(id entity.EntityID, dmg int) {
	b, ok := g.bldgs.Get(id)
	if !ok || dmg <= 0 {
		return
	}
	b.Damage += dmg
	if b.HP() > 0 {
		g.bldgs.Set(id, b)
		return
	}
	g.message(b.X, b.Y, "The %s was smashed down.", b.Kind)
	g.removeBuilding(id)
}

// coverFor is the cover (percent) the target at tp has against a shot
// from ap: the best of the buildings on the cells beside it, facing the
// shooter.
func (g *Game) coverFor(ap, tp Position) int {
	if weapon.Distance(ap.X, ap.Y, tp.X, tp.Y) <= 1 {
		return 0
	}
	sx, sy := sign(ap.X-tp.X), sign(ap.Y-tp.Y)
	cells := [][2]int{{tp.X + sx, tp.Y + sy}}
	if sx != 0 && sy != 0 {
		cells = append(cells, [2]int{tp.X + sx, tp.Y}, [2]int{tp.X, tp.Y + sy})
	}
	best := 0
	for _, c := range cells {
		if bid := g.layerBldgAt(c[0], c[1], construction.LayerObject); bid != 0 {
			b, _ := g.bldgs.Get(bid)
			best = max(best, construction.CoverPct(b.Kind))
		}
	}
	return best
}

func sign(v int) int {
	switch {
	case v > 0:
		return 1
	case v < 0:
		return -1
	}
	return 0
}

// trapAt is a built spike trap on (x, y).
func (g *Game) trapAt(x, y int) entity.EntityID {
	bid := g.layerBldgAt(x, y, construction.LayerObject)
	if bid == 0 {
		return 0
	}
	if b, _ := g.bldgs.Get(bid); b.Kind == construction.SpikeTrap {
		return bid
	}
	return 0
}

// hasTraps is a cheap check before looking for traps under movers.
func (g *Game) hasTraps() bool {
	for _, id := range g.bldgIDs {
		if b, ok := g.bldgs.Get(id); ok && b.Kind == construction.SpikeTrap {
			return true
		}
	}
	return false
}

// springTrap: a raider or predator stepped on (x, y).
func (g *Game) springTrap(victim entity.EntityID, x, y int) {
	tid := g.trapAt(x, y)
	if tid == 0 || g.rng.IntN(100) >= trapChance {
		return
	}
	t, _ := g.bldgs.Get(tid)
	who := g.name(victim)
	if a, ok := g.animals.Get(victim); ok {
		who = "a " + a.Species.String()
	}
	g.message(x, y, "A spike trap caught %s.", who)
	g.removeBuilding(tid)
	g.placeFrameStuff(construction.SpikeTrap, t.Stuff, construction.South, x, y) // re-lay it
	g.hurt(victim, trapDamage, 0)
}

// trapsUnderfoot springs traps under raiders and predators that just
// moved.
func (g *Game) trapsUnderfoot(movers []entity.EntityID) {
	if len(movers) == 0 || !g.hasTraps() {
		return
	}
	for _, id := range movers {
		if p, ok := g.positions.Get(id); ok {
			g.springTrap(id, p.X, p.Y)
		}
	}
}

// trapVictim reports animals that spring traps: predators.
func trapVictim(a animal.State) bool { return animal.DefOf(a.Species).Predator }

// assignRepair sends a builder to mend the nearest damaged building, but
// not while raiders are still on their feet.
func (g *Game) assignRepair(id entity.EntityID) bool {
	if standing, _ := g.Raiders(); standing > 0 {
		return false
	}
	pos, _ := g.positions.Get(id)
	var best entity.EntityID
	bestD := 1 << 30
	for _, bid := range g.bldgIDs {
		b, ok := g.bldgs.Get(bid)
		if !ok || b.Damage <= 0 || b.Deconstruct {
			continue
		}
		if _, taken := g.reserve.EntityOwner(bid); taken {
			continue
		}
		if d := pathing.Dist(pos.X, pos.Y, b.X, b.Y); d < bestD || (d == bestD && bid < best) {
			best, bestD = bid, d
		}
	}
	if best == 0 {
		return false
	}
	b, _ := g.bldgs.Get(best)
	tx, ty := g.workSpot(id, b.X, b.Y)
	return g.startJob(id, job.Record{Kind: job.Repair, Target: best, TX: tx, TY: ty, WorkLeft: b.Damage})
}

func (g *Game) doRepair(id entity.EntityID, j job.Record) {
	b, ok := g.bldgs.Get(j.Target)
	if !ok || b.Damage <= 0 {
		j.State = job.Complete
		g.jobs.Set(id, j)
		return
	}
	if g.workTick(id) && uint64(g.clock.Now())%3 == 0 {
		b.Damage = max(0, b.Damage-repairPerTick)
		g.bldgs.Set(j.Target, b)
	}
	j.WorkLeft = b.Damage
	if b.Damage == 0 {
		j.State = job.Complete
	}
	g.jobs.Set(id, j)
}

// DamageAt harms the object-layer building on (x, y) (a dev hook).
func (g *Game) DamageAt(x, y, dmg int) {
	if bid := g.layerBldgAt(x, y, construction.LayerObject); bid != 0 {
		g.damageBuilding(bid, dmg)
	}
}
