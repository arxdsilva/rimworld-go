package rage

import (
	"rimworld-go/internal/domain/ai/pathing"
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/combat/weapon"
	"rimworld-go/internal/domain/sim/entity"
)

// Breaking through (EAI-001): a wild animal shut out from its foe (a
// manhunter, a hive insect, a hunting predator) batters the shut door in
// its way; a hive insect batters walls too. Fences count as solid: no one
// climbs or breaks them. The way is planned as the mechanoids' is
// (combat.Tactics.Block): as if the doors (and walls) were down, and the
// first one on it is the block.
//
// Hysteresis: a bash starts only on a failed path search; the animal then
// holds that one block (Memory.Bash) until it is down or held open, or
// for BashGiveUp ticks, after which it drops it and waits for the path
// retry before a new search may start another (TestAnimalBashNoFlapOnHeldDoor).
//
// Cost: only a foe worth breaking in for starts a search (a manhunter's
// or an insect's any foe, a predator's only a person or a tame animal: a
// wild predator after wild prey just paths), the search keeps to a box
// around the animal and its foe (breachMargin) within a budget of nodes
// (breachNodes a tile, at least breachFloor: a near foe's whole box), and
// runs only when a region check says the foe could be reached with doors
// open (the tame layer; an insect, which breaks walls too, skips it). A
// foe with no way through is marked (a Bash at X -1, Y minus the times
// running, for the foe in Memory.Lock) for breachWait: no path or breach
// search for it meanwhile, and the animal's next look takes a reachable
// foe instead (target.go).

// BashGiveUp is how long an animal batters one block before giving up.
const BashGiveUp = 2500

// bashRetry: an animal that gave up a block searches for a path again
// this many ticks later (the game's own path retry).
const bashRetry = 90

// BreachRetry: an animal whose breach search found no block searches
// again only this many ticks later (the first time: breachWait).
const BreachRetry = 600

// breachMargin: the breach search keeps to the box around the animal and
// its foe grown by this many tiles.
const breachMargin = 12

// breachNodes and breachFloor bound the breach search: it expands at
// most breachNodes nodes per tile between the animal and its foe (plus
// breachMargin), and never fewer than breachFloor (a box 64 tiles a side:
// a foe within about 40 tiles is searched in full). A way through is
// found well within it; a search with none (a hive insect's foe behind
// fences or rock) gives up long before it floods a far foe's box.
const (
	breachNodes = 16
	breachFloor = 4096
)

// breachWait is how long a foe marked unreachable (Bash X -1) stays
// marked: BreachRetry, doubled for each time running it was found so
// (Y -1, -2, ...), at most 8 times.
func breachWait(bs Bash) uint64 {
	return BreachRetry << (min(max(-bs.Y, 1), 4) - 1)
}

// breaker: a wild animal that breaks through to its foe: one enraged
// (hive insects are, for good), or a predator after a person or a tame
// animal.
func (r *Rage) breaker(a animal.State, def animal.Def, now uint64) bool {
	switch {
	case a.Tame:
		return false
	case a.Enraged(now):
		return true
	case !def.Predator || a.Target == 0:
		return false
	}
	if o := r.h.Ref(a.Target); o != nil {
		return o.Tame
	}
	return r.h.IsPawn(a.Target)
}

// shutOut starts a bash for animal id (state a), whose path to its foe at
// tp just failed: the first block on its way, unless it holds one.
func (r *Rage) shutOut(id entity.EntityID, a *animal.State, tp Pos, now uint64) {
	def := animal.DefOf(a.Species)
	if !r.breaker(*a, def, now) {
		return
	}
	bs, held := r.m.Bash[id]
	if held && (bs.X >= 0 || r.m.unreachable(id, a.Target, now)) {
		return // a block held, or no way to this foe found lately
	}
	pos := r.pos(id)
	c, ok := Pos{}, def.Hive || r.ReachesFoe(true, pos, tp) // a region check first
	if ok {
		c, ok = block(r, pos, tp, a.Tame, def.Hive)
	}
	if !ok {
		c = Pos{X: -1, Y: -1} // none: remembered (breachWait) for this foe
		if l := r.m.Lock[id]; l.Foe != a.Target {
			r.m.lock(id, Lock{Foe: a.Target, Until: now}) // a predator's: the mark's foe
		} else if held && bs.X < 0 {
			c.Y = max(bs.Y, -4) - 1 // again: remembered longer
		}
	}
	r.m.hold(id, Bash{X: c.X, Y: c.Y, Since: now})
}

