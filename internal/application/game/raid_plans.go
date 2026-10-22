package game

import (
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/ai/pathing"
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/combat/weapon"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/terrain"
)

// Raid sizes and plans. The storyteller weighs the colony (its wealth,
// its colonists and how long it has stood) into raid points, and spends
// them on raiders: clubs are cheap, rifles dear. Most raids assault head
// on; later ones may lay siege (dig in behind sandbags out of reach and
// shoot at whoever comes close, then storm in) or, against a colony shut
// behind walls and rock, send sappers who tunnel straight through and
// step around traps.

type raidPlan uint8

const (
	planAssault raidPlan = iota
	planSiege
	planSappers
)

func (p raidPlan) String() string {
	switch p {
	case planSiege:
		return "siege"
	case planSappers:
		return "sappers"
	}
	return "assault"
}

const (
	maxScaledRaiders = 8
	clubCost         = 60
	knifeCost        = 50
	spearCost        = 70
	rifleCost        = 90
	// siegeHold is how long a siege camp holds before it storms the colony.
	siegeHold = 5 * clock.TicksPerHour
	// siegeSetup: raiders still not at the camp after this long dig in anyway.
	siegeSetup = 2 * clock.TicksPerHour
	siegeRange = 18 // camp distance from the colony
	rockHP     = 90
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
		if b, ok := g.bldgs.Get(id); ok {
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
	return w
}

// raidPoints is what the storyteller may spend on a raid on this day.
func (g *Game) raidPoints(day int) int {
	colonists := 0
	for _, id := range g.pawnIDs {
		if g.standingPawn(id) {
			colonists++
		}
	}
	return g.Wealth()/40 + colonists*30 + day*2
}

// meleeKit is what a raider without a rifle may carry.
var meleeKit = []item.Kind{item.Club, item.Knife, item.Spear}

func meleeCost(k item.Kind) int {
	switch k {
	case item.Knife:
		return knifeCost
	case item.Spear:
		return spearCost
	}
	return clubCost
}

// raidKit spends points on raiders' weapons (at least one raider).
func (g *Game) raidKit(points int) []item.Kind {
	var kit []item.Kind
	for len(kit) < maxScaledRaiders && (points >= knifeCost || len(kit) == 0) {
		if points >= rifleCost && g.rng.IntN(100) < 55 {
			kit = append(kit, item.Rifle)
			points -= rifleCost
			continue
		}
		k := meleeKit[g.rng.IntN(len(meleeKit))]
		if cost := meleeCost(k); cost > points && len(kit) > 0 {
			k = item.Knife // the cheapest blade fits what is left
		}
		kit = append(kit, k)
		points -= meleeCost(k)
	}
	return kit
}

// scaledRaid is the storyteller's raid: sized by raid points, with a plan
// that suits the colony.
func (g *Game) scaledRaid(day int) {
	kit := g.raidKit(g.raidPoints(day))
	plan := planAssault
	switch roll := g.rng.IntN(100); {
	case day >= 10 && roll < 50 && g.enclosed():
		plan = planSappers
	case day >= 12 && roll < 30:
		plan = planSiege
	}
	g.raidWith(kit, plan)
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
		if _, ok := pathing.Find(hm, ex, ey, p.X, p.Y); ok {
			return false
		}
	}
	return true
}

// Siege sends a siege raid of n raiders (dev hook).
func (g *Game) Siege(n int) { g.raidWith(g.kitOf(n), planSiege) }

// Sappers sends n sappers (dev hook).
func (g *Game) Sappers(n int) { g.raidWith(g.kitOf(n), planSappers) }

func (g *Game) kitOf(n int) []item.Kind {
	kit := make([]item.Kind, n)
	for i := range kit {
		kit[i] = item.Club
		switch g.rng.IntN(4) {
		case 0, 1:
			kit[i] = item.Rifle
		case 2:
			kit[i] = item.Knife
		case 3:
			kit[i] = item.Spear
		}
	}
	return kit
}

// campSpot is where a siege digs in: siegeRange tiles out from the
// colony towards the raiders' edge.
func (g *Game) campSpot(ex, ey int) (int, int) {
	hx, hy := g.home()
	dx, dy := ex-hx, ey-hy
	d := max(1, weapon.Distance(ex, ey, hx, hy))
	if d <= siegeRange {
		return ex, ey
	}
	x, y := hx+dx*siegeRange/d, hy+dy*siegeRange/d
	if cx, cy, ok := g.walkableNear(x, y, 4); ok {
		return cx, cy
	}
	return ex, ey
}

