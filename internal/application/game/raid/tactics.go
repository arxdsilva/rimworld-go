package raid

import (
	"cmp"
	"fmt"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/combat/weapon"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
)

// Raider tactics (CBT-4): threat-weighted targets, cover under fire,
// stepping out of a grenade's blast, grabbing loot on the way out, a
// prisoner breaking out, and a siege's resupply pods.
//
// Targets. wiki: Raider, raiders go for whoever threatens them most: a
// colonist with a gun, then a turret, then a colonist with a melee
// weapon, then the unarmed, nearer before farther. Each candidate in
// sight (and the nearest colonist or ally anywhere) scores its distance
// less its danger bonus; the lowest wins (ties: the lower id). A raider
// keeps its target while its attack lasts.
//
// Cover. A raider with a gun under fire (its target shoots back, or it is
// hurt) looks for the firing spot with the most cover within coverRadius
// (while it walks in too, once its gun reaches), moves there only for
// coverMargin more cover than it has and fights from there, and after a
// move looks again only after coverHold (TestRaiderCoverDoesNotFlap); a
// look that found nothing better is tried again after coverLook, and a
// raider in the open far from its target first bounds coverRadius/2
// tiles closer (original tuning). A raid's first looks spread over
// coverSpread ticks.

// Tactics is what raiders and mechanoids need to fight with the ground
// (value only, ≤ 8 methods).
type Tactics interface {
	// CoverSpot is the free tile within r of (x, y) from which id with d
	// can hit (tx, ty), with the most cover against fire from there, its
	// cover and the cover on (x, y) now (ok false: none can hit).
	CoverSpot(id entity.EntityID, x, y, r int, d weapon.Def, tx, ty int) (cx, cy, best, here int, ok bool)
	// Danger is the landing tile and reach of an explosive in flight
	// whose blast would catch (x, y).
	Danger(x, y int) (lx, ly, reach int, ok bool)
	// Colonists appends the standing colonists within r of (x, y).
	Colonists(x, y, r int, buf []Spot) []Spot
	// Block is the first wall or shut door on id's way to (tx, ty) and
	// the tile to hit it from; Bash is one blow on it with d (false: gone
	// or out of touch).
	Block(id entity.EntityID, tx, ty int) (b entity.EntityID, sx, sy int, ok bool)
	Bash(id, b entity.EntityID, d weapon.Def) bool
	// Crop: a sown crop stands unburnt on (x, y).
	Crop(x, y int) bool
	// Drop leaves qty of kind on (x, y) (a resupply pod's cargo).
	Drop(kind item.Kind, qty, x, y int)
	// Restock fills siege mortar m's rack again.
	Restock(m entity.EntityID)
}

// Spot is a creature and its tile.
type Spot struct {
	ID   entity.EntityID
	X, Y int
}

const (
	// Danger bonuses, tiles taken off a target's distance (original
	// tuning: the wiki gives the order, not the weights).
	threatRanged = 8
	threatTurret = 6
	threatMelee  = 4
	// coverRadius, coverMargin (permille) and coverHold (ticks): see
	// above (original tuning; wiki: Cover gives the values).
	coverRadius = 12
	coverMargin = 150
	coverHold   = 600
	coverLook   = 60
	coverSpread = 60
	// retreatLoot: a raider leaving grabs a stack worth taking this close
	// (original tuning; wiki: Raider, raiders may take items as they go).
	retreatLoot = 8
	// siegeResupply: a camped siege gets a pod this often, with
	// raiddef's 12 meals (wiki: Raid) and a refill of its mortars
	// (interval original tuning).
	siegeResupply = clock.TicksPerDay
	resupplyMeals = 12
)

// target is the threat-weighted target of a raider at (x, y) armed with
// d: a colonist in sight or reach, a powered turret in sight, or the
// nearest colonist or ally anywhere (ok false: none).
func (rs *Raiders) target(x, y int, d weapon.Def) (entity.EntityID, int, int, bool) {
	h := rs.h
	id, tx, ty, ok := h.RaidTarget(x, y)
	if rs.t == nil {
		if !ok {
			return h.TurretFor(x, y, 1<<20, 2*SightRange)
		}
		if tid, bx, by, tok := h.TurretFor(x, y, weapon.Distance(x, y, tx, ty), SightRange); tok {
			return tid, bx, by, true
		}
		return id, tx, ty, ok
	}
	best := 1 << 30
	if ok {
		best = weapon.Distance(x, y, tx, ty) - rs.danger(id)
	}
	rs.spots = rs.t.Colonists(x, y, max(SightRange, d.Range), rs.spots[:0])
	for _, s := range rs.spots {
		sc := weapon.Distance(x, y, s.X, s.Y) - rs.danger(s.ID)
		if sc < best || sc == best && s.ID < id {
			id, tx, ty, best, ok = s.ID, s.X, s.Y, sc, true
		}
	}
	sight := SightRange
	if !ok {
		sight = 2 * SightRange // wiki: Raid, a reachable powered turret is a target too
	}
	if tid, bx, by, tok := h.TurretFor(x, y, 1<<20, sight); tok && weapon.Distance(x, y, bx, by)-threatTurret < best {
		return tid, bx, by, true
	}
	return id, tx, ty, ok
}

