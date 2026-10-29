package game

import (
	"cmp"
	"slices"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/ai/pathing"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/entity"
)

// Defences, game side. Buildings take damage, break and are mended.
// Raiders and wild animals cannot open the colony's doors (hostileMap);
// raiders shut out plan through breachMap. Cover, traps and breaching are
// the combat package (combat/defense.go).

const (
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
	var mask [4]uint64 // one bit per kind: every kind is cacheable
	for _, k := range kinds {
		mask[k>>6] |= 1 << (uint(k) & 63)
	}
	return g.cellSet(mask).get(g.structVer, func() tileSet { return g.scanBuildingCells(kinds) })
}

// cellSet is the cache for key (a kind mask; bit 255, no kind, marks the
// shut doors).
func (g *Game) cellSet(key [4]uint64) *cellCache {
	if g.cellSets == nil {
		g.cellSets = map[[4]uint64]*cellCache{}
	}
	c := g.cellSets[key]
	if c == nil {
		c = &cellCache{}
		g.cellSets[key] = c
	}
	return c
}

// shutDoors is the doors not held open (DEF-3): shut to raiders and wild
// animals (a shared cached set; holding a door open bumps structVer).
func (g *Game) shutDoors() tileSet {
	return g.cellSet([4]uint64{3: 1 << 63}).get(g.structVer, func() tileSet {
		return g.combat.Doors.Shut(slices.Clone(g.buildingCells(construction.Door, construction.Autodoor)), g.grid.Width())
	})
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
// doors are shut to them, unless held open (DEF-3).
type hostileMap struct {
	g     *Game
	doors tileSet
}

func (g *Game) hostileMap() hostileMap {
	return hostileMap{g: g, doors: g.shutDoors()}
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
	return g.softPathQuery(id, g.grid) // area_soft.go: a restricted pawn prefers its area; fire costs
}

// doorAt reports a built door on (x, y) shut to hostiles: any door (or
// autodoor) not held open (DEF-3, combat/door.go).
func (g *Game) doorAt(x, y int) bool {
	bid := g.layerBldgAt(x, y, construction.LayerObject)
	if bid == 0 {
		return false
	}
	b, _ := g.bldgs.Get(bid)
	return b.Kind.IsDoor() && !g.combat.Doors.Held(bid)
}

// damageBuilding harms a building, destroying it at zero hit points.
func (g *Game) damageBuilding(id entity.EntityID, dmg int) {
	b, ok := g.bldgs.Get(id)
	if !ok || dmg <= 0 {
		return
	}
	if b.Occupant != 0 {
		g.openCasketGroup(id) // casket.go: harming a sealed casket opens its group
		b, _ = g.bldgs.Get(id)
	}
	b.Damage += dmg
	if b.HP() > 0 {
		g.bldgs.Set(id, b)
		return
	}
	g.message(b.X, b.Y, "The %s was smashed down.", b.Kind)
	if construction.DefOf(b.Kind).Foundation {
		g.collapseBridge(b) // bridge_remove.go: what it held up goes with it
	}
	g.removeBuilding(id)
	if b.Kind == construction.SentryTurret {
		// combat/turret.go (PLAY-024): it blows up once it is gone and its
		// blueprint laid, so the blast never finds it standing
		defer g.combat.Turrets.Blast(id, b)
	}
	if b.Ancient {
		return // vault.go: an ancient piece leaves no blueprint to rebuild
	}
	// leave a blueprint to rebuild it, as it was (the salvage is set aside)
	cells, _, _ := construction.Footprint(b.Kind, b.Facing, b.X, b.Y)
	for _, c := range cells {
		if construction.DefOf(b.Kind).Foundation {
			break // bridge_remove.go: what lies in the shallows stays there
		}
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
					// found or marked stays so (haul_claim.go)
					g.dropStackOf(item.Stack{Kind: st.Kind, Qty: st.Qty, Unclaimed: st.Unclaimed, HaulMarked: st.HaulMarked}, nx, ny)
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

func sign(v int) int { return cmp.Compare(v, 0) }

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
			if !g.inHome(b.Y*g.grid.Width() + b.X) {
				continue // PLAY-015: only buildings inside Home are mended
			}
			if !g.areaAllows(id, b.X, b.Y) {
				continue // areas.go: and inside the repairer's allowed area
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