// thinkSiege runs a siege raider before the storm: walk to the camp, dig
// in, then shoot whoever comes in range. It reports whether it acted.
func (g *Game) thinkSiege(id entity.EntityID, r raiderState, now uint64) bool {
	gi := r.Group
	grp := &g.raidGroups[gi]
	if grp.Plan != planSiege || r.Mode == raidFlee || grp.Stormed {
		return false
	}
	if grp.Started == 0 {
		grp.Started = now
	}
	pos, _ := g.positions.Get(id)
	if !grp.Camped {
		if weapon.Distance(pos.X, pos.Y, grp.CampX, grp.CampY) > 3 && now-grp.Started < siegeSetup {
			if j, _ := g.jobs.Get(id); !j.Active() || (now+uint64(id))%180 == 0 {
				if x, y, ok := g.walkableNear(grp.CampX+g.rng.IntN(5)-2, grp.CampY+g.rng.IntN(5)-2, 2); ok {
					g.startJob(id, job.Record{Kind: job.Move, TX: x, TY: y})
				}
			}
			return true
		}
		g.digIn(gi, now)
	}
	if now >= grp.Until {
		grp.Stormed = true
		g.threat(pos.X, pos.Y, "The besiegers are storming the colony!")
		return false
	}
	// hold the camp; shoot anyone who comes in range
	if j, _ := g.jobs.Get(id); j.Kind == job.Attack && j.Active() {
		g.doCombat(id, j, now)
		if j2, _ := g.jobs.Get(id); j2.State == job.Goto {
			g.interrupt(id, job.Cancelled) // never leave the camp to chase
		}
		return true
	}
	if t, tp, ok := g.nearestColonist(pos); ok && g.canHit(pos, weaponDefOf(g, id), tp) {
		g.reserve.ReleaseOwner(id)
		g.jobs.Set(id, job.Record{Kind: job.Attack, Target: t, TX: tp.X, TY: tp.Y, State: job.Do,
			WorkLeft: aimTicks(weaponDefOf(g, id))})
	}
	return true
}

// digIn throws up sandbags in front of each besieger of group gi.
func (g *Game) digIn(gi int, now uint64) {
	grp := &g.raidGroups[gi]
	grp.Camped, grp.Until = true, now+siegeHold
	hx, hy := g.home()
	for _, rid := range g.raiderIDs {
		if g.raiders[rid].Group != gi || !g.standingRaider(rid) {
			continue
		}
		p, _ := g.positions.Get(rid)
		sx, sy := p.X+sign(hx-p.X), p.Y+sign(hy-p.Y)
		g.BuildNow("sandbags", "wood", "south", sx, sy)
		g.interrupt(rid, job.Cancelled)
	}
	g.threat(grp.CampX, grp.CampY, "The raiders have dug in for a siege. They will storm the colony in a few hours.")
}

// sapperMap is the map as sappers see it when planning a tunnel: walls,
// doors and rock all give way; traps are stepped around.
func (g *Game) sapperBreaks() tileSet {
	set := g.buildingCells(construction.Wall, construction.Door)
	w := g.grid.Width()
	for y := 0; y < g.grid.Height(); y++ {
		for x := 0; x < w; x++ {
			if terrain.IsRockWall(g.terr.At(x, y)) {
				set[y*w+x] = true
			}
		}
	}
	return set
}

// sapper reports a raider of a sapper raid.
func (g *Game) sapper(id entity.EntityID) bool {
	r, ok := g.raiders[id]
	return ok && r.Group < len(g.raidGroups) && g.raidGroups[r.Group].Plan == planSappers
}

// sapperMap avoids traps on top of the doors rule.
type sapperMap struct {
	hostileMap
	traps tileSet
}

func (s sapperMap) Walkable(x, y int) bool {
	return s.hostileMap.Walkable(x, y) && !s.traps[y*s.g.grid.Width()+x]
}

// digRock: a sapper hacks at a rock tile; it opens up at zero.
func (g *Game) digRock(x, y, dmg int) {
	if !terrain.IsRockWall(g.terr.At(x, y)) {
		return
	}
	k := [2]int{x, y}
	g.rockDamage[k] += dmg
	if g.rockDamage[k] < rockHP {
		return
	}
	delete(g.rockDamage, k)
	g.setTerrain(x, y, terrain.Stone) // clears any ore (SIM-029)
	g.roofOnMined(x, y)
}