// danger is colonist id's danger bonus by what it wields.
func (rs *Raiders) danger(id entity.EntityID) int {
	switch d := rs.h.Weapon(id); {
	case d.Ranged:
		return threatRanged
	case d.Damage > weapon.DefOf(weapon.Fists).Damage:
		return threatMelee
	}
	return 0
}

// cover moves raider id at (x, y), with injury hurt, fighting target at
// (tx, ty), into better cover if it is under fire and its look is due.
// It reports whether it set off.
func (rs *Raiders) cover(id entity.EntityID, r *Raider, hurt int, target entity.EntityID, x, y, tx, ty int, now uint64) bool {
	h := rs.h
	d := h.Weapon(id)
	if rs.t == nil || !d.Ranged || now < r.CoverAt {
		return false
	}
	if _, _, creature := h.Pos(target); creature && !h.Weapon(target).Ranged && hurt == 0 {
		return false // not under fire (a turret always shoots back)
	}
	if r.CoverAt == 0 { // a raid's first looks spread over coverSpread ticks
		r.CoverAt = now + uint64(id)%coverSpread
		h.SetRaider(id, *r)
		return false
	}
	r.CoverAt = now + coverLook
	h.SetRaider(id, *r)
	cx, cy, best, here, ok := rs.t.CoverSpot(id, x, y, coverRadius, d, tx, ty)
	if !ok || cx == x && cy == y || best < here+coverMargin {
		return rs.advance(id, r, x, y, tx, ty, here)
	}
	r.CoverAt = now + coverHold
	h.Interrupt(id, job.Cancelled)
	if !h.StartJob(id, job.Record{Kind: job.Move, TX: cx, TY: cy}) {
		return false
	}
	r.CoverX, r.CoverY = cx, cy
	h.SetRaider(id, *r)
	return true
}

// underFire: raider id at (x, y), with injury hurt, has a gun that
// reaches its target at (tx, ty), and is hurt or the target's gun
// reaches it: it stops walking in and fights from cover (CBT-4).
func (rs *Raiders) underFire(id entity.EntityID, hurt int, target entity.EntityID, x, y, tx, ty int) bool {
	h := rs.h
	d := h.Weapon(id)
	if rs.t == nil || !d.Ranged || !weapon.ReachesAt(d, x, y, tx, ty) {
		return false
	}
	if _, _, creature := h.Pos(target); !creature {
		return true // a turret in reach
	}
	return hurt > 0 || weapon.ReachesAt(h.Weapon(target), tx, ty, x, y) && h.Weapon(target).Ranged
}

// advance bounds raider id at (x, y), in the open (cover here) and no
// cover in reach, coverRadius/2 tiles on towards its target at (tx, ty)
// while that is further than coverRadius, to look again from there. It
// reports whether it set off.
func (rs *Raiders) advance(id entity.EntityID, r *Raider, x, y, tx, ty, here int) bool {
	h := rs.h
	dist := weapon.Distance(x, y, tx, ty)
	if here > 0 || dist <= coverRadius {
		return false
	}
	step := coverRadius / 2
	ax, ay, ok := h.WalkableNear(x+(tx-x)*step/dist, y+(ty-y)*step/dist, 1)
	if !ok || ax == x && ay == y || !h.StartJob(id, job.Record{Kind: job.Move, TX: ax, TY: ay}) {
		return false
	}
	r.CoverX, r.CoverY = ax, ay
	h.SetRaider(id, *r)
	return true
}

// covering: raider r is walking j into the cover it chose.
func covering(r Raider, j job.Record, now uint64) bool {
	return j.Kind == job.Move && j.Active() && now < r.CoverAt && j.TX == r.CoverX && j.TY == r.CoverY
}