// bashing runs animal id's bash, if it holds one: it walks beside the
// block, then strikes it on its attack's cooldown. It reports false once
// the block is down or open (the animal paths on at once) or given up
// (it waits for the path retry).
func (r *Rage) bashing(id entity.EntityID, a *animal.State, pos Pos, now uint64) bool {
	bs, ok := r.m.Bash[id]
	if !ok {
		return false
	}
	if bs.X < 0 {
		return false // no way through found: kept (Chase, shutOut)
	}
	def := animal.DefOf(a.Species)
	if now-bs.Since > BashGiveUp || !r.breaker(*a, def, now) {
		delete(r.m.Bash, id)
		a.Path, a.PathI, a.RetryAt = nil, 0, now+bashRetry
		return false
	}
	b, ok := r.h.BlockAt(bs.X, bs.Y, def.Hive)
	if !ok {
		delete(r.m.Bash, id) // down, or held open: on through it
		a.Path, a.PathI, a.RetryAt = nil, 0, 0
		return false
	}
	if weapon.Distance(pos.X, pos.Y, bs.X, bs.Y) <= 1 {
		a.Path, a.PathI = nil, 0
		if a.Cooldown == 0 && r.h.Bash(id, b, def.Attack, def.Damage) {
			a.Cooldown = def.Cooldown
		}
		return true
	}
	if a.PathI < len(a.Path) && weapon.Distance(a.GoalX, a.GoalY, bs.X, bs.Y) <= 1 {
		return true // on its way to the block
	}
	if now < a.RetryAt {
		return true // waiting out a failed search
	}
	s, ok := standBy(r, a.Tame, pos, Pos{X: bs.X, Y: bs.Y})
	if !ok {
		delete(r.m.Bash, id) // no way to it any more
		return false
	}
	*a = r.h.GoTo(id, *a, s.X, s.Y)
	return true
}

// breachQuery is an animal's walk with the blocks it may break (shut
// doors; walls too) counted open, inside the box x0..x1, y0..y1 and while
// the rage's budget lasts; fences stay shut.
type breachQuery struct {
	pathing.Query
	r              *Rage
	walls          bool
	x0, y0, x1, y1 int
}

func (q breachQuery) Walkable(x, y int) bool {
	if x < q.x0 || x > q.x1 || y < q.y0 || y > q.y1 || q.r.budget <= 0 {
		return false
	}
	q.r.budget--
	if q.Query.Walkable(x, y) {
		return true
	}
	_, ok := q.r.h.BlockAt(x, y, q.walls)
	return ok
}

// block is the first block on the way from pos to tp planned through the
// blocks (breachQuery), as the mechanoids plan (combat.Tactics.Block).
func block(r *Rage, pos, tp Pos, tame, walls bool) (Pos, bool) {
	q := r.h.PathQuery(tame)
	bq := breachQuery{Query: q, r: r, walls: walls,
		x0: min(pos.X, tp.X) - breachMargin, y0: min(pos.Y, tp.Y) - breachMargin,
		x1: max(pos.X, tp.X) + breachMargin, y1: max(pos.Y, tp.Y) + breachMargin}
	r.budget = 8 * max(breachFloor, breachNodes*(weapon.Distance(pos.X, pos.Y, tp.X, tp.Y)+breachMargin)) // 8 checks a node
	path, ok := r.srch.Find(bq, pos.X, pos.Y, tp.X, tp.Y)
	if !ok {
		return Pos{}, false
	}
	for _, p := range path {
		if !q.Walkable(p.X, p.Y) {
			if _, b := r.h.BlockAt(p.X, p.Y, walls); b {
				return Pos{X: p.X, Y: p.Y}, true
			}
		}
	}
	return Pos{}, false
}

// standBy is the walkable cell beside block b nearest pos that a walk
// from pos reaches.
func standBy(r *Rage, tame bool, pos, b Pos) (Pos, bool) {
	q := r.h.PathQuery(tame)
	best, found := Pos{}, false
	for dy := -1; dy <= 1; dy++ {
		for dx := -1; dx <= 1; dx++ {
			n := Pos{X: b.X + dx, Y: b.Y + dy}
			if dx == 0 && dy == 0 || n.X < 0 || n.Y < 0 || n.X >= q.Width() || n.Y >= q.Height() || !q.Walkable(n.X, n.Y) {
				continue
			}
			if found && DistSq(n, pos) >= DistSq(best, pos) {
				continue
			}
			if r.h.LayerReach(q, pos, n.X, n.Y) {
				best, found = n, true
			}
		}
	}
	return best, found
}

// Bashing: animal id is battering a door or wall to reach a foe (the
// inspector's "Breaking through the door").
func (r *Rage) Bashing(id entity.EntityID, a animal.State, now uint64) bool {
	bs, ok := r.m.Bash[id]
	return ok && bs.X >= 0 && a.Mode == animal.Attack && now-bs.Since <= BashGiveUp
}
