package game

import (
	"slices"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/ai/pathing"
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/combat/projectile"
	"rimworld-go/internal/domain/combat/weapon"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/terrain"
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
	// rebuildFirst: rebuild blueprints count as this many tiles closer
	// when builders and haulers pick their next job.
	rebuildFirst = 1000
)

// tileSet is a set of map cells (index y*w+x), one bit per cell. Sets
// from buildingCells and sapperBreaks are shared caches: read only.
type tileSet []uint64

func newTileSet(cells int) tileSet { return make(tileSet, (cells+63)/64) }

// has reports cell i (false off the set).
func (s tileSet) has(i int) bool {
	return i >= 0 && i>>6 < len(s) && s[i>>6]&(1<<(uint(i)&63)) != 0
}

func (s tileSet) add(i int) {
	if i >= 0 && i>>6 < len(s) {
		s[i>>6] |= 1 << (uint(i) & 63)
	}
}

// cellCache is a tileSet derived from buildings (and terrain), rebuilt
// when structVer — bumped by every building and terrain change — moves.
type cellCache struct {
	set tileSet
	ver uint64
	ok  bool
}

func (c *cellCache) get(ver uint64, build func() tileSet) tileSet {
	if !c.ok || c.ver != ver {
		c.set, c.ver, c.ok = build(), ver, true
	} else if selfCheck() {
		if fresh := build(); !slices.Equal(fresh, c.set) {
			panic("cellCache: stale building/terrain cell set (structVer not bumped?)")
		}
	}
	return c.set
}

// buildingCells indexes buildings of the given kinds (a shared cached
// set: read only).
func (g *Game) buildingCells(kinds ...construction.Kind) tileSet {
	var mask uint64
	for _, k := range kinds {
		if k >= 64 {
			return g.scanBuildingCells(kinds) // not maskable: uncached
		}
		mask |= 1 << uint(k)
	}
	if g.cellSets == nil {
		g.cellSets = map[uint64]*cellCache{}
	}
	c := g.cellSets[mask]
	if c == nil {
		c = &cellCache{}
		g.cellSets[mask] = c
	}
	return c.get(g.structVer, func() tileSet { return g.scanBuildingCells(kinds) })
}

func (g *Game) scanBuildingCells(kinds []construction.Kind) tileSet {
	w := g.grid.Width()
	set := newTileSet(w * g.grid.Height())
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
				set.add(c[1]*w + c[0])
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
	return h.g.grid.Walkable(x, y) && !h.doors.has(y*h.g.grid.Width()+x)
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
	return b.g.grid.Walkable(x, y) || b.breaks.has(y*b.g.grid.Width()+x)
}

// pathQuery is the walkability a mover plans with.
func (g *Game) pathQuery(id entity.EntityID) pathing.Query {
	switch {
	case g.sapper(id):
		return sapperMap{hostileMap: g.hostileMap(), traps: g.buildingCells(construction.SpikeTrap)}
	case g.isRaider(id):
		return g.hostileMap()
	}
	return g.areaPathQuery(id, g.grid) // areas.go: a restricted pawn keeps to its area
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
	breaks := g.buildingCells(construction.Wall, construction.Door)
	if g.sapper(id) {
		breaks = g.sapperBreaks() // sappers tunnel through rock too
	}
	bm := breachMap{g: g, breaks: breaks}
	path, ok := g.searcher().Find(bm, pos.X, pos.Y, tp.X, tp.Y)
	if !ok {
		return false
	}
	q := g.pathQuery(id)
	sx, sy := pos.X, pos.Y
	for _, p := range path {
		if !q.Walkable(p.X, p.Y) {
			bid := g.layerBldgAt(p.X, p.Y, construction.LayerObject)
			if bid == 0 && !terrain.IsRockWall(g.terr.At(p.X, p.Y)) {
				return false // a trap in the way: plan again later
			}
			rec := job.Record{Kind: job.Breach, Target: bid, DestX: p.X, DestY: p.Y, TX: sx, TY: sy, WorkLeft: breachCooldown}
			return g.startJob(id, rec)
		}
		sx, sy = p.X, p.Y
	}
	return false
}

// doBreach: a raider beside a door or wall hits it on cooldown.
func (g *Game) doBreach(id entity.EntityID, j job.Record) {
	if j.Target == 0 {
		g.doDig(id, j)
		return
	}
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
		Tick: uint64(g.clock.Now()), Hit: true, Melee: true, Damage: dmg, Building: true,
		Attacker: id, Weapon: weaponDefOf(g, id).Name})
	g.lastStrike[id] = strikeRec{Tick: uint64(g.clock.Now())}
	g.damageBuilding(j.Target, dmg)
}