// dodge steps raider id at (x, y) out of the blast of an explosive about
// to land (wiki: Raider, raiders run from grenades). It reports whether
// it is getting clear.
func (rs *Raiders) dodge(id entity.EntityID, j job.Record, x, y int) bool {
	if rs.t == nil {
		return false
	}
	lx, ly, reach, ok := rs.t.Danger(x, y)
	if !ok {
		return false
	}
	if j.Kind == job.Move && j.Active() {
		if _, _, _, bad := rs.t.Danger(j.TX, j.TY); !bad {
			return true // already on its way out
		}
	}
	dx, dy := cmp.Compare(x, lx), cmp.Compare(y, ly)
	if dx == 0 && dy == 0 {
		dx = 1
	}
	for _, v := range [...][2]int{{dx, dy}, {dx - dy, dx + dy}, {dx + dy, dy - dx}, {-dy, dx}, {dy, -dx}} {
		vx, vy := cmp.Compare(v[0], 0), cmp.Compare(v[1], 0)
		tx, ty, ok := rs.h.WalkableNear(lx+vx*(reach+1), ly+vy*(reach+1), 1)
		if _, _, _, bad := rs.t.Danger(tx, ty); ok && !bad && rs.h.StartJob(id, job.Record{Kind: job.Move, TX: tx, TY: ty}) {
			return true
		}
	}
	return false
}

// grab: a raider leaving picks up a stack worth taking within
// retreatLoot on its way (one load; it looks when it stops or every
// mischiefEvery ticks). It reports whether it acted.
func (rs *Raiders) grab(id entity.EntityID, r Raider, j job.Record, x, y int, now uint64) bool {
	h := rs.h
	if rs.m == nil || r.LootQty > 0 || r.Escaping || now < r.RetryAt || j.Active() && (now+uint64(id))%mischiefEvery != 0 {
		return false
	}
	stack, sx, sy, ok := rs.m.Loot(x, y, retreatLoot)
	if !ok {
		r.RetryAt = now + RaiderRetry*5
		h.SetRaider(id, r)
		return false
	}
	if sx != x || sy != y {
		if j.Kind == job.Move && j.Active() && j.TX == sx && j.TY == sy {
			return true
		}
		if !h.StartJob(id, job.Record{Kind: job.Move, TX: sx, TY: sy}) {
			r.RetryAt = now + RaiderRetry
			h.SetRaider(id, r)
			return false
		}
		return true
	}
	k, n, ok := rs.m.Steal(id, stack)
	if !ok {
		return false
	}
	r.Loot, r.LootQty = k, n
	h.SetRaider(id, r)
	h.Interrupt(id, job.Cancelled)
	text := fmt.Sprintf("%s grabbed %d %s on the way out.", h.Name(id), n, k)
	h.Message(x, y, text)
	rs.log(text)
	return true
}

// escape runs a prisoner breaking out at (x, y): it fights a colonist in
// its way, bashes through a door that holds it, and otherwise runs for
// the edge (the flight below). It reports whether it acted.
func (rs *Raiders) escape(id entity.EntityID, j job.Record, x, y int, now uint64) bool {
	h := rs.h
	switch {
	case j.Kind == job.Attack && j.Active():
		h.Strike(id, j, now)
		return true
	case j.Kind == job.Breach && j.Active():
		h.Breach(id, j)
		return true
	}
	if t, tx, ty, ok := h.NearestColonist(x, y); ok && weapon.Distance(x, y, tx, ty) <= 1 {
		h.ReleaseOwner(id)
		h.SetJob(id, job.Record{Kind: job.Attack, Target: t, TX: tx, TY: ty, State: job.Do, WorkLeft: h.AimTicks(h.Weapon(id))})
		return true
	}
	if rs.unreachable(id) {
		ex, ey := h.NearestEdge(x, y)
		return h.StartBreach(id, ex, ey)
	}
	return false
}

// resupply lands a camped siege's supply pod beside its camp when due:
// meals for the besiegers and a refill of their mortars (wiki: Raid).
func (rs *Raiders) resupply(grp *Group, now uint64) {
	if rs.a == nil || rs.t == nil || !grp.Camped || grp.Stormed {
		return
	}
	if grp.Supply == 0 {
		grp.Supply = now + siegeResupply
		return
	}
	if now < grp.Supply {
		return
	}
	grp.Supply = now + siegeResupply
	x, y, ok := rs.h.WalkableNear(grp.CampX+rs.h.Roll(5)-2, grp.CampY+rs.h.Roll(5)-2, 3)
	if !ok {
		return
	}
	if sx, sy, ok := rs.a.Crash(x, y); ok {
		rs.t.Drop(item.SurvivalMeal, resupplyMeals, sx, sy)
	}
	for _, m := range grp.Mortars {
		rs.t.Restock(m)
	}
	rs.h.Message(x, y, "A drop pod brings the besiegers supplies.")
	rs.log("A drop pod brought the besiegers supplies.")
}