// doDig: a sapper beside a rock tile hacks at it on cooldown.
func (g *Game) doDig(id entity.EntityID, j job.Record) {
	if !terrain.IsRockWall(g.terr.At(j.DestX, j.DestY)) {
		j.State = job.Complete
		g.jobs.Set(id, j)
		return
	}
	if j.State == job.Goto {
		return
	}
	pos, _ := g.positions.Get(id)
	if abs(pos.X-j.DestX) > 1 || abs(pos.Y-j.DestY) > 1 {
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
	dmg := max(10, weapon.DefOf(g.weaponOf(id)).Damage)
	g.traces = append(g.traces, projectile.Trace{FromX: pos.X, FromY: pos.Y, ToX: j.DestX, ToY: j.DestY,
		Tick: uint64(g.clock.Now()), Hit: true, Melee: true, Damage: dmg, Building: true,
		Attacker: id, Weapon: weaponDefOf(g, id).Name})
	g.lastStrike[id] = strikeRec{Tick: uint64(g.clock.Now())}
	g.digRock(j.DestX, j.DestY, dmg)
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
	// leave a blueprint to rebuild it, as it was (the salvage is set aside)
	cells, _, _ := construction.Footprint(b.Kind, b.Facing, b.X, b.Y)
	for _, c := range cells {
		g.clearItemsFrom(c[0], c[1], cells)
	}
	g.placeFrameStuff(b.Kind, b.Stuff, b.Facing, b.X, b.Y)
	if fid := g.layerFrameAt(b.X, b.Y, construction.DefOf(b.Kind).Layer); fid != 0 {
		fr, _ := g.frames.Get(fid)
		fr.Rebuild = true
		g.frames.Set(fid, fr)
	}
}

// clearItemsFrom moves loose stacks off (x, y) to the nearest free tile
// outside the given cells.
func (g *Game) clearItemsFrom(x, y int, cells [][2]int) {
	for {
		sid := g.itemAt(x, y)
		if sid == 0 {
			return
		}
		st, _ := g.stacks.Get(sid)
		moved := false
		for r := 1; r <= 3 && !moved; r++ {
			for dy := -r; dy <= r && !moved; dy++ {
				for dx := -r; dx <= r && !moved; dx++ {
					nx, ny := x+dx, y+dy
					if !g.grid.Walkable(nx, ny) || g.itemAt(nx, ny) != 0 || covers(cells, nx, ny) {
						continue
					}
					g.deleteItem(sid)
					g.dropStack(st.Kind, st.Qty, nx, ny)
					moved = true
				}
			}
		}
		if !moved {
			return
		}
	}
}

func covers(cells [][2]int, x, y int) bool {
	for _, c := range cells {
		if c[0] == x && c[1] == y {
			return true
		}
	}
	return false
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
		if b := g.bldgs.Ref(id); b != nil && b.Kind == construction.SpikeTrap {
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
	g.traces = append(g.traces, projectile.Trace{FromX: x, FromY: y, ToX: x, ToY: y,
		Tick: uint64(g.clock.Now()), Hit: true, Melee: true, Damage: trapDamage})
	g.removeBuilding(tid)
	g.placeFrameStuff(construction.SpikeTrap, t.Stuff, construction.South, x, y) // re-lay it
	g.logEvent(victim, logEntry{Kind: logTrap, Amount: trapDamage})
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

// trapVictim reports animals that spring traps: wild predators and
// manhunters.
func (g *Game) trapVictim(a animal.State) bool {
	return !a.Tame && (animal.DefOf(a.Species).Predator || a.Enraged(uint64(g.clock.Now())))
}

// assignRepair sends a builder to mend the nearest damaged building, but
// not while raiders are still on their feet.
func (g *Game) assignRepair(id entity.EntityID) bool {
	if standing, _ := g.Raiders(); standing > 0 {
		return false
	}
	pos, _ := g.positions.Get(id)
	var tried []entity.EntityID
	for range workTries { // SIM-027: the next nearest when one can't be started
		var best entity.EntityID
		bestD := 1 << 30
		for _, bid := range g.bldgIDs {
			b := g.bldgs.Ref(bid) // read only: nothing below changes the store
			if b == nil || b.Damage <= 0 || b.Deconstruct || g.swapFrameFor(bid) != 0 {
				continue // a pending material change repairs it anyway (PLAY-011)
			}
			if notColonyPiece(*b) {
				continue // setup_mapgen.go: nobody mends a ruin
			}
			if _, taken := g.reserve.EntityOwner(bid); taken {
				continue
			}
			if slices.Contains(tried, bid) || g.postponedFor(id, job.Repair, bid, 0, 0) {
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
		if g.startJob(id, job.Record{Kind: job.Repair, Target: best, TX: tx, TY: ty, WorkLeft: b.Damage}) {
			return true
		}
		tried = append(tried, best)
	}
	return false
}

func (g *Game) doRepair(id entity.EntityID, j job.Record) {
	b, ok := g.bldgs.Get(j.Target)
	if !ok || b.Damage <= 0 {
		j.State = job.Complete
		g.jobs.Set(id, j)
		return
	}
	if u := g.workProgress(id, skConstruction, true); u > 0 && uint64(g.clock.Now())%3 == 0 { // skills.go
		b.Damage = max(0, b.Damage-repairPerTick*u)
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
